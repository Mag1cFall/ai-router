<script setup lang="ts">
import { computed } from 'vue'

import { getProtocolLabel } from '../lib/dashboard'

const props = withDefaults(
  defineProps<{
    protocol: string
    compact?: boolean
  }>(),
  {
    compact: false,
  },
)

const normalizedProtocol = computed<'openai' | 'claude' | 'gemini' | 'unknown'>(() => {
  const protocol = props.protocol.trim().toLowerCase()

  if (protocol === 'openai' || protocol === 'claude' || protocol === 'gemini') {
    return protocol
  }

  return 'unknown'
})
</script>

<template>
  <span
    class="protocol-badge"
    :class="[`protocol-badge--${normalizedProtocol}`, { 'protocol-badge--compact': compact }]"
  >
    {{ getProtocolLabel(protocol) }}
  </span>
</template>

<style scoped>
.protocol-badge {
  --badge-text: var(--protocol-unknown-text);
  --badge-border: var(--protocol-unknown-border);
  --badge-bg: var(--protocol-unknown-bg);
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 84px;
  min-height: 32px;
  padding: 0 12px;
  border: 1px solid var(--badge-border);
  border-radius: 999px;
  background: var(--badge-bg);
  color: var(--badge-text);
  font-size: 0.78rem;
  font-weight: 700;
  letter-spacing: 0.04em;
  text-transform: uppercase;
}

.protocol-badge--openai {
  --badge-text: var(--protocol-openai-text);
  --badge-border: var(--protocol-openai-border);
  --badge-bg: var(--protocol-openai-bg);
}

.protocol-badge--claude {
  --badge-text: var(--protocol-claude-text);
  --badge-border: var(--protocol-claude-border);
  --badge-bg: var(--protocol-claude-bg);
}

.protocol-badge--gemini {
  --badge-text: var(--protocol-gemini-text);
  --badge-border: var(--protocol-gemini-border);
  --badge-bg: var(--protocol-gemini-bg);
}

.protocol-badge--compact {
  min-width: 74px;
  min-height: 28px;
  padding: 0 10px;
  font-size: 0.72rem;
}
</style>
