<script setup lang="ts">
withDefaults(
  defineProps<{
    label: string
    value: string
    hint: string
    tone?: 'primary' | 'success' | 'info' | 'warning'
  }>(),
  {
    tone: 'primary',
  },
)
</script>

<template>
  <article class="metric-card" :class="`metric-card--${tone}`">
    <div class="metric-card__header">
      <span class="metric-card__icon" aria-hidden="true">
        <slot name="icon" />
      </span>
      <p class="metric-card__label">{{ label }}</p>
    </div>

    <p class="metric-card__value">{{ value }}</p>
    <p class="metric-card__hint">{{ hint }}</p>
  </article>
</template>

<style scoped>
.metric-card {
  --metric-accent: var(--primary-color);
  --metric-accent-border: rgba(139, 134, 128, 0.26);
  --metric-accent-bg: rgba(139, 134, 128, 0.14);
  display: grid;
  gap: 12px;
  min-height: 168px;
  padding: 18px 20px;
  border: 1px solid var(--border-color);
  border-radius: var(--radius-lg);
  background: linear-gradient(180deg, rgba(29, 27, 24, 0.98), rgba(21, 20, 18, 0.94));
  box-shadow: var(--shadow-sm);
  transition:
    transform var(--transition-fast),
    border-color var(--transition-fast),
    box-shadow var(--transition-fast),
    background-color var(--transition-fast);
}

.metric-card:hover {
  transform: translateY(-2px);
  border-color: var(--border-hover);
  box-shadow: var(--shadow-md);
  background: linear-gradient(180deg, rgba(38, 35, 32, 0.98), rgba(29, 27, 24, 0.96));
}

.metric-card--success {
  --metric-accent: var(--success-color);
  --metric-accent-border: var(--status-success-border);
  --metric-accent-bg: var(--status-success-bg);
}

.metric-card--info {
  --metric-accent: var(--primary-active);
  --metric-accent-border: rgba(166, 160, 153, 0.3);
  --metric-accent-bg: rgba(139, 134, 128, 0.16);
}

.metric-card--warning {
  --metric-accent: var(--warning-color);
  --metric-accent-border: var(--status-warning-border);
  --metric-accent-bg: var(--status-warning-bg);
}

.metric-card__header {
  display: flex;
  align-items: center;
  gap: 12px;
}

.metric-card__icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 42px;
  height: 42px;
  border: 1px solid var(--metric-accent-border);
  border-radius: var(--radius-md);
  background: var(--metric-accent-bg);
  color: var(--metric-accent);
}

.metric-card__icon :deep(svg) {
  width: 20px;
  height: 20px;
}

.metric-card__label,
.metric-card__hint {
  margin: 0;
}

.metric-card__label {
  color: var(--text-secondary);
  font-size: 0.92rem;
  font-weight: 700;
}

.metric-card__value {
  margin: 0;
  color: var(--text-primary);
  font-family: var(--font-mono);
  font-size: clamp(1.55rem, 3.4vw, 2.2rem);
  font-weight: 800;
  line-height: 1.08;
  letter-spacing: -0.04em;
}

.metric-card__hint {
  color: var(--text-tertiary);
  font-size: 0.84rem;
  line-height: 1.65;
}

@media (max-width: 720px) {
  .metric-card {
    min-height: auto;
  }
}
</style>
