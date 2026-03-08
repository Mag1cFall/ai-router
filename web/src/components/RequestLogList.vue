<script setup lang="ts">
import { formatLatency, formatTimestamp, getStatusTone } from '../lib/dashboard'
import type { RequestLogRecord } from '../types/dashboard'
import ProtocolBadge from './ProtocolBadge.vue'

defineProps<{
  logs: RequestLogRecord[]
  loading: boolean
}>()
</script>

<template>
  <div v-if="loading" class="empty-state">正在加载请求日志…</div>

  <div v-else-if="logs.length === 0" class="empty-state">暂无日志数据。</div>

  <ul v-else class="log-list" aria-live="polite">
    <li v-for="log in logs" :key="log.id" class="log-item">
      <div class="log-item__top">
        <time :datetime="log.timestamp">{{ formatTimestamp(log.timestamp) }}</time>
        <ProtocolBadge :protocol="log.sourceProtocol" />
      </div>

      <div class="log-item__main">
        <div class="log-item__target">
          <span class="log-item__label">目标 Provider</span>
          <strong>{{ log.provider }}</strong>
        </div>

        <div class="log-item__model">
          <span class="log-item__label">模型</span>
          <code>{{ log.model }}</code>
        </div>

        <div class="log-item__metrics">
          <span class="status-pill" :class="`status-pill--${getStatusTone(log.statusCode)}`">
            {{ log.statusCode || '—' }}
          </span>
          <span class="latency-pill">{{ formatLatency(log.latencyMs) }}</span>
        </div>
      </div>
    </li>
  </ul>
</template>

<style scoped>
.log-list {
  display: grid;
  gap: 14px;
  margin: 0;
  padding: 0;
  list-style: none;
}

.log-item {
  display: grid;
  gap: 14px;
  padding: 18px;
  border: 1px solid rgba(148, 163, 184, 0.14);
  border-radius: 18px;
  background: linear-gradient(180deg, rgba(15, 23, 42, 0.82), rgba(2, 6, 23, 0.72));
}

.log-item__top,
.log-item__main {
  display: flex;
  justify-content: space-between;
  gap: 14px;
  align-items: center;
}

.log-item__top time {
  color: rgba(203, 213, 225, 0.88);
  font-size: 0.94rem;
  font-variant-numeric: tabular-nums;
}

.log-item__target,
.log-item__model {
  display: grid;
  gap: 6px;
}

.log-item__target strong,
.log-item__model code {
  color: #f8fafc;
  font-size: 1rem;
}

.log-item__model code {
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.log-item__label {
  color: rgba(148, 163, 184, 0.88);
  font-size: 0.78rem;
  font-weight: 700;
  letter-spacing: 0.06em;
  text-transform: uppercase;
}

.log-item__metrics {
  display: flex;
  align-items: center;
  gap: 10px;
}

.status-pill,
.latency-pill {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-height: 34px;
  padding: 0 12px;
  border-radius: 999px;
  font-weight: 800;
}

.status-pill--ok {
  color: #bbf7d0;
  background: rgba(20, 83, 45, 0.42);
}

.status-pill--warn {
  color: #fde68a;
  background: rgba(120, 53, 15, 0.42);
}

.status-pill--error {
  color: #fecaca;
  background: rgba(127, 29, 29, 0.4);
}

.status-pill--idle {
  color: #cbd5e1;
  background: rgba(51, 65, 85, 0.62);
}

.latency-pill {
  color: #dbeafe;
  background: rgba(30, 64, 175, 0.34);
}

.empty-state {
  display: grid;
  place-items: center;
  min-height: 240px;
  border: 1px dashed rgba(148, 163, 184, 0.18);
  border-radius: 18px;
  color: rgba(148, 163, 184, 0.9);
  text-align: center;
}

@media (max-width: 960px) {
  .log-item__main {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    align-items: flex-start;
  }

  .log-item__metrics {
    grid-column: 1 / -1;
    justify-content: flex-start;
  }
}

@media (max-width: 640px) {
  .log-item__top,
  .log-item__main {
    display: grid;
    grid-template-columns: 1fr;
  }

  .log-item__model code {
    white-space: normal;
    word-break: break-word;
  }
}
</style>
