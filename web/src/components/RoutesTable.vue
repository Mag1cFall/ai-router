<script setup lang="ts">
import type { RouteRecord } from '../types/dashboard'

defineProps<{
  routes: RouteRecord[]
  loading: boolean
}>()
</script>

<template>
  <div v-if="loading" class="empty-state">正在加载 Route 规则…</div>

  <div v-else-if="routes.length === 0" class="empty-state">暂无 Route 规则。</div>

  <div v-else class="table-wrap">
    <table class="data-table">
      <thead>
        <tr>
          <th scope="col">Match Model</th>
          <th scope="col">Provider</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="route in routes" :key="`${route.matchModel}-${route.provider}`">
          <td class="data-table__pattern"><code>{{ route.matchModel }}</code></td>
          <td class="data-table__primary">{{ route.provider }}</td>
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
  min-width: 480px;
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

.data-table__pattern code {
  color: var(--text-primary);
  background: var(--bg-code-strong);
  border-color: rgba(139, 134, 128, 0.28);
  font-size: 0.88rem;
}

.data-table__primary {
  font-weight: 700;
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
