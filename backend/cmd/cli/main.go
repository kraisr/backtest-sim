package main

import (
	"flag"
	"fmt"
	"os"

	"backtest-sim/backend/internal/backtest"
	"backtest-sim/backend/internal/data"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// Parse command-line flags
	flag.Usage = func() {
		output := flag.CommandLine.Output()
		fmt.Fprintln(output, "Usage:")
		fmt.Fprintln(output, "  go run ./cmd/cli -csv ../data/SPY.csv -cash 10000 -short-window 20 -long-window 50")
		fmt.Fprintln(output)
		fmt.Fprintln(output, "Flags:")
		flag.VisitAll(func(f *flag.Flag) {
			fmt.Fprintf(output, "  -%s\n", f.Name)
			fmt.Fprintf(output, "      %s (default: %s)\n", f.Usage, f.DefValue)
		})
	}

	csvPtr := flag.String("csv", "../data/SPY.csv", "path to an OHLCV CSV file")
	initialCashPtr := flag.Float64("cash", 10000, "starting portfolio cash in dollars")
	feeBpsPtr := flag.Float64("fee-bps", 0, "transaction fee per trade in basis points; 10 means 0.10%")
	slippageBpsPtr := flag.Float64("slippage-bps", 0, "execution slippage in basis points; 5 means 0.05%")
	shortWindowPtr := flag.Int("short-window", 20, "short moving average window in trading days")
	longWindowPtr := flag.Int("long-window", 50, "long moving average window in trading days")

	flag.Parse()

	// Load candles
	candles, err := data.LoadCSV(*csvPtr)
	if err != nil {
		return fmt.Errorf("load csv: %w", err)
	}

	config := backtest.BacktestConfig{
		InitialCash: *initialCashPtr,
		FeeBps:      *feeBpsPtr,
		SlippageBps: *slippageBpsPtr,
		ShortWindow: *shortWindowPtr,
		LongWindow:  *longWindowPtr,
	}

	result, err := backtest.RunMovingAverageBacktest(candles, config)
	if err != nil {
		return fmt.Errorf("run moving average backtest: %w", err)
	}

	printSummary(result, len(candles))

	return nil
}

func printSummary(result backtest.BacktestResult, candleCount int) {
	fmt.Println("BacktestSim Moving Average Backtest")
	fmt.Println("==================================")
	fmt.Println()

	fmt.Println("Configuration")
	fmt.Printf("  Candles:        %d\n", candleCount)
	fmt.Printf("  Initial cash:   $%.2f\n", result.Config.InitialCash)
	fmt.Printf("  Short window:   %d\n", result.Config.ShortWindow)
	fmt.Printf("  Long window:    %d\n", result.Config.LongWindow)
	fmt.Printf("  Fee:            %.2f bps\n", result.Config.FeeBps)
	fmt.Printf("  Slippage:       %.2f bps\n", result.Config.SlippageBps)
	fmt.Println()

	fmt.Println("Results")
	fmt.Printf("  Strategy final value:   $%.2f\n", result.Strategy.FinalValue)
	fmt.Printf("  Strategy return:        %s\n", formatPercent(result.TotalReturn))
	fmt.Printf("  Benchmark final value:  $%.2f\n", result.Benchmark.FinalValue)
	fmt.Printf("  Benchmark return:       %s\n", formatPercent(result.BenchmarkReturn))
	fmt.Printf("  Excess return:          %s\n", formatPercent(result.ExcessReturn))
	fmt.Printf("  Max drawdown:           %s\n", formatPercent(result.MaxDrawdown))
	fmt.Printf("  Win rate:               %s\n", formatPercent(result.WinRate))
	fmt.Println()

	fmt.Println("Activity")
	fmt.Printf("  Signals:         %d\n", len(result.Signals))
	fmt.Printf("  Strategy trades: %d\n", result.NumberOfTrades)
}

func formatPercent(value float64) string {
	return fmt.Sprintf("%.2f%%", value*100)
}
