<template>
  <div class="metric-card" :style="{ borderColor: color }">
    <div class="label">{{ label }}</div>
    <div class="value" :style="{ color }">{{ value }}</div>
    <div class="sub">{{ sub }}</div>
  </div>
</template>

<script setup lang="ts">
defineProps<{
  label: string
  value: string | number
  sub?: string
  color?: string
}>()
</script>

<style scoped>
.metric-card {
  background: #161b22;
  border: 1px solid;
  border-radius: 8px;
  padding: 16px;
  display: flex;
  flex-direction: column;
  gap: 6px;
  transition: border-color 0.2s;
}
.metric-card:hover {
  border-color: #8b949e !important;
}
.label {
  font-size: 12px;
  color: #8b949e;
  font-weight: 500;
  text-transform: uppercase;
  letter-spacing: 0.5px;
}
.value {
  font-size: 28px;
  font-weight: 700;
  line-height: 1;
}
.sub {
  font-size: 12px;
  color: #6e7681;
}
</style>