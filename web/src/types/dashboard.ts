export type KnownProtocol = 'openai' | 'claude' | 'gemini' | 'unknown'

export type ConnectionState = 'live' | 'polling' | 'offline'

export interface ProviderRecord {
  name: string
  protocol: string
  endpoint: string
}

export interface RouteRecord {
  matchModel: string
  provider: string
}

export interface RequestLogRecord {
  id: string
  timestamp: string
  sourceProtocol: string
  provider: string
  model: string
  statusCode: number
  latencyMs: number
}

export interface DashboardStatsSeed {
  uptimeMs?: number
  totalRequests?: number
  successfulRequests?: number
  successRate?: number
}

export interface DashboardStats {
  uptimeMs: number
  totalRequests: number
  successfulRequests: number
  successRate: number
}

export interface LogsResponseData {
  logs: RequestLogRecord[]
  summary: DashboardStatsSeed
}

export interface LogsResult extends LogsResponseData {
  stats: DashboardStats
}

export interface SectionErrors {
  providers: string | null
  routes: string | null
  logs: string | null
}
