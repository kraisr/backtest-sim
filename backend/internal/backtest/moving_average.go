package backtest

import (
	"fmt"

	"backtest-sim/backend/internal/data"
)

func SimpleMovingAverage(values []float64, window int) ([]float64, error) {
	if window <= 0 {
		return nil, fmt.Errorf("window must be positive")
	}

	if len(values) < window {
		return nil, fmt.Errorf("not enough values for window %d", window)
	}

	averages := make([]float64, len(values))

	var sum float64

	for i, value := range values {
		sum += value
		if i >= window {
			sum -= values[i-window]
		}

		if i >= window-1 {
			averages[i] = sum / float64(window)
		}
	}

	return averages, nil
}

func MovingAverageCrossoverSignals(candles []data.Candle, shortWindow int, longWindow int) ([]Signal, error) {
	if len(candles) == 0 {
		return nil, fmt.Errorf("candles cannot be empty")
	}

	if shortWindow <= 0 || longWindow <= 0 {
		return nil, fmt.Errorf("windows must be positive")
	}

	if shortWindow >= longWindow {
		return nil, fmt.Errorf("short window must be less than long window")
	}

	if len(candles) < longWindow {
		return nil, fmt.Errorf("not enough candles for long window %d", longWindow)
	}

	closes := make([]float64, len(candles))
	for i, candle := range candles {
		closes[i] = candle.Close
	}

	shortMA, err := SimpleMovingAverage(closes, shortWindow)
	if err != nil {
		return nil, err
	}

	longMA, err := SimpleMovingAverage(closes, longWindow)
	if err != nil {
		return nil, err
	}

	var signals []Signal

	for i := longWindow; i < len(candles); i++ {
		prevShort := shortMA[i-1]
		prevLong := longMA[i-1]
		currShort := shortMA[i]
		currLong := longMA[i]

		// Short moving average crossed upward -> BUY
		if prevShort <= prevLong && currShort > currLong {
			signals = append(signals, Signal{
				Date: candles[i].Date,
				Type: SignalBuy,
			})
		}
		// Short moving average crossed downward -> SELL
		if prevShort >= prevLong && currShort < currLong {
			signals = append(signals, Signal{
				Date: candles[i].Date,
				Type: SignalSell,
			})
		}
	}

	return signals, nil
}
