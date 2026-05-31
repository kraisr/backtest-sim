package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"backtest-sim/backend/internal/backtest"
	"backtest-sim/backend/internal/data"
	"backtest-sim/backend/internal/queue"
	"github.com/redis/go-redis/v9"
)

const (
	dataDir  = "../data"
	queueKey = "queue:simulations"
)

func main() {
	ctx := context.Background()
	workerCount := flag.Int("workers", 1, "number of concurrent workers in this process")
	redisAddr := flag.String("redis-addr", "localhost:6379", "redis address")
	flag.Parse()

	if *workerCount <= 0 {
		log.Fatal("workers must be positive")
	}

	// Create Redis client
	client := redis.NewClient(&redis.Options{
		Addr:     *redisAddr,
		Password: "",
		DB:       0,
	})
	defer client.Close()

	// Ping Redis
	if err := client.Ping(ctx).Err(); err != nil {
		log.Fatalf("connect to redis: %v", err)
	}

	simulationQueue := queue.NewRedisQueue(client, queueKey)
	resultStore := queue.NewResultStore(client, 24*time.Hour)
	metricsStore := queue.NewMetricsStore(client, queueKey)
	candleCache := newCandleCache()
	log.Printf("worker listening on %s with %d workers", queueKey, *workerCount)

	var wg sync.WaitGroup
	for workerID := 1; workerID <= *workerCount; workerID++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			runWorker(ctx, workerID, simulationQueue, resultStore, metricsStore, candleCache)
		}(workerID)
	}

	wg.Wait()
}

func runWorker(ctx context.Context, workerID int, simulationQueue *queue.RedisQueue, resultStore *queue.ResultStore, metricsStore *queue.MetricsStore, candles *candleCache) {
	for {
		// Wait for the next job, but wake up periodically if the queue is empty
		job, err := simulationQueue.Dequeue(ctx, 5*time.Second)
		if err != nil {
			if errors.Is(err, redis.Nil) {
				log.Printf("worker=%d no jobs available", workerID)
				continue
			}

			log.Printf("worker=%d dequeue job: %v", workerID, err)
			continue
		}

		if err := processJob(ctx, workerID, job, resultStore, metricsStore, candles); err != nil {
			log.Printf("worker=%d process job id=%s: %v", workerID, job.ID, err)
		}
	}
}

func processJob(ctx context.Context, workerID int, job queue.Job, store *queue.ResultStore, metrics *queue.MetricsStore, candles *candleCache) error {
	startedAt := time.Now()
	log.Printf(
		"worker=%d started job id=%s sweep_id=%s ticker=%s short_window=%d long_window=%d initial_cash=%.2f",
		workerID,
		job.ID,
		job.SweepID,
		job.Ticker,
		job.ShortWindow,
		job.LongWindow,
		job.InitialCash,
	)

	// Mark the job as running before doing the expensive backtest work
	if err := store.SetStatus(ctx, job.ID, queue.StatusRunning); err != nil {
		return fmt.Errorf("set running status: %w", err)
	}

	if err := metrics.RecordJobStarted(ctx, job, startedAt); err != nil {
		log.Printf("worker=%d record started metric job id=%s: %v", workerID, job.ID, err)
	}

	// Run the local backtest engine using the parameters from the queued job
	result, err := runBacktestJob(job, candles)
	if err != nil {
		if storeErr := markJobFailed(ctx, store, job.ID, err); storeErr != nil {
			return storeErr
		}

		if metricsErr := metrics.RecordJobFailed(ctx, time.Since(startedAt)); metricsErr != nil {
			log.Printf("worker=%d record failed metric job id=%s: %v", workerID, job.ID, metricsErr)
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

	if err := metrics.RecordJobCompleted(ctx, time.Since(startedAt)); err != nil {
		log.Printf("worker=%d record completed metric job id=%s: %v", workerID, job.ID, err)
	}

	log.Printf("worker=%d completed job id=%s final_value=%.2f return=%.2f%%", workerID, job.ID, result.Strategy.FinalValue, result.TotalReturn*100)
	return nil
}

func runBacktestJob(job queue.Job, candles *candleCache) (backtest.BacktestResult, error) {
	csvPath, err := csvPathForTicker(job.Ticker)
	if err != nil {
		return backtest.BacktestResult{}, err
	}

	marketData, err := candles.Load(csvPath)
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

	result, err := backtest.RunMovingAverageBacktest(marketData, config)
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

type candleCache struct {
	mu            sync.RWMutex
	candlesByPath map[string][]data.Candle
}

func newCandleCache() *candleCache {
	return &candleCache{
		candlesByPath: make(map[string][]data.Candle),
	}
}

func (cache *candleCache) Load(path string) ([]data.Candle, error) {
	// Fast path: reuse parsed candles when another job already loaded this CSV
	cache.mu.RLock()
	candles, ok := cache.candlesByPath[path]
	cache.mu.RUnlock()
	if ok {
		return candles, nil
	}

	cache.mu.Lock()
	defer cache.mu.Unlock()

	// Check again in case another worker loaded the CSV while this worker waited
	candles, ok = cache.candlesByPath[path]
	if ok {
		return candles, nil
	}

	// Slow path: parse and store the CSV once for this worker process
	candles, err := data.LoadCSV(path)
	if err != nil {
		return nil, err
	}

	cache.candlesByPath[path] = candles
	return candles, nil
}
