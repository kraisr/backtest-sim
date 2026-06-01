export type JobStatus = 'queued' | 'running' | 'completed' | 'failed';
export type SweepStatus = 'queued' | 'running' | 'completed' | 'failed';

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

export type SweepRequest = {
  ticker: string;
  initial_cash: number;
  fee_bps: number;
  slippage_bps: number;
  short_windows: number[];
  long_windows: number[];
};

export type CreateSweepResponse = {
  id: string;
  status: SweepStatus;
  run_count: number;
  runs: CreateRunResponse[];
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

export type SweepRunStatusResponse = {
  id: string;
  status: JobStatus;
  job?: RunJob;
  result?: RunResult;
  error?: string;
};

export type SweepStatusResponse = {
  id: string;
  status: SweepStatus;
  run_count: number;
  queued: number;
  running: number;
  completed: number;
  failed: number;
  runs: SweepRunStatusResponse[];
  best_run?: SweepRunStatusResponse;
};

export type ApiError = {
  error: string;
};
