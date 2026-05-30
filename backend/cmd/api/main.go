package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"backtest-sim/backend/internal/api"
	"backtest-sim/backend/internal/queue"
	"github.com/redis/go-redis/v9"
)

func main() {
	ctx := context.Background()

	// Create Redis client used by the API to enqueue and inspect runs
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

	// Build the API router and attach all registered routes
	router := api.NewRouter(simulationQueue, resultStore)

	log.Println("api listening on :8080")
	if err := http.ListenAndServe(":8080", router); err != nil {
		log.Fatal(err)
	}
}
