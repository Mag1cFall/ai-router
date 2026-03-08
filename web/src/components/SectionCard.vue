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
  min-height: 320px;
  padding: 24px;
  border: 1px solid rgba(148, 163, 184, 0.16);
  border-radius: 24px;
  background: linear-gradient(180deg, rgba(15, 23, 42, 0.9), rgba(15, 23, 42, 0.72));
  box-shadow: 0 20px 46px rgba(2, 6, 23, 0.28);
}

.section-card__header {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  align-items: flex-start;
}

.section-card__title {
  margin: 0;
  color: #f8fafc;
  font-family: 'Sora', 'Segoe UI', sans-serif;
  font-size: 1.18rem;
  font-weight: 700;
}

.section-card__subtitle {
  margin: 8px 0 0;
  color: rgba(148, 163, 184, 0.92);
  font-size: 0.92rem;
}

.section-card__count {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  min-width: 82px;
  color: #93c5fd;
}

.section-card__count span {
  font-family: 'Sora', 'Segoe UI', sans-serif;
  font-size: 1.52rem;
  font-weight: 800;
  line-height: 1;
}

.section-card__count small {
  margin-top: 4px;
  color: rgba(148, 163, 184, 0.9);
  font-size: 0.82rem;
}

.section-card__error {
  margin: 0;
  padding: 12px 14px;
  border: 1px solid rgba(248, 113, 113, 0.24);
  border-radius: 14px;
  color: #fecaca;
  background: rgba(127, 29, 29, 0.28);
  line-height: 1.6;
}

.section-card__body {
  min-height: 0;
}

@media (max-width: 720px) {
  .section-card {
    padding: 18px;
    min-height: auto;
  }

  .section-card__header {
    flex-direction: column;
  }

  .section-card__count {
    align-items: flex-start;
  }
}
</style>
