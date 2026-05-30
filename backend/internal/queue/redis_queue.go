package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisQueue struct {
	client *redis.Client
	key    string
}

func NewRedisQueue(client *redis.Client, key string) *RedisQueue {
	return &RedisQueue{
		client: client,
		key:    key,
	}
}

func (q *RedisQueue) Enqueue(ctx context.Context, job Job) error {
	// Convert the Go job struct into JSON before storing it in Redis
	payload, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("marshal job: %w", err)
	}

	// LPUSH adds the job to the left side of the Redis list
	if err := q.client.LPush(ctx, q.key, payload).Err(); err != nil {
		return fmt.Errorf("enqueue job: %w", err)
	}

	return nil
}

func (q *RedisQueue) Dequeue(ctx context.Context, timeout time.Duration) (Job, error) {
	// BRPOP waits until a job is available or the timeout expires
	values, err := q.client.BRPop(ctx, timeout, q.key).Result()
	if err != nil {
		return Job{}, fmt.Errorf("dequeue job: %w", err)
	}

	if len(values) != 2 {
		return Job{}, fmt.Errorf("unexpected redis response length %d", len(values))
	}

	// Redis returns [queue key, payload], so the JSON job is the second value
	var job Job
	if err := json.Unmarshal([]byte(values[1]), &job); err != nil {
		return Job{}, fmt.Errorf("unmarshal job: %w", err)
	}

	return job, nil
}
