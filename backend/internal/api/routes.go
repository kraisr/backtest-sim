package api

import "net/http"

// NewRouter registers API routes and returns an HTTP handler for the server
func NewRouter(jobs RunQueue, store RunStore) http.Handler {
	return NewRouterWithMetrics(jobs, store, noopRunMetrics{})
}

// NewRouterWithMetrics registers API routes with queue metrics enabled
func NewRouterWithMetrics(jobs RunQueue, store RunStore, metrics RunMetrics) http.Handler {
	mux := http.NewServeMux()

	// Health endpoint used to confirm the API process is running
	mux.HandleFunc("/health", HealthHandler)
	// Metrics endpoint used by Prometheus or local benchmark checks
	mux.HandleFunc("/metrics", MetricsHandler(metrics))
	// Tickers endpoint used to get the supported market data symbols
	mux.HandleFunc("/api/tickers", TickersHandler)
	// Strategies endpoint used to get the supported trading strategies
	mux.HandleFunc("/api/strategies", StrategiesHandler)
	// Runs collection endpoint used to create asynchronous runs
	mux.HandleFunc("/api/runs", RunsHandler(jobs, store, metrics))
	// Run detail endpoint used to poll status and retrieve results
	mux.HandleFunc("/api/runs/", RunStatusHandler(store))
	// Sweeps collection endpoint used to create parameter search batches
	mux.HandleFunc("/api/sweeps", SweepsHandler(jobs, store, metrics))
	// Sweep detail endpoint used to poll aggregate and per-run status
	mux.HandleFunc("/api/sweeps/", SweepStatusHandler(store))

	return mux
}
