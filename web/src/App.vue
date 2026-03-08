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

const transportLabel = computed(() => {
  if (connectionState.value === 'live') {
    return '实时流'
  }

  if (connectionState.value === 'offline') {
    return '接口离线'
  }

  return '轮询模式'
})
</script>

<template>
  <div class="app-shell">
    <div class="app-shell__glow" aria-hidden="true"></div>

    <header class="hero-panel">
      <div class="hero-panel__content">
        <p class="hero-panel__eyebrow">AI Router · Control Panel</p>
        <h1>协议路由管理面板</h1>
        <p class="hero-panel__description">
          统一查看 Provider、模型路由规则与实时请求轨迹，快速确认 OpenAI、Claude、Gemini 的转发情况。
        </p>
      </div>

      <div class="hero-panel__actions">
        <span class="hero-panel__chip" :class="`hero-panel__chip--${connectionState}`">
          {{ transportLabel }}
        </span>
        <button
          class="hero-panel__button"
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
        subtitle="GET /api/logs · WebSocket 可选"
        :count="logs.length"
        count-label="日志"
        :error="errors.logs"
      >
        <RequestLogList :logs="logs" :loading="loading && logs.length === 0" />
      </SectionCard>
    </main>
  </div>
</template>

<style scoped>
.app-shell {
  position: relative;
  width: min(1320px, calc(100vw - 32px));
  margin: 0 auto;
  padding: 48px 0 64px;
}

.app-shell__glow {
  position: absolute;
  inset: 0;
  z-index: -1;
  background:
    radial-gradient(circle at top left, rgba(59, 130, 246, 0.18), transparent 34%),
    radial-gradient(circle at 85% 12%, rgba(249, 115, 22, 0.16), transparent 26%),
    radial-gradient(circle at 50% 100%, rgba(34, 197, 94, 0.12), transparent 28%);
  filter: blur(24px);
  pointer-events: none;
}

.hero-panel {
  display: flex;
  justify-content: space-between;
  gap: 24px;
  align-items: flex-start;
  margin-bottom: 28px;
  padding: 28px;
  border: 1px solid rgba(148, 163, 184, 0.18);
  border-radius: 28px;
  background:
    linear-gradient(135deg, rgba(15, 23, 42, 0.94), rgba(15, 23, 42, 0.82)),
    linear-gradient(135deg, rgba(34, 197, 94, 0.1), rgba(59, 130, 246, 0.12));
  box-shadow: 0 28px 64px rgba(2, 6, 23, 0.36);
  backdrop-filter: blur(18px);
}

.hero-panel__content {
  max-width: 720px;
}

.hero-panel__eyebrow {
  margin: 0 0 12px;
  color: rgba(148, 163, 184, 0.92);
  font-family: 'Sora', 'Segoe UI', sans-serif;
  font-size: 0.78rem;
  font-weight: 700;
  letter-spacing: 0.22em;
  text-transform: uppercase;
}

.hero-panel h1 {
  margin: 0;
  color: #f8fafc;
  font-family: 'Sora', 'Segoe UI', sans-serif;
  font-size: clamp(2.2rem, 5vw, 3.5rem);
  line-height: 1.02;
}

.hero-panel__description {
  margin: 16px 0 0;
  max-width: 56ch;
  color: rgba(226, 232, 240, 0.82);
  font-size: 1rem;
  line-height: 1.75;
}

.hero-panel__actions {
  display: flex;
  flex-direction: column;
  gap: 12px;
  align-items: flex-end;
  min-width: 180px;
}

.hero-panel__chip {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-height: 42px;
  padding: 0 16px;
  border: 1px solid rgba(148, 163, 184, 0.18);
  border-radius: 999px;
  color: #e2e8f0;
  background: rgba(15, 23, 42, 0.7);
  font-size: 0.92rem;
  font-weight: 700;
}

.hero-panel__chip--live {
  border-color: rgba(34, 197, 94, 0.45);
  color: #bbf7d0;
  background: rgba(20, 83, 45, 0.45);
}

.hero-panel__chip--polling {
  border-color: rgba(59, 130, 246, 0.35);
  color: #bfdbfe;
  background: rgba(30, 41, 59, 0.78);
}

.hero-panel__chip--offline {
  border-color: rgba(248, 113, 113, 0.35);
  color: #fecaca;
  background: rgba(69, 10, 10, 0.44);
}

.hero-panel__button {
  min-width: 150px;
  min-height: 46px;
  padding: 0 18px;
  border: 1px solid rgba(96, 165, 250, 0.3);
  border-radius: 14px;
  color: #eff6ff;
  background: linear-gradient(135deg, rgba(37, 99, 235, 0.95), rgba(30, 64, 175, 0.95));
  font-weight: 700;
  letter-spacing: 0.02em;
  box-shadow: 0 12px 28px rgba(30, 64, 175, 0.32);
}

.hero-panel__button:hover:enabled {
  transform: translateY(-1px);
  box-shadow: 0 18px 34px rgba(30, 64, 175, 0.4);
}

.hero-panel__button:disabled {
  cursor: progress;
  opacity: 0.72;
}

.dashboard-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 24px;
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
  .app-shell {
    width: min(100vw - 20px, 1320px);
    padding: 22px 0 40px;
  }

  .hero-panel {
    flex-direction: column;
    padding: 20px;
  }

  .hero-panel__actions {
    width: 100%;
    align-items: stretch;
  }

  .hero-panel__button,
  .hero-panel__chip {
    width: 100%;
  }
}
</style>
