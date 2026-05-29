package api

import (
	"encoding/json"
	"net/http"
)

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

// writeJSON writes a JSON response with the provided HTTP status code
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(value); err != nil {
		http.Error(w, "encode json response", http.StatusInternalServerError)
	}
}
