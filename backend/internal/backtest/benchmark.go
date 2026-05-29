package backtest

import (
	"backtest-sim/backend/internal/data"
	"fmt"
)

// BuyAndHold simulates buying at the first close and holding through the final candle
func BuyAndHold(candles []data.Candle, initialCash float64) (PortfolioResult, error) {
	if len(candles) == 0 {
		return PortfolioResult{}, fmt.Errorf("candles cannot be empty")
	}

	if initialCash <= 0 {
		return PortfolioResult{}, fmt.Errorf("initial cash must be positive")
	}

	// Validate candle close prices
	for _, candle := range candles {
		if candle.Close <= 0 {
			return PortfolioResult{}, fmt.Errorf("candle close must be positive")
		}
	}

	// BUY at first candle close price
	shares := initialCash / candles[0].Close
	trades := []Trade{
		{
			Date:                candles[0].Date,
			Side:                TradeBuy,
			Price:               candles[0].Close,
			Shares:              shares,
			Fee:                 0,
			CashAfterTrade:      0,
			PortfolioValueAfter: initialCash,
		},
	}

	// HOLD through the whole period and evaluate each day
	equityCurve := make([]EquityPoint, len(candles))
	for i := range candles {
		equityCurve[i] = EquityPoint{
			Date:   candles[i].Date,
			Equity: shares * candles[i].Close,
		}
	}

	finalValue := equityCurve[len(equityCurve)-1].Equity

	return PortfolioResult{
		FinalValue:  finalValue,
		Cash:        0,
		Shares:      shares,
		Trades:      trades,
		EquityCurve: equityCurve,
	}, nil
}
