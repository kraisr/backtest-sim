package api

import "net/http"

// NewRouter registers API routes and returns an HTTP handler for the server
func NewRouter() http.Handler {
	mux := http.NewServeMux()

	// Health endpoint used to confirm the API process is running
	mux.HandleFunc("/health", HealthHandler)
	// Tickers endpoint used to get the supported market data symbols
	mux.HandleFunc("/api/tickers", TickersHandler)
	// Strategies endpoint used to get the supported trading strategies
	mux.HandleFunc("/api/strategies", StrategiesHandler)

	return mux
}
