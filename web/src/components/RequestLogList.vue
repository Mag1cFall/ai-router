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

  <div v-else class="log-shell">
    <div class="log-legend" aria-hidden="true">
      <span>时间</span>
      <span>协议</span>
      <span>模型</span>
      <span>Provider</span>
      <span>状态码</span>
      <span>延迟</span>
    </div>

    <ul class="log-list" aria-live="polite">
      <li v-for="log in logs" :key="log.id" class="log-row">
        <span class="log-row__cell log-row__cell--time" data-label="时间">
          <time :datetime="log.timestamp">{{ formatTimestamp(log.timestamp) }}</time>
        </span>

        <span class="log-row__cell log-row__cell--protocol" data-label="协议">
          <ProtocolBadge :protocol="log.sourceProtocol" compact />
        </span>

        <span class="log-row__cell log-row__cell--model" data-label="模型">
          <code>{{ log.model }}</code>
        </span>

        <span class="log-row__cell log-row__cell--provider" data-label="Provider">
          {{ log.provider }}
        </span>

        <span class="log-row__cell log-row__cell--status" data-label="状态码">
          <span class="status-code" :class="`status-code--${getStatusTone(log.statusCode)}`">
            {{ log.statusCode || '—' }}
          </span>
        </span>

        <span class="log-row__cell log-row__cell--latency" data-label="延迟">
          <span class="latency-pill">{{ formatLatency(log.latencyMs) }}</span>
        </span>
      </li>
    </ul>
  </div>
</template>

<style scoped>
.log-shell {
  display: grid;
  gap: 10px;
}

.log-legend,
.log-row {
  display: grid;
  grid-template-columns: 132px 92px minmax(180px, 1.4fr) minmax(140px, 1fr) 84px 84px;
  gap: 12px;
  align-items: center;
}

.log-legend {
  padding: 0 14px;
}

.log-legend span {
  color: var(--text-tertiary);
  font-size: 0.72rem;
  font-weight: 700;
  letter-spacing: 0.1em;
  text-transform: uppercase;
}

.log-list {
  margin: 0;
  padding: 0;
  list-style: none;
  display: grid;
  gap: 10px;
}

.log-row {
  padding: 12px 14px;
  border: 1px solid var(--border-color);
  border-radius: var(--radius-lg);
  background: rgba(29, 27, 24, 0.92);
  transition:
    transform var(--transition-fast),
    border-color var(--transition-fast),
    background-color var(--transition-fast);
}

.log-row:hover {
  transform: translateY(-1px);
  border-color: var(--border-hover);
  background: var(--bg-hover);
}

.log-row__cell {
  min-width: 0;
  color: var(--text-primary);
  font-size: 0.84rem;
}

.log-row__cell time,
.log-row__cell--provider {
  color: var(--text-secondary);
  font-size: 0.82rem;
  font-variant-numeric: tabular-nums;
}

.log-row__cell--model code {
  width: 100%;
  justify-content: flex-start;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.status-code,
.latency-pill {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-height: 32px;
  padding: 0 10px;
  border: 1px solid transparent;
  border-radius: 999px;
  font-family: var(--font-mono);
  font-size: 0.78rem;
  font-weight: 700;
}

.status-code--success {
  color: var(--status-success-text);
  border-color: var(--status-success-border);
  background: var(--status-success-bg);
}

.status-code--warning {
  color: var(--status-warning-text);
  border-color: var(--status-warning-border);
  background: var(--status-warning-bg);
}

.status-code--danger {
  color: var(--status-danger-text);
  border-color: var(--status-danger-border);
  background: var(--status-danger-bg);
}

.status-code--neutral {
  color: var(--status-neutral-text);
  border-color: var(--status-neutral-border);
  background: var(--status-neutral-bg);
}

.latency-pill {
  color: var(--primary-active);
  border-color: rgba(139, 134, 128, 0.24);
  background: rgba(139, 134, 128, 0.14);
}

.empty-state {
  display: grid;
  place-items: center;
  min-height: 240px;
  border: 1px dashed var(--border-color);
  border-radius: var(--radius-lg);
  color: var(--text-secondary);
  text-align: center;
  background: rgba(29, 27, 24, 0.52);
}

@media (max-width: 1080px) {
  .log-legend {
    display: none;
  }

  .log-row {
    grid-template-columns: repeat(2, minmax(0, 1fr));
    padding: 14px;
  }

  .log-row__cell {
    display: grid;
    gap: 6px;
    align-content: start;
  }

  .log-row__cell::before {
    content: attr(data-label);
    color: var(--text-tertiary);
    font-size: 0.7rem;
    font-weight: 700;
    letter-spacing: 0.1em;
    text-transform: uppercase;
  }
}

@media (max-width: 640px) {
  .log-row {
    grid-template-columns: 1fr;
  }

  .log-row__cell--model code {
    white-space: normal;
    word-break: break-word;
  }
}
</style>
