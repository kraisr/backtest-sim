package queue

import (
	"context"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func TestMetricsStoreRecordsSnapshot(t *testing.T) {
	store, client, queueKey := newTestMetricsStore(t)
	ctx := context.Background()
	enqueuedAt := time.Now().Add(-2 * time.Second)

	if err := client.LPush(ctx, queueKey, "job-1", "job-2").Err(); err != nil {
		t.Fatalf("seed queue depth: %v", err)
	}

	if err := store.RecordJobQueued(ctx); err != nil {
		t.Fatalf("record queued: %v", err)
	}

	if err := store.RecordJobStarted(ctx, Job{ID: "job-1", EnqueuedAt: enqueuedAt}, time.Now()); err != nil {
		t.Fatalf("record started: %v", err)
	}

	if err := store.RecordJobCompleted(ctx, 150*time.Millisecond); err != nil {
		t.Fatalf("record completed: %v", err)
	}

	if err := store.RecordJobFailed(ctx, 50*time.Millisecond); err != nil {
		t.Fatalf("record failed: %v", err)
	}

	snapshot, err := store.Snapshot(ctx)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}

	if snapshot.QueueDepth != 2 {
		t.Fatalf("expected queue depth 2, got %d", snapshot.QueueDepth)
	}

	if snapshot.JobsQueued != 1 {
		t.Fatalf("expected queued jobs 1, got %d", snapshot.JobsQueued)
	}

	if snapshot.JobsStarted != 1 {
		t.Fatalf("expected started jobs 1, got %d", snapshot.JobsStarted)
	}

	if snapshot.JobsCompleted != 1 {
		t.Fatalf("expected completed jobs 1, got %d", snapshot.JobsCompleted)
	}

	if snapshot.JobsFailed != 1 {
		t.Fatalf("expected failed jobs 1, got %d", snapshot.JobsFailed)
	}

	if snapshot.JobDurationCount != 2 {
		t.Fatalf("expected duration count 2, got %d", snapshot.JobDurationCount)
	}

	if snapshot.JobDurationSecondsSum <= 0 {
		t.Fatalf("expected positive duration sum, got %f", snapshot.JobDurationSecondsSum)
	}

	if snapshot.QueueLatencyCount != 1 {
		t.Fatalf("expected latency count 1, got %d", snapshot.QueueLatencyCount)
	}

	if snapshot.QueueLatencySecondsSum <= 0 {
		t.Fatalf("expected positive latency sum, got %f", snapshot.QueueLatencySecondsSum)
	}
}

func newTestMetricsStore(t *testing.T) (*MetricsStore, *redis.Client, string) {
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

	queueKey := testQueueKey(t)
	metricKeys := []string{
		queueKey,
		metricJobsQueuedKey,
		metricJobsStartedKey,
		metricJobsCompletedKey,
		metricJobsFailedKey,
		metricJobDurationCountKey,
		metricJobDurationSumKey,
		metricQueueLatencyCountKey,
		metricQueueLatencySumKey,
	}

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = client.Del(ctx, metricKeys...).Err()
		_ = client.Close()
	})

	_ = client.Del(ctx, metricKeys...).Err()
	return NewMetricsStore(client, queueKey), client, queueKey
}
