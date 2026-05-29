package backtest

import (
	"strings"
	"testing"
	"time"
)

func TestTotalReturn(t *testing.T) {
	tests := []struct {
		name         string
		initialValue float64
		finalValue   float64
		want         float64
	}{
		{
			name:         "gain",
			initialValue: 10000,
			finalValue:   12000,
			want:         0.2,
		},
		{
			name:         "loss",
			initialValue: 10000,
			finalValue:   8000,
			want:         -0.2,
		},
		{
			name:         "flat",
			initialValue: 10000,
			finalValue:   10000,
			want:         0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := TotalReturn(tt.initialValue, tt.finalValue)
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}

			assertFloatEqual(t, got, tt.want)
		})
	}
}

func TestTotalReturnRejectsNonpositiveInitialValue(t *testing.T) {
	_, err := TotalReturn(0, 12000)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !strings.Contains(err.Error(), "initial value must be positive") {
		t.Fatalf("expected initial value error, got %v", err)
	}
}

func TestMaxDrawdown(t *testing.T) {
	curve := makeEquityCurve(t, []float64{10000, 12000, 9000, 11000, 8000, 13000})

	got, err := MaxDrawdown(curve)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	assertFloatEqual(t, got, 0.3333333333333333)
}

func TestMaxDrawdownReturnsZeroForRisingCurve(t *testing.T) {
	curve := makeEquityCurve(t, []float64{10000, 11000, 12000})

	got, err := MaxDrawdown(curve)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	assertFloatEqual(t, got, 0)
}

func TestMaxDrawdownRejectsInvalidEquityCurve(t *testing.T) {
	tests := []struct {
		name      string
		values    []float64
		wantError string
	}{
		{
			name:      "empty curve",
			values:    nil,
			wantError: "equity curve cannot be empty",
		},
		{
			name:      "zero first equity",
			values:    []float64{0, 10000},
			wantError: "equity must be positive",
		},
		{
			name:      "negative later equity",
			values:    []float64{10000, -1},
			wantError: "equity must be positive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var curve []EquityPoint
			if tt.values != nil {
				curve = makeEquityCurve(t, tt.values)
			}

			_, err := MaxDrawdown(curve)
			if err == nil {
				t.Fatal("expected error, got nil")
			}

			if !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("expected error containing %q, got %v", tt.wantError, err)
			}
		})
	}
}

func TestWinRate(t *testing.T) {
	trades := []Trade{
		{Side: TradeBuy, Price: 100, Shares: 10},
		{Side: TradeSell, Price: 120, Shares: 10},
		{Side: TradeBuy, Price: 130, Shares: 10},
		{Side: TradeSell, Price: 110, Shares: 10},
	}

	got := WinRate(trades)

	assertFloatEqual(t, got, 0.5)
}

func TestWinRateReturnsZeroWhenThereAreNoCompletedTrades(t *testing.T) {
	tests := []struct {
		name   string
		trades []Trade
	}{
		{
			name:   "no trades",
			trades: nil,
		},
		{
			name: "open position",
			trades: []Trade{
				{Side: TradeBuy, Price: 100, Shares: 10},
			},
		},
		{
			name: "sell without buy",
			trades: []Trade{
				{Side: TradeSell, Price: 120, Shares: 10},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := WinRate(tt.trades)
			assertFloatEqual(t, got, 0)
		})
	}
}

func TestWinRateIgnoresOpenPositionAfterCompletedTrade(t *testing.T) {
	trades := []Trade{
		{Side: TradeBuy, Price: 100, Shares: 10},
		{Side: TradeSell, Price: 120, Shares: 10},
		{Side: TradeBuy, Price: 130, Shares: 10},
	}

	got := WinRate(trades)

	assertFloatEqual(t, got, 1)
}

func makeEquityCurve(t *testing.T, values []float64) []EquityPoint {
	t.Helper()

	start, err := time.Parse("2006-01-02", "2020-01-01")
	if err != nil {
		t.Fatalf("parse start date: %v", err)
	}

	curve := make([]EquityPoint, len(values))
	for i, value := range values {
		curve[i] = EquityPoint{
			Date:   start.AddDate(0, 0, i),
			Equity: value,
		}
	}

	return curve
}
