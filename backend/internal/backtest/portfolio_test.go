package backtest

import (
	"strings"
	"testing"
)

func TestSimulatePortfolioNoSignalsKeepsInitialCash(t *testing.T) {
	candles := makeCandles(t, []float64{100, 110, 120})
	config := PortfolioConfig{InitialCash: 10000}

	result, err := SimulatePortfolio(candles, nil, config)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	assertFloatEqual(t, result.FinalValue, 10000)
	assertFloatEqual(t, result.Cash, 10000)
	assertFloatEqual(t, result.Shares, 0)

	if len(result.Trades) != 0 {
		t.Fatalf("expected no trades, got %d", len(result.Trades))
	}

	if len(result.EquityCurve) != len(candles) {
		t.Fatalf("expected %d equity points, got %d", len(candles), len(result.EquityCurve))
	}
}

func TestSimulatePortfolioBuysWithAvailableCash(t *testing.T) {
	candles := makeCandles(t, []float64{100, 120})
	signals := []Signal{
		{Date: candles[0].Date, Type: SignalBuy},
	}
	config := PortfolioConfig{InitialCash: 10000}

	result, err := SimulatePortfolio(candles, signals, config)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(result.Trades) != 1 {
		t.Fatalf("expected 1 trade, got %d", len(result.Trades))
	}

	trade := result.Trades[0]
	if trade.Side != TradeBuy {
		t.Fatalf("expected BUY trade, got %s", trade.Side)
	}

	assertFloatEqual(t, trade.Price, 100)
	assertFloatEqual(t, trade.Shares, 100)
	assertFloatEqual(t, trade.CashAfterTrade, 0)
	assertFloatEqual(t, result.FinalValue, 12000)
}

func TestSimulatePortfolioBuyThenSell(t *testing.T) {
	candles := makeCandles(t, []float64{100, 120, 150})
	signals := []Signal{
		{Date: candles[0].Date, Type: SignalBuy},
		{Date: candles[1].Date, Type: SignalSell},
	}
	config := PortfolioConfig{InitialCash: 10000}

	result, err := SimulatePortfolio(candles, signals, config)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(result.Trades) != 2 {
		t.Fatalf("expected 2 trades, got %d", len(result.Trades))
	}

	if result.Trades[0].Side != TradeBuy {
		t.Fatalf("expected first trade to be BUY, got %s", result.Trades[0].Side)
	}

	if result.Trades[1].Side != TradeSell {
		t.Fatalf("expected second trade to be SELL, got %s", result.Trades[1].Side)
	}

	assertFloatEqual(t, result.FinalValue, 12000)
	assertFloatEqual(t, result.Cash, 12000)
	assertFloatEqual(t, result.Shares, 0)
}

func TestSimulatePortfolioIgnoresInvalidPositionActions(t *testing.T) {
	candles := makeCandles(t, []float64{100, 110, 120, 130})
	signals := []Signal{
		{Date: candles[0].Date, Type: SignalSell},
		{Date: candles[1].Date, Type: SignalBuy},
		{Date: candles[2].Date, Type: SignalBuy},
	}
	config := PortfolioConfig{InitialCash: 10000}

	result, err := SimulatePortfolio(candles, signals, config)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(result.Trades) != 1 {
		t.Fatalf("expected only 1 valid trade, got %d", len(result.Trades))
	}

	assertFloatEqual(t, result.Trades[0].Price, 110)
	assertFloatEqual(t, result.FinalValue, 10000.0/110*130)
}

func TestSimulatePortfolioAppliesFeesAndSlippage(t *testing.T) {
	candles := makeCandles(t, []float64{100, 120})
	signals := []Signal{
		{Date: candles[0].Date, Type: SignalBuy},
		{Date: candles[1].Date, Type: SignalSell},
	}
	config := PortfolioConfig{
		InitialCash: 10000,
		FeeBps:      100,
		SlippageBps: 100,
	}

	result, err := SimulatePortfolio(candles, signals, config)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(result.Trades) != 2 {
		t.Fatalf("expected 2 trades, got %d", len(result.Trades))
	}

	assertFloatEqual(t, result.Trades[0].Price, 101)
	assertFloatEqual(t, result.Trades[0].Shares, 10000/(101*1.01))
	assertFloatEqual(t, result.Trades[1].Price, 118.8)

	if result.FinalValue >= 12000 {
		t.Fatalf("expected fees and slippage to reduce final value below 12000, got %f", result.FinalValue)
	}
}

func TestSimulatePortfolioRejectsInvalidInputs(t *testing.T) {
	validCandles := makeCandles(t, []float64{100, 110})

	tests := []struct {
		name      string
		candles   []float64
		config    PortfolioConfig
		wantError string
	}{
		{
			name:      "empty candles",
			candles:   nil,
			config:    PortfolioConfig{InitialCash: 10000},
			wantError: "candles cannot be empty",
		},
		{
			name:      "nonpositive initial cash",
			candles:   []float64{100, 110},
			config:    PortfolioConfig{InitialCash: 0},
			wantError: "initial cash must be positive",
		},
		{
			name:      "negative fee",
			candles:   []float64{100, 110},
			config:    PortfolioConfig{InitialCash: 10000, FeeBps: -1},
			wantError: "fee bps cannot be negative",
		},
		{
			name:      "negative slippage",
			candles:   []float64{100, 110},
			config:    PortfolioConfig{InitialCash: 10000, SlippageBps: -1},
			wantError: "slippage bps cannot be negative",
		},
		{
			name:      "nonpositive close",
			candles:   []float64{100, 0},
			config:    PortfolioConfig{InitialCash: 10000},
			wantError: "candle close must be positive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			candles := validCandles
			if tt.candles != nil {
				candles = makeCandles(t, tt.candles)
			} else {
				candles = nil
			}

			_, err := SimulatePortfolio(candles, nil, tt.config)
			if err == nil {
				t.Fatal("expected error, got nil")
			}

			if !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("expected error containing %q, got %v", tt.wantError, err)
			}
		})
	}
}

func assertFloatEqual(t *testing.T, got float64, want float64) {
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
