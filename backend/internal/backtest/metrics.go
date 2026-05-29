package backtest

import "fmt"

// TotalReturn returns the overall portfolio return as a decimal
// Use this to compare the starting portfolio value to the final portfolio value
func TotalReturn(initialValue, finalValue float64) (float64, error) {
	if initialValue <= 0 {
		return 0, fmt.Errorf("initial value must be positive")
	}

	return (finalValue - initialValue) / initialValue, nil
}

// MaxDrawdown returns the largest percentage drop from a previous equity peak
// Use this to measure the worst peak-to-trough decline during a backtest
func MaxDrawdown(equityCurve []EquityPoint) (float64, error) {
	if len(equityCurve) == 0 {
		return 0, fmt.Errorf("equity curve cannot be empty")
	}

	peak := equityCurve[0].Equity
	if peak <= 0 {
		return 0, fmt.Errorf("equity must be positive")
	}

	var maxDrawdown float64
	for _, point := range equityCurve {
		if point.Equity <= 0 {
			return 0, fmt.Errorf("equity must be positive")
		}

		// Update the running high-water mark before measuring the drop
		if point.Equity > peak {
			peak = point.Equity
		}

		drawdown := (peak - point.Equity) / peak
		if drawdown > maxDrawdown {
			maxDrawdown = drawdown
		}
	}

	return maxDrawdown, nil
}

// WinRate returns the fraction of completed round-trip trades that were profitable
// Use this with the trade log from SimulatePortfolio; open positions are ignored
func WinRate(trades []Trade) float64 {
	var entry Trade
	var hasEntry bool
	var wins int
	var completed int

	for i := range trades {
		trade := trades[i]

		if trade.Side == TradeBuy {
			entry = trade
			hasEntry = true
			continue
		}

		if trade.Side == TradeSell && hasEntry {
			completed++

			// Pair each SELL with the most recent unmatched BUY
			buyCost := entry.Price * entry.Shares
			sellValue := trade.Price * trade.Shares

			if sellValue > buyCost {
				wins++
			}

			hasEntry = false
		}
	}

	if completed == 0 {
		return 0
	}

	return float64(wins) / float64(completed)
}
