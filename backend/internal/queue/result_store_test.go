package queue

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

type testStoredResult struct {
	FinalValue float64 `json:"final_value"`
	Trades     int     `json:"trades"`
}

func TestResultStoreSetGetStatus(t *testing.T) {
	store, _, jobID := newTestResultStore(t)
	ctx := context.Background()

	if err := store.SetStatus(ctx, jobID, StatusRunning); err != nil {
		t.Fatalf("set status: %v", err)
	}

	got, err := store.GetStatus(ctx, jobID)
	if err != nil {
		t.Fatalf("get status: %v", err)
	}

	if got != StatusRunning {
		t.Fatalf("expected status %q, got %q", StatusRunning, got)
	}
}

func TestResultStoreSetGetJob(t *testing.T) {
	store, _, jobID := newTestResultStore(t)
	ctx := context.Background()
	want := Job{
		ID:          jobID,
		Ticker:      "SPY",
		InitialCash: 10000,
		FeeBps:      5,
		SlippageBps: 2,
		ShortWindow: 20,
		LongWindow:  50,
	}

	if err := store.SetJob(ctx, want); err != nil {
		t.Fatalf("set job: %v", err)
	}

	got, err := store.GetJob(ctx, jobID)
	if err != nil {
		t.Fatalf("get job: %v", err)
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected job %+v, got %+v", want, got)
	}
}

func TestResultStoreSetGetResult(t *testing.T) {
	store, _, jobID := newTestResultStore(t)
	ctx := context.Background()
	want := testStoredResult{
		FinalValue: 12345.67,
		Trades:     4,
	}

	if err := store.SetResult(ctx, jobID, want); err != nil {
		t.Fatalf("set result: %v", err)
	}

	var got testStoredResult
	if err := store.GetResult(ctx, jobID, &got); err != nil {
		t.Fatalf("get result: %v", err)
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected result %+v, got %+v", want, got)
	}
}

func TestResultStoreSetGetError(t *testing.T) {
	store, _, jobID := newTestResultStore(t)
	ctx := context.Background()
	want := "load market data: file not found"

	if err := store.SetError(ctx, jobID, want); err != nil {
		t.Fatalf("set error: %v", err)
	}

	got, err := store.GetError(ctx, jobID)
	if err != nil {
		t.Fatalf("get error: %v", err)
	}

	if got != want {
		t.Fatalf("expected error message %q, got %q", want, got)
	}
}

func TestResultStoreRejectsEmptyJobID(t *testing.T) {
	store, _, _ := newTestResultStore(t)
	ctx := context.Background()

	err := store.SetStatus(ctx, "", StatusQueued)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !strings.Contains(err.Error(), "job id cannot be empty") {
		t.Fatalf("expected empty job id error, got %v", err)
	}
}

func newTestResultStore(t *testing.T) (*ResultStore, *redis.Client, string) {
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

	jobID := testQueueKey(t)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = client.Del(ctx, jobKey(jobID), statusKey(jobID), resultKey(jobID), errorKey(jobID)).Err()
		_ = client.Close()
	})

	return NewResultStore(client, time.Minute), client, jobID
}
