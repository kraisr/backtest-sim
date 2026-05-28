package backtest

import "time"

type SignalType string

const (
	SignalHold SignalType = "HOLD"
	SignalBuy  SignalType = "BUY"
	SignalSell SignalType = "SELL"
)

type Signal struct {
	Date time.Time
	Type SignalType
}
