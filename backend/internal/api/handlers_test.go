package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHealthHandler(t *testing.T) {
	router := NewRouter()

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
	router := NewRouter()

	req := httptest.NewRequest(http.MethodPost, "/health", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}

func TestTickersHandler(t *testing.T) {
	router := NewRouter()

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
	router := NewRouter()

	req := httptest.NewRequest(http.MethodPost, "/api/tickers", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}

func TestStrategiesHandler(t *testing.T) {
	router := NewRouter()

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
	router := NewRouter()

	req := httptest.NewRequest(http.MethodPost, "/api/strategies", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}

func TestRunsHandler(t *testing.T) {
	dataDir := writeRunTestData(t)
	router := NewRouterWithDataDir(dataDir)

	body := `{
		"ticker": "SPY",
		"initial_cash": 10000,
		"fee_bps": 0,
		"slippage_bps": 0,
		"short_window": 2,
		"long_window": 3
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/runs", strings.NewReader(body))
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d with body %q", http.StatusOK, rec.Code, rec.Body.String())
	}

	if contentType := rec.Header().Get("Content-Type"); contentType != "application/json" {
		t.Fatalf("expected content type application/json, got %q", contentType)
	}

	var response runResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode run response: %v", err)
	}

	if response.Strategy != "moving_average_crossover" {
		t.Fatalf("expected moving_average_crossover strategy, got %q", response.Strategy)
	}

	if response.Ticker != "SPY" {
		t.Fatalf("expected ticker SPY, got %q", response.Ticker)
	}

	assertAPIFloatEqual(t, response.StrategyFinalValue, 8000)
	assertAPIFloatEqual(t, response.StrategyReturn, -0.2)
	assertAPIFloatEqual(t, response.BenchmarkFinalValue, 6000)
	assertAPIFloatEqual(t, response.BenchmarkReturn, -0.4)
	assertAPIFloatEqual(t, response.ExcessReturn, 0.2)
	assertAPIFloatEqual(t, response.MaxDrawdown, 1.0/3.0)
	assertAPIFloatEqual(t, response.WinRate, 0)

	if response.Signals != 2 {
		t.Fatalf("expected 2 signals, got %d", response.Signals)
	}

	if response.Trades != 2 {
		t.Fatalf("expected 2 trades, got %d", response.Trades)
	}
}

func TestRunsHandlerRejectsUnsupportedMethod(t *testing.T) {
	router := NewRouterWithDataDir(writeRunTestData(t))

	req := httptest.NewRequest(http.MethodGet, "/api/runs", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}

func TestRunsHandlerRejectsInvalidJSON(t *testing.T) {
	router := NewRouterWithDataDir(writeRunTestData(t))

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
	router := NewRouterWithDataDir(writeRunTestData(t))

	body := `{
		"ticker": "AAPL",
		"initial_cash": 10000,
		"short_window": 2,
		"long_window": 3
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
	router := NewRouterWithDataDir(writeRunTestData(t))

	body := `{
		"ticker": "SPY",
		"initial_cash": 10000,
		"short_window": 3,
		"long_window": 3
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

func writeRunTestData(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "SPY.csv")
	content := `date,open,high,low,close,volume
2020-01-01,5,5,5,5,1000
2020-01-02,4,4,4,4,1000
2020-01-03,3,3,3,3,1000
2020-01-04,4,4,4,4,1000
2020-01-05,5,5,5,5,1000
2020-01-06,6,6,6,6,1000
2020-01-07,5,5,5,5,1000
2020-01-08,4,4,4,4,1000
2020-01-09,3,3,3,3,1000
`

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write test market data: %v", err)
	}

	return dir
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
