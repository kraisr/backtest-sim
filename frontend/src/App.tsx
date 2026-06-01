import { Fragment, useEffect, useMemo, useState, type ReactNode } from 'react';
import {
  Alert,
  AppShell,
  Badge,
  Box,
  Button,
  Divider,
  Group,
  NumberInput,
  Paper,
  Progress,
  Select,
  SimpleGrid,
  Stack,
  Table,
  Text,
  TextInput,
  ThemeIcon,
  Title,
} from '@mantine/core';
import { useForm } from '@mantine/form';
import {
  IconActivity,
  IconAlertCircle,
  IconChartLine,
  IconClock,
  IconCurrencyDollar,
  IconGauge,
  IconListCheck,
  IconPlayerPlay,
  IconSearch,
  IconTrendingDown,
  IconTrendingUp,
  IconTrophy,
} from '@tabler/icons-react';
import { createSweep, fetchHealth, fetchSweep } from './api';
import type {
  CreateSweepResponse,
  JobStatus,
  RunJob,
  RunResult,
  SweepRequest,
  SweepRunStatusResponse,
  SweepStatus,
  SweepStatusResponse,
} from './types';

type ApiHealth = 'checking' | 'online' | 'offline';

type SweepFormValues = {
  ticker: string;
  initial_cash: number;
  fee_bps: number;
  slippage_bps: number;
  short_windows: string;
  long_windows: string;
};

type ParsedWindows = {
  values: number[];
  error: string | null;
};

type GridStats = {
  shortCount: number;
  longCount: number;
  rawCount: number;
  validCount: number;
  skippedCount: number;
};

type CompletedSweepRun = SweepRunStatusResponse & {
  job: RunJob;
  result: RunResult;
};

const maxSweepRuns = 250;
const rankedResultsPageSize = 10;

const defaultSweepValues: SweepFormValues = {
  ticker: 'SPY',
  initial_cash: 10000,
  fee_bps: 0,
  slippage_bps: 0,
  short_windows: '5, 10, 15, 20, 25, 30, 35, 40, 45, 50',
  long_windows: '60, 80, 100, 120, 140, 160, 180, 200, 220, 250',
};

function App() {
  const [apiHealth, setApiHealth] = useState<ApiHealth>('checking');
  const [activeSweep, setActiveSweep] = useState<CreateSweepResponse | null>(null);
  const [sweepDetails, setSweepDetails] = useState<SweepStatusResponse | null>(null);
  const [submitError, setSubmitError] = useState<string | null>(null);
  const [pollError, setPollError] = useState<string | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [lastUpdated, setLastUpdated] = useState<string | null>(null);
  const [visibleRankCount, setVisibleRankCount] = useState(rankedResultsPageSize);

  const form = useForm<SweepFormValues>({
    initialValues: defaultSweepValues,
    validate: {
      initial_cash: (value) => (value > 0 ? null : 'Cash must be positive'),
      fee_bps: (value) => (value >= 0 ? null : 'Fee cannot be negative'),
      slippage_bps: (value) => (value >= 0 ? null : 'Slippage cannot be negative'),
      short_windows: (value) => parseWindowList(value, 'Short windows').error,
      long_windows: (value, values) => {
        const longWindows = parseWindowList(value, 'Long windows');
        if (longWindows.error) {
          return longWindows.error;
        }

        const shortWindows = parseWindowList(values.short_windows, 'Short windows');
        if (shortWindows.error) {
          return null;
        }

        const runCount = countValidPairs(shortWindows.values, longWindows.values);
        if (runCount === 0) {
          return 'At least one short window must be smaller than a long window';
        }

        if (runCount > maxSweepRuns) {
          return `Sweep cannot exceed ${maxSweepRuns} runs`;
        }

        return null;
      },
    },
  });

  const currentStatus = sweepDetails?.status ?? activeSweep?.status;
  const hasActiveSweep = Boolean(activeSweep?.id);
  const bestRun = sweepDetails?.best_run;
  const bestResult = bestRun?.result;
  const displayedRuns = sweepDetails?.runs ?? activeSweep?.runs ?? [];
  const rankedRuns = useMemo(() => buildRankedRuns(displayedRuns), [displayedRuns]);
  const visibleRankedRuns = useMemo(() => rankedRuns.slice(0, visibleRankCount), [rankedRuns, visibleRankCount]);
  const completedRuns = useMemo(() => buildCompletedRuns(displayedRuns), [displayedRuns]);
  const gridStats = useMemo(
    () => estimateGridStats(form.values.short_windows, form.values.long_windows),
    [form.values.short_windows, form.values.long_windows],
  );
  const runCountEstimate = gridStats?.validCount ?? null;

  const completedCount = sweepDetails?.completed ?? 0;
  const failedCount = sweepDetails?.failed ?? 0;
  const runCount = sweepDetails?.run_count ?? activeSweep?.run_count ?? runCountEstimate ?? 0;
  const finishedCount = completedCount + failedCount;
  const progressValue = runCount > 0 ? (finishedCount / runCount) * 100 : 0;
  const hiddenRankCount = Math.max(0, rankedRuns.length - visibleRankedRuns.length);

  useEffect(() => {
    let cancelled = false;

    async function checkHealth() {
      try {
        await fetchHealth();
        if (!cancelled) {
          setApiHealth('online');
        }
      } catch {
        if (!cancelled) {
          setApiHealth('offline');
        }
      }
    }

    void checkHealth();
    const intervalID = window.setInterval(checkHealth, 10000);

    return () => {
      cancelled = true;
      window.clearInterval(intervalID);
    };
  }, []);

  useEffect(() => {
    const sweepID = activeSweep?.id;
    if (!sweepID) {
      return;
    }

    const stableSweepID = sweepID;
    let cancelled = false;
    let intervalID = 0;

    // Poll the sweep endpoint until the aggregate status reaches a terminal state
    async function pollSweep() {
      try {
        const response = await fetchSweep(stableSweepID);
        if (cancelled) {
          return;
        }

        setSweepDetails(response);
        setPollError(null);
        setLastUpdated(new Date().toLocaleTimeString());

        if (response.status === 'completed' || response.status === 'failed') {
          window.clearInterval(intervalID);
        }
      } catch (error) {
        if (!cancelled) {
          setPollError(error instanceof Error ? error.message : 'Unable to fetch sweep status');
        }
      }
    }

    void pollSweep();
    intervalID = window.setInterval(pollSweep, 1200);

    return () => {
      cancelled = true;
      window.clearInterval(intervalID);
    };
  }, [activeSweep?.id]);

  async function handleSubmit(values: SweepFormValues) {
    setSubmitError(null);
    setPollError(null);
    setIsSubmitting(true);
    setSweepDetails(null);
    setLastUpdated(null);
    setVisibleRankCount(rankedResultsPageSize);

    try {
      // Convert the form strings into the numeric grid expected by the API
      const request = buildSweepRequest(values);
      const response = await createSweep(request);

      setActiveSweep(response);
      setSweepDetails({
        id: response.id,
        status: response.status,
        run_count: response.run_count,
        queued: response.run_count,
        running: 0,
        completed: 0,
        failed: 0,
        runs: response.runs,
      });
    } catch (error) {
      setSubmitError(error instanceof Error ? error.message : 'Unable to create sweep');
    } finally {
      setIsSubmitting(false);
    }
  }

  return (
    <AppShell header={{ height: 68 }} padding="md">
      <AppShell.Header className="app-header">
        <Group h="100%" px="md" justify="space-between" wrap="nowrap">
          <Group gap="sm" wrap="nowrap">
            <ThemeIcon size={38} radius="md" color="teal" variant="light">
              <IconChartLine size={22} />
            </ThemeIcon>
            <Box>
              <Title order={3} lh={1.1}>
                BacktestSim
              </Title>
              <Text size="xs" c="dimmed" mt={3}>
                Parallel Strategy Search
              </Text>
            </Box>
          </Group>

          <Badge color={apiHealthColor(apiHealth)} variant="light" size="lg" leftSection={<IconActivity size={14} />}>
            {apiHealthLabel(apiHealth)}
          </Badge>
        </Group>
      </AppShell.Header>

      <AppShell.Main className="app-main">
        <Stack className="content-shell" gap="lg">
          <SimpleGrid cols={{ base: 1, lg: 2 }} spacing="lg" verticalSpacing="lg">
            <Paper className="panel" p="lg" radius="md">
              <form onSubmit={form.onSubmit(handleSubmit)}>
                <Stack gap="md">
                  <Group justify="space-between" align="flex-start">
                    <Box>
                      <Text fw={700} size="lg">
                        Strategy sweep
                      </Text>
                      <Text size="sm" c="dimmed">
                        Moving average crossover
                      </Text>
                    </Box>
                    <Badge color="teal" variant="light" leftSection={<IconSearch size={14} />}>
                      {runCountEstimate ?? 0} runs
                    </Badge>
                  </Group>

                  <SimpleGrid cols={{ base: 1, sm: 2 }} spacing="md">
                    <Select
                      label="Ticker"
                      data={[{ value: 'SPY', label: 'SPY' }]}
                      {...form.getInputProps('ticker')}
                    />
                    <NumberInput
                      label="Initial cash"
                      min={1}
                      step={1000}
                      prefix="$"
                      thousandSeparator=","
                      {...form.getInputProps('initial_cash')}
                    />
                    <TextInput
                      label="Short windows"
                      placeholder="5, 10, 20"
                      {...form.getInputProps('short_windows')}
                    />
                    <TextInput
                      label="Long windows"
                      placeholder="50, 100, 200"
                      {...form.getInputProps('long_windows')}
                    />
                    <NumberInput
                      label="Fee"
                      min={0}
                      step={1}
                      suffix=" bps"
                      {...form.getInputProps('fee_bps')}
                    />
                    <NumberInput
                      label="Slippage"
                      min={0}
                      step={1}
                      suffix=" bps"
                      {...form.getInputProps('slippage_bps')}
                    />
                  </SimpleGrid>

                  <Group gap="xs">
                    <Badge color="gray" variant="light">
                      Grid {gridStats?.rawCount ?? '-'}
                    </Badge>
                    <Badge color="teal" variant="light">
                      Runs {gridStats?.validCount ?? '-'}
                    </Badge>
                    {gridStats && gridStats.skippedCount > 0 ? (
                      <Badge color="orange" variant="light">
                        Skipped {gridStats.skippedCount}
                      </Badge>
                    ) : null}
                  </Group>

                  {submitError ? (
                    <Alert color="red" icon={<IconAlertCircle size={18} />} radius="md">
                      {submitError}
                    </Alert>
                  ) : null}

                  <Button
                    type="submit"
                    size="md"
                    leftSection={<IconPlayerPlay size={18} />}
                    loading={isSubmitting}
                    disabled={apiHealth === 'offline'}
                  >
                    Run sweep
                  </Button>
                </Stack>
              </form>
            </Paper>

            <Paper className="panel status-panel" p="lg" radius="md">
              <Stack gap="md">
                <Group justify="space-between" align="flex-start">
                  <Box>
                    <Text fw={700} size="lg">
                      Sweep progress
                    </Text>
                    <Text size="sm" c="dimmed" className="detail-value">
                      {activeSweep?.id ?? 'No sweep submitted'}
                    </Text>
                  </Box>
                  <StatusBadge status={currentStatus} />
                </Group>

                <Stack gap={8}>
                  <Group justify="space-between" gap="sm">
                    <Text size="sm" c="dimmed">
                      Completed
                    </Text>
                    <Text size="sm" fw={700}>
                      {finishedCount}/{runCount}
                    </Text>
                  </Group>
                  <Progress value={progressValue} color="teal" size="lg" radius="md" />
                </Stack>

                <SimpleGrid cols={{ base: 2, xs: 4 }} spacing="sm">
                  <CountPill label="Queued" value={sweepDetails?.queued ?? activeSweep?.run_count ?? 0} color="blue" />
                  <CountPill label="Running" value={sweepDetails?.running ?? 0} color="yellow" />
                  <CountPill label="Done" value={completedCount} color="teal" />
                  <CountPill label="Failed" value={failedCount} color="red" />
                </SimpleGrid>

                <Divider />

                {hasActiveSweep ? (
                  <Stack gap="sm">
                    <DetailRow label="Status URL" value={activeSweep?.status_url ?? '-'} />
                    <DetailRow label="Last update" value={lastUpdated ?? '-'} />
                    <DetailRow label="Best windows" value={bestRun?.job ? formatWindowPair(bestRun.job) : '-'} />
                  </Stack>
                ) : (
                  <Stack align="center" justify="center" className="empty-state">
                    <ThemeIcon color="gray" variant="light" size={52} radius="md">
                      <IconClock size={28} />
                    </ThemeIcon>
                    <Text c="dimmed" ta="center">
                      Waiting for a sweep
                    </Text>
                  </Stack>
                )}

                {pollError ? (
                  <Alert color="red" icon={<IconAlertCircle size={18} />} radius="md">
                    {pollError}
                  </Alert>
                ) : null}
              </Stack>
            </Paper>
          </SimpleGrid>

          <SimpleGrid cols={{ base: 1, sm: 2, lg: 4 }} spacing="lg">
            <MetricCard
              label="Best value"
              value={bestResult ? formatCurrency(bestResult.strategy_final_value) : '-'}
              icon={<IconCurrencyDollar size={22} />}
              color="teal"
            />
            <MetricCard
              label="Best return"
              value={bestResult ? formatPercent(bestResult.strategy_return) : '-'}
              icon={<IconTrendingUp size={22} />}
              color={metricColor(bestResult?.strategy_return)}
            />
            <MetricCard
              label="Excess return"
              value={bestResult ? formatPercent(bestResult.excess_return) : '-'}
              icon={<IconGauge size={22} />}
              color={metricColor(bestResult?.excess_return)}
            />
            <MetricCard
              label="Max drawdown"
              value={bestResult ? formatPercent(bestResult.max_drawdown) : '-'}
              icon={<IconTrendingDown size={22} />}
              color="red"
            />
          </SimpleGrid>

          <Paper className="panel" p="lg" radius="md">
            <ReturnHeatmap runs={completedRuns} bestRunID={bestRun?.id} />
          </Paper>

          <Paper className="panel" p="lg" radius="md">
            <Stack gap="md">
              <Group justify="space-between" align="flex-start">
                <Box>
                  <Text fw={700} size="lg">
                    Ranked results
                  </Text>
                  <Text size="sm" c="dimmed">
                    {rankedRuns.length > 0
                      ? `Showing ${visibleRankedRuns.length} of ${rankedRuns.length} parameter combinations`
                      : 'No results yet'}
                  </Text>
                </Box>
                <Badge color="teal" variant="light" leftSection={<IconListCheck size={14} />}>
                  {completedCount} completed
                </Badge>
              </Group>

              <Table.ScrollContainer minWidth={980}>
                <Table highlightOnHover verticalSpacing="sm">
                  <Table.Thead>
                    <Table.Tr>
                      <Table.Th>Rank</Table.Th>
                      <Table.Th>Windows</Table.Th>
                      <Table.Th>Status</Table.Th>
                      <Table.Th>Strategy return</Table.Th>
                      <Table.Th>Excess return</Table.Th>
                      <Table.Th>Max drawdown</Table.Th>
                      <Table.Th>Win rate</Table.Th>
                      <Table.Th>Trades</Table.Th>
                      <Table.Th>Signals</Table.Th>
                    </Table.Tr>
                  </Table.Thead>
                  <Table.Tbody>
                    {rankedRuns.length === 0 ? (
                      <Table.Tr>
                        <Table.Td colSpan={9}>
                          <Text c="dimmed" ta="center" py="lg">
                            Submit a sweep to populate the comparison table
                          </Text>
                        </Table.Td>
                      </Table.Tr>
                    ) : (
                      visibleRankedRuns.map((run, index) => (
                        <Table.Tr key={run.id} className={run.id === bestRun?.id ? 'best-run-row' : undefined}>
                          <Table.Td>
                            <Group gap={6} wrap="nowrap">
                              <Text fw={700}>{run.result ? index + 1 : '-'}</Text>
                              {run.id === bestRun?.id ? <IconTrophy size={16} className="best-run-icon" /> : null}
                            </Group>
                          </Table.Td>
                          <Table.Td>
                            <Text fw={600}>{run.job ? formatWindowPair(run.job) : '-'}</Text>
                          </Table.Td>
                          <Table.Td>
                            <StatusBadge status={run.status} />
                          </Table.Td>
                          <Table.Td>
                            <ResultValue result={run.result} value="strategy_return" />
                          </Table.Td>
                          <Table.Td>
                            <ResultValue result={run.result} value="excess_return" />
                          </Table.Td>
                          <Table.Td>
                            <ResultValue result={run.result} value="max_drawdown" invertTone />
                          </Table.Td>
                          <Table.Td>
                            <ResultValue result={run.result} value="win_rate" />
                          </Table.Td>
                          <Table.Td>{run.result ? run.result.trades : '-'}</Table.Td>
                          <Table.Td>{run.result ? run.result.signals : '-'}</Table.Td>
                        </Table.Tr>
                      ))
                    )}
                  </Table.Tbody>
                </Table>
              </Table.ScrollContainer>

              {rankedRuns.length > rankedResultsPageSize ? (
                <Group justify="center" gap="sm">
                  {hiddenRankCount > 0 ? (
                    <Button
                      variant="light"
                      onClick={() =>
                        setVisibleRankCount((count) => Math.min(count + rankedResultsPageSize, rankedRuns.length))
                      }
                    >
                      Show {Math.min(rankedResultsPageSize, hiddenRankCount)} more
                    </Button>
                  ) : null}
                  {visibleRankCount > rankedResultsPageSize ? (
                    <Button variant="subtle" color="gray" onClick={() => setVisibleRankCount(rankedResultsPageSize)}>
                      Show top {rankedResultsPageSize}
                    </Button>
                  ) : null}
                </Group>
              ) : null}
            </Stack>
          </Paper>
        </Stack>
      </AppShell.Main>
    </AppShell>
  );
}

function StatusBadge({ status }: { status?: JobStatus | SweepStatus }) {
  return (
    <Badge color={statusColor(status)} variant="light" size="lg">
      {status ?? 'idle'}
    </Badge>
  );
}

function CountPill({ label, value, color }: { label: string; value: number; color: string }) {
  return (
    <Paper className="count-pill" radius="md" p="sm">
      <Text size="xs" c="dimmed">
        {label}
      </Text>
      <Text fw={800} c={color}>
        {value}
      </Text>
    </Paper>
  );
}

function MetricCard({
  label,
  value,
  icon,
  color,
}: {
  label: string;
  value: string;
  icon: ReactNode;
  color: string;
}) {
  return (
    <Paper className="metric-card" p="lg" radius="md">
      <Group justify="space-between" align="flex-start" wrap="nowrap">
        <Box>
          <Text size="sm" c="dimmed">
            {label}
          </Text>
          <Text className="metric-value" fw={800}>
            {value}
          </Text>
        </Box>
        <ThemeIcon color={color} variant="light" radius="md" size={42}>
          {icon}
        </ThemeIcon>
      </Group>
    </Paper>
  );
}

function DetailRow({ label, value }: { label: string; value: string }) {
  return (
    <Group justify="space-between" gap="md" wrap="nowrap" className="detail-row">
      <Text size="sm" c="dimmed">
        {label}
      </Text>
      <Text size="sm" fw={600} ta="right" className="detail-value">
        {value}
      </Text>
    </Group>
  );
}

function ReturnHeatmap({ runs, bestRunID }: { runs: CompletedSweepRun[]; bestRunID?: string }) {
  const shortWindows = uniqueSorted(runs.map((run) => run.job.short_window));
  const longWindows = uniqueSorted(runs.map((run) => run.job.long_window));
  const runByPair = new Map(runs.map((run) => [windowPairKey(run.job.short_window, run.job.long_window), run]));
  const domain = valueDomain(runs.map((run) => run.result.strategy_return));

  return (
    <Stack gap="md">
      <Group justify="space-between" align="flex-start">
        <Box>
          <Text fw={700} size="lg">
            Return heatmap
          </Text>
          <Text size="sm" c="dimmed">
            Short window by long window
          </Text>
        </Box>
        <Badge color="teal" variant="light">
          {runs.length} complete
        </Badge>
      </Group>

      {runs.length === 0 ? (
        <EmptyVisualization label="Waiting for completed runs" />
      ) : (
        <Box className="heatmap-scroll">
          <Box
            className="heatmap-grid"
            style={{
              gridTemplateColumns: `72px repeat(${longWindows.length}, minmax(100px, 1fr))`,
            }}
          >
            <Box className="heatmap-axis-corner" />
            {longWindows.map((longWindow) => (
              <Text key={longWindow} size="xs" c="dimmed" ta="center" fw={700}>
                L{longWindow}
              </Text>
            ))}

            {shortWindows.map((shortWindow) => (
              <Fragment key={shortWindow}>
                <Text size="xs" c="dimmed" fw={700}>
                  S{shortWindow}
                </Text>
                {longWindows.map((longWindow) => {
                  const run = runByPair.get(windowPairKey(shortWindow, longWindow));
                  const isBest = run?.id === bestRunID;

                  return (
                    <Box
                      key={`${shortWindow}-${longWindow}`}
                      className={`heatmap-cell${isBest ? ' heatmap-cell-best' : ''}`}
                      style={{
                        background: run ? heatmapColor(run.result.strategy_return, domain) : undefined,
                      }}
                    >
                      <Text size="xs" fw={800}>
                        {run ? formatPercent(run.result.strategy_return) : '-'}
                      </Text>
                    </Box>
                  );
                })}
              </Fragment>
            ))}
          </Box>
        </Box>
      )}
    </Stack>
  );
}

function EmptyVisualization({ label }: { label: string }) {
  return (
    <Stack align="center" justify="center" className="chart-empty">
      <ThemeIcon color="gray" variant="light" size={46} radius="md">
        <IconChartLine size={24} />
      </ThemeIcon>
      <Text c="dimmed" ta="center" size="sm">
        {label}
      </Text>
    </Stack>
  );
}

function ResultValue({
  result,
  value,
  invertTone = false,
}: {
  result?: RunResult;
  value: 'strategy_return' | 'excess_return' | 'max_drawdown' | 'win_rate';
  invertTone?: boolean;
}) {
  if (!result) {
    return <Text c="dimmed">-</Text>;
  }

  const numberValue = result[value];
  const isPositive = invertTone ? numberValue <= 0 : numberValue >= 0;

  return (
    <Text fw={700} c={isPositive ? 'teal' : 'red'}>
      {formatPercent(numberValue)}
    </Text>
  );
}

function buildSweepRequest(values: SweepFormValues): SweepRequest {
  const shortWindows = parseWindowList(values.short_windows, 'Short windows');
  const longWindows = parseWindowList(values.long_windows, 'Long windows');

  if (shortWindows.error) {
    throw new Error(shortWindows.error);
  }

  if (longWindows.error) {
    throw new Error(longWindows.error);
  }

  const runCount = countValidPairs(shortWindows.values, longWindows.values);
  if (runCount === 0) {
    throw new Error('At least one short window must be smaller than a long window');
  }

  if (runCount > maxSweepRuns) {
    throw new Error(`Sweep cannot exceed ${maxSweepRuns} runs`);
  }

  return {
    ticker: values.ticker,
    initial_cash: Number(values.initial_cash),
    fee_bps: Number(values.fee_bps),
    slippage_bps: Number(values.slippage_bps),
    short_windows: shortWindows.values,
    long_windows: longWindows.values,
  };
}

function parseWindowList(rawValue: string, label: string): ParsedWindows {
  const parts = rawValue
    .split(',')
    .map((part) => part.trim())
    .filter(Boolean);

  if (parts.length === 0) {
    return { values: [], error: `${label} cannot be empty` };
  }

  const seen = new Set<number>();
  const values: number[] = [];

  for (const part of parts) {
    const window = Number(part);
    if (!Number.isInteger(window) || window <= 0) {
      return { values: [], error: `${label} must contain positive whole numbers` };
    }

    if (!seen.has(window)) {
      seen.add(window);
      values.push(window);
    }
  }

  values.sort((a, b) => a - b);
  return { values, error: null };
}

function countValidPairs(shortWindows: number[], longWindows: number[]) {
  let count = 0;

  for (const shortWindow of shortWindows) {
    for (const longWindow of longWindows) {
      if (shortWindow < longWindow) {
        count += 1;
      }
    }
  }

  return count;
}

function estimateGridStats(shortWindowsRaw: string, longWindowsRaw: string): GridStats | null {
  const shortWindows = parseWindowList(shortWindowsRaw, 'Short windows');
  const longWindows = parseWindowList(longWindowsRaw, 'Long windows');

  if (shortWindows.error || longWindows.error) {
    return null;
  }

  const rawCount = shortWindows.values.length * longWindows.values.length;
  const validCount = countValidPairs(shortWindows.values, longWindows.values);

  return {
    shortCount: shortWindows.values.length,
    longCount: longWindows.values.length,
    rawCount,
    validCount,
    skippedCount: rawCount - validCount,
  };
}

function buildRankedRuns(runs: SweepRunStatusResponse[]) {
  return [...runs].sort((a, b) => {
    if (a.result && b.result) {
      return b.result.strategy_return - a.result.strategy_return;
    }

    if (a.result) {
      return -1;
    }

    if (b.result) {
      return 1;
    }

    return statusSortOrder(a.status) - statusSortOrder(b.status);
  });
}

function buildCompletedRuns(runs: SweepRunStatusResponse[]): CompletedSweepRun[] {
  return runs.filter(isCompletedRun).sort((a, b) => b.result.strategy_return - a.result.strategy_return);
}

function isCompletedRun(run: SweepRunStatusResponse): run is CompletedSweepRun {
  return Boolean(run.job && run.result);
}

function statusSortOrder(status: JobStatus) {
  switch (status) {
    case 'running':
      return 0;
    case 'queued':
      return 1;
    case 'failed':
      return 2;
    case 'completed':
      return 3;
    default:
      return 4;
  }
}

function formatWindowPair(job: { short_window: number; long_window: number }) {
  return `MA ${job.short_window} / ${job.long_window}`;
}

function windowPairKey(shortWindow: number, longWindow: number) {
  return `${shortWindow}:${longWindow}`;
}

function uniqueSorted(values: number[]) {
  return Array.from(new Set(values)).sort((a, b) => a - b);
}

function valueDomain(values: number[]) {
  if (values.length === 0) {
    return { min: 0, max: 0 };
  }

  return {
    min: Math.min(...values),
    max: Math.max(...values),
  };
}

function heatmapColor(value: number, domain: { min: number; max: number }) {
  if (domain.max === domain.min) {
    return value >= 0 ? 'rgba(18, 184, 134, 0.58)' : 'rgba(250, 82, 82, 0.58)';
  }

  const ratio = (value - domain.min) / (domain.max - domain.min);
  const low = [92, 124, 250];
  const mid = [250, 176, 5];
  const high = [18, 184, 134];
  const color = ratio < 0.5 ? interpolateColor(low, mid, ratio * 2) : interpolateColor(mid, high, (ratio - 0.5) * 2);

  return `rgb(${color[0]}, ${color[1]}, ${color[2]})`;
}

function interpolateColor(start: number[], end: number[], ratio: number) {
  return start.map((channel, index) => Math.round(channel + (end[index] - channel) * ratio));
}

function apiHealthLabel(health: ApiHealth) {
  if (health === 'online') {
    return 'API online';
  }

  if (health === 'offline') {
    return 'API offline';
  }

  return 'Checking API';
}

function apiHealthColor(health: ApiHealth) {
  if (health === 'online') {
    return 'teal';
  }

  if (health === 'offline') {
    return 'red';
  }

  return 'gray';
}

function statusColor(status?: JobStatus | SweepStatus) {
  switch (status) {
    case 'queued':
      return 'blue';
    case 'running':
      return 'yellow';
    case 'completed':
      return 'teal';
    case 'failed':
      return 'red';
    default:
      return 'gray';
  }
}

function metricColor(value?: number) {
  if (value === undefined) {
    return 'gray';
  }

  return value >= 0 ? 'teal' : 'red';
}

function formatCurrency(value: number) {
  return new Intl.NumberFormat('en-US', {
    style: 'currency',
    currency: 'USD',
    maximumFractionDigits: 2,
  }).format(value);
}

function formatPercent(value: number) {
  return `${(value * 100).toFixed(2)}%`;
}

export default App;
