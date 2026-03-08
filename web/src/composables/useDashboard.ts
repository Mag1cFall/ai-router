import { computed, onBeforeUnmount, onMounted, reactive, ref } from 'vue'

import { buildDashboardStats, mergeLogRecords } from '../lib/dashboard'
import { fetchLogs, fetchProviders, fetchRoutes, openLogsStream } from '../services/dashboardApi'
import type {
  ConnectionState,
  DashboardStats,
  ProviderRecord,
  RequestLogRecord,
  RouteRecord,
  SectionErrors,
} from '../types/dashboard'

const REFRESH_INTERVAL_MS = 10_000
const LOG_LIMIT = 80

function createEmptyStats(): DashboardStats {
  return {
    uptimeMs: 0,
    totalRequests: 0,
    successfulRequests: 0,
    successRate: 0,
  }
}

function isAbortError(error: unknown): boolean {
  return error instanceof DOMException && error.name === 'AbortError'
}

function getErrorMessage(error: unknown): string {
  if (error instanceof Error) {
    return error.message
  }

  return '接口暂不可用'
}

export function useDashboard() {
  const providers = ref<ProviderRecord[]>([])
  const routes = ref<RouteRecord[]>([])
  const logs = ref<RequestLogRecord[]>([])
  const stats = ref<DashboardStats>(createEmptyStats())
  const loading = ref(true)
  const refreshing = ref(false)
  const lastUpdatedAt = ref<string | null>(null)
  const connectionState = ref<ConnectionState>('polling')
  const errors = reactive<SectionErrors>({
    providers: null,
    routes: null,
    logs: null,
  })

  let refreshTimer: ReturnType<typeof setInterval> | null = null
  let abortController: AbortController | null = null
  let logsSocket: WebSocket | null = null

  const hasData = computed(
    () => providers.value.length > 0 || routes.value.length > 0 || logs.value.length > 0,
  )

  async function refresh() {
    abortController?.abort()
    abortController = new AbortController()
    refreshing.value = true

    const [providersResult, routesResult, logsResult] = await Promise.allSettled([
      fetchProviders(abortController.signal),
      fetchRoutes(abortController.signal),
      fetchLogs(abortController.signal),
    ])

    if (providersResult.status === 'fulfilled') {
      providers.value = providersResult.value
      errors.providers = null
    } else if (!isAbortError(providersResult.reason)) {
      errors.providers = getErrorMessage(providersResult.reason)
    }

    if (routesResult.status === 'fulfilled') {
      routes.value = routesResult.value
      errors.routes = null
    } else if (!isAbortError(routesResult.reason)) {
      errors.routes = getErrorMessage(routesResult.reason)
    }

    if (logsResult.status === 'fulfilled') {
      logs.value = logsResult.value.logs.slice(0, LOG_LIMIT)
      stats.value = logsResult.value.stats
      errors.logs = null

      if (connectionState.value !== 'live') {
        connectionState.value = 'polling'
      }
    } else if (!isAbortError(logsResult.reason)) {
      errors.logs = getErrorMessage(logsResult.reason)
    }

    lastUpdatedAt.value = new Date().toISOString()
    loading.value = false
    refreshing.value = false

    if (!hasData.value && errors.logs) {
      connectionState.value = 'offline'
    }
  }

  function connectLogsStream() {
    logsSocket = openLogsStream((payload) => {
      logs.value = mergeLogRecords(logs.value, payload.logs, LOG_LIMIT)
      const hasSummary = Object.values(payload.summary).some((value) => value !== undefined)

      stats.value = hasSummary ? payload.stats : buildDashboardStats(logs.value)
      errors.logs = null
      lastUpdatedAt.value = new Date().toISOString()
    })

    if (!logsSocket) {
      return
    }

    logsSocket.addEventListener('open', () => {
      connectionState.value = 'live'
    })

    logsSocket.addEventListener('close', () => {
      if (connectionState.value === 'live') {
        connectionState.value = 'polling'
      }
    })

    logsSocket.addEventListener('error', () => {
      connectionState.value = 'polling'
    })
  }

  function startRefreshLoop() {
    refreshTimer = window.setInterval(() => {
      void refresh()
    }, REFRESH_INTERVAL_MS)
  }

  onMounted(() => {
    void refresh()
    connectLogsStream()
    startRefreshLoop()
  })

  onBeforeUnmount(() => {
    abortController?.abort()

    if (refreshTimer !== null) {
      window.clearInterval(refreshTimer)
    }

    logsSocket?.close()
  })

  return {
    providers,
    routes,
    logs,
    stats,
    loading,
    refreshing,
    lastUpdatedAt,
    connectionState,
    errors,
    refresh,
  }
}
