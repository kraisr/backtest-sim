package queue

import (
	"context"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func TestRedisQueueEnqueueDequeue(t *testing.T) {
	queue, _, _ := newTestRedisQueue(t)
	ctx := context.Background()
	job := Job{
		ID:          "job-1",
		Ticker:      "SPY",
		InitialCash: 10000,
		FeeBps:      10,
		SlippageBps: 5,
		ShortWindow: 20,
		LongWindow:  50,
	}

	if err := queue.Enqueue(ctx, job); err != nil {
		t.Fatalf("enqueue job: %v", err)
	}

	got, err := queue.Dequeue(ctx, time.Second)
	if err != nil {
		t.Fatalf("dequeue job: %v", err)
	}

	if !reflect.DeepEqual(got, job) {
		t.Fatalf("expected dequeued job %+v, got %+v", job, got)
	}
}

func TestRedisQueuePreservesFIFOOrder(t *testing.T) {
	queue, _, _ := newTestRedisQueue(t)
	ctx := context.Background()
	first := Job{ID: "job-1", Ticker: "SPY", InitialCash: 10000, ShortWindow: 20, LongWindow: 50}
	second := Job{ID: "job-2", Ticker: "SPY", InitialCash: 10000, ShortWindow: 30, LongWindow: 100}

	if err := queue.Enqueue(ctx, first); err != nil {
		t.Fatalf("enqueue first job: %v", err)
	}

	if err := queue.Enqueue(ctx, second); err != nil {
		t.Fatalf("enqueue second job: %v", err)
	}

	gotFirst, err := queue.Dequeue(ctx, time.Second)
	if err != nil {
		t.Fatalf("dequeue first job: %v", err)
	}

	gotSecond, err := queue.Dequeue(ctx, time.Second)
	if err != nil {
		t.Fatalf("dequeue second job: %v", err)
	}

	if gotFirst.ID != first.ID {
		t.Fatalf("expected first dequeued job ID %q, got %q", first.ID, gotFirst.ID)
	}

	if gotSecond.ID != second.ID {
		t.Fatalf("expected second dequeued job ID %q, got %q", second.ID, gotSecond.ID)
	}
}

func TestRedisQueueDequeueReturnsErrorWhenEmpty(t *testing.T) {
	queue, _, _ := newTestRedisQueue(t)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := queue.Dequeue(ctx, time.Second)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !strings.Contains(err.Error(), "dequeue job") {
		t.Fatalf("expected dequeue job error, got %v", err)
	}
}

func TestRedisQueueReturnsErrorForInvalidPayload(t *testing.T) {
	queue, client, key := newTestRedisQueue(t)
	ctx := context.Background()

	if err := client.LPush(ctx, key, "not-json").Err(); err != nil {
		t.Fatalf("push invalid payload: %v", err)
	}

	_, err := queue.Dequeue(ctx, time.Second)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !strings.Contains(err.Error(), "unmarshal job") {
		t.Fatalf("expected unmarshal job error, got %v", err)
	}
}

func newTestRedisQueue(t *testing.T) (*RedisQueue, *redis.Client, string) {
	t.Helper()

	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   15,
	})

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		t.Skipf("redis is not available at localhost:6379: %v", err)
	}

	key := testQueueKey(t)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = client.Del(ctx, key).Err()
		_ = client.Close()
	})

	return NewRedisQueue(client, key), client, key
}

func testQueueKey(t *testing.T) string {
	t.Helper()

	name := strings.NewReplacer("/", ":", " ", "_").Replace(t.Name())
	return "test:queue:" + name + ":" + strconv.FormatInt(time.Now().UnixNano(), 10)
}
