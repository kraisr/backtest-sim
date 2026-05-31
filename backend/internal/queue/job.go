package queue

import "time"

type Job struct {
	ID          string    `json:"id"`
	SweepID     string    `json:"sweep_id,omitempty"`
	Ticker      string    `json:"ticker"`
	InitialCash float64   `json:"initial_cash"`
	FeeBps      float64   `json:"fee_bps"`
	SlippageBps float64   `json:"slippage_bps"`
	ShortWindow int       `json:"short_window"`
	LongWindow  int       `json:"long_window"`
	EnqueuedAt  time.Time `json:"enqueued_at"`
}
