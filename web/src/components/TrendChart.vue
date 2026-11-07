<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import * as echarts from 'echarts'

export interface Series {
  name: string
  values: number[]
  color: string
  unit?: string
  area?: boolean
  dashed?: boolean
}

const props = withDefaults(defineProps<{
  labels: string[]
  series: Series[]
  height?: string
  yMax?: number
  showLegend?: boolean
  digits?: number
}>(), { height: '220px', showLegend: false, digits: 1 })

const el = ref<HTMLDivElement>()
let chart: echarts.ECharts | null = null
let ro: ResizeObserver | null = null

function render() {
  if (!el.value) return
  if (!chart) chart = echarts.init(el.value, undefined, { renderer: 'canvas' })
  chart.setOption({
    animationDuration: 500,
    grid: { left: 8, right: 12, top: props.showLegend ? 34 : 14, bottom: 4, containLabel: true },
    tooltip: {
      trigger: 'axis',
      backgroundColor: 'rgba(10,15,29,0.95)',
      borderColor: 'rgba(255,255,255,0.12)',
      textStyle: { color: '#e9eefc', fontSize: 12 },
      axisPointer: { type: 'line', lineStyle: { color: 'rgba(255,255,255,0.2)' } },
      valueFormatter: (v: number) => `${Number(v).toFixed(props.digits)}`,
    },
    legend: props.showLegend
      ? { show: true, top: 0, right: 0, textStyle: { color: '#8b98b8', fontSize: 11 }, itemWidth: 10, itemHeight: 10, icon: 'roundRect' }
      : { show: false },
    xAxis: {
      type: 'category',
      boundaryGap: false,
      data: props.labels,
      axisLine: { lineStyle: { color: 'rgba(255,255,255,0.08)' } },
      axisTick: { show: false },
      axisLabel: { color: '#6b7896', fontSize: 10, hideOverlap: true },
    },
    yAxis: {
      type: 'value',
      max: props.yMax,
      axisLine: { show: false },
      axisTick: { show: false },
      axisLabel: { color: '#6b7896', fontSize: 10 },
      splitLine: { lineStyle: { color: 'rgba(255,255,255,0.055)', type: 'dashed' } },
    },
    series: props.series.map((s) => ({
      name: s.name,
      type: 'line',
      smooth: 0.35,
      symbol: 'none',
      data: s.values,
      lineStyle: { width: 2, color: s.color, type: s.dashed ? 'dashed' : 'solid' },
      itemStyle: { color: s.color },
      areaStyle: s.area === false ? undefined : {
        color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
          { offset: 0, color: hexA(s.color, 0.34) },
          { offset: 1, color: hexA(s.color, 0.01) },
        ]),
      },
    })),
  }, true)
}

function hexA(hex: string, alpha: number) {
  const h = hex.replace('#', '')
  const r = parseInt(h.slice(0, 2), 16)
  const g = parseInt(h.slice(2, 4), 16)
  const b = parseInt(h.slice(4, 6), 16)
  return `rgba(${r},${g},${b},${alpha})`
}

onMounted(() => {
  render()
  if (el.value) {
    ro = new ResizeObserver(() => chart?.resize())
    ro.observe(el.value)
  }
})
onBeforeUnmount(() => {
  ro?.disconnect()
  chart?.dispose()
  chart = null
})
watch(() => [props.labels, props.series], render, { deep: true })
</script>

<template>
  <div ref="el" class="trend-chart" :style="{ height }" />
</template>

<style scoped>
.trend-chart { width: 100%; }
</style>
