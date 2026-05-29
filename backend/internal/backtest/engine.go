package backtest

import "backtest-sim/backend/internal/data"

type BacktestConfig struct {
	InitialCash float64
	FeeBps      float64
	SlippageBps float64
	ShortWindow int
	LongWindow  int
}

type BacktestResult struct {
	Config          BacktestConfig
	Signals         []Signal
	Strategy        PortfolioResult
	Benchmark       PortfolioResult
	TotalReturn     float64
	BenchmarkReturn float64
	ExcessReturn    float64
	MaxDrawdown     float64
	WinRate         float64
	NumberOfTrades  int
}

func RunMovingAverageBacktest(candles []data.Candle, config BacktestConfig) (BacktestResult, error) {
	// Generate moving average crossover signals
	signals, err := MovingAverageCrossoverSignals(candles, config.ShortWindow, config.LongWindow)
	if err != nil {
		return BacktestResult{}, err
	}

	// Simulate the strategy portfolio
	strategy, err := SimulatePortfolio(candles, signals, PortfolioConfig{
		InitialCash: config.InitialCash,
		FeeBps:      config.FeeBps,
		SlippageBps: config.SlippageBps,
	})
	if err != nil {
		return BacktestResult{}, err
	}

	// Simulate Buy and Hold benchmark
	benchmark, err := BuyAndHold(candles, config.InitialCash)
	if err != nil {
		return BacktestResult{}, err
	}

	// Calculate total return
	totalReturn, err := TotalReturn(config.InitialCash, strategy.FinalValue)
	if err != nil {
		return BacktestResult{}, err
	}
	benchmarkReturn, err := TotalReturn(config.InitialCash, benchmark.FinalValue)
	if err != nil {
		return BacktestResult{}, err
	}

	// Calculate max drawdown
	maxDrawdown, err := MaxDrawdown(strategy.EquityCurve)
	if err != nil {
		return BacktestResult{}, err
	}

	// Calculate win rate
	winRate := WinRate(strategy.Trades)

	return BacktestResult{
		Config:          config,
		Signals:         signals,
		Strategy:        strategy,
		Benchmark:       benchmark,
		TotalReturn:     totalReturn,
		BenchmarkReturn: benchmarkReturn,
		ExcessReturn:    totalReturn - benchmarkReturn,
		MaxDrawdown:     maxDrawdown,
		WinRate:         winRate,
		NumberOfTrades:  len(strategy.Trades),
	}, nil
}
