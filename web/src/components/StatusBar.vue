<script setup lang="ts">
import { computed } from 'vue'

import { formatSuccessRate, formatTimestamp, formatUptime } from '../lib/dashboard'
import type { ConnectionState, DashboardStats } from '../types/dashboard'
import MetricCard from './MetricCard.vue'

const props = defineProps<{
  stats: DashboardStats
  lastUpdatedAt: string | null
  connectionState: ConnectionState
  providerCount: number
}>()

const lastUpdatedLabel = computed(() => {
  if (!props.lastUpdatedAt) {
    return '等待首次刷新'
  }

  return formatTimestamp(props.lastUpdatedAt)
})

const connectionLabel = computed(() => {
  if (props.connectionState === 'live') {
    return '实时推送已连接'
  }

  if (props.connectionState === 'offline') {
    return '接口暂不可达'
  }

  return '轮询刷新中'
})
</script>

<template>
  <section class="status-bar">
    <div class="status-bar__metrics">
      <MetricCard
        label="总请求"
        :value="stats.totalRequests.toLocaleString('zh-CN')"
        hint="优先显示接口摘要，摘要缺失时回退到当前日志窗口。"
        tone="primary"
      >
        <template #icon>
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
            <path d="M4 18h16"></path>
            <path d="M6 14l3-3 3 2 5-6 1 1"></path>
            <path d="M17 7h3v3"></path>
          </svg>
        </template>
      </MetricCard>

      <MetricCard
        label="成功率"
        :value="formatSuccessRate(stats.successRate)"
        hint="2xx 请求计入成功，请求量增加后会即时同步更新。"
        tone="success"
      >
        <template #icon>
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
            <path d="M5 12.5l4.2 4.2L19 7"></path>
            <path d="M19 12v6H5V6h8"></path>
          </svg>
        </template>
      </MetricCard>

      <MetricCard
        label="运行时间"
        :value="formatUptime(stats.uptimeMs)"
        hint="优先采用接口摘要，没有摘要时按日志时间跨度估算。"
        tone="info"
      >
        <template #icon>
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
            <circle cx="12" cy="12" r="8"></circle>
            <path d="M12 8v5l3 2"></path>
          </svg>
        </template>
      </MetricCard>

      <MetricCard
        label="Provider 数量"
        :value="providerCount.toLocaleString('zh-CN')"
        hint="按当前配置接口返回的 Provider 列表实时统计。"
        tone="warning"
      >
        <template #icon>
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
            <path d="M4 8h7"></path>
            <path d="M13 8h7"></path>
            <path d="M6.5 16h11"></path>
            <circle cx="7" cy="8" r="3"></circle>
            <circle cx="17" cy="8" r="3"></circle>
            <circle cx="12" cy="16" r="3"></circle>
          </svg>
        </template>
      </MetricCard>
    </div>

    <div class="status-bar__meta">
      <span class="status-bar__meta-item">最近刷新：{{ lastUpdatedLabel }}</span>
      <span class="status-bar__meta-item" :class="`status-bar__meta-item--${connectionState}`">
        数据通道：{{ connectionLabel }}
      </span>
    </div>
  </section>
</template>

<style scoped>
.status-bar {
  display: grid;
  gap: 14px;
  margin-bottom: 20px;
}

.status-bar__metrics {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 16px;
}

.status-bar__meta {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
  padding: 0 4px;
}

.status-bar__meta-item {
  display: inline-flex;
  align-items: center;
  min-height: 34px;
  padding: 0 12px;
  border: 1px solid var(--border-color);
  border-radius: 999px;
  background: rgba(29, 27, 24, 0.78);
  color: var(--text-secondary);
  font-size: 0.86rem;
}

.status-bar__meta-item--live {
  border-color: var(--status-success-border);
  color: var(--status-success-text);
}

.status-bar__meta-item--offline {
  border-color: var(--status-danger-border);
  color: var(--status-danger-text);
}

@media (max-width: 960px) {
  .status-bar__metrics {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 720px) {
  .status-bar__metrics {
    grid-template-columns: 1fr;
  }

  .status-bar__meta {
    flex-direction: column;
  }
}
</style>
