export type JobStatus = 'queued' | 'running' | 'completed' | 'failed';

export type RunRequest = {
  ticker: string;
  initial_cash: number;
  fee_bps: number;
  slippage_bps: number;
  short_window: number;
  long_window: number;
};

export type CreateRunResponse = {
  id: string;
  status: JobStatus;
  status_url: string;
};

export type RunJob = {
  ticker: string;
  initial_cash: number;
  fee_bps: number;
  slippage_bps: number;
  short_window: number;
  long_window: number;
};

export type RunResult = {
  strategy: string;
  strategy_final_value: number;
  strategy_return: number;
  benchmark_final_value: number;
  benchmark_return: number;
  excess_return: number;
  max_drawdown: number;
  win_rate: number;
  signals: number;
  trades: number;
};

export type RunStatusResponse = {
  id: string;
  status: JobStatus;
  job?: RunJob;
  result?: RunResult;
  error?: string;
};

export type ApiError = {
  error: string;
};
