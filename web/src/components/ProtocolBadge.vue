<script setup lang="ts">
import { computed } from 'vue'

import { getProtocolLabel } from '../lib/dashboard'

const props = defineProps<{
  protocol: string
}>()

const normalizedProtocol = computed(() => props.protocol.trim().toLowerCase() || 'unknown')
</script>

<template>
  <span class="protocol-badge" :class="`protocol-badge--${normalizedProtocol}`">
    {{ getProtocolLabel(protocol) }}
  </span>
</template>

<style scoped>
.protocol-badge {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 86px;
  min-height: 32px;
  padding: 0 12px;
  border-radius: 999px;
  border: 1px solid rgba(148, 163, 184, 0.18);
  font-size: 0.82rem;
  font-weight: 800;
  letter-spacing: 0.04em;
  text-transform: uppercase;
}

.protocol-badge--openai {
  color: #bbf7d0;
  border-color: rgba(34, 197, 94, 0.34);
  background: rgba(20, 83, 45, 0.42);
}

.protocol-badge--claude {
  color: #fed7aa;
  border-color: rgba(249, 115, 22, 0.34);
  background: rgba(124, 45, 18, 0.4);
}

.protocol-badge--gemini {
  color: #bfdbfe;
  border-color: rgba(59, 130, 246, 0.34);
  background: rgba(30, 58, 138, 0.38);
}

.protocol-badge--unknown {
  color: #cbd5e1;
  background: rgba(30, 41, 59, 0.72);
}
</style>
