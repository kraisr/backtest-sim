package queue

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

type MetricsSnapshot struct {
	QueueDepth             int64
	JobsQueued             int64
	JobsStarted            int64
	JobsCompleted          int64
	JobsFailed             int64
	JobDurationCount       int64
	JobDurationSecondsSum  float64
	QueueLatencyCount      int64
	QueueLatencySecondsSum float64
}

type MetricsStore struct {
	client   *redis.Client
	queueKey string
}

func NewMetricsStore(client *redis.Client, queueKey string) *MetricsStore {
	return &MetricsStore{
		client:   client,
		queueKey: queueKey,
	}
}

// RecordJobQueued increments the count of jobs accepted by the API
func (s *MetricsStore) RecordJobQueued(ctx context.Context) error {
	if err := s.client.Incr(ctx, metricJobsQueuedKey).Err(); err != nil {
		return fmt.Errorf("record queued job: %w", err)
	}

	return nil
}

// RecordJobStarted increments started jobs and observes queue wait time
func (s *MetricsStore) RecordJobStarted(ctx context.Context, job Job, startedAt time.Time) error {
	pipe := s.client.Pipeline()
	pipe.Incr(ctx, metricJobsStartedKey)

	if !job.EnqueuedAt.IsZero() {
		latency := startedAt.Sub(job.EnqueuedAt)
		if latency >= 0 {
			pipe.Incr(ctx, metricQueueLatencyCountKey)
			pipe.IncrByFloat(ctx, metricQueueLatencySumKey, latency.Seconds())
		}
	}

	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("record started job: %w", err)
	}

	return nil
}

// RecordJobCompleted increments completed jobs and observes worker run duration
func (s *MetricsStore) RecordJobCompleted(ctx context.Context, duration time.Duration) error {
	return s.recordFinishedJob(ctx, metricJobsCompletedKey, duration)
}

// RecordJobFailed increments failed jobs and observes worker run duration
func (s *MetricsStore) RecordJobFailed(ctx context.Context, duration time.Duration) error {
	return s.recordFinishedJob(ctx, metricJobsFailedKey, duration)
}

// Snapshot reads the current queue and worker metrics for the /metrics endpoint
func (s *MetricsStore) Snapshot(ctx context.Context) (MetricsSnapshot, error) {
	queueDepth, err := s.client.LLen(ctx, s.queueKey).Result()
	if err != nil {
		return MetricsSnapshot{}, fmt.Errorf("read queue depth: %w", err)
	}

	jobsQueued, err := s.readInt(ctx, metricJobsQueuedKey)
	if err != nil {
		return MetricsSnapshot{}, err
	}

	jobsStarted, err := s.readInt(ctx, metricJobsStartedKey)
	if err != nil {
		return MetricsSnapshot{}, err
	}

	jobsCompleted, err := s.readInt(ctx, metricJobsCompletedKey)
	if err != nil {
		return MetricsSnapshot{}, err
	}

	jobsFailed, err := s.readInt(ctx, metricJobsFailedKey)
	if err != nil {
		return MetricsSnapshot{}, err
	}

	durationCount, err := s.readInt(ctx, metricJobDurationCountKey)
	if err != nil {
		return MetricsSnapshot{}, err
	}

	durationSum, err := s.readFloat(ctx, metricJobDurationSumKey)
	if err != nil {
		return MetricsSnapshot{}, err
	}

	latencyCount, err := s.readInt(ctx, metricQueueLatencyCountKey)
	if err != nil {
		return MetricsSnapshot{}, err
	}

	latencySum, err := s.readFloat(ctx, metricQueueLatencySumKey)
	if err != nil {
		return MetricsSnapshot{}, err
	}

	return MetricsSnapshot{
		QueueDepth:             queueDepth,
		JobsQueued:             jobsQueued,
		JobsStarted:            jobsStarted,
		JobsCompleted:          jobsCompleted,
		JobsFailed:             jobsFailed,
		JobDurationCount:       durationCount,
		JobDurationSecondsSum:  durationSum,
		QueueLatencyCount:      latencyCount,
		QueueLatencySecondsSum: latencySum,
	}, nil
}

func (s *MetricsStore) recordFinishedJob(ctx context.Context, counterKey string, duration time.Duration) error {
	pipe := s.client.Pipeline()
	pipe.Incr(ctx, counterKey)
	pipe.Incr(ctx, metricJobDurationCountKey)
	pipe.IncrByFloat(ctx, metricJobDurationSumKey, duration.Seconds())

	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("record finished job: %w", err)
	}

	return nil
}

func (s *MetricsStore) readInt(ctx context.Context, key string) (int64, error) {
	value, err := s.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return 0, nil
		}

		return 0, fmt.Errorf("read metric %s: %w", key, err)
	}

	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse metric %s: %w", key, err)
	}

	return parsed, nil
}

func (s *MetricsStore) readFloat(ctx context.Context, key string) (float64, error) {
	value, err := s.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return 0, nil
		}

		return 0, fmt.Errorf("read metric %s: %w", key, err)
	}

	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, fmt.Errorf("parse metric %s: %w", key, err)
	}

	return parsed, nil
}

const (
	metricJobsQueuedKey        = "metrics:jobs:queued"
	metricJobsStartedKey       = "metrics:jobs:started"
	metricJobsCompletedKey     = "metrics:jobs:completed"
	metricJobsFailedKey        = "metrics:jobs:failed"
	metricJobDurationCountKey  = "metrics:job_duration_seconds:count"
	metricJobDurationSumKey    = "metrics:job_duration_seconds:sum"
	metricQueueLatencyCountKey = "metrics:queue_latency_seconds:count"
	metricQueueLatencySumKey   = "metrics:queue_latency_seconds:sum"
)
