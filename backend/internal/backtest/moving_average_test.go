package backtest

import (
	"strings"
	"testing"
	"time"

	"backtest-sim/backend/internal/data"
)

func TestSimpleMovingAverage(t *testing.T) {
	values := []float64{10, 20, 30, 40, 50}

	averages, err := SimpleMovingAverage(values, 3)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	want := []float64{0, 0, 20, 30, 40}
	assertFloatSliceEqual(t, averages, want)
}

func TestSimpleMovingAverageRejectsInvalidWindow(t *testing.T) {
	_, err := SimpleMovingAverage([]float64{10, 20, 30}, 0)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !strings.Contains(err.Error(), "window must be positive") {
		t.Fatalf("expected positive window error, got %v", err)
	}
}

func TestSimpleMovingAverageRejectsInsufficientValues(t *testing.T) {
	_, err := SimpleMovingAverage([]float64{10, 20}, 3)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !strings.Contains(err.Error(), "not enough values") {
		t.Fatalf("expected not enough values error, got %v", err)
	}
}

func TestMovingAverageCrossoverSignalsFindsBuyAndSell(t *testing.T) {
	candles := makeCandles(t, []float64{5, 4, 3, 4, 5, 6, 5, 4, 3})

	signals, err := MovingAverageCrossoverSignals(candles, 2, 3)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(signals) != 2 {
		t.Fatalf("expected 2 signals, got %d", len(signals))
	}

	assertSignal(t, signals[0], SignalBuy, "2020-01-05")
	assertSignal(t, signals[1], SignalSell, "2020-01-08")
}

func TestMovingAverageCrossoverSignalsReturnsNoSignalsWhenNoCrossover(t *testing.T) {
	candles := makeCandles(t, []float64{1, 2, 3, 4, 5})

	signals, err := MovingAverageCrossoverSignals(candles, 2, 3)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(signals) != 0 {
		t.Fatalf("expected no signals, got %d", len(signals))
	}
}

func TestMovingAverageCrossoverSignalsRejectsEmptyCandles(t *testing.T) {
	_, err := MovingAverageCrossoverSignals(nil, 2, 3)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !strings.Contains(err.Error(), "candles cannot be empty") {
		t.Fatalf("expected empty candles error, got %v", err)
	}
}

func TestMovingAverageCrossoverSignalsRejectsInvalidWindows(t *testing.T) {
	candles := makeCandles(t, []float64{1, 2, 3, 4})

	tests := []struct {
		name        string
		shortWindow int
		longWindow  int
		wantError   string
	}{
		{
			name:        "zero short window",
			shortWindow: 0,
			longWindow:  3,
			wantError:   "windows must be positive",
		},
		{
			name:        "short window equals long window",
			shortWindow: 3,
			longWindow:  3,
			wantError:   "short window must be less than long window",
		},
		{
			name:        "short window greater than long window",
			shortWindow: 4,
			longWindow:  3,
			wantError:   "short window must be less than long window",
		},
		{
			name:        "not enough candles",
			shortWindow: 2,
			longWindow:  5,
			wantError:   "not enough candles",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := MovingAverageCrossoverSignals(candles, tt.shortWindow, tt.longWindow)
			if err == nil {
				t.Fatal("expected error, got nil")
			}

			if !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("expected error containing %q, got %v", tt.wantError, err)
			}
		})
	}
}

func makeCandles(t *testing.T, closes []float64) []data.Candle {
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

func assertSignal(t *testing.T, signal Signal, signalType SignalType, date string) {
	t.Helper()

	if signal.Type != signalType {
		t.Fatalf("expected signal type %s, got %s", signalType, signal.Type)
	}

	if got := signal.Date.Format("2006-01-02"); got != date {
		t.Fatalf("expected signal date %s, got %s", date, got)
	}
}

func assertFloatSliceEqual(t *testing.T, got []float64, want []float64) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("expected %d values, got %d", len(want), len(got))
	}

	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("expected value at index %d to be %f, got %f", i, want[i], got[i])
		}
	}
}
