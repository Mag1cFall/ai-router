<script setup lang="ts">
import { computed } from 'vue'

import { formatSuccessRate, formatTimestamp, formatUptime } from '../lib/dashboard'
import type { ConnectionState, DashboardStats } from '../types/dashboard'
import MetricCard from './MetricCard.vue'

const props = defineProps<{
  stats: DashboardStats
  lastUpdatedAt: string | null
  connectionState: ConnectionState
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
        label="运行时间"
        :value="formatUptime(stats.uptimeMs)"
        hint="优先采用接口摘要，没有摘要时按日志时间跨度估算。"
      />
      <MetricCard
        label="总请求数"
        :value="stats.totalRequests.toLocaleString('zh-CN')"
        hint="来自日志接口返回值；没有摘要时使用当前日志窗口数量。"
      />
      <MetricCard
        label="成功率"
        :value="formatSuccessRate(stats.successRate)"
        hint="2xx 与 3xx 视为成功请求。"
      />
    </div>

    <div class="status-bar__meta">
      <span>最近刷新：{{ lastUpdatedLabel }}</span>
      <span>数据通道：{{ connectionLabel }}</span>
    </div>
  </section>
</template>

<style scoped>
.status-bar {
  display: grid;
  gap: 16px;
  margin-bottom: 24px;
}

.status-bar__metrics {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 16px;
}

.status-bar__meta {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  padding: 0 4px;
  color: rgba(148, 163, 184, 0.92);
  font-size: 0.9rem;
}

@media (max-width: 960px) {
  .status-bar__metrics {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 720px) {
  .status-bar__meta {
    flex-direction: column;
  }
}
</style>
