package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type JobStatus string

const (
	StatusQueued    JobStatus = "queued"
	StatusRunning   JobStatus = "running"
	StatusCompleted JobStatus = "completed"
	StatusFailed    JobStatus = "failed"
)

type ResultStore struct {
	client *redis.Client
	ttl    time.Duration
}

func NewResultStore(client *redis.Client, ttl time.Duration) *ResultStore {
	return &ResultStore{
		client: client,
		ttl:    ttl,
	}
}

// SetSweep stores a parameter sweep and the run ids it created
func (s *ResultStore) SetSweep(ctx context.Context, sweep Sweep) error {
	if err := validateJobID(sweep.ID); err != nil {
		return err
	}

	payload, err := json.Marshal(sweep)
	if err != nil {
		return fmt.Errorf("marshal sweep: %w", err)
	}

	if err := s.client.Set(ctx, sweepKey(sweep.ID), payload, s.ttl).Err(); err != nil {
		return fmt.Errorf("set sweep: %w", err)
	}

	return nil
}

// GetSweep returns the original sweep request and generated run ids
func (s *ResultStore) GetSweep(ctx context.Context, sweepID string) (Sweep, error) {
	if err := validateJobID(sweepID); err != nil {
		return Sweep{}, err
	}

	payload, err := s.client.Get(ctx, sweepKey(sweepID)).Bytes()
	if err != nil {
		return Sweep{}, fmt.Errorf("get sweep: %w", err)
	}

	var sweep Sweep
	if err := json.Unmarshal(payload, &sweep); err != nil {
		return Sweep{}, fmt.Errorf("unmarshal sweep: %w", err)
	}

	return sweep, nil
}

// SetJob stores the original job request so the API can show run metadata later
func (s *ResultStore) SetJob(ctx context.Context, job Job) error {
	if err := validateJobID(job.ID); err != nil {
		return err
	}

	payload, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("marshal job: %w", err)
	}

	if err := s.client.Set(ctx, jobKey(job.ID), payload, s.ttl).Err(); err != nil {
		return fmt.Errorf("set job: %w", err)
	}

	return nil
}

// GetJob returns the original job request for a run
func (s *ResultStore) GetJob(ctx context.Context, jobID string) (Job, error) {
	if err := validateJobID(jobID); err != nil {
		return Job{}, err
	}

	payload, err := s.client.Get(ctx, jobKey(jobID)).Bytes()
	if err != nil {
		return Job{}, fmt.Errorf("get job: %w", err)
	}

	var job Job
	if err := json.Unmarshal(payload, &job); err != nil {
		return Job{}, fmt.Errorf("unmarshal job: %w", err)
	}

	return job, nil
}

// SetStatus stores the current lifecycle status for a job
func (s *ResultStore) SetStatus(ctx context.Context, jobID string, status JobStatus) error {
	if err := validateJobID(jobID); err != nil {
		return err
	}

	if err := s.client.Set(ctx, statusKey(jobID), string(status), s.ttl).Err(); err != nil {
		return fmt.Errorf("set job status: %w", err)
	}

	return nil
}

// GetStatus returns the latest lifecycle status for a job
func (s *ResultStore) GetStatus(ctx context.Context, jobID string) (JobStatus, error) {
	if err := validateJobID(jobID); err != nil {
		return "", err
	}

	value, err := s.client.Get(ctx, statusKey(jobID)).Result()
	if err != nil {
		return "", fmt.Errorf("get job status: %w", err)
	}

	return JobStatus(value), nil
}

// SetResult stores a JSON-encoded backtest result for a completed job
func (s *ResultStore) SetResult(ctx context.Context, jobID string, result any) error {
	if err := validateJobID(jobID); err != nil {
		return err
	}

	payload, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("marshal job result: %w", err)
	}

	if err := s.client.Set(ctx, resultKey(jobID), payload, s.ttl).Err(); err != nil {
		return fmt.Errorf("set job result: %w", err)
	}

	return nil
}

// GetResult loads a JSON-encoded backtest result into the provided destination
func (s *ResultStore) GetResult(ctx context.Context, jobID string, destination any) error {
	if err := validateJobID(jobID); err != nil {
		return err
	}

	payload, err := s.client.Get(ctx, resultKey(jobID)).Bytes()
	if err != nil {
		return fmt.Errorf("get job result: %w", err)
	}

	if err := json.Unmarshal(payload, destination); err != nil {
		return fmt.Errorf("unmarshal job result: %w", err)
	}

	return nil
}

// SetError stores the failure reason for a failed job
func (s *ResultStore) SetError(ctx context.Context, jobID string, message string) error {
	if err := validateJobID(jobID); err != nil {
		return err
	}

	if err := s.client.Set(ctx, errorKey(jobID), message, s.ttl).Err(); err != nil {
		return fmt.Errorf("set job error: %w", err)
	}

	return nil
}

// GetError returns the stored failure reason for a job
func (s *ResultStore) GetError(ctx context.Context, jobID string) (string, error) {
	if err := validateJobID(jobID); err != nil {
		return "", err
	}

	message, err := s.client.Get(ctx, errorKey(jobID)).Result()
	if err != nil {
		return "", fmt.Errorf("get job error: %w", err)
	}

	return message, nil
}

func validateJobID(jobID string) error {
	if jobID == "" {
		return fmt.Errorf("job id cannot be empty")
	}

	return nil
}

func sweepKey(sweepID string) string {
	return fmt.Sprintf("sweep:%s:request", sweepID)
}

func jobKey(jobID string) string {
	return fmt.Sprintf("job:%s:request", jobID)
}

func statusKey(jobID string) string {
	return fmt.Sprintf("job:%s:status", jobID)
}

func resultKey(jobID string) string {
	return fmt.Sprintf("job:%s:result", jobID)
}

func errorKey(jobID string) string {
	return fmt.Sprintf("job:%s:error", jobID)
}
