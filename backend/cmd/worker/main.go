package main

import (
	"context"
	"errors"
	"log"
	"time"

	"backtest-sim/backend/internal/queue"
	"github.com/redis/go-redis/v9"
)

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

		log.Printf(
			"dequeued job id=%s ticker=%s short_window=%d long_window=%d initial_cash=%.2f",
			job.ID,
			job.Ticker,
			job.ShortWindow,
			job.LongWindow,
			job.InitialCash,
		)
	}
}
