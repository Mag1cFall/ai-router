<script setup lang="ts">
import type { ProviderRecord } from '../types/dashboard'
import ProtocolBadge from './ProtocolBadge.vue'

defineProps<{
  providers: ProviderRecord[]
  loading: boolean
}>()
</script>

<template>
  <div v-if="loading" class="empty-state">正在加载 Provider 列表…</div>

  <div v-else-if="providers.length === 0" class="empty-state">暂无 Provider 数据。</div>

  <div v-else class="table-wrap">
    <table class="data-table">
      <thead>
        <tr>
          <th scope="col">Provider</th>
          <th scope="col">协议</th>
          <th scope="col">Endpoint</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="provider in providers" :key="provider.name">
          <td class="data-table__primary">{{ provider.name }}</td>
          <td><ProtocolBadge :protocol="provider.protocol" /></td>
          <td class="data-table__endpoint">{{ provider.endpoint }}</td>
        </tr>
      </tbody>
    </table>
  </div>
</template>

<style scoped>
.table-wrap {
  overflow-x: auto;
}

.data-table {
  width: 100%;
  min-width: 620px;
  border-collapse: separate;
  border-spacing: 0 10px;
}

.data-table th,
.data-table td {
  padding: 14px 16px;
  text-align: left;
}

.data-table th {
  color: var(--text-tertiary);
  font-size: 0.78rem;
  font-weight: 700;
  letter-spacing: 0.1em;
  text-transform: uppercase;
}

.data-table td {
  border-top: 1px solid var(--border-color);
  border-bottom: 1px solid var(--border-color);
  background: rgba(29, 27, 24, 0.88);
  color: var(--text-primary);
  line-height: 1.6;
  transition:
    border-color var(--transition-fast),
    background-color var(--transition-fast);
}

.data-table td:first-child {
  border-left: 1px solid var(--border-color);
  border-radius: var(--radius-md) 0 0 var(--radius-md);
}

.data-table td:last-child {
  border-right: 1px solid var(--border-color);
  border-radius: 0 var(--radius-md) var(--radius-md) 0;
}

.data-table tbody tr:hover td {
  border-color: var(--border-hover);
  background: var(--bg-hover);
}

.data-table__primary {
  font-weight: 700;
}

.data-table__endpoint {
  color: var(--text-secondary);
  font-family: var(--font-mono);
  font-size: 0.82rem;
  word-break: break-all;
}

.empty-state {
  display: grid;
  place-items: center;
  min-height: 220px;
  border: 1px dashed var(--border-color);
  border-radius: var(--radius-lg);
  color: var(--text-secondary);
  text-align: center;
  background: rgba(29, 27, 24, 0.52);
}
</style>
