package api

import "net/http"

const defaultDataDir = "../data"

// NewRouter registers API routes and returns an HTTP handler for the server
func NewRouter() http.Handler {
	return NewRouterWithDataDir(defaultDataDir)
}

// NewRouterWithDataDir registers API routes using the provided market data directory
func NewRouterWithDataDir(dataDir string) http.Handler {
	mux := http.NewServeMux()

	// Health endpoint used to confirm the API process is running
	mux.HandleFunc("/health", HealthHandler)
	// Tickers endpoint used to get the supported market data symbols
	mux.HandleFunc("/api/tickers", TickersHandler)
	// Strategies endpoint used to get the supported trading strategies
	mux.HandleFunc("/api/strategies", StrategiesHandler)
	// Runs endpoint used to generate a report by running a simulation
	mux.HandleFunc("/api/runs", RunsHandler(dataDir))

	return mux
}
