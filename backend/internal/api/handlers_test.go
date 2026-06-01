package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"backtest-sim/backend/internal/backtest"
	"backtest-sim/backend/internal/queue"
	"github.com/redis/go-redis/v9"
)

func TestHealthHandler(t *testing.T) {
	_, _, router := newTestRouter()

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	if contentType := rec.Header().Get("Content-Type"); contentType != "application/json" {
		t.Fatalf("expected content type application/json, got %q", contentType)
	}

	if body := strings.TrimSpace(rec.Body.String()); body != `{"status":"ok"}` {
		t.Fatalf("expected health response body, got %q", body)
	}
}

func TestHealthHandlerRejectsUnsupportedMethod(t *testing.T) {
	_, _, router := newTestRouter()

	req := httptest.NewRequest(http.MethodPost, "/health", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}

func TestTickersHandler(t *testing.T) {
	_, _, router := newTestRouter()

	req := httptest.NewRequest(http.MethodGet, "/api/tickers", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	if contentType := rec.Header().Get("Content-Type"); contentType != "application/json" {
		t.Fatalf("expected content type application/json, got %q", contentType)
	}

	if body := strings.TrimSpace(rec.Body.String()); body != `{"tickers":["SPY"]}` {
		t.Fatalf("expected tickers response body, got %q", body)
	}
}

func TestTickersHandlerRejectsUnsupportedMethod(t *testing.T) {
	_, _, router := newTestRouter()

	req := httptest.NewRequest(http.MethodPost, "/api/tickers", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}

func TestStrategiesHandler(t *testing.T) {
	_, _, router := newTestRouter()

	req := httptest.NewRequest(http.MethodGet, "/api/strategies", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	if contentType := rec.Header().Get("Content-Type"); contentType != "application/json" {
		t.Fatalf("expected content type application/json, got %q", contentType)
	}

	want := `{"strategies":[{"id":"moving_average_crossover","name":"Moving Average Crossover"}]}`
	if body := strings.TrimSpace(rec.Body.String()); body != want {
		t.Fatalf("expected strategies response body, got %q", body)
	}
}

func TestStrategiesHandlerRejectsUnsupportedMethod(t *testing.T) {
	_, _, router := newTestRouter()

	req := httptest.NewRequest(http.MethodPost, "/api/strategies", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}

func TestRunsHandlerCreatesQueuedRun(t *testing.T) {
	runQueue, store, router := newTestRouter()

	body := `{
		"ticker": "spy",
		"initial_cash": 10000,
		"fee_bps": 0,
		"slippage_bps": 0,
		"short_window": 20,
		"long_window": 50
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/runs", strings.NewReader(body))
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected status %d, got %d with body %q", http.StatusAccepted, rec.Code, rec.Body.String())
	}

	var response createRunResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode create run response: %v", err)
	}

	if response.ID == "" {
		t.Fatal("expected generated run id")
	}

	if response.Status != queue.StatusQueued {
		t.Fatalf("expected queued status, got %q", response.Status)
	}

	if response.StatusURL != "/api/runs/"+response.ID {
		t.Fatalf("expected status url for run id, got %q", response.StatusURL)
	}

	if len(runQueue.jobs) != 1 {
		t.Fatalf("expected 1 queued job, got %d", len(runQueue.jobs))
	}

	job := runQueue.jobs[0]
	if job.ID != response.ID {
		t.Fatalf("expected queued job id %q, got %q", response.ID, job.ID)
	}

	if job.Ticker != "SPY" {
		t.Fatalf("expected normalized ticker SPY, got %q", job.Ticker)
	}

	if store.statuses[response.ID] != queue.StatusQueued {
		t.Fatalf("expected stored queued status, got %q", store.statuses[response.ID])
	}

	if storedJob := store.jobs[response.ID]; storedJob.ID != response.ID {
		t.Fatalf("expected stored job id %q, got %q", response.ID, storedJob.ID)
	}
}

func TestRunsHandlerRejectsUnsupportedMethod(t *testing.T) {
	_, _, router := newTestRouter()

	req := httptest.NewRequest(http.MethodGet, "/api/runs", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}

func TestRunsHandlerRejectsInvalidJSON(t *testing.T) {
	_, _, router := newTestRouter()

	req := httptest.NewRequest(http.MethodPost, "/api/runs", strings.NewReader(`{`))
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}

	if body := strings.TrimSpace(rec.Body.String()); body != `{"error":"invalid json request body"}` {
		t.Fatalf("expected invalid json error response, got %q", body)
	}
}

func TestRunsHandlerRejectsUnsupportedTicker(t *testing.T) {
	_, _, router := newTestRouter()

	body := `{
		"ticker": "AAPL",
		"initial_cash": 10000,
		"short_window": 20,
		"long_window": 50
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/runs", strings.NewReader(body))
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}

	if !strings.Contains(rec.Body.String(), "unsupported ticker") {
		t.Fatalf("expected unsupported ticker error, got %q", rec.Body.String())
	}
}

func TestRunsHandlerRejectsInvalidBacktestConfig(t *testing.T) {
	_, _, router := newTestRouter()

	body := `{
		"ticker": "SPY",
		"initial_cash": 10000,
		"short_window": 50,
		"long_window": 20
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/runs", strings.NewReader(body))
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}

	if !strings.Contains(rec.Body.String(), "short window must be less than long window") {
		t.Fatalf("expected short window validation error, got %q", rec.Body.String())
	}
}

func TestRunStatusHandlerReturnsQueuedRun(t *testing.T) {
	_, store, router := newTestRouter()
	job := testRunJob("run_queued")
	store.jobs[job.ID] = job
	store.statuses[job.ID] = queue.StatusQueued

	req := httptest.NewRequest(http.MethodGet, "/api/runs/"+job.ID, nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d with body %q", http.StatusOK, rec.Code, rec.Body.String())
	}

	var response runStatusResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode run status response: %v", err)
	}

	if response.ID != job.ID {
		t.Fatalf("expected run id %q, got %q", job.ID, response.ID)
	}

	if response.Status != queue.StatusQueued {
		t.Fatalf("expected queued status, got %q", response.Status)
	}

	if response.Job == nil || response.Job.Ticker != "SPY" {
		t.Fatalf("expected job metadata for SPY, got %+v", response.Job)
	}

	if response.Result != nil {
		t.Fatalf("expected no result for queued run, got %+v", response.Result)
	}
}

func TestRunStatusHandlerReturnsCompletedRun(t *testing.T) {
	_, store, router := newTestRouter()
	job := testRunJob("run_completed")
	store.jobs[job.ID] = job
	store.statuses[job.ID] = queue.StatusCompleted
	store.results[job.ID] = backtest.BacktestResult{
		Signals: []backtest.Signal{
			{Type: backtest.SignalBuy},
			{Type: backtest.SignalSell},
		},
		Strategy: backtest.PortfolioResult{
			FinalValue: 12000,
			EquityCurve: []backtest.EquityPoint{
				{Date: time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC), Equity: 10000},
				{Date: time.Date(2024, 1, 3, 0, 0, 0, 0, time.UTC), Equity: 12000},
			},
		},
		Benchmark: backtest.PortfolioResult{
			FinalValue: 11000,
			EquityCurve: []backtest.EquityPoint{
				{Date: time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC), Equity: 10000},
				{Date: time.Date(2024, 1, 3, 0, 0, 0, 0, time.UTC), Equity: 11000},
			},
		},
		TotalReturn:     0.2,
		BenchmarkReturn: 0.1,
		ExcessReturn:    0.1,
		MaxDrawdown:     0.05,
		WinRate:         1,
		NumberOfTrades:  2,
	}

	req := httptest.NewRequest(http.MethodGet, "/api/runs/"+job.ID, nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d with body %q", http.StatusOK, rec.Code, rec.Body.String())
	}

	var response runStatusResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode completed run response: %v", err)
	}

	if response.Status != queue.StatusCompleted {
		t.Fatalf("expected completed status, got %q", response.Status)
	}

	if response.Result == nil {
		t.Fatal("expected completed run result")
	}

	assertAPIFloatEqual(t, response.Result.StrategyFinalValue, 12000)
	assertAPIFloatEqual(t, response.Result.StrategyReturn, 0.2)
	assertAPIFloatEqual(t, response.Result.BenchmarkFinalValue, 11000)
	assertAPIFloatEqual(t, response.Result.BenchmarkReturn, 0.1)
	assertAPIFloatEqual(t, response.Result.ExcessReturn, 0.1)
	assertAPIFloatEqual(t, response.Result.MaxDrawdown, 0.05)
	assertAPIFloatEqual(t, response.Result.WinRate, 1)

	if response.Result.Signals != 2 {
		t.Fatalf("expected 2 signals, got %d", response.Result.Signals)
	}

	if response.Result.Trades != 2 {
		t.Fatalf("expected 2 trades, got %d", response.Result.Trades)
	}

	if len(response.Result.StrategyEquityCurve) != 2 {
		t.Fatalf("expected 2 strategy equity points, got %d", len(response.Result.StrategyEquityCurve))
	}

	if response.Result.StrategyEquityCurve[1].Date != "2024-01-03" {
		t.Fatalf("expected second strategy equity date 2024-01-03, got %q", response.Result.StrategyEquityCurve[1].Date)
	}

	assertAPIFloatEqual(t, response.Result.StrategyEquityCurve[1].Equity, 12000)

	if len(response.Result.BenchmarkEquityCurve) != 2 {
		t.Fatalf("expected 2 benchmark equity points, got %d", len(response.Result.BenchmarkEquityCurve))
	}

	assertAPIFloatEqual(t, response.Result.BenchmarkEquityCurve[1].Equity, 11000)
}

func TestRunStatusHandlerReturnsFailedRun(t *testing.T) {
	_, store, router := newTestRouter()
	job := testRunJob("run_failed")
	store.jobs[job.ID] = job
	store.statuses[job.ID] = queue.StatusFailed
	store.errors[job.ID] = "load market data: file not found"

	req := httptest.NewRequest(http.MethodGet, "/api/runs/"+job.ID, nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d with body %q", http.StatusOK, rec.Code, rec.Body.String())
	}

	var response runStatusResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode failed run response: %v", err)
	}

	if response.Status != queue.StatusFailed {
		t.Fatalf("expected failed status, got %q", response.Status)
	}

	if response.Error != "load market data: file not found" {
		t.Fatalf("expected stored error message, got %q", response.Error)
	}
}

func TestRunStatusHandlerReturnsNotFound(t *testing.T) {
	_, _, router := newTestRouter()

	req := httptest.NewRequest(http.MethodGet, "/api/runs/missing", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, rec.Code)
	}

	if body := strings.TrimSpace(rec.Body.String()); body != `{"error":"run not found"}` {
		t.Fatalf("expected run not found response, got %q", body)
	}
}

func TestRunStatusHandlerRejectsUnsupportedMethod(t *testing.T) {
	_, _, router := newTestRouter()

	req := httptest.NewRequest(http.MethodPost, "/api/runs/run_123", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}

func TestSweepsHandlerCreatesQueuedRuns(t *testing.T) {
	runQueue, store, router := newTestRouter()

	body := `{
		"ticker": "SPY",
		"initial_cash": 10000,
		"fee_bps": 0,
		"slippage_bps": 0,
		"short_windows": [5, 10],
		"long_windows": [20, 50]
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/sweeps", strings.NewReader(body))
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected status %d, got %d with body %q", http.StatusAccepted, rec.Code, rec.Body.String())
	}

	var response createSweepResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode create sweep response: %v", err)
	}

	if response.ID == "" {
		t.Fatal("expected generated sweep id")
	}

	if response.RunCount != 4 {
		t.Fatalf("expected 4 runs, got %d", response.RunCount)
	}

	if len(runQueue.jobs) != 4 {
		t.Fatalf("expected 4 queued jobs, got %d", len(runQueue.jobs))
	}

	if len(store.sweeps) != 1 {
		t.Fatalf("expected 1 stored sweep, got %d", len(store.sweeps))
	}

	for _, job := range runQueue.jobs {
		if job.SweepID != response.ID {
			t.Fatalf("expected sweep id %q, got %q", response.ID, job.SweepID)
		}

		if store.statuses[job.ID] != queue.StatusQueued {
			t.Fatalf("expected queued status for job %s, got %q", job.ID, store.statuses[job.ID])
		}
	}
}

func TestSweepsHandlerRejectsEmptyWindowGrid(t *testing.T) {
	_, _, router := newTestRouter()

	body := `{
		"ticker": "SPY",
		"initial_cash": 10000,
		"short_windows": [50],
		"long_windows": [20]
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/sweeps", strings.NewReader(body))
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}

	if !strings.Contains(rec.Body.String(), "at least one valid short/long window pair") {
		t.Fatalf("expected invalid grid error, got %q", rec.Body.String())
	}
}

func TestSweepStatusHandlerReturnsAggregateStatus(t *testing.T) {
	_, store, router := newTestRouter()
	sweep := queue.Sweep{
		ID:     "sweep_1",
		Ticker: "SPY",
		RunIDs: []string{
			"run_1",
			"run_2",
		},
	}
	store.sweeps[sweep.ID] = sweep

	firstJob := testRunJob("run_1")
	secondJob := testRunJob("run_2")
	store.jobs[firstJob.ID] = firstJob
	store.jobs[secondJob.ID] = secondJob
	store.statuses[firstJob.ID] = queue.StatusCompleted
	store.statuses[secondJob.ID] = queue.StatusRunning
	store.results[firstJob.ID] = backtest.BacktestResult{
		Strategy: backtest.PortfolioResult{
			FinalValue: 12000,
		},
		Benchmark: backtest.PortfolioResult{
			FinalValue: 10000,
		},
		TotalReturn:     0.2,
		BenchmarkReturn: 0,
		ExcessReturn:    0.2,
	}

	req := httptest.NewRequest(http.MethodGet, "/api/sweeps/"+sweep.ID, nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d with body %q", http.StatusOK, rec.Code, rec.Body.String())
	}

	var response sweepStatusResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode sweep status response: %v", err)
	}

	if response.Status != "running" {
		t.Fatalf("expected running sweep, got %q", response.Status)
	}

	if response.Completed != 1 || response.Running != 1 {
		t.Fatalf("expected 1 completed and 1 running, got completed=%d running=%d", response.Completed, response.Running)
	}

	if response.BestRun == nil || response.BestRun.ID != firstJob.ID {
		t.Fatalf("expected best run %q, got %+v", firstJob.ID, response.BestRun)
	}

	if response.BestRun.Result != nil && response.BestRun.Result.StrategyEquityCurve != nil {
		t.Fatal("expected sweep result to omit strategy equity curve")
	}
}

func TestMetricsHandlerReturnsPrometheusText(t *testing.T) {
	runQueue := &fakeRunQueue{}
	store := newFakeRunStore()
	metrics := &fakeRunMetrics{
		snapshot: queue.MetricsSnapshot{
			QueueDepth:             3,
			JobsQueued:             5,
			JobsStarted:            4,
			JobsCompleted:          2,
			JobsFailed:             1,
			JobDurationCount:       3,
			JobDurationSecondsSum:  1.25,
			QueueLatencyCount:      4,
			QueueLatencySecondsSum: 0.5,
		},
	}
	router := NewRouterWithMetrics(runQueue, store, metrics)

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	if !strings.Contains(rec.Body.String(), `backtestsim_queue_depth 3`) {
		t.Fatalf("expected queue depth metric, got %q", rec.Body.String())
	}

	if !strings.Contains(rec.Body.String(), `backtestsim_jobs_total{status="completed"} 2`) {
		t.Fatalf("expected completed job metric, got %q", rec.Body.String())
	}
}

type fakeRunQueue struct {
	jobs []queue.Job
	err  error
}

func (q *fakeRunQueue) Enqueue(ctx context.Context, job queue.Job) error {
	if q.err != nil {
		return q.err
	}

	q.jobs = append(q.jobs, job)
	return nil
}

type fakeRunStore struct {
	sweeps   map[string]queue.Sweep
	jobs     map[string]queue.Job
	statuses map[string]queue.JobStatus
	results  map[string]backtest.BacktestResult
	errors   map[string]string
}

func (s *fakeRunStore) SetSweep(ctx context.Context, sweep queue.Sweep) error {
	s.sweeps[sweep.ID] = sweep
	return nil
}

func (s *fakeRunStore) GetSweep(ctx context.Context, sweepID string) (queue.Sweep, error) {
	sweep, ok := s.sweeps[sweepID]
	if !ok {
		return queue.Sweep{}, redis.Nil
	}

	return sweep, nil
}

func (s *fakeRunStore) SetJob(ctx context.Context, job queue.Job) error {
	s.jobs[job.ID] = job
	return nil
}

func (s *fakeRunStore) GetJob(ctx context.Context, jobID string) (queue.Job, error) {
	job, ok := s.jobs[jobID]
	if !ok {
		return queue.Job{}, redis.Nil
	}

	return job, nil
}

func (s *fakeRunStore) SetStatus(ctx context.Context, jobID string, status queue.JobStatus) error {
	s.statuses[jobID] = status
	return nil
}

func (s *fakeRunStore) GetStatus(ctx context.Context, jobID string) (queue.JobStatus, error) {
	status, ok := s.statuses[jobID]
	if !ok {
		return "", redis.Nil
	}

	return status, nil
}

func (s *fakeRunStore) GetResult(ctx context.Context, jobID string, destination any) error {
	result, ok := s.results[jobID]
	if !ok {
		return redis.Nil
	}

	payload, err := json.Marshal(result)
	if err != nil {
		return err
	}

	return json.Unmarshal(payload, destination)
}

func (s *fakeRunStore) SetError(ctx context.Context, jobID string, message string) error {
	s.errors[jobID] = message
	return nil
}

func (s *fakeRunStore) GetError(ctx context.Context, jobID string) (string, error) {
	message, ok := s.errors[jobID]
	if !ok {
		return "", redis.Nil
	}

	return message, nil
}

type fakeRunMetrics struct {
	queued   int
	snapshot queue.MetricsSnapshot
}

func (m *fakeRunMetrics) RecordJobQueued(ctx context.Context) error {
	m.queued++
	return nil
}

func (m *fakeRunMetrics) Snapshot(ctx context.Context) (queue.MetricsSnapshot, error) {
	return m.snapshot, nil
}

func newTestRouter() (*fakeRunQueue, *fakeRunStore, http.Handler) {
	runQueue := &fakeRunQueue{}
	store := newFakeRunStore()

	return runQueue, store, NewRouter(runQueue, store)
}

func newFakeRunStore() *fakeRunStore {
	store := &fakeRunStore{
		sweeps:   make(map[string]queue.Sweep),
		jobs:     make(map[string]queue.Job),
		statuses: make(map[string]queue.JobStatus),
		results:  make(map[string]backtest.BacktestResult),
		errors:   make(map[string]string),
	}

	return store
}

func testRunJob(id string) queue.Job {
	return queue.Job{
		ID:          id,
		Ticker:      "SPY",
		InitialCash: 10000,
		FeeBps:      0,
		SlippageBps: 0,
		ShortWindow: 20,
		LongWindow:  50,
	}
}

func assertAPIFloatEqual(t *testing.T, got float64, want float64) {
	t.Helper()

	const tolerance = 0.000001
	diff := got - want
	if diff < 0 {
		diff = -diff
	}

	if diff > tolerance {
		t.Fatalf("expected %f, got %f", want, got)
	}
}
