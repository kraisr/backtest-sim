package api

import "net/http"

// NewRouter registers API routes and returns an HTTP handler for the server
func NewRouter(jobs RunQueue, store RunStore) http.Handler {
	mux := http.NewServeMux()

	// Health endpoint used to confirm the API process is running
	mux.HandleFunc("/health", HealthHandler)
	// Tickers endpoint used to get the supported market data symbols
	mux.HandleFunc("/api/tickers", TickersHandler)
	// Strategies endpoint used to get the supported trading strategies
	mux.HandleFunc("/api/strategies", StrategiesHandler)
	// Runs collection endpoint used to create asynchronous runs
	mux.HandleFunc("/api/runs", RunsHandler(jobs, store))
	// Run detail endpoint used to poll status and retrieve results
	mux.HandleFunc("/api/runs/", RunStatusHandler(store))

	return mux
}
