<script setup lang="ts">
import { computed } from 'vue'

const props = withDefaults(defineProps<{
  label: string
  value: string | number
  suffix?: string
  hint?: string
  percent?: number
  tone?: string
  icon?: any
}>(), { tone: '#3b82f6', percent: undefined })

const barWidth = computed(() => Math.max(2, Math.min(100, props.percent ?? 0)))
</script>

<template>
  <div class="stat-card glass">
    <div class="head">
      <div class="label">{{ label }}</div>
      <div v-if="icon" class="icon-wrap" :style="{ color: tone, background: `${tone}1f`, borderColor: `${tone}38` }">
        <n-icon :component="icon" :size="17" />
      </div>
    </div>
    <div class="value">
      <span>{{ value }}</span>
      <small v-if="suffix">{{ suffix }}</small>
    </div>
    <div v-if="percent !== undefined" class="track">
      <div class="bar" :style="{ width: barWidth + '%', background: tone }" />
    </div>
    <div v-if="hint" class="hint">{{ hint }}</div>
  </div>
</template>

<style scoped>
.stat-card { padding: 18px 18px 16px; position: relative; overflow: hidden; }

.head { display: flex; align-items: center; justify-content: space-between; gap: 8px; }
.label { font-size: 12.5px; color: var(--muted); letter-spacing: 0.2px; }
.icon-wrap { width: 32px; height: 32px; border-radius: 10px; display: grid; place-items: center; border: 1px solid; }
.value { margin-top: 10px; font-size: 26px; font-weight: 720; letter-spacing: -0.8px; line-height: 1.1; }
.value small { font-size: 13px; font-weight: 500; color: var(--muted); margin-left: 4px; letter-spacing: 0; }
.track { margin-top: 13px; height: 5px; background: rgba(255,255,255,0.06); border-radius: 99px; overflow: hidden; }
.bar { height: 100%; border-radius: 99px; transition: width 0.5s ease; }
.hint { margin-top: 9px; font-size: 11.5px; color: var(--muted); }
</style>
