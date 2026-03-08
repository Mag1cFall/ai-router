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
          <th scope="col">Name</th>
          <th scope="col">Protocol</th>
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
  border-collapse: collapse;
}

.data-table th,
.data-table td {
  padding: 14px 16px;
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
  line-height: 1.6;
}

.data-table tbody tr:hover {
  background: rgba(15, 23, 42, 0.58);
}

.data-table__primary {
  font-weight: 700;
}

.data-table__endpoint {
  color: #cbd5e1;
  word-break: break-all;
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
