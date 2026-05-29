package backtest

import (
	"strings"
	"testing"
)

func TestRunMovingAverageBacktest(t *testing.T) {
	candles := makeCandles(t, []float64{5, 4, 3, 4, 5, 6, 5, 4, 3})
	config := BacktestConfig{
		InitialCash: 10000,
		ShortWindow: 2,
		LongWindow:  3,
	}

	result, err := RunMovingAverageBacktest(candles, config)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if result.Config != config {
		t.Fatalf("expected result config to match input config")
	}

	if len(result.Signals) != 2 {
		t.Fatalf("expected 2 signals, got %d", len(result.Signals))
	}

	assertSignal(t, result.Signals[0], SignalBuy, "2020-01-05")
	assertSignal(t, result.Signals[1], SignalSell, "2020-01-08")

	assertFloatEqual(t, result.Strategy.FinalValue, 8000)
	assertFloatEqual(t, result.Benchmark.FinalValue, 6000)
	assertFloatEqual(t, result.TotalReturn, -0.2)
	assertFloatEqual(t, result.BenchmarkReturn, -0.4)
	assertFloatEqual(t, result.ExcessReturn, 0.2)
	assertFloatEqual(t, result.MaxDrawdown, 1.0/3.0)
	assertFloatEqual(t, result.WinRate, 0)

	if result.NumberOfTrades != 2 {
		t.Fatalf("expected 2 strategy trades, got %d", result.NumberOfTrades)
	}
}

func TestRunMovingAverageBacktestPropagatesStrategyValidationError(t *testing.T) {
	candles := makeCandles(t, []float64{1, 2, 3, 4})
	config := BacktestConfig{
		InitialCash: 10000,
		ShortWindow: 3,
		LongWindow:  3,
	}

	_, err := RunMovingAverageBacktest(candles, config)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !strings.Contains(err.Error(), "short window must be less than long window") {
		t.Fatalf("expected short window validation error, got %v", err)
	}
}

func TestRunMovingAverageBacktestPropagatesPortfolioValidationError(t *testing.T) {
	candles := makeCandles(t, []float64{5, 4, 3, 4, 5})
	config := BacktestConfig{
		InitialCash: 0,
		ShortWindow: 2,
		LongWindow:  3,
	}

	_, err := RunMovingAverageBacktest(candles, config)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !strings.Contains(err.Error(), "initial cash must be positive") {
		t.Fatalf("expected initial cash validation error, got %v", err)
	}
}
