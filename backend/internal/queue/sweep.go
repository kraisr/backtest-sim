package queue

import "time"

type Sweep struct {
	ID           string    `json:"id"`
	Ticker       string    `json:"ticker"`
	InitialCash  float64   `json:"initial_cash"`
	FeeBps       float64   `json:"fee_bps"`
	SlippageBps  float64   `json:"slippage_bps"`
	ShortWindows []int     `json:"short_windows"`
	LongWindows  []int     `json:"long_windows"`
	RunIDs       []string  `json:"run_ids"`
	CreatedAt    time.Time `json:"created_at"`
}
