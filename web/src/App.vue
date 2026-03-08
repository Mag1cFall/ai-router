<script setup lang="ts">
import { computed } from 'vue'

import RequestLogList from './components/RequestLogList.vue'
import RoutesTable from './components/RoutesTable.vue'
import SectionCard from './components/SectionCard.vue'
import StatusBar from './components/StatusBar.vue'
import ProvidersTable from './components/ProvidersTable.vue'
import { useDashboard } from './composables/useDashboard'

const {
  providers,
  routes,
  logs,
  stats,
  errors,
  loading,
  refreshing,
  lastUpdatedAt,
  connectionState,
  refresh,
} = useDashboard()

const providerCount = computed(() => providers.value.length)

const transportLabel = computed(() => {
  if (connectionState.value === 'live') {
    return 'WebSocket 实时推送'
  }

  if (connectionState.value === 'offline') {
    return '接口离线'
  }

  return '轮询刷新中'
})

const connectionTitle = computed(() => {
  if (connectionState.value === 'live') {
    return '已连接'
  }

  if (connectionState.value === 'offline') {
    return '离线'
  }

  return '同步中'
})
</script>

<template>
  <div class="dashboard-page">
    <div class="dashboard-page__glow" aria-hidden="true"></div>

    <header class="dashboard-header">
      <div class="dashboard-header__content">
        <p class="dashboard-header__eyebrow">AI Router Dashboard</p>
        <h1>协议路由管理面板</h1>
        <p class="dashboard-header__description">
          统一查看 Provider、模型路由规则与实时请求轨迹，快速确认 OpenAI、Claude、Gemini 的转发情况。
        </p>
      </div>

      <div class="dashboard-header__actions">
        <div class="connection-indicator" :class="`connection-indicator--${connectionState}`">
          <span class="connection-indicator__dot" aria-hidden="true"></span>
          <div class="connection-indicator__content">
            <strong>{{ connectionTitle }}</strong>
            <span>{{ transportLabel }}</span>
          </div>
        </div>

        <button
          class="dashboard-header__button"
          type="button"
          :disabled="refreshing"
          aria-label="刷新管理面板数据"
          @click="refresh"
        >
          {{ refreshing ? '刷新中…' : '立即刷新' }}
        </button>
      </div>
    </header>

    <StatusBar
      :stats="stats"
      :last-updated-at="lastUpdatedAt"
      :connection-state="connectionState"
      :provider-count="providerCount"
    />

    <main class="dashboard-grid">
      <SectionCard
        title="Provider 列表"
        subtitle="GET /api/providers"
        :count="providers.length"
        count-label="Provider"
        :error="errors.providers"
      >
        <ProvidersTable
          :providers="providers"
          :loading="loading && providers.length === 0"
        />
      </SectionCard>

      <SectionCard
        title="Route 规则"
        subtitle="GET /api/routes"
        :count="routes.length"
        count-label="规则"
        :error="errors.routes"
      >
        <RoutesTable :routes="routes" :loading="loading && routes.length === 0" />
      </SectionCard>

      <SectionCard
        class="dashboard-grid__full"
        title="实时请求日志"
        subtitle="GET /api/logs · 最新在上"
        :count="logs.length"
        count-label="条"
        :error="errors.logs"
      >
        <RequestLogList :logs="logs" :loading="loading && logs.length === 0" />
      </SectionCard>
    </main>
  </div>
</template>

<style scoped>
.dashboard-page {
  position: relative;
  width: min(1280px, calc(100vw - 32px));
  margin: 0 auto;
  padding: 40px 0 56px;
}

.dashboard-page__glow {
  position: absolute;
  inset: 0;
  z-index: -1;
  background:
    radial-gradient(circle at top left, rgba(139, 134, 128, 0.18), transparent 32%),
    radial-gradient(circle at 92% 8%, rgba(16, 185, 129, 0.08), transparent 18%),
    radial-gradient(circle at 52% 100%, rgba(209, 138, 80, 0.08), transparent 22%);
  filter: blur(18px);
  pointer-events: none;
}

.dashboard-header {
  display: flex;
  justify-content: space-between;
  gap: 20px;
  align-items: center;
  margin-bottom: 24px;
  padding: 24px 28px;
  border: 1px solid var(--border-color);
  border-radius: var(--radius-xl);
  background: linear-gradient(180deg, rgba(29, 27, 24, 0.96), rgba(21, 20, 18, 0.92));
  box-shadow: var(--shadow-lg);
}

.dashboard-header__content {
  max-width: 760px;
}

.dashboard-header__eyebrow {
  margin: 0 0 10px;
  color: var(--text-secondary);
  font-size: 0.76rem;
  font-weight: 700;
  letter-spacing: 0.2em;
  text-transform: uppercase;
}

.dashboard-header h1 {
  margin: 0;
  color: var(--text-primary);
  font-size: clamp(2rem, 5vw, 3.1rem);
  line-height: 1.04;
  letter-spacing: -0.04em;
}

.dashboard-header__description {
  margin: 14px 0 0;
  max-width: 56ch;
  color: var(--text-secondary);
  font-size: 0.98rem;
  line-height: 1.7;
}

.dashboard-header__actions {
  display: flex;
  flex-direction: column;
  gap: 12px;
  align-items: flex-end;
  min-width: 220px;
}

.connection-indicator {
  display: inline-flex;
  align-items: center;
  gap: 12px;
  width: 100%;
  min-height: 62px;
  padding: 12px 16px;
  border: 1px solid var(--border-color);
  border-radius: var(--radius-lg);
  background: var(--bg-tertiary);
  box-shadow: var(--shadow-sm);
}

.connection-indicator__dot {
  width: 12px;
  height: 12px;
  flex-shrink: 0;
  border-radius: 999px;
  background: var(--primary-color);
}

.connection-indicator__content {
  display: grid;
  gap: 2px;
}

.connection-indicator__content strong {
  color: var(--text-primary);
  font-size: 0.95rem;
}

.connection-indicator__content span {
  color: var(--text-secondary);
  font-size: 0.82rem;
}

.connection-indicator--live {
  border-color: var(--status-success-border);
  background: linear-gradient(180deg, rgba(29, 27, 24, 0.96), rgba(6, 78, 59, 0.2));
}

.connection-indicator--live .connection-indicator__dot {
  background: var(--success-color);
  box-shadow: 0 0 0 4px rgba(16, 185, 129, 0.12);
  animation: status-pulse 1.8s ease-in-out infinite;
}

.connection-indicator--offline {
  border-color: var(--status-danger-border);
  background: linear-gradient(180deg, rgba(29, 27, 24, 0.96), rgba(198, 87, 70, 0.14));
}

.connection-indicator--offline .connection-indicator__dot {
  background: var(--error-color);
}

@keyframes status-pulse {
  0%,
  100% {
    transform: scale(1);
    opacity: 1;
  }

  50% {
    transform: scale(1.18);
    opacity: 0.75;
  }
}

.dashboard-header__button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 100%;
  min-height: 44px;
  padding: 0 16px;
  border: 1px solid rgba(139, 134, 128, 0.26);
  border-radius: var(--radius-md);
  color: var(--text-primary);
  background: linear-gradient(180deg, rgba(139, 134, 128, 0.14), rgba(139, 134, 128, 0.08));
  font-size: 0.92rem;
  font-weight: 700;
  box-shadow: var(--shadow-sm);
}

.dashboard-header__button:hover:enabled {
  transform: translateY(-2px);
  border-color: var(--border-hover);
  background: linear-gradient(180deg, rgba(139, 134, 128, 0.22), rgba(139, 134, 128, 0.12));
  box-shadow: var(--shadow-md);
}

.dashboard-header__button:disabled {
  cursor: progress;
  opacity: 0.68;
}

.dashboard-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 20px;
}

.dashboard-grid__full {
  grid-column: 1 / -1;
}

@media (max-width: 1024px) {
  .dashboard-grid {
    grid-template-columns: 1fr;
  }

  .dashboard-grid__full {
    grid-column: auto;
  }
}

@media (max-width: 720px) {
  .dashboard-page {
    width: min(100vw - 20px, 1320px);
    padding: 24px 0 36px;
  }

  .dashboard-header {
    flex-direction: column;
    padding: 20px;
    align-items: stretch;
  }

  .dashboard-header__actions {
    width: 100%;
    align-items: stretch;
  }

  .dashboard-header__button,
  .connection-indicator {
    width: 100%;
  }
}
</style>
