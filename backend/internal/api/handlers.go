package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"backtest-sim/backend/internal/backtest"
	"backtest-sim/backend/internal/queue"
	"github.com/redis/go-redis/v9"
)

// RunQueue is the API dependency that adds jobs to the worker queue
type RunQueue interface {
	Enqueue(ctx context.Context, job queue.Job) error
}

// RunStore is the API dependency that stores and reads run state
type RunStore interface {
	SetJob(ctx context.Context, job queue.Job) error
	GetJob(ctx context.Context, jobID string) (queue.Job, error)
	SetStatus(ctx context.Context, jobID string, status queue.JobStatus) error
	GetStatus(ctx context.Context, jobID string) (queue.JobStatus, error)
	GetResult(ctx context.Context, jobID string, destination any) error
	SetError(ctx context.Context, jobID string, message string) error
	GetError(ctx context.Context, jobID string) (string, error)
}

type healthResponse struct {
	Status string `json:"status"`
}

type tickersResponse struct {
	Tickers []string `json:"tickers"`
}

type strategyResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type strategiesResponse struct {
	Strategies []strategyResponse `json:"strategies"`
}

type runRequest struct {
	Ticker      string  `json:"ticker"`
	InitialCash float64 `json:"initial_cash"`
	FeeBps      float64 `json:"fee_bps"`
	SlippageBps float64 `json:"slippage_bps"`
	ShortWindow int     `json:"short_window"`
	LongWindow  int     `json:"long_window"`
}

type createRunResponse struct {
	ID        string          `json:"id"`
	Status    queue.JobStatus `json:"status"`
	StatusURL string          `json:"status_url"`
}

type runStatusResponse struct {
	ID     string             `json:"id"`
	Status queue.JobStatus    `json:"status"`
	Job    *runJobResponse    `json:"job,omitempty"`
	Result *runResultResponse `json:"result,omitempty"`
	Error  string             `json:"error,omitempty"`
}

type runJobResponse struct {
	Ticker      string  `json:"ticker"`
	InitialCash float64 `json:"initial_cash"`
	FeeBps      float64 `json:"fee_bps"`
	SlippageBps float64 `json:"slippage_bps"`
	ShortWindow int     `json:"short_window"`
	LongWindow  int     `json:"long_window"`
}

type runResultResponse struct {
	Strategy            string  `json:"strategy"`
	StrategyFinalValue  float64 `json:"strategy_final_value"`
	StrategyReturn      float64 `json:"strategy_return"`
	BenchmarkFinalValue float64 `json:"benchmark_final_value"`
	BenchmarkReturn     float64 `json:"benchmark_return"`
	ExcessReturn        float64 `json:"excess_return"`
	MaxDrawdown         float64 `json:"max_drawdown"`
	WinRate             float64 `json:"win_rate"`
	Signals             int     `json:"signals"`
	Trades              int     `json:"trades"`
}

type errorResponse struct {
	Error string `json:"error"`
}

// HealthHandler returns a simple status response for uptime checks
func HealthHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	writeJSON(w, http.StatusOK, healthResponse{
		Status: "ok",
	})
}

// TickersHandler returns the supported market data symbols
func TickersHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	writeJSON(w, http.StatusOK, tickersResponse{
		Tickers: []string{"SPY"},
	})
}

// StrategiesHandler returns the supported trading strategies
func StrategiesHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	writeJSON(w, http.StatusOK, strategiesResponse{
		Strategies: []strategyResponse{
			{
				ID:   "moving_average_crossover",
				Name: "Moving Average Crossover",
			},
		},
	})
}

// RunsHandler validates a run request, stores it, and enqueues it for workers
func RunsHandler(jobs RunQueue, store RunStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// This endpoint creates a run, so only POST requests are accepted
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Decode and validate the user request before touching Redis
		var request runRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			writeErrorJSON(w, http.StatusBadRequest, "invalid json request body")
			return
		}

		runID, err := newRunID()
		if err != nil {
			writeErrorJSON(w, http.StatusInternalServerError, "create run id")
			return
		}

		job, err := buildRunJob(runID, request)
		if err != nil {
			writeErrorJSON(w, http.StatusBadRequest, err.Error())
			return
		}

		// Store metadata and queued status before making the job visible to workers
		if err := store.SetJob(r.Context(), job); err != nil {
			writeErrorJSON(w, http.StatusInternalServerError, "store run request")
			return
		}

		if err := store.SetStatus(r.Context(), job.ID, queue.StatusQueued); err != nil {
			writeErrorJSON(w, http.StatusInternalServerError, "store run status")
			return
		}

		if err := jobs.Enqueue(r.Context(), job); err != nil {
			_ = store.SetError(r.Context(), job.ID, err.Error())
			_ = store.SetStatus(r.Context(), job.ID, queue.StatusFailed)
			writeErrorJSON(w, http.StatusInternalServerError, "enqueue run")
			return
		}

		writeJSON(w, http.StatusAccepted, createRunResponse{
			ID:        job.ID,
			Status:    queue.StatusQueued,
			StatusURL: "/api/runs/" + job.ID,
		})
	}
}

// RunStatusHandler returns queued/running/completed/failed details for one run
func RunStatusHandler(store RunStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// The route is /api/runs/{id}, so the id comes from the path suffix
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		runID := strings.TrimPrefix(r.URL.Path, "/api/runs/")
		if runID == "" || strings.Contains(runID, "/") {
			writeErrorJSON(w, http.StatusBadRequest, "run id is required")
			return
		}

		status, err := store.GetStatus(r.Context(), runID)
		if err != nil {
			if isMissingRedisValue(err) {
				writeErrorJSON(w, http.StatusNotFound, "run not found")
				return
			}

			writeErrorJSON(w, http.StatusInternalServerError, "load run status")
			return
		}

		response := runStatusResponse{
			ID:     runID,
			Status: status,
		}

		// Add the original request metadata when it is available
		job, err := store.GetJob(r.Context(), runID)
		if err != nil {
			if !isMissingRedisValue(err) {
				writeErrorJSON(w, http.StatusInternalServerError, "load run request")
				return
			}
		} else {
			response.Job = buildRunJobResponse(job)
		}

		// Completed runs include compact result metrics for the UI
		if status == queue.StatusCompleted {
			var result backtest.BacktestResult
			if err := store.GetResult(r.Context(), runID, &result); err != nil {
				writeErrorJSON(w, http.StatusInternalServerError, "load run result")
				return
			}

			response.Result = buildRunResultResponse(result)
		}

		// Failed runs include the stored error message when available
		if status == queue.StatusFailed {
			message, err := store.GetError(r.Context(), runID)
			if err != nil {
				if !isMissingRedisValue(err) {
					writeErrorJSON(w, http.StatusInternalServerError, "load run error")
					return
				}
			} else {
				response.Error = message
			}
		}

		writeJSON(w, http.StatusOK, response)
	}
}

func buildRunJob(runID string, request runRequest) (queue.Job, error) {
	ticker := strings.ToUpper(strings.TrimSpace(request.Ticker))
	if ticker != "SPY" {
		return queue.Job{}, fmt.Errorf("unsupported ticker %q", request.Ticker)
	}

	if request.InitialCash <= 0 {
		return queue.Job{}, fmt.Errorf("initial cash must be positive")
	}

	if request.FeeBps < 0 {
		return queue.Job{}, fmt.Errorf("fee bps cannot be negative")
	}

	if request.SlippageBps < 0 {
		return queue.Job{}, fmt.Errorf("slippage bps cannot be negative")
	}

	if request.ShortWindow <= 0 || request.LongWindow <= 0 {
		return queue.Job{}, fmt.Errorf("windows must be positive")
	}

	if request.ShortWindow >= request.LongWindow {
		return queue.Job{}, fmt.Errorf("short window must be less than long window")
	}

	return queue.Job{
		ID:          runID,
		Ticker:      ticker,
		InitialCash: request.InitialCash,
		FeeBps:      request.FeeBps,
		SlippageBps: request.SlippageBps,
		ShortWindow: request.ShortWindow,
		LongWindow:  request.LongWindow,
	}, nil
}

func buildRunJobResponse(job queue.Job) *runJobResponse {
	return &runJobResponse{
		Ticker:      job.Ticker,
		InitialCash: job.InitialCash,
		FeeBps:      job.FeeBps,
		SlippageBps: job.SlippageBps,
		ShortWindow: job.ShortWindow,
		LongWindow:  job.LongWindow,
	}
}

func buildRunResultResponse(result backtest.BacktestResult) *runResultResponse {
	return &runResultResponse{
		Strategy:            "moving_average_crossover",
		StrategyFinalValue:  result.Strategy.FinalValue,
		StrategyReturn:      result.TotalReturn,
		BenchmarkFinalValue: result.Benchmark.FinalValue,
		BenchmarkReturn:     result.BenchmarkReturn,
		ExcessReturn:        result.ExcessReturn,
		MaxDrawdown:         result.MaxDrawdown,
		WinRate:             result.WinRate,
		Signals:             len(result.Signals),
		Trades:              result.NumberOfTrades,
	}
}

func newRunID() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}

	return "run_" + hex.EncodeToString(bytes), nil
}

func isMissingRedisValue(err error) bool {
	return errors.Is(err, redis.Nil)
}

// writeJSON writes a JSON response with the provided HTTP status code
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(value); err != nil {
		http.Error(w, "encode json response", http.StatusInternalServerError)
	}
}

func writeErrorJSON(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, errorResponse{
		Error: message,
	})
}
