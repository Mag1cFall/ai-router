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
  border-collapse: collapse;
}

.data-table th,
.data-table td {
  padding: 15px 16px;
  border-bottom: 1px solid rgba(148, 163, 184, 0.12);
  text-align: left;
}

.data-table th {
  color: rgba(148, 163, 184, 0.92);
  font-size: 0.8rem;
  font-weight: 800;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.data-table td {
  color: #e2e8f0;
}

.data-table tbody tr:hover {
  background: rgba(15, 23, 42, 0.58);
}

.data-table__pattern code {
  color: #93c5fd;
  font-size: 0.95rem;
}

.data-table__primary {
  font-weight: 700;
}

.empty-state {
  display: grid;
  place-items: center;
  min-height: 220px;
  border: 1px dashed rgba(148, 163, 184, 0.18);
  border-radius: 18px;
  color: rgba(148, 163, 184, 0.9);
  text-align: center;
}
</style>
