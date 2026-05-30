import type { ApiError, CreateRunResponse, RunRequest, RunStatusResponse } from './types';

const API_BASE_URL = import.meta.env.VITE_API_BASE_URL ?? '';

async function requestJSON<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(`${API_BASE_URL}${path}`, {
    headers: {
      'Content-Type': 'application/json',
      ...init?.headers,
    },
    ...init,
  });

  const payload = (await response.json().catch(() => null)) as ApiError | T | null;
  if (!response.ok) {
    const hasErrorMessage = typeof payload === 'object' && payload !== null && 'error' in payload;
    const message = hasErrorMessage ? payload.error : `Request failed with ${response.status}`;
    throw new Error(message);
  }

  return payload as T;
}

export async function fetchHealth(): Promise<{ status: string }> {
  return requestJSON<{ status: string }>('/health');
}

export async function createRun(request: RunRequest): Promise<CreateRunResponse> {
  return requestJSON<CreateRunResponse>('/api/runs', {
    method: 'POST',
    body: JSON.stringify(request),
  });
}

export async function fetchRun(id: string): Promise<RunStatusResponse> {
  return requestJSON<RunStatusResponse>(`/api/runs/${id}`);
}
