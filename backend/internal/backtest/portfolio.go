package backtest

import (
	"fmt"
	"time"

	"backtest-sim/backend/internal/data"
)

type PortfolioConfig struct {
	InitialCash float64
	FeeBps      float64
	SlippageBps float64
}

type TradeSide string

const (
	TradeBuy  TradeSide = "BUY"
	TradeSell TradeSide = "SELL"
)

type Trade struct {
	Date                time.Time
	Side                TradeSide
	Price               float64
	Shares              float64
	Fee                 float64
	CashAfterTrade      float64
	PortfolioValueAfter float64
}

type EquityPoint struct {
	Date   time.Time
	Equity float64
}

type PortfolioResult struct {
	FinalValue  float64
	Cash        float64
	Shares      float64
	Trades      []Trade
	EquityCurve []EquityPoint
}

func SimulatePortfolio(candles []data.Candle, signals []Signal, config PortfolioConfig) (PortfolioResult, error) {
	if len(candles) == 0 {
		return PortfolioResult{}, fmt.Errorf("candles cannot be empty")
	}

	if config.InitialCash <= 0 {
		return PortfolioResult{}, fmt.Errorf("initial cash must be positive")
	}

	if config.FeeBps < 0 {
		return PortfolioResult{}, fmt.Errorf("fee bps cannot be negative")
	}

	if config.SlippageBps < 0 {
		return PortfolioResult{}, fmt.Errorf("slippage bps cannot be negative")
	}

	for _, candle := range candles {
		if candle.Close <= 0 {
			return PortfolioResult{}, fmt.Errorf("candle close must be positive")
		}
	}

	cash := config.InitialCash
	var shares float64
	var trades []Trade
	var equityCurve []EquityPoint

	feeRate := config.FeeBps / 10000
	slippageRate := config.SlippageBps / 10000

	// Convert signals to date lookups so each candle is processed once.
	signalsByDate := make(map[string]SignalType)
	for _, signal := range signals {
		signalsByDate[signal.Date.Format("2006-01-02")] = signal.Type
	}

	for _, candle := range candles {
		key := candle.Date.Format("2006-01-02")
		signalType, hasSignal := signalsByDate[key]

		if hasSignal {
			if signalType == SignalBuy && shares == 0 {
				// BUY uses all available cash while reserving enough for fees.
				executionPrice := candle.Close * (1 + slippageRate)
				sharesToBuy := cash / (executionPrice * (1 + feeRate))
				notional := sharesToBuy * executionPrice
				fee := notional * feeRate

				cash = cash - notional - fee
				shares = sharesToBuy

				trades = append(trades, Trade{
					Date:                candle.Date,
					Side:                TradeBuy,
					Price:               executionPrice,
					Shares:              sharesToBuy,
					Fee:                 fee,
					CashAfterTrade:      cash,
					PortfolioValueAfter: cash + shares*candle.Close,
				})
			}

			if signalType == SignalSell && shares > 0 {
				// SELL exits the entire position; this MVP is long-only.
				executionPrice := candle.Close * (1 - slippageRate)
				soldShares := shares
				notional := soldShares * executionPrice
				fee := notional * feeRate

				cash = cash + notional - fee
				shares = 0

				trades = append(trades, Trade{
					Date:                candle.Date,
					Side:                TradeSell,
					Price:               executionPrice,
					Shares:              soldShares,
					Fee:                 fee,
					CashAfterTrade:      cash,
					PortfolioValueAfter: cash,
				})
			}
		}

		equity := cash + shares*candle.Close
		equityCurve = append(equityCurve, EquityPoint{
			Date:   candle.Date,
			Equity: equity,
		})
	}

	finalValue := cash + shares*candles[len(candles)-1].Close

	return PortfolioResult{
		FinalValue:  finalValue,
		Cash:        cash,
		Shares:      shares,
		Trades:      trades,
		EquityCurve: equityCurve,
	}, nil
}
