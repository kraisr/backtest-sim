package main

import (
	"context"
	"log"
	"time"

	"backtest-sim/backend/internal/queue"
	"github.com/redis/go-redis/v9"
)

func main() {
	ctx := context.Background()

	// Create Redis client for the local development Redis instance
	client := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
	})
	defer client.Close()

	if err := client.Ping(ctx).Err(); err != nil {
		log.Fatalf("connect to redis: %v", err)
	}

	simulationQueue := queue.NewRedisQueue(client, "queue:simulations")
	resultStore := queue.NewResultStore(client, 24*time.Hour)
	job := queue.Job{
		ID:          "dev-job-1",
		Ticker:      "SPY",
		InitialCash: 10000,
		FeeBps:      0,
		SlippageBps: 0,
		ShortWindow: 20,
		LongWindow:  50,
	}

	// Store the job request and mark it as queued before making it available for workers
	if err := resultStore.SetJob(ctx, job); err != nil {
		log.Fatalf("set job request: %v", err)
	}

	if err := resultStore.SetStatus(ctx, job.ID, queue.StatusQueued); err != nil {
		log.Fatalf("set queued status: %v", err)
	}

	// Add one development job so a running worker can dequeue it
	if err := simulationQueue.Enqueue(ctx, job); err != nil {
		log.Fatalf("enqueue job: %v", err)
	}

	log.Printf("enqueued job id=%s ticker=%s", job.ID, job.Ticker)
}
