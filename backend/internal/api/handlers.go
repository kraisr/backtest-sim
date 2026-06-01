package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

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
	SetSweep(ctx context.Context, sweep queue.Sweep) error
	GetSweep(ctx context.Context, sweepID string) (queue.Sweep, error)
	SetJob(ctx context.Context, job queue.Job) error
	GetJob(ctx context.Context, jobID string) (queue.Job, error)
	SetStatus(ctx context.Context, jobID string, status queue.JobStatus) error
	GetStatus(ctx context.Context, jobID string) (queue.JobStatus, error)
	GetResult(ctx context.Context, jobID string, destination any) error
	SetError(ctx context.Context, jobID string, message string) error
	GetError(ctx context.Context, jobID string) (string, error)
}

// RunMetrics is the API dependency that records and exposes queue metrics
type RunMetrics interface {
	RecordJobQueued(ctx context.Context) error
	Snapshot(ctx context.Context) (queue.MetricsSnapshot, error)
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

type sweepRequest struct {
	Ticker       string  `json:"ticker"`
	InitialCash  float64 `json:"initial_cash"`
	FeeBps       float64 `json:"fee_bps"`
	SlippageBps  float64 `json:"slippage_bps"`
	ShortWindows []int   `json:"short_windows"`
	LongWindows  []int   `json:"long_windows"`
}

type createRunResponse struct {
	ID        string          `json:"id"`
	Status    queue.JobStatus `json:"status"`
	StatusURL string          `json:"status_url"`
}

type createSweepResponse struct {
	ID        string              `json:"id"`
	Status    string              `json:"status"`
	RunCount  int                 `json:"run_count"`
	Runs      []createRunResponse `json:"runs"`
	StatusURL string              `json:"status_url"`
}

type runStatusResponse struct {
	ID     string             `json:"id"`
	Status queue.JobStatus    `json:"status"`
	Job    *runJobResponse    `json:"job,omitempty"`
	Result *runResultResponse `json:"result,omitempty"`
	Error  string             `json:"error,omitempty"`
}

type sweepStatusResponse struct {
	ID        string                   `json:"id"`
	Status    string                   `json:"status"`
	RunCount  int                      `json:"run_count"`
	Queued    int                      `json:"queued"`
	Running   int                      `json:"running"`
	Completed int                      `json:"completed"`
	Failed    int                      `json:"failed"`
	Runs      []sweepRunStatusResponse `json:"runs"`
	BestRun   *sweepRunStatusResponse  `json:"best_run,omitempty"`
}

type sweepRunStatusResponse struct {
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
	Strategy             string                     `json:"strategy"`
	StrategyFinalValue   float64                    `json:"strategy_final_value"`
	StrategyReturn       float64                    `json:"strategy_return"`
	BenchmarkFinalValue  float64                    `json:"benchmark_final_value"`
	BenchmarkReturn      float64                    `json:"benchmark_return"`
	ExcessReturn         float64                    `json:"excess_return"`
	MaxDrawdown          float64                    `json:"max_drawdown"`
	WinRate              float64                    `json:"win_rate"`
	Signals              int                        `json:"signals"`
	Trades               int                        `json:"trades"`
	StrategyEquityCurve  []equityCurvePointResponse `json:"strategy_equity_curve,omitempty"`
	BenchmarkEquityCurve []equityCurvePointResponse `json:"benchmark_equity_curve,omitempty"`
}

type equityCurvePointResponse struct {
	Date   string  `json:"date"`
	Equity float64 `json:"equity"`
}

type errorResponse struct {
	Error string `json:"error"`
}

type noopRunMetrics struct{}

func (noopRunMetrics) RecordJobQueued(ctx context.Context) error {
	return nil
}

func (noopRunMetrics) Snapshot(ctx context.Context) (queue.MetricsSnapshot, error) {
	return queue.MetricsSnapshot{}, nil
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
func RunsHandler(jobs RunQueue, store RunStore, metrics RunMetrics) http.HandlerFunc {
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
		job.EnqueuedAt = time.Now().UTC()

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

		if err := metrics.RecordJobQueued(r.Context()); err != nil {
			writeErrorJSON(w, http.StatusInternalServerError, "record queued metric")
			return
		}

		writeJSON(w, http.StatusAccepted, createRunResponse{
			ID:        job.ID,
			Status:    queue.StatusQueued,
			StatusURL: "/api/runs/" + job.ID,
		})
	}
}

// SweepsHandler creates many moving-average runs from short/long window grids
func SweepsHandler(jobs RunQueue, store RunStore, metrics RunMetrics) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// A sweep expands one request into many queued runs
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var request sweepRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			writeErrorJSON(w, http.StatusBadRequest, "invalid json request body")
			return
		}

		sweep, runJobs, runResponses, err := buildSweep(request)
		if err != nil {
			writeErrorJSON(w, http.StatusBadRequest, err.Error())
			return
		}

		// Store the sweep before queuing child jobs so status polling can find it
		if err := store.SetSweep(r.Context(), sweep); err != nil {
			writeErrorJSON(w, http.StatusInternalServerError, "store sweep request")
			return
		}

		for _, job := range runJobs {
			if err := store.SetJob(r.Context(), job); err != nil {
				writeErrorJSON(w, http.StatusInternalServerError, "store sweep run")
				return
			}

			if err := store.SetStatus(r.Context(), job.ID, queue.StatusQueued); err != nil {
				writeErrorJSON(w, http.StatusInternalServerError, "store sweep run status")
				return
			}

			if err := jobs.Enqueue(r.Context(), job); err != nil {
				_ = store.SetError(r.Context(), job.ID, err.Error())
				_ = store.SetStatus(r.Context(), job.ID, queue.StatusFailed)
				writeErrorJSON(w, http.StatusInternalServerError, "enqueue sweep run")
				return
			}

			if err := metrics.RecordJobQueued(r.Context()); err != nil {
				writeErrorJSON(w, http.StatusInternalServerError, "record queued metric")
				return
			}
		}

		writeJSON(w, http.StatusAccepted, createSweepResponse{
			ID:        sweep.ID,
			Status:    "queued",
			RunCount:  len(sweep.RunIDs),
			Runs:      runResponses,
			StatusURL: "/api/sweeps/" + sweep.ID,
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

		// Completed run detail includes summary metrics plus equity curves for charts
		if status == queue.StatusCompleted {
			var result backtest.BacktestResult
			if err := store.GetResult(r.Context(), runID, &result); err != nil {
				writeErrorJSON(w, http.StatusInternalServerError, "load run result")
				return
			}

			response.Result = buildRunResultResponse(result, true)
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

// SweepStatusHandler returns aggregate and per-run status for a sweep
func SweepStatusHandler(store RunStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// The route is /api/sweeps/{id}, so the id comes from the path suffix
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		sweepID := strings.TrimPrefix(r.URL.Path, "/api/sweeps/")
		if sweepID == "" || strings.Contains(sweepID, "/") {
			writeErrorJSON(w, http.StatusBadRequest, "sweep id is required")
			return
		}

		sweep, err := store.GetSweep(r.Context(), sweepID)
		if err != nil {
			if isMissingRedisValue(err) {
				writeErrorJSON(w, http.StatusNotFound, "sweep not found")
				return
			}

			writeErrorJSON(w, http.StatusInternalServerError, "load sweep")
			return
		}

		response, err := buildSweepStatusResponse(r.Context(), store, sweep)
		if err != nil {
			writeErrorJSON(w, http.StatusInternalServerError, err.Error())
			return
		}

		writeJSON(w, http.StatusOK, response)
	}
}

// MetricsHandler exposes Redis-backed metrics in Prometheus text format
func MetricsHandler(metrics RunMetrics) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		snapshot, err := metrics.Snapshot(r.Context())
		if err != nil {
			http.Error(w, "load metrics", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, formatPrometheusMetrics(snapshot))
	}
}

func buildSweep(request sweepRequest) (queue.Sweep, []queue.Job, []createRunResponse, error) {
	ticker := strings.ToUpper(strings.TrimSpace(request.Ticker))
	if ticker != "SPY" {
		return queue.Sweep{}, nil, nil, fmt.Errorf("unsupported ticker %q", request.Ticker)
	}

	if request.InitialCash <= 0 {
		return queue.Sweep{}, nil, nil, fmt.Errorf("initial cash must be positive")
	}

	if request.FeeBps < 0 {
		return queue.Sweep{}, nil, nil, fmt.Errorf("fee bps cannot be negative")
	}

	if request.SlippageBps < 0 {
		return queue.Sweep{}, nil, nil, fmt.Errorf("slippage bps cannot be negative")
	}

	shortWindows, err := uniquePositiveInts(request.ShortWindows, "short windows")
	if err != nil {
		return queue.Sweep{}, nil, nil, err
	}

	longWindows, err := uniquePositiveInts(request.LongWindows, "long windows")
	if err != nil {
		return queue.Sweep{}, nil, nil, err
	}

	sweepID, err := newSweepID()
	if err != nil {
		return queue.Sweep{}, nil, nil, fmt.Errorf("create sweep id: %w", err)
	}

	createdAt := time.Now().UTC()
	var jobs []queue.Job
	var responses []createRunResponse
	var runIDs []string

	for _, shortWindow := range shortWindows {
		for _, longWindow := range longWindows {
			if shortWindow >= longWindow {
				continue
			}

			if len(jobs) >= maxSweepRuns {
				return queue.Sweep{}, nil, nil, fmt.Errorf("sweep cannot create more than %d runs", maxSweepRuns)
			}

			runID, err := newRunID()
			if err != nil {
				return queue.Sweep{}, nil, nil, fmt.Errorf("create run id: %w", err)
			}

			job := queue.Job{
				ID:          runID,
				SweepID:     sweepID,
				Ticker:      ticker,
				InitialCash: request.InitialCash,
				FeeBps:      request.FeeBps,
				SlippageBps: request.SlippageBps,
				ShortWindow: shortWindow,
				LongWindow:  longWindow,
				EnqueuedAt:  createdAt,
			}

			jobs = append(jobs, job)
			runIDs = append(runIDs, runID)
			responses = append(responses, createRunResponse{
				ID:        runID,
				Status:    queue.StatusQueued,
				StatusURL: "/api/runs/" + runID,
			})
		}
	}

	if len(jobs) == 0 {
		return queue.Sweep{}, nil, nil, fmt.Errorf("sweep must contain at least one valid short/long window pair")
	}

	return queue.Sweep{
		ID:           sweepID,
		Ticker:       ticker,
		InitialCash:  request.InitialCash,
		FeeBps:       request.FeeBps,
		SlippageBps:  request.SlippageBps,
		ShortWindows: shortWindows,
		LongWindows:  longWindows,
		RunIDs:       runIDs,
		CreatedAt:    createdAt,
	}, jobs, responses, nil
}

func buildSweepStatusResponse(ctx context.Context, store RunStore, sweep queue.Sweep) (sweepStatusResponse, error) {
	response := sweepStatusResponse{
		ID:       sweep.ID,
		RunCount: len(sweep.RunIDs),
		Runs:     make([]sweepRunStatusResponse, 0, len(sweep.RunIDs)),
	}

	var bestRun *sweepRunStatusResponse
	for _, runID := range sweep.RunIDs {
		runStatus, err := buildSweepRunStatus(ctx, store, runID)
		if err != nil {
			return sweepStatusResponse{}, err
		}

		switch runStatus.Status {
		case queue.StatusQueued:
			response.Queued++
		case queue.StatusRunning:
			response.Running++
		case queue.StatusCompleted:
			response.Completed++
		case queue.StatusFailed:
			response.Failed++
		}

		if runStatus.Result != nil && (bestRun == nil || runStatus.Result.StrategyReturn > bestRun.Result.StrategyReturn) {
			candidate := runStatus
			bestRun = &candidate
		}

		response.Runs = append(response.Runs, runStatus)
	}

	response.Status = sweepStatus(response)
	response.BestRun = bestRun
	return response, nil
}

func buildSweepRunStatus(ctx context.Context, store RunStore, runID string) (sweepRunStatusResponse, error) {
	status, err := store.GetStatus(ctx, runID)
	if err != nil {
		return sweepRunStatusResponse{}, fmt.Errorf("load sweep run status")
	}

	response := sweepRunStatusResponse{
		ID:     runID,
		Status: status,
	}

	job, err := store.GetJob(ctx, runID)
	if err != nil {
		if !isMissingRedisValue(err) {
			return sweepRunStatusResponse{}, fmt.Errorf("load sweep run request")
		}
	} else {
		response.Job = buildRunJobResponse(job)
	}

	if status == queue.StatusCompleted {
		var result backtest.BacktestResult
		if err := store.GetResult(ctx, runID, &result); err != nil {
			return sweepRunStatusResponse{}, fmt.Errorf("load sweep run result")
		}

		response.Result = buildRunResultResponse(result, false)
	}

	if status == queue.StatusFailed {
		message, err := store.GetError(ctx, runID)
		if err != nil {
			if !isMissingRedisValue(err) {
				return sweepRunStatusResponse{}, fmt.Errorf("load sweep run error")
			}
		} else {
			response.Error = message
		}
	}

	return response, nil
}

func sweepStatus(response sweepStatusResponse) string {
	if response.Failed > 0 && response.Completed+response.Failed == response.RunCount {
		return "failed"
	}

	if response.Completed == response.RunCount {
		return "completed"
	}

	if response.Running > 0 || response.Completed > 0 || response.Failed > 0 {
		return "running"
	}

	return "queued"
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

func uniquePositiveInts(values []int, name string) ([]int, error) {
	if len(values) == 0 {
		return nil, fmt.Errorf("%s cannot be empty", name)
	}

	seen := make(map[int]bool)
	var unique []int
	for _, value := range values {
		if value <= 0 {
			return nil, fmt.Errorf("%s must be positive", name)
		}

		if !seen[value] {
			seen[value] = true
			unique = append(unique, value)
		}
	}

	sort.Ints(unique)
	return unique, nil
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

func buildRunResultResponse(result backtest.BacktestResult, includeEquityCurves bool) *runResultResponse {
	response := &runResultResponse{
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

	if includeEquityCurves {
		response.StrategyEquityCurve = buildEquityCurveResponse(result.Strategy.EquityCurve)
		response.BenchmarkEquityCurve = buildEquityCurveResponse(result.Benchmark.EquityCurve)
	}

	return response
}

func buildEquityCurveResponse(points []backtest.EquityPoint) []equityCurvePointResponse {
	response := make([]equityCurvePointResponse, 0, len(points))
	for _, point := range points {
		response = append(response, equityCurvePointResponse{
			Date:   point.Date.Format("2006-01-02"),
			Equity: point.Equity,
		})
	}

	return response
}

func newRunID() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}

	return "run_" + hex.EncodeToString(bytes), nil
}

func newSweepID() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}

	return "sweep_" + hex.EncodeToString(bytes), nil
}

func isMissingRedisValue(err error) bool {
	return errors.Is(err, redis.Nil)
}

func formatPrometheusMetrics(snapshot queue.MetricsSnapshot) string {
	return fmt.Sprintf(`# HELP backtestsim_queue_depth Number of jobs waiting in Redis
# TYPE backtestsim_queue_depth gauge
backtestsim_queue_depth %d
# HELP backtestsim_jobs_total Number of jobs observed by status
# TYPE backtestsim_jobs_total counter
backtestsim_jobs_total{status="queued"} %d
backtestsim_jobs_total{status="started"} %d
backtestsim_jobs_total{status="completed"} %d
backtestsim_jobs_total{status="failed"} %d
# HELP backtestsim_job_duration_seconds Worker execution duration
# TYPE backtestsim_job_duration_seconds summary
backtestsim_job_duration_seconds_count %d
backtestsim_job_duration_seconds_sum %.6f
# HELP backtestsim_queue_latency_seconds Time from enqueue to worker start
# TYPE backtestsim_queue_latency_seconds summary
backtestsim_queue_latency_seconds_count %d
backtestsim_queue_latency_seconds_sum %.6f
`,
		snapshot.QueueDepth,
		snapshot.JobsQueued,
		snapshot.JobsStarted,
		snapshot.JobsCompleted,
		snapshot.JobsFailed,
		snapshot.JobDurationCount,
		snapshot.JobDurationSecondsSum,
		snapshot.QueueLatencyCount,
		snapshot.QueueLatencySecondsSum,
	)
}

const maxSweepRuns = 250

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
