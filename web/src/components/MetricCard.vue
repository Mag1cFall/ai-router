<script setup lang="ts">
defineProps<{
  label: string
  value: string
  hint: string
}>()
</script>

<template>
  <article class="metric-card">
    <p class="metric-card__label">{{ label }}</p>
    <p class="metric-card__value">{{ value }}</p>
    <p class="metric-card__hint">{{ hint }}</p>
  </article>
</template>

<style scoped>
.metric-card {
  display: grid;
  gap: 10px;
  padding: 18px 20px;
  border: 1px solid rgba(148, 163, 184, 0.14);
  border-radius: 22px;
  background: linear-gradient(180deg, rgba(15, 23, 42, 0.94), rgba(2, 6, 23, 0.76));
}

.metric-card__label,
.metric-card__hint {
  margin: 0;
}

.metric-card__label {
  color: rgba(148, 163, 184, 0.92);
  font-size: 0.88rem;
  font-weight: 700;
  letter-spacing: 0.06em;
  text-transform: uppercase;
}

.metric-card__value {
  margin: 0;
  color: #f8fafc;
  font-family: 'Sora', 'Segoe UI', sans-serif;
  font-size: clamp(1.7rem, 4vw, 2.3rem);
  font-weight: 800;
  line-height: 1;
}

.metric-card__hint {
  color: rgba(203, 213, 225, 0.8);
  font-size: 0.9rem;
  line-height: 1.5;
}
</style>
