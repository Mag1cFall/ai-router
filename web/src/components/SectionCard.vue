<script setup lang="ts">
defineProps<{
  title: string
  subtitle: string
  count: number
  countLabel: string
  error?: string | null
}>()
</script>

<template>
  <section class="section-card">
    <header class="section-card__header">
      <div>
        <p class="section-card__title">{{ title }}</p>
        <p class="section-card__subtitle">{{ subtitle }}</p>
      </div>

      <div class="section-card__count">
        <span>{{ count }}</span>
        <small>{{ countLabel }}</small>
      </div>
    </header>

    <p v-if="error" class="section-card__error">{{ error }}</p>

    <div class="section-card__body">
      <slot />
    </div>
  </section>
</template>

<style scoped>
.section-card {
  display: flex;
  flex-direction: column;
  gap: 18px;
  min-height: 0;
  padding: 24px;
  border: 1px solid var(--border-color);
  border-radius: var(--radius-xl);
  background: linear-gradient(180deg, rgba(29, 27, 24, 0.98), rgba(21, 20, 18, 0.94));
  box-shadow: var(--shadow-md);
  transition:
    transform var(--transition-fast),
    border-color var(--transition-fast),
    box-shadow var(--transition-fast);
}

.section-card:hover {
  transform: translateY(-2px);
  border-color: var(--border-hover);
  box-shadow: var(--shadow-lg);
}

.section-card__header {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  align-items: flex-start;
}

.section-card__title {
  margin: 0;
  color: var(--text-primary);
  font-size: 1.12rem;
  font-weight: 700;
}

.section-card__subtitle {
  margin: 8px 0 0;
  color: var(--text-secondary);
  font-size: 0.9rem;
}

.section-card__count {
  display: inline-flex;
  flex-direction: column;
  align-items: flex-end;
  justify-content: center;
  min-width: 90px;
  padding: 10px 12px;
  border: 1px solid rgba(139, 134, 128, 0.24);
  border-radius: var(--radius-md);
  background: rgba(139, 134, 128, 0.12);
}

.section-card__count span {
  color: var(--text-primary);
  font-family: var(--font-mono);
  font-size: 1.3rem;
  font-weight: 800;
  line-height: 1;
}

.section-card__count small {
  margin-top: 4px;
  color: var(--text-tertiary);
  font-size: 0.82rem;
}

.section-card__error {
  margin: 0;
  padding: 12px 14px;
  border: 1px solid var(--status-danger-border);
  border-radius: var(--radius-md);
  color: var(--status-danger-text);
  background: var(--bg-danger-soft);
  line-height: 1.6;
}

.section-card__body {
  min-height: 0;
  overflow: hidden;
}

@media (max-width: 720px) {
  .section-card {
    padding: 18px;
  }

  .section-card__header {
    flex-direction: column;
  }

  .section-card__count {
    align-items: flex-start;
    width: 100%;
  }
}
</style>
