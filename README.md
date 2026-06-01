# BacktestSim

BacktestSim is a Go backtesting platform for running a Moving Average Crossover strategy on historical stock data.

## Demo

https://github.com/user-attachments/assets/a2cc2b0d-9f9d-45be-b9e6-c39f691b4e32

## Features

- Generate Moving Average Crossover BUY/SELL signals
- Simulate a long-only portfolio
- Compare against buy-and-hold
- Calculate return, max drawdown, win rate, and excess return
- Run parameter sweeps across multiple moving-average windows

## Run

From the backend module:

```bash
cd backend
go test ./...
go run ./cmd/cli -csv ../data/SPY.csv -cash 10000 -short-window 20 -long-window 50
```

With fees and slippage:

```bash
go run ./cmd/cli -csv ../data/SPY.csv -cash 10000 -short-window 20 -long-window 50 -fee-bps 10 -slippage-bps 5
```

Show CLI help:

```bash
go run ./cmd/cli -h
```

## API

Start Redis from the project root:

```bash
docker compose up -d redis
```

Run the worker and API server from the backend module in separate terminals:

```bash
go run ./cmd/worker -workers 4
go run ./cmd/api
```

Available endpoints:

| Method | Path | Description |
| --- | --- | --- |
| `GET` | `/health` | Returns API health status |
| `GET` | `/api/tickers` | Lists supported tickers |
| `GET` | `/api/strategies` | Lists supported strategies |
| `POST` | `/api/runs` | Queues an asynchronous Moving Average Crossover backtest |
| `GET` | `/api/runs/{id}` | Returns run status, result, or error details |
| `POST` | `/api/sweeps` | Queues many runs from short/long window grids |
| `GET` | `/api/sweeps/{id}` | Returns aggregate and per-run sweep status |
| `GET` | `/metrics` | Returns Prometheus-format queue and worker metrics |

`POST /api/runs` expects a JSON body with:

```json
{
  "ticker": "SPY",
  "initial_cash": 10000,
  "fee_bps": 0,
  "slippage_bps": 0,
  "short_window": 20,
  "long_window": 50
}
```

The create response includes a run id, queued status, and status URL. Poll the status URL to see whether the run is queued, running, completed, or failed.

Completed run results include strategy return, benchmark return, excess return, max drawdown, win rate, signal count, and trade count.

`POST /api/sweeps` expects a JSON body with:

```json
{
  "ticker": "SPY",
  "initial_cash": 10000,
  "fee_bps": 0,
  "slippage_bps": 0,
  "short_windows": [5, 10, 20],
  "long_windows": [50, 100, 200]
}
```

Sweeps create one queued run for each valid short/long pair where the short window is smaller than the long window.

## Frontend

Run the frontend from the frontend module:

```bash
cd frontend
npm install
npm run dev
```

The Vite dev server proxies API requests to `http://localhost:8080`, so Redis, the worker, and the API server should be running first.

## Example Output

```text
Strategy final value:   $34104.18
Strategy return:        241.04%
Benchmark final value:  $80538.09
Benchmark return:       705.38%
Excess return:          -464.34%
Max drawdown:           28.89%
Win rate:               58.82%
Signals:                103
Strategy trades:        103
```

Exact numbers depend on the CSV data and strategy parameters.

## CSV Format

CSV files must use lowercase headers:

```csv
date,open,high,low,close,volume
```

Example:

```csv
2020-01-02,323.54,324.89,322.53,324.87,59151200
```

## Structure

```text
backend/cmd/cli          CLI entrypoint
backend/cmd/api          API server entrypoint
backend/cmd/worker       Redis worker entrypoint
backend/cmd/sweep-benchmark async sweep benchmark command
backend/internal/queue   Redis queue and result storage
backend/internal/data    CSV loading and market data models
backend/internal/backtest strategy, portfolio, benchmark, and metrics logic
backend/internal/api     HTTP routes and handlers
data/SPY.csv             sample historical data
frontend                 Mantine React frontend
```
