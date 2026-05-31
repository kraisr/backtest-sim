package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

type sweepRequest struct {
	Ticker       string  `json:"ticker"`
	InitialCash  float64 `json:"initial_cash"`
	FeeBps       float64 `json:"fee_bps"`
	SlippageBps  float64 `json:"slippage_bps"`
	ShortWindows []int   `json:"short_windows"`
	LongWindows  []int   `json:"long_windows"`
}

type createSweepResponse struct {
	ID        string `json:"id"`
	Status    string `json:"status"`
	RunCount  int    `json:"run_count"`
	StatusURL string `json:"status_url"`
}

type sweepStatusResponse struct {
	ID        string `json:"id"`
	Status    string `json:"status"`
	RunCount  int    `json:"run_count"`
	Queued    int    `json:"queued"`
	Running   int    `json:"running"`
	Completed int    `json:"completed"`
	Failed    int    `json:"failed"`
}

type apiError struct {
	Error string `json:"error"`
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	apiBaseURL := flag.String("api", "http://localhost:8080", "API base URL")
	ticker := flag.String("ticker", "SPY", "ticker symbol")
	initialCash := flag.Float64("cash", 10000, "starting portfolio cash")
	feeBps := flag.Float64("fee-bps", 0, "transaction fee in basis points")
	slippageBps := flag.Float64("slippage-bps", 0, "execution slippage in basis points")
	shortWindowsCSV := flag.String("shorts", "5,10,20,30", "comma-separated short moving average windows")
	longWindowsCSV := flag.String("longs", "50,100,150,200", "comma-separated long moving average windows")
	pollInterval := flag.Duration("poll-interval", 500*time.Millisecond, "how often to poll sweep status")
	timeout := flag.Duration("timeout", 2*time.Minute, "maximum time to wait for sweep completion")
	flag.Parse()

	shortWindows, err := parseWindows(*shortWindowsCSV)
	if err != nil {
		return fmt.Errorf("parse shorts: %w", err)
	}

	longWindows, err := parseWindows(*longWindowsCSV)
	if err != nil {
		return fmt.Errorf("parse longs: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	request := sweepRequest{
		Ticker:       *ticker,
		InitialCash:  *initialCash,
		FeeBps:       *feeBps,
		SlippageBps:  *slippageBps,
		ShortWindows: shortWindows,
		LongWindows:  longWindows,
	}

	// Submit the sweep and measure how quickly the API accepts queued work
	submitStartedAt := time.Now()
	sweep, err := createSweep(ctx, client, *apiBaseURL, request)
	if err != nil {
		return err
	}
	submitDuration := time.Since(submitStartedAt)

	// Poll the aggregate sweep endpoint until all child runs finish
	runStartedAt := time.Now()
	status, err := waitForSweep(ctx, client, *apiBaseURL, sweep.StatusURL, *pollInterval)
	if err != nil {
		return err
	}
	runDuration := time.Since(runStartedAt)

	printSummary(sweep, status, submitDuration, runDuration)
	return nil
}

func createSweep(ctx context.Context, client *http.Client, apiBaseURL string, request sweepRequest) (createSweepResponse, error) {
	body, err := json.Marshal(request)
	if err != nil {
		return createSweepResponse{}, fmt.Errorf("marshal sweep request: %w", err)
	}

	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, apiBaseURL+"/api/sweeps", bytes.NewReader(body))
	if err != nil {
		return createSweepResponse{}, fmt.Errorf("build sweep request: %w", err)
	}
	httpRequest.Header.Set("Content-Type", "application/json")

	var response createSweepResponse
	if err := doJSON(client, httpRequest, &response); err != nil {
		return createSweepResponse{}, err
	}

	return response, nil
}

func waitForSweep(ctx context.Context, client *http.Client, apiBaseURL string, statusURL string, interval time.Duration) (sweepStatusResponse, error) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		status, err := getSweepStatus(ctx, client, apiBaseURL, statusURL)
		if err != nil {
			return sweepStatusResponse{}, err
		}

		if status.Status == "completed" || status.Status == "failed" {
			return status, nil
		}

		select {
		case <-ctx.Done():
			return sweepStatusResponse{}, fmt.Errorf("wait for sweep: %w", ctx.Err())
		case <-ticker.C:
		}
	}
}

func getSweepStatus(ctx context.Context, client *http.Client, apiBaseURL string, statusURL string) (sweepStatusResponse, error) {
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodGet, apiBaseURL+statusURL, nil)
	if err != nil {
		return sweepStatusResponse{}, fmt.Errorf("build status request: %w", err)
	}

	var response sweepStatusResponse
	if err := doJSON(client, httpRequest, &response); err != nil {
		return sweepStatusResponse{}, err
	}

	return response, nil
}

func doJSON(client *http.Client, request *http.Request, destination any) error {
	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		var apiErr apiError
		if err := json.Unmarshal(body, &apiErr); err == nil && apiErr.Error != "" {
			return errors.New(apiErr.Error)
		}

		return fmt.Errorf("request failed with status %d", response.StatusCode)
	}

	if err := json.Unmarshal(body, destination); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	return nil
}

func parseWindows(value string) ([]int, error) {
	parts := strings.Split(value, ",")
	windows := make([]int, 0, len(parts))

	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}

		window, err := strconv.Atoi(trimmed)
		if err != nil {
			return nil, fmt.Errorf("invalid window %q: %w", trimmed, err)
		}

		if window <= 0 {
			return nil, fmt.Errorf("window must be positive")
		}

		windows = append(windows, window)
	}

	if len(windows) == 0 {
		return nil, fmt.Errorf("at least one window is required")
	}

	return windows, nil
}

func printSummary(sweep createSweepResponse, status sweepStatusResponse, submitDuration time.Duration, runDuration time.Duration) {
	jobsPerSecond := float64(status.Completed+status.Failed) / runDuration.Seconds()

	fmt.Println("BacktestSim Sweep Benchmark")
	fmt.Println("===========================")
	fmt.Println()
	fmt.Printf("Sweep ID:         %s\n", sweep.ID)
	fmt.Printf("Runs submitted:   %d\n", sweep.RunCount)
	fmt.Printf("Runs completed:   %d\n", status.Completed)
	fmt.Printf("Runs failed:      %d\n", status.Failed)
	fmt.Printf("API submit time:  %s\n", submitDuration.Round(time.Millisecond))
	fmt.Printf("Worker time:      %s\n", runDuration.Round(time.Millisecond))
	fmt.Printf("Throughput:       %.2f jobs/sec\n", jobsPerSecond)
}
