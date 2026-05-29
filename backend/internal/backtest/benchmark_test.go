package backtest

import (
	"strings"
	"testing"
	"time"

	"backtest-sim/backend/internal/data"
)

func TestBuyAndHold(t *testing.T) {
	candles := makeBenchmarkCandles(t, []float64{100, 110, 90, 150})

	result, err := BuyAndHold(candles, 10000)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	assertFloatEqual(t, result.FinalValue, 15000)
	assertFloatEqual(t, result.Cash, 0)
	assertFloatEqual(t, result.Shares, 100)

	if len(result.Trades) != 1 {
		t.Fatalf("expected 1 trade, got %d", len(result.Trades))
	}

	trade := result.Trades[0]
	if trade.Side != TradeBuy {
		t.Fatalf("expected BUY trade, got %s", trade.Side)
	}

	assertFloatEqual(t, trade.Price, 100)
	assertFloatEqual(t, trade.Shares, 100)
	assertFloatEqual(t, trade.Fee, 0)
	assertFloatEqual(t, trade.CashAfterTrade, 0)
	assertFloatEqual(t, trade.PortfolioValueAfter, 10000)

	wantEquity := []float64{10000, 11000, 9000, 15000}
	if len(result.EquityCurve) != len(wantEquity) {
		t.Fatalf("expected %d equity points, got %d", len(wantEquity), len(result.EquityCurve))
	}

	for i, want := range wantEquity {
		assertFloatEqual(t, result.EquityCurve[i].Equity, want)
		if !result.EquityCurve[i].Date.Equal(candles[i].Date) {
			t.Fatalf("expected equity point %d date to match candle date", i)
		}
	}
}

func TestBuyAndHoldSingleCandle(t *testing.T) {
	candles := makeBenchmarkCandles(t, []float64{100})

	result, err := BuyAndHold(candles, 10000)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	assertFloatEqual(t, result.FinalValue, 10000)
	assertFloatEqual(t, result.Shares, 100)

	if len(result.Trades) != 1 {
		t.Fatalf("expected 1 trade, got %d", len(result.Trades))
	}

	if len(result.EquityCurve) != 1 {
		t.Fatalf("expected 1 equity point, got %d", len(result.EquityCurve))
	}
}

func TestBuyAndHoldRejectsInvalidInputs(t *testing.T) {
	tests := []struct {
		name        string
		closes      []float64
		initialCash float64
		wantError   string
	}{
		{
			name:        "empty candles",
			closes:      nil,
			initialCash: 10000,
			wantError:   "candles cannot be empty",
		},
		{
			name:        "zero initial cash",
			closes:      []float64{100, 110},
			initialCash: 0,
			wantError:   "initial cash must be positive",
		},
		{
			name:        "negative initial cash",
			closes:      []float64{100, 110},
			initialCash: -1,
			wantError:   "initial cash must be positive",
		},
		{
			name:        "zero close",
			closes:      []float64{100, 0},
			initialCash: 10000,
			wantError:   "candle close must be positive",
		},
		{
			name:        "negative close",
			closes:      []float64{-100, 110},
			initialCash: 10000,
			wantError:   "candle close must be positive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var candles []data.Candle
			if tt.closes != nil {
				candles = makeBenchmarkCandles(t, tt.closes)
			}

			_, err := BuyAndHold(candles, tt.initialCash)
			if err == nil {
				t.Fatal("expected error, got nil")
			}

			if !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("expected error containing %q, got %v", tt.wantError, err)
			}
		})
	}
}

func makeBenchmarkCandles(t *testing.T, closes []float64) []data.Candle {
	t.Helper()

	start, err := time.Parse("2006-01-02", "2020-01-01")
	if err != nil {
		t.Fatalf("parse start date: %v", err)
	}

	candles := make([]data.Candle, len(closes))
	for i, closePrice := range closes {
		candles[i] = data.Candle{
			Date:   start.AddDate(0, 0, i),
			Open:   closePrice,
			High:   closePrice,
			Low:    closePrice,
			Close:  closePrice,
			Volume: 1000,
		}
	}

	return candles
}
