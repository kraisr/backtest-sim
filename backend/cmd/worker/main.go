package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"time"

	"backtest-sim/backend/internal/backtest"
	"backtest-sim/backend/internal/data"
	"backtest-sim/backend/internal/queue"
	"github.com/redis/go-redis/v9"
)

const dataDir = "../data"

func main() {
	ctx := context.Background()

	// Create Redis client
	client := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
	})
	defer client.Close()

	// Ping Redis
	if err := client.Ping(ctx).Err(); err != nil {
		log.Fatalf("connect to redis: %v", err)
	}

	simulationQueue := queue.NewRedisQueue(client, "queue:simulations")
	resultStore := queue.NewResultStore(client, 24*time.Hour)
	log.Println("worker listening on queue:simulations")

	for {
		// Wait for the next job, but wake up periodically if the queue is empty
		job, err := simulationQueue.Dequeue(ctx, 5*time.Second)
		if err != nil {
			if errors.Is(err, redis.Nil) {
				log.Println("no jobs available")
				continue
			}

			log.Printf("dequeue job: %v", err)
			continue
		}

		if err := processJob(ctx, job, resultStore); err != nil {
			log.Printf("process job id=%s: %v", job.ID, err)
		}
	}
}

func processJob(ctx context.Context, job queue.Job, store *queue.ResultStore) error {
	log.Printf(
		"started job id=%s ticker=%s short_window=%d long_window=%d initial_cash=%.2f",
		job.ID,
		job.Ticker,
		job.ShortWindow,
		job.LongWindow,
		job.InitialCash,
	)

	// Mark the job as running before doing the expensive backtest work
	if err := store.SetStatus(ctx, job.ID, queue.StatusRunning); err != nil {
		return fmt.Errorf("set running status: %w", err)
	}

	// Run the local backtest engine using the parameters from the queued job
	result, err := runBacktestJob(job)
	if err != nil {
		if storeErr := markJobFailed(ctx, store, job.ID, err); storeErr != nil {
			return storeErr
		}

		return err
	}

	// Persist the result first so a completed job always has result data available
	if err := store.SetResult(ctx, job.ID, result); err != nil {
		return fmt.Errorf("set job result: %w", err)
	}

	if err := store.SetStatus(ctx, job.ID, queue.StatusCompleted); err != nil {
		return fmt.Errorf("set completed status: %w", err)
	}

	log.Printf("completed job id=%s final_value=%.2f return=%.2f%%", job.ID, result.Strategy.FinalValue, result.TotalReturn*100)
	return nil
}

func runBacktestJob(job queue.Job) (backtest.BacktestResult, error) {
	csvPath, err := csvPathForTicker(job.Ticker)
	if err != nil {
		return backtest.BacktestResult{}, err
	}

	candles, err := data.LoadCSV(csvPath)
	if err != nil {
		return backtest.BacktestResult{}, fmt.Errorf("load market data: %w", err)
	}

	config := backtest.BacktestConfig{
		InitialCash: job.InitialCash,
		FeeBps:      job.FeeBps,
		SlippageBps: job.SlippageBps,
		ShortWindow: job.ShortWindow,
		LongWindow:  job.LongWindow,
	}

	result, err := backtest.RunMovingAverageBacktest(candles, config)
	if err != nil {
		return backtest.BacktestResult{}, fmt.Errorf("run moving average backtest: %w", err)
	}

	return result, nil
}

func markJobFailed(ctx context.Context, store *queue.ResultStore, jobID string, jobErr error) error {
	if err := store.SetError(ctx, jobID, jobErr.Error()); err != nil {
		return fmt.Errorf("set job error: %w", err)
	}

	if err := store.SetStatus(ctx, jobID, queue.StatusFailed); err != nil {
		return fmt.Errorf("set failed status: %w", err)
	}

	return nil
}

func csvPathForTicker(ticker string) (string, error) {
	normalizedTicker := strings.ToUpper(ticker)
	if normalizedTicker != "SPY" {
		return "", fmt.Errorf("unsupported ticker %q", ticker)
	}

	return filepath.Join(dataDir, normalizedTicker+".csv"), nil
}
