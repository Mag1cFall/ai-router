import { buildDashboardStats, normalizeLogsResponse, normalizeProvidersResponse, normalizeRoutesResponse } from '../lib/dashboard'
import type { LogsResult, ProviderRecord, RouteRecord } from '../types/dashboard'

const API_BASE_URL = (import.meta.env.VITE_API_BASE_URL || '').replace(/\/$/, '')
const ENABLE_LOGS_WS = import.meta.env.VITE_ENABLE_LOGS_WS === 'true'

function getRequestUrl(path: string): string {
  return `${API_BASE_URL}${path}`
}

async function requestJson<T>(path: string, signal?: AbortSignal): Promise<T> {
  const response = await fetch(getRequestUrl(path), {
    method: 'GET',
    headers: {
      Accept: 'application/json',
    },
    signal,
  })

  if (!response.ok) {
    throw new Error(`请求失败：${response.status} ${response.statusText}`)
  }

  return (await response.json()) as T
}

export async function fetchProviders(signal?: AbortSignal): Promise<ProviderRecord[]> {
  const payload = await requestJson<unknown>('/api/providers', signal)
  return normalizeProvidersResponse(payload)
}

export async function fetchRoutes(signal?: AbortSignal): Promise<RouteRecord[]> {
  const payload = await requestJson<unknown>('/api/routes', signal)
  return normalizeRoutesResponse(payload)
}

export async function fetchLogs(signal?: AbortSignal): Promise<LogsResult> {
  const payload = await requestJson<unknown>('/api/logs', signal)
  const data = normalizeLogsResponse(payload)

  return {
    ...data,
    stats: buildDashboardStats(data.logs, data.summary),
  }
}

export function openLogsStream(onMessage: (logs: LogsResult) => void): WebSocket | null {
  if (!ENABLE_LOGS_WS || typeof window === 'undefined') {
    return null
  }

  const rawUrl = import.meta.env.VITE_LOGS_WS_URL?.trim()
  const streamUrl = rawUrl
    ? rawUrl
    : `${window.location.protocol === 'https:' ? 'wss' : 'ws'}://${window.location.host}/api/logs/ws`

  const socket = new WebSocket(streamUrl)

  socket.addEventListener('message', (event) => {
    const parsedPayload = JSON.parse(event.data) as unknown
    const data = normalizeLogsResponse(parsedPayload)

    onMessage({
      ...data,
      stats: buildDashboardStats(data.logs, data.summary),
    })
  })

  return socket
}
