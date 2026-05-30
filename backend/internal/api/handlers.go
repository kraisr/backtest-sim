package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"

	"backtest-sim/backend/internal/backtest"
	"backtest-sim/backend/internal/data"
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

type runRequest struct {
	Ticker      string  `json:"ticker"`
	InitialCash float64 `json:"initial_cash"`
	FeeBps      float64 `json:"fee_bps"`
	SlippageBps float64 `json:"slippage_bps"`
	ShortWindow int     `json:"short_window"`
	LongWindow  int     `json:"long_window"`
}

type runResponse struct {
	Strategy            string  `json:"strategy"`
	Ticker              string  `json:"ticker"`
	InitialCash         float64 `json:"initial_cash"`
	FeeBps              float64 `json:"fee_bps"`
	SlippageBps         float64 `json:"slippage_bps"`
	ShortWindow         int     `json:"short_window"`
	LongWindow          int     `json:"long_window"`
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

// RunsHandler runs a synchronous moving average backtest and returns summary metrics
func RunsHandler(dataDir string) http.HandlerFunc {
	// dataDir is captured by this returned handler, so tests can use temp data
	// while the real server uses the default ../data directory
	return func(w http.ResponseWriter, r *http.Request) {
		// This endpoint creates a run, so only POST requests are accepted
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Decode the JSON request body into a Go struct
		var request runRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			writeErrorJSON(w, http.StatusBadRequest, "invalid json request body")
			return
		}

		// Normalize user input so "spy" and "SPY" behave the same
		ticker := strings.ToUpper(request.Ticker)
		csvPath, err := tickerCSVPath(dataDir, ticker)
		if err != nil {
			writeErrorJSON(w, http.StatusBadRequest, err.Error())
			return
		}

		// Load the static market data before running the backtest engine
		candles, err := data.LoadCSV(csvPath)
		if err != nil {
			writeErrorJSON(w, http.StatusInternalServerError, "load market data")
			return
		}

		// Convert the API request into the engine config type
		config := backtest.BacktestConfig{
			InitialCash: request.InitialCash,
			FeeBps:      request.FeeBps,
			SlippageBps: request.SlippageBps,
			ShortWindow: request.ShortWindow,
			LongWindow:  request.LongWindow,
		}

		// Run the existing synchronous local engine
		result, err := backtest.RunMovingAverageBacktest(candles, config)
		if err != nil {
			writeErrorJSON(w, http.StatusBadRequest, err.Error())
			return
		}

		// Return a compact API response instead of exposing the full internal result
		writeJSON(w, http.StatusOK, runResponse{
			Strategy:            "moving_average_crossover",
			Ticker:              ticker,
			InitialCash:         config.InitialCash,
			FeeBps:              config.FeeBps,
			SlippageBps:         config.SlippageBps,
			ShortWindow:         config.ShortWindow,
			LongWindow:          config.LongWindow,
			StrategyFinalValue:  result.Strategy.FinalValue,
			StrategyReturn:      result.TotalReturn,
			BenchmarkFinalValue: result.Benchmark.FinalValue,
			BenchmarkReturn:     result.BenchmarkReturn,
			ExcessReturn:        result.ExcessReturn,
			MaxDrawdown:         result.MaxDrawdown,
			WinRate:             result.WinRate,
			Signals:             len(result.Signals),
			Trades:              result.NumberOfTrades,
		})
	}
}

func tickerCSVPath(dataDir string, ticker string) (string, error) {
	if ticker != "SPY" {
		return "", fmt.Errorf("unsupported ticker %q", ticker)
	}

	return filepath.Join(dataDir, ticker+".csv"), nil
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
