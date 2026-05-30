import { useEffect, useMemo, useState, type ReactNode } from 'react';
import {
  Alert,
  AppShell,
  Badge,
  Box,
  Button,
  Divider,
  Group,
  LoadingOverlay,
  NumberInput,
  Paper,
  Select,
  SimpleGrid,
  Stack,
  Table,
  Text,
  ThemeIcon,
  Title,
} from '@mantine/core';
import { useForm } from '@mantine/form';
import {
  IconActivity,
  IconAlertCircle,
  IconChartLine,
  IconCheck,
  IconClock,
  IconCurrencyDollar,
  IconPlayerPlay,
  IconRefresh,
  IconTrendingDown,
  IconTrendingUp,
} from '@tabler/icons-react';
import { createRun, fetchHealth, fetchRun } from './api';
import type { CreateRunResponse, JobStatus, RunRequest, RunResult, RunStatusResponse } from './types';

type ApiHealth = 'checking' | 'online' | 'offline';

const defaultRunValues: RunRequest = {
  ticker: 'SPY',
  initial_cash: 10000,
  fee_bps: 0,
  slippage_bps: 0,
  short_window: 20,
  long_window: 50,
};

function App() {
  const [apiHealth, setApiHealth] = useState<ApiHealth>('checking');
  const [activeRun, setActiveRun] = useState<CreateRunResponse | null>(null);
  const [runDetails, setRunDetails] = useState<RunStatusResponse | null>(null);
  const [submitError, setSubmitError] = useState<string | null>(null);
  const [pollError, setPollError] = useState<string | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [lastUpdated, setLastUpdated] = useState<string | null>(null);

  const form = useForm<RunRequest>({
    mode: 'uncontrolled',
    initialValues: defaultRunValues,
    validate: {
      initial_cash: (value) => (value > 0 ? null : 'Cash must be positive'),
      fee_bps: (value) => (value >= 0 ? null : 'Fee cannot be negative'),
      slippage_bps: (value) => (value >= 0 ? null : 'Slippage cannot be negative'),
      short_window: (value, values) => (value > 0 && value < values.long_window ? null : 'Short window must be smaller'),
      long_window: (value, values) => (value > values.short_window ? null : 'Long window must be larger'),
    },
  });

  const currentStatus = runDetails?.status ?? activeRun?.status;
  const hasActiveRun = Boolean(activeRun?.id);
  const isRunWorking = currentStatus === 'queued' || currentStatus === 'running';
  const result = runDetails?.result;

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
    const runID = activeRun?.id;
    if (!runID) {
      return;
    }

    const stableRunID = runID;
    let cancelled = false;
    let intervalID = 0;

    // Poll the run endpoint until the worker reaches a terminal status
    async function pollRun() {
      try {
        const response = await fetchRun(stableRunID);
        if (cancelled) {
          return;
        }

        setRunDetails(response);
        setPollError(null);
        setLastUpdated(new Date().toLocaleTimeString());

        if (response.status === 'completed' || response.status === 'failed') {
          window.clearInterval(intervalID);
        }
      } catch (error) {
        if (!cancelled) {
          setPollError(error instanceof Error ? error.message : 'Unable to fetch run status');
        }
      }
    }

    void pollRun();
    intervalID = window.setInterval(pollRun, 1500);

    return () => {
      cancelled = true;
      window.clearInterval(intervalID);
    };
  }, [activeRun?.id]);

  async function handleSubmit(values: RunRequest) {
    setSubmitError(null);
    setPollError(null);
    setIsSubmitting(true);
    setRunDetails(null);
    setLastUpdated(null);

    try {
      const response = await createRun(values);
      setActiveRun(response);
      setRunDetails({
        id: response.id,
        status: response.status,
      });
    } catch (error) {
      setSubmitError(error instanceof Error ? error.message : 'Unable to create run');
    } finally {
      setIsSubmitting(false);
    }
  }

  const resultRows = useMemo(() => buildResultRows(result), [result]);

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
                Moving Average Crossover
              </Text>
            </Box>
          </Group>

          <Badge color={apiHealthColor(apiHealth)} variant="light" size="lg" leftSection={<IconActivity size={14} />}>
            {apiHealthLabel(apiHealth)}
          </Badge>
        </Group>
      </AppShell.Header>

      <AppShell.Main className="app-main">
        <Stack gap="lg">
          <SimpleGrid cols={{ base: 1, lg: 2 }} spacing="lg" verticalSpacing="lg">
            <Paper className="panel" p="lg" radius="md">
              <form onSubmit={form.onSubmit(handleSubmit)}>
                <Stack gap="md">
                  <Group justify="space-between" align="flex-start">
                    <Box>
                      <Text fw={700} size="lg">
                        New run
                      </Text>
                      <Text size="sm" c="dimmed">
                        Configure strategy parameters
                      </Text>
                    </Box>
                    <Badge color="teal" variant="light">
                      SPY
                    </Badge>
                  </Group>

                  <SimpleGrid cols={{ base: 1, sm: 2 }} spacing="md">
                    <Select
                      label="Ticker"
                      data={[{ value: 'SPY', label: 'SPY' }]}
                      key={form.key('ticker')}
                      {...form.getInputProps('ticker')}
                    />
                    <NumberInput
                      label="Initial cash"
                      min={1}
                      step={1000}
                      prefix="$"
                      thousandSeparator=","
                      key={form.key('initial_cash')}
                      {...form.getInputProps('initial_cash')}
                    />
                    <NumberInput
                      label="Short window"
                      min={1}
                      step={1}
                      key={form.key('short_window')}
                      {...form.getInputProps('short_window')}
                    />
                    <NumberInput
                      label="Long window"
                      min={2}
                      step={1}
                      key={form.key('long_window')}
                      {...form.getInputProps('long_window')}
                    />
                    <NumberInput
                      label="Fee"
                      min={0}
                      step={1}
                      suffix=" bps"
                      key={form.key('fee_bps')}
                      {...form.getInputProps('fee_bps')}
                    />
                    <NumberInput
                      label="Slippage"
                      min={0}
                      step={1}
                      suffix=" bps"
                      key={form.key('slippage_bps')}
                      {...form.getInputProps('slippage_bps')}
                    />
                  </SimpleGrid>

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
                    Run backtest
                  </Button>
                </Stack>
              </form>
            </Paper>

            <Paper className="panel status-panel" p="lg" radius="md">
              <LoadingOverlay visible={isRunWorking} overlayProps={{ blur: 1 }} />
              <Stack gap="md">
                <Group justify="space-between" align="flex-start">
                  <Box>
                    <Text fw={700} size="lg">
                      Latest run
                    </Text>
                    <Text size="sm" c="dimmed">
                      {activeRun?.id ?? 'No run submitted'}
                    </Text>
                  </Box>
                  <StatusBadge status={currentStatus} />
                </Group>

                <Divider />

                {hasActiveRun ? (
                  <Stack gap="sm">
                    <RunDetailRow label="Run ID" value={activeRun?.id ?? '-'} />
                    <RunDetailRow label="Status URL" value={activeRun?.status_url ?? '-'} />
                    <RunDetailRow label="Last update" value={lastUpdated ?? '-'} />
                    {runDetails?.error ? <RunDetailRow label="Error" value={runDetails.error} tone="red" /> : null}
                  </Stack>
                ) : (
                  <Stack align="center" justify="center" className="empty-state">
                    <ThemeIcon color="gray" variant="light" size={52} radius="md">
                      <IconClock size={28} />
                    </ThemeIcon>
                    <Text c="dimmed" ta="center">
                      Waiting for a run
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
              label="Strategy value"
              value={result ? formatCurrency(result.strategy_final_value) : '-'}
              icon={<IconCurrencyDollar size={22} />}
              color="teal"
            />
            <MetricCard
              label="Strategy return"
              value={result ? formatPercent(result.strategy_return) : '-'}
              icon={<IconTrendingUp size={22} />}
              color={result && result.strategy_return < 0 ? 'red' : 'green'}
            />
            <MetricCard
              label="Excess return"
              value={result ? formatPercent(result.excess_return) : '-'}
              icon={<IconRefresh size={22} />}
              color={result && result.excess_return < 0 ? 'orange' : 'teal'}
            />
            <MetricCard
              label="Max drawdown"
              value={result ? formatPercent(result.max_drawdown) : '-'}
              icon={<IconTrendingDown size={22} />}
              color="red"
            />
          </SimpleGrid>

          <Paper className="panel" p="lg" radius="md">
            <Stack gap="md">
              <Group justify="space-between">
                <Box>
                  <Text fw={700} size="lg">
                    Result details
                  </Text>
                  <Text size="sm" c="dimmed">
                    {result ? result.strategy : 'No completed result'}
                  </Text>
                </Box>
                {result ? (
                  <Badge color="teal" variant="light" leftSection={<IconCheck size={14} />}>
                    Completed
                  </Badge>
                ) : null}
              </Group>

              <Table.ScrollContainer minWidth={620}>
                <Table highlightOnHover verticalSpacing="sm">
                  <Table.Thead>
                    <Table.Tr>
                      <Table.Th>Metric</Table.Th>
                      <Table.Th>Value</Table.Th>
                    </Table.Tr>
                  </Table.Thead>
                  <Table.Tbody>
                    {resultRows.map((row) => (
                      <Table.Tr key={row.label}>
                        <Table.Td>{row.label}</Table.Td>
                        <Table.Td>
                          <Text fw={600}>{row.value}</Text>
                        </Table.Td>
                      </Table.Tr>
                    ))}
                  </Table.Tbody>
                </Table>
              </Table.ScrollContainer>
            </Stack>
          </Paper>
        </Stack>
      </AppShell.Main>
    </AppShell>
  );
}

function StatusBadge({ status }: { status?: JobStatus }) {
  return (
    <Badge color={statusColor(status)} variant="light" size="lg">
      {status ?? 'idle'}
    </Badge>
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

function RunDetailRow({ label, value, tone }: { label: string; value: string; tone?: string }) {
  return (
    <Group justify="space-between" gap="md" wrap="nowrap" className="detail-row">
      <Text size="sm" c="dimmed">
        {label}
      </Text>
      <Text size="sm" fw={600} c={tone} ta="right" className="detail-value">
        {value}
      </Text>
    </Group>
  );
}

function buildResultRows(result?: RunResult) {
  if (!result) {
    return [
      { label: 'Strategy final value', value: '-' },
      { label: 'Strategy return', value: '-' },
      { label: 'Benchmark final value', value: '-' },
      { label: 'Benchmark return', value: '-' },
      { label: 'Excess return', value: '-' },
      { label: 'Max drawdown', value: '-' },
      { label: 'Win rate', value: '-' },
      { label: 'Signals', value: '-' },
      { label: 'Trades', value: '-' },
    ];
  }

  return [
    { label: 'Strategy final value', value: formatCurrency(result.strategy_final_value) },
    { label: 'Strategy return', value: formatPercent(result.strategy_return) },
    { label: 'Benchmark final value', value: formatCurrency(result.benchmark_final_value) },
    { label: 'Benchmark return', value: formatPercent(result.benchmark_return) },
    { label: 'Excess return', value: formatPercent(result.excess_return) },
    { label: 'Max drawdown', value: formatPercent(result.max_drawdown) },
    { label: 'Win rate', value: formatPercent(result.win_rate) },
    { label: 'Signals', value: result.signals.toString() },
    { label: 'Trades', value: result.trades.toString() },
  ];
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

function statusColor(status?: JobStatus) {
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
