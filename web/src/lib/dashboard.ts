import type {
  DashboardStats,
  DashboardStatsSeed,
  KnownProtocol,
  LogsResponseData,
  ProviderRecord,
  RequestLogRecord,
  RouteRecord,
} from '../types/dashboard'

const REQUEST_SUCCESS_MIN = 200
const REQUEST_SUCCESS_MAX = 399

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null
}

function getString(record: Record<string, unknown>, keys: string[], fallback = '—'): string {
  for (const key of keys) {
    const value = record[key]
    if (typeof value === 'string') {
      const trimmedValue = value.trim()
      if (trimmedValue) {
        return trimmedValue
      }
    }
  }

  return fallback
}

function getNumber(record: Record<string, unknown>, keys: string[], fallback = 0): number {
  for (const key of keys) {
    const value = record[key]
    if (typeof value === 'number' && Number.isFinite(value)) {
      return value
    }

    if (typeof value === 'string') {
      const parsedValue = Number.parseFloat(value)
      if (Number.isFinite(parsedValue)) {
        return parsedValue
      }
    }
  }

  return fallback
}

function getOptionalNumber(record: Record<string, unknown>, keys: string[]): number | undefined {
  for (const key of keys) {
    const value = record[key]
    if (typeof value === 'number' && Number.isFinite(value)) {
      return value
    }

    if (typeof value === 'string') {
      const parsedValue = Number.parseFloat(value)
      if (Number.isFinite(parsedValue)) {
        return parsedValue
      }
    }
  }

  return undefined
}

function getArray(record: Record<string, unknown>, keys: string[]): unknown[] {
  for (const key of keys) {
    const value = record[key]
    if (Array.isArray(value)) {
      return value
    }
  }

  return []
}

function normalizeTimestamp(value: unknown): string {
  if (typeof value === 'number' && Number.isFinite(value)) {
    const milliseconds = value > 1_000_000_000_000 ? value : value * 1000
    return new Date(milliseconds).toISOString()
  }

  if (typeof value === 'string') {
    const trimmedValue = value.trim()
    if (!trimmedValue) {
      return new Date().toISOString()
    }

    const numericValue = Number(trimmedValue)
    if (Number.isFinite(numericValue)) {
      const milliseconds = numericValue > 1_000_000_000_000 ? numericValue : numericValue * 1000
      return new Date(milliseconds).toISOString()
    }

    const parsedValue = Date.parse(trimmedValue)
    if (!Number.isNaN(parsedValue)) {
      return new Date(parsedValue).toISOString()
    }
  }

  return new Date().toISOString()
}

function normalizeProtocol(value: unknown): string {
  if (typeof value !== 'string') {
    return 'unknown'
  }

  const protocol = value.trim().toLowerCase()
  if (!protocol) {
    return 'unknown'
  }

  return protocol
}

function extractPayloadArray(payload: unknown, keys: string[]): unknown[] {
  if (Array.isArray(payload)) {
    return payload
  }

  if (!isRecord(payload)) {
    return []
  }

  const nestedArray = getArray(payload, keys)
  if (nestedArray.length > 0) {
    return nestedArray
  }

  if (
    keys.includes('logs') &&
    ('timestamp' in payload || 'time' in payload || 'created_at' in payload || 'model' in payload)
  ) {
    return [payload]
  }

  return []
}

function uniqueLogs(logs: RequestLogRecord[]): RequestLogRecord[] {
  const map = new Map<string, RequestLogRecord>()

  for (const log of logs) {
    if (!map.has(log.id)) {
      map.set(log.id, log)
    }
  }

  return [...map.values()].sort((left, right) => Date.parse(right.timestamp) - Date.parse(left.timestamp))
}

function getSuccessfulRequests(logs: RequestLogRecord[]): number {
  return logs.filter((log) => log.statusCode >= REQUEST_SUCCESS_MIN && log.statusCode <= REQUEST_SUCCESS_MAX).length
}

function getUptimeMs(logs: RequestLogRecord[]): number {
  if (logs.length === 0) {
    return 0
  }

  const timestamps = logs
    .map((log) => Date.parse(log.timestamp))
    .filter((value) => !Number.isNaN(value))

  if (timestamps.length === 0) {
    return 0
  }

  const oldestLog = Math.min(...timestamps)
  return Math.max(Date.now() - oldestLog, 0)
}

export function normalizeProvidersResponse(payload: unknown): ProviderRecord[] {
  const providers = extractPayloadArray(payload, ['providers', 'data', 'items'])

  return providers
    .map((provider, index) => {
      const record = isRecord(provider) ? provider : {}

      return {
        name: getString(record, ['name'], `provider-${index + 1}`),
        protocol: normalizeProtocol(getString(record, ['protocol'], 'unknown')),
        endpoint: getString(record, ['endpoint']),
      }
    })
    .filter((provider) => provider.name !== '—')
}

export function normalizeRoutesResponse(payload: unknown): RouteRecord[] {
  const routes = extractPayloadArray(payload, ['routes', 'data', 'items'])

  return routes
    .map((route) => {
      const record = isRecord(route) ? route : {}

      return {
        matchModel: getString(record, ['match_model', 'matchModel']),
        provider: getString(record, ['provider', 'provider_name', 'providerName']),
      }
    })
    .filter((route) => route.matchModel !== '—')
}

export function normalizeLogsResponse(payload: unknown): LogsResponseData {
  const container = isRecord(payload) ? payload : {}
  const uptimeMs = getOptionalNumber(container, ['uptime_ms', 'runtime_ms'])
  const uptimeSeconds = getOptionalNumber(container, ['uptime_seconds', 'runtime_seconds'])
  const logs = extractPayloadArray(payload, ['logs', 'data', 'items', 'entries'])
    .map((log, index) => {
      const record = isRecord(log) ? log : {}
      const timestamp = normalizeTimestamp(
        record.timestamp ?? record.time ?? record.created_at ?? record.createdAt ?? record.ts,
      )
      const provider = getString(record, ['provider', 'target_provider', 'provider_name', 'targetProvider'])
      const model = getString(record, ['model', 'model_name', 'request_model', 'modelName'])
      const sourceProtocol = normalizeProtocol(
        record.source_protocol ?? record.request_protocol ?? record.protocol ?? record.sourceProtocol,
      )
      const statusCode = Math.round(
        getNumber(record, ['status_code', 'status', 'statusCode', 'http_status', 'httpStatus']),
      )
      const latencyMs = Math.round(
        getNumber(record, ['latency_ms', 'latency', 'latencyMs', 'duration_ms', 'elapsed_ms']),
      )
      const id = getString(record, ['id'], `${timestamp}-${provider}-${model}-${statusCode || index}`)

      return {
        id,
        timestamp,
        sourceProtocol,
        provider,
        model,
        statusCode,
        latencyMs,
      }
    })

  return {
    logs: uniqueLogs(logs),
    summary: {
      uptimeMs: uptimeMs ?? (uptimeSeconds !== undefined ? uptimeSeconds * 1000 : undefined),
      totalRequests: getOptionalNumber(container, ['total_requests', 'totalRequests', 'count']),
      successfulRequests: getOptionalNumber(container, ['successful_requests', 'successfulRequests']),
      successRate: getOptionalNumber(container, ['success_rate', 'successRate']),
    },
  }
}

export function buildDashboardStats(
  logs: RequestLogRecord[],
  summary: DashboardStatsSeed = {},
): DashboardStats {
  const totalRequests = Math.max(Math.round(summary.totalRequests ?? logs.length), 0)
  const fallbackSuccessfulRequests = getSuccessfulRequests(logs)
  const successRate = summary.successRate
  const normalizedSuccessRate =
    successRate === undefined
      ? totalRequests > 0
        ? ((summary.successfulRequests ?? fallbackSuccessfulRequests) / totalRequests) * 100
        : 0
      : successRate <= 1
        ? successRate * 100
        : successRate
  const successfulRequests = Math.max(
    Math.round(summary.successfulRequests ?? (normalizedSuccessRate / 100) * totalRequests),
    0,
  )

  return {
    uptimeMs: Math.max(Math.round(summary.uptimeMs ?? getUptimeMs(logs)), 0),
    totalRequests,
    successfulRequests,
    successRate: Number(Math.min(Math.max(normalizedSuccessRate, 0), 100).toFixed(1)),
  }
}

export function mergeLogRecords(
  currentLogs: RequestLogRecord[],
  incomingLogs: RequestLogRecord[],
  limit = 80,
): RequestLogRecord[] {
  return uniqueLogs([...incomingLogs, ...currentLogs]).slice(0, limit)
}

export function formatTimestamp(timestamp: string): string {
  const date = new Date(timestamp)
  if (Number.isNaN(date.getTime())) {
    return '时间未知'
  }

  return new Intl.DateTimeFormat('zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    hour12: false,
  }).format(date)
}

export function formatUptime(uptimeMs: number): string {
  if (uptimeMs <= 0) {
    return '0 秒'
  }

  const totalSeconds = Math.floor(uptimeMs / 1000)
  const days = Math.floor(totalSeconds / 86400)
  const hours = Math.floor((totalSeconds % 86400) / 3600)
  const minutes = Math.floor((totalSeconds % 3600) / 60)
  const seconds = totalSeconds % 60
  const parts: string[] = []

  if (days > 0) {
    parts.push(`${days} 天`)
  }

  if (hours > 0) {
    parts.push(`${hours} 小时`)
  }

  if (minutes > 0 && parts.length < 2) {
    parts.push(`${minutes} 分钟`)
  }

  if (parts.length === 0) {
    parts.push(`${seconds} 秒`)
  }

  return parts.slice(0, 2).join(' ')
}

export function formatSuccessRate(successRate: number): string {
  return `${successRate.toFixed(successRate % 1 === 0 ? 0 : 1)}%`
}

export function formatLatency(latencyMs: number): string {
  return `${Math.max(Math.round(latencyMs), 0)} ms`
}

export function getProtocolLabel(protocol: string): string {
  const normalizedProtocol = normalizeProtocol(protocol) as KnownProtocol

  switch (normalizedProtocol) {
    case 'openai':
      return 'OpenAI'
    case 'claude':
      return 'Claude'
    case 'gemini':
      return 'Gemini'
    default:
      return 'Unknown'
  }
}

export function getStatusTone(statusCode: number): 'ok' | 'warn' | 'error' | 'idle' {
  if (statusCode >= 200 && statusCode < 300) {
    return 'ok'
  }

  if (statusCode >= 300 && statusCode < 400) {
    return 'warn'
  }

  if (statusCode >= 400) {
    return 'error'
  }

  return 'idle'
}
