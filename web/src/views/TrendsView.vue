<template>
  <app-layout>
    <div class="page">
      <div class="page-header">
        <h2>趋势分析</h2>
        <n-space>
          <n-select v-model:value="selectedNode" :options="nodeOptions" style="width:180px" placeholder="选择节点" />
          <n-select v-model:value="duration" :options="durationOptions" style="width:120px" />
          <n-button size="small" :loading="loading" @click="load">加载</n-button>
        </n-space>
      </div>

      <n-tabs type="line" animated @update:value="onTabChange">
        <n-tab-pane name="system" tab="系统指标">
          <div v-if="historyData.length === 0 && !loading" class="empty-state">
            <div class="empty-icon">📊</div>
            <div>暂无历史数据，请等待数据采集或调整时间范围</div>
          </div>
          <template v-else>
            <div class="charts-row">
              <div class="chart-box">
                <div class="chart-title">CPU 趋势</div>
                <div ref="cpuRef" style="height:240px" />
              </div>
              <div class="chart-box">
                <div class="chart-title">内存趋势</div>
                <div ref="memRef" style="height:240px" />
              </div>
            </div>
            <div class="charts-row">
              <div class="chart-box">
                <div class="chart-title">磁盘 I/O 速率</div>
                <div ref="diskRef" style="height:240px" />
              </div>
              <div class="chart-box">
                <div class="chart-title">网络流量</div>
                <div ref="netRef" style="height:240px" />
              </div>
            </div>
          </template>

          <div class="stats-row">
            <div class="stat-box">
              <div class="stat-label">CPU 均值</div>
              <div class="stat-val" :style="{color: avgCpu > 80 ? '#f85149' : '#3fb950'}">{{ avgCpu }}%</div>
              <div class="stat-sub">峰值 {{ maxCpu }}% · 低谷 {{ minCpu }}%</div>
            </div>
            <div class="stat-box">
              <div class="stat-label">内存均值</div>
              <div class="stat-val" :style="{color: avgMem > 80 ? '#f85149' : '#bc8cff'}">{{ avgMem }}%</div>
              <div class="stat-sub">峰值 {{ maxMem }}% · 低谷 {{ minMem }}%</div>
            </div>
            <div class="stat-box">
              <div class="stat-label">磁盘均值</div>
              <div class="stat-val" :style="{color: avgDisk > 90 ? '#f85149' : '#f0883e'}">{{ avgDisk }}%</div>
              <div class="stat-sub">峰值 {{ maxDisk }}% · 低谷 {{ minDisk }}%</div>
            </div>
            <div class="stat-box">
              <div class="stat-label">网络总量</div>
              <div class="stat-val" style="color:#58a6ff">{{ totalNet }} GB</div>
              <div class="stat-sub">入 {{ totalIn }} · 出 {{ totalOut }}</div>
            </div>
          </div>
        </n-tab-pane>

        <n-tab-pane name="compare" tab="趋势对比">
          <div v-if="historyData.length === 0 && !loading" class="empty-state">
            <div class="empty-icon">📈</div>
            <div>暂无数据，请先在系统指标页加载数据</div>
          </div>
          <template v-else>
            <n-space style="margin-bottom:12px" align="center">
              <span style="color:#8b949e;font-size:13px">对比指标：</span>
              <n-select v-model:value="compareMetric" :options="compareMetricOptions" style="width:140px" size="small" />
            </n-space>
            <div class="stats-row" style="margin-bottom:16px">
              <div class="stat-box">
                <div class="stat-label">当前均值</div>
                <div class="stat-val" style="color:#3fb950">{{ compareCurrentAvg }}</div>
              </div>
              <div class="stat-box">
                <div class="stat-label">前期均值</div>
                <div class="stat-val" style="color:#58a6ff">{{ comparePreviousAvg }}</div>
              </div>
              <div class="stat-box">
                <div class="stat-label">变化趋势</div>
                <div class="stat-val" :style="{color: compareTrend === 'rising' ? '#f85149' : compareTrend === 'falling' ? '#3fb950' : '#d29922'}">{{ compareTrendLabel }}</div>
              </div>
              <div class="stat-box">
                <div class="stat-label">变化幅度</div>
                <div class="stat-val" :style="{color: parseFloat(compareChange) > 0 ? '#f85149' : parseFloat(compareChange) < 0 ? '#3fb950' : '#d29922'}">{{ compareChange }}</div>
              </div>
            </div>
            <div class="chart-box" style="margin-bottom:16px">
              <div class="chart-title">{{ compareMetricLabel }} 趋势对比</div>
              <div ref="compareCpuMemRef" style="height:280px" />
            </div>
            <div class="chart-box">
              <div class="chart-title">磁盘 & 网络 综合对比</div>
              <div ref="compareDiskNetRef" style="height:280px" />
            </div>
          </template>
        </n-tab-pane>

        <n-tab-pane name="anomaly" tab="异常检测">
          <div v-if="historyData.length === 0 && !loading" class="empty-state">
            <div class="empty-icon">🔍</div>
            <div>暂无数据，请先在系统指标页加载数据</div>
          </div>
          <template v-else>
            <div class="chart-box" style="margin-bottom:16px">
              <div class="chart-title">CPU 异常检测 (均值±2σ)</div>
              <div ref="anomalyCpuRef" style="height:260px" />
            </div>
            <div class="chart-box">
              <div class="chart-title">内存异常检测 (均值±2σ)</div>
              <div ref="anomalyMemRef" style="height:260px" />
            </div>
            <div class="anomaly-summary" v-if="anomalyPoints.length > 0">
              <div class="anomaly-title">检测到 {{ anomalyPoints.length }} 个异常点</div>
              <div class="anomaly-list">
                <div v-for="(p, i) in anomalyPoints.slice(0, 20)" :key="i" class="anomaly-item">
                  <span class="anomaly-time">{{ formatAnomalyTime(p.time) }}</span>
                  <span class="anomaly-metric">{{ p.metric }}</span>
                  <span class="anomaly-val">{{ p.value.toFixed(1) }}%</span>
                  <span class="anomaly-range">正常范围 {{ p.lower.toFixed(1) }}% ~ {{ p.upper.toFixed(1) }}%</span>
                </div>
              </div>
            </div>
            <div class="anomaly-summary" v-else>
              <div class="anomaly-title" style="color:#3fb950">未检测到异常，系统运行正常</div>
            </div>
          </template>
        </n-tab-pane>

      </n-tabs>

      <div class="report-box">
        <div class="report-title">周报摘要</div>
        <div class="report-content">
          <p>节点：<strong>{{ selectedNodeName }}</strong> · 时间范围：{{ durationLabel }}</p>
          <p>CPU 平均负载 <strong>{{ avgCpu }}%</strong>，{{ avgCpu > 80 ? '⚠️ 建议关注' : '✅ 运行正常' }}</p>
          <p>内存平均使用 <strong>{{ avgMem }}%</strong>，{{ avgMem > 80 ? '⚠️ 建议关注' : '✅ 运行正常' }}</p>
          <p>磁盘平均使用 <strong>{{ avgDisk }}%</strong>，{{ avgDisk > 90 ? '⚠️ 磁盘即将满' : '✅ 空间充足' }}</p>
          <p>网络总计流量 <strong>{{ totalNet }} GB</strong></p>
          <p v-if="anomalyPoints.length > 0">异常检测：发现 <strong>{{ anomalyPoints.length }}</strong> 个异常数据点</p>
          <p v-else>异常检测：<strong>✅ 未发现异常</strong></p>
        </div>
        <n-button size="small" style="margin-top:8px" @click="exportReport">导出报告</n-button>
      </div>
    </div>
  </app-layout>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, watch, onUnmounted, nextTick } from 'vue'
import * as echarts from 'echarts/core'
import { LineChart, ScatterChart } from 'echarts/charts'
import { GridComponent, TooltipComponent, LegendComponent, MarkLineComponent, MarkAreaComponent } from 'echarts/components'
import { CanvasRenderer } from 'echarts/renderers'
echarts.use([LineChart, ScatterChart, GridComponent, TooltipComponent, LegendComponent, MarkLineComponent, MarkAreaComponent, CanvasRenderer])
import AppLayout from '@/components/AppLayout.vue'
import client from '@/api/client'
import type { Snapshot } from '@/types'

const selectedNode = ref<string>('self')
const duration = ref('7d')
const historyData = ref<Snapshot[]>([])
const loading = ref(false)

const cpuRef = ref<HTMLDivElement>()
const memRef = ref<HTMLDivElement>()
const diskRef = ref<HTMLDivElement>()
const netRef = ref<HTMLDivElement>()
const compareCpuMemRef = ref<HTMLDivElement>()
const compareDiskNetRef = ref<HTMLDivElement>()
const anomalyCpuRef = ref<HTMLDivElement>()
const anomalyMemRef = ref<HTMLDivElement>()

let cpuChart: echarts.ECharts | null = null
let memChart: echarts.ECharts | null = null
let diskChart: echarts.ECharts | null = null
let netChart: echarts.ECharts | null = null
let compareCpuMemChart: echarts.ECharts | null = null
let compareDiskNetChart: echarts.ECharts | null = null
let anomalyCpuChart: echarts.ECharts | null = null
let anomalyMemChart: echarts.ECharts | null = null

const durationOptions = [
  { label: '1小时', value: '1h' },
  { label: '6小时', value: '6h' },
  { label: '24小时', value: '1d' },
  { label: '7天', value: '7d' },
  { label: '30天', value: '30d' },
]
const durationLabel = computed(() => durationOptions.find(d => d.value === duration.value)?.label || '')

const compareMetric = ref('cpu')
const compareMetricOptions = [
  { label: 'CPU 使用率', value: 'cpu' },
  { label: '内存使用率', value: 'memory' },
  { label: '磁盘使用率', value: 'disk' },
  { label: '1分钟负载', value: 'load1' },
]
const compareMetricLabel = computed(() => compareMetricOptions.find(o => o.value === compareMetric.value)?.label || '')

const compareData = ref<{ current: any[]; previous: any[] }>({ current: [], previous: [] })

const compareCurrentAvg = computed(() => {
  const keys = compareMetric.value === 'cpu' ? ['cpu.usage_percent', 'cpu_usage'] : compareMetric.value === 'memory' ? ['memory.usage_percent', 'mem_usage_percent'] : compareMetric.value === 'disk' ? ['disk.usage_percent', 'disk_usage_percent'] : ['load.load1', 'load1']
  if (compareData.value.current.length > 0) {
    const curVals = compareData.value.current.map(h => getVal(h, keys))
    return curVals.length ? Math.round(curVals.reduce((s, v) => s + v, 0) / curVals.length) + (compareMetric.value === 'load1' ? '' : '%') : '--'
  }
  const half = Math.floor(historyData.value.length / 2)
  if (half === 0) return '--'
  const secondHalf = historyData.value.slice(half).map(h => getVal(h, keys))
  return secondHalf.length ? Math.round(secondHalf.reduce((s, v) => s + v, 0) / secondHalf.length) + (compareMetric.value === 'load1' ? '' : '%') : '--'
})
const comparePreviousAvg = computed(() => {
  if (compareData.value.previous.length === 0) {
    const half = Math.floor(historyData.value.length / 2)
    if (half === 0) return '--'
    const keys = compareMetric.value === 'cpu' ? ['cpu.usage_percent', 'cpu_usage'] : compareMetric.value === 'memory' ? ['memory.usage_percent', 'mem_usage_percent'] : compareMetric.value === 'disk' ? ['disk.usage_percent', 'disk_usage_percent'] : ['load.load1', 'load1']
    const firstHalf = historyData.value.slice(0, half).map(h => getVal(h, keys))
    return firstHalf.length ? Math.round(firstHalf.reduce((s, v) => s + v, 0) / firstHalf.length) + (compareMetric.value === 'load1' ? '' : '%') : '--'
  }
  const keys = compareMetric.value === 'cpu' ? ['cpu.usage_percent', 'cpu_usage'] : compareMetric.value === 'memory' ? ['memory.usage_percent', 'mem_usage_percent'] : compareMetric.value === 'disk' ? ['disk.usage_percent', 'disk_usage_percent'] : ['load.load1', 'load1']
  const prevVals = compareData.value.previous.map(h => getVal(h, keys))
  return prevVals.length ? Math.round(prevVals.reduce((s, v) => s + v, 0) / prevVals.length) + (compareMetric.value === 'load1' ? '' : '%') : '--'
})
const compareTrend = computed(() => {
  const cur = parseFloat(compareCurrentAvg.value as string)
  const prev = parseFloat(comparePreviousAvg.value as string)
  if (isNaN(cur) || isNaN(prev) || prev === 0) return 'stable'
  const diff = ((cur - prev) / prev) * 100
  if (diff > 5) return 'rising'
  if (diff < -5) return 'falling'
  return 'stable'
})
const compareTrendLabel = computed(() => compareTrend.value === 'rising' ? '↑ 上升' : compareTrend.value === 'falling' ? '↓ 下降' : '→ 平稳')
const compareChange = computed(() => {
  const cur = parseFloat(compareCurrentAvg.value as string)
  const prev = parseFloat(comparePreviousAvg.value as string)
  if (isNaN(cur) || isNaN(prev) || prev === 0) return '0%'
  return ((cur - prev) / prev * 100).toFixed(1) + '%'
})

const nodeOptions = computed(() => {
  return [{ label: '本机', value: 'self' }]
})
const selectedNodeName = computed(() => '本机')

function toTs(val: string | number | undefined): number {
  if (!val) return Date.now()
  if (typeof val === 'number') return val > 9999999999 ? val : val * 1000
  return new Date(val).getTime()
}

function getVal(h: any, keys: string[]): number {
  for (const k of keys) {
    const parts = k.split('.')
    let v: any = h
    for (const p of parts) {
      if (v == null) break
      v = v[p]
    }
    if (typeof v === 'number' && !isNaN(v)) return v
  }
  return 0
}

function vals(keys: string[]): number[] {
  return historyData.value.map(h => getVal(h, keys))
}

function sumArr(arr: number[]): number { return arr.reduce((s, v) => s + (v || 0), 0) }
function avg(arr: number[]): number { return arr.length ? Math.round(arr.reduce((s, v) => s + v, 0) / arr.length) : 0 }
function max(arr: number[]): number { return arr.length ? Math.round(Math.max(...arr)) : 0 }
function min(arr: number[]): number { return arr.length ? Math.round(Math.min(...arr)) : 0 }

const avgCpu = computed(() => avg(vals(['cpu.usage_percent', 'cpu_usage'])))
const maxCpu = computed(() => max(vals(['cpu.usage_percent', 'cpu_usage'])))
const minCpu = computed(() => min(vals(['cpu.usage_percent', 'cpu_usage'])))
const avgMem = computed(() => avg(vals(['memory.usage_percent', 'mem_usage_percent'])))
const maxMem = computed(() => max(vals(['memory.usage_percent', 'mem_usage_percent'])))
const minMem = computed(() => min(vals(['memory.usage_percent', 'mem_usage_percent'])))
const avgDisk = computed(() => avg(vals(['disk.usage_percent', 'disk_usage_percent'])))
const maxDisk = computed(() => max(vals(['disk.usage_percent', 'disk_usage_percent'])))
const minDisk = computed(() => min(vals(['disk.usage_percent', 'disk_usage_percent'])))

const netInVals = computed(() => vals(['network.bytes_recv', 'bytes_recv']))
const netOutVals = computed(() => vals(['network.bytes_sent', 'bytes_sent']))

const totalNet = computed(() => ((sumArr(netInVals.value) + sumArr(netOutVals.value)) / 1024 / 1024 / 1024).toFixed(2))
const totalIn = computed(() => (sumArr(netInVals.value) / 1024 / 1024 / 1024).toFixed(2) + ' GB')
const totalOut = computed(() => (sumArr(netOutVals.value) / 1024 / 1024 / 1024).toFixed(2) + ' GB')

interface AnomalyPoint { time: number; metric: string; value: number; lower: number; upper: number }
const anomalyPoints = computed<AnomalyPoint[]>(() => {
  const cpuVals = vals(['cpu.usage_percent', 'cpu_usage'])
  const memVals = vals(['memory.usage_percent', 'mem_usage_percent'])
  const points: AnomalyPoint[] = []
  const detectAnomaly = (data: number[], metric: string) => {
    if (data.length < 5) return
    const mean = data.reduce((s, v) => s + v, 0) / data.length
    const std = Math.sqrt(data.reduce((s, v) => s + (v - mean) ** 2, 0) / data.length)
    const upper = mean + 2 * std
    const lower = mean - 2 * std
    data.forEach((v, i) => {
      if (v > upper || v < lower) {
        const h = historyData.value[i]
        if (h) {
          points.push({ time: toTs(h.timestamp), metric, value: v, lower: Math.max(0, lower), upper: Math.min(100, upper) })
        }
      }
    })
  }
  detectAnomaly(cpuVals, 'CPU')
  detectAnomaly(memVals, '内存')
  return points.sort((a, b) => b.time - a.time)
})

function formatAnomalyTime(ts: number): string {
  try { return new Date(ts).toLocaleString('zh-CN', { hour12: false }) }
  catch { return String(ts) }
}

function hexToRgba(hex: string, alpha: number): string {
  const h = hex.replace('#', '')
  const r = parseInt(h.substring(0, 2), 16)
  const g = parseInt(h.substring(2, 4), 16)
  const b = parseInt(h.substring(4, 6), 16)
  return `rgba(${r},${g},${b},${alpha})`
}

function createGradientColor(color: string, opacity: number = 0.2): any {
  const topColor = color.startsWith('#') ? hexToRgba(color, opacity) : color.replace('rgb', 'rgba').replace(')', `, ${opacity})`)
  const bottomColor = color.startsWith('#') ? hexToRgba(color, 0) : color.replace('rgb', 'rgba').replace(')', ', 0)')
  return {
    type: 'linear',
    x: 0, y: 0, x2: 0, y2: 1,
    colorStops: [
      { offset: 0, color: topColor },
      { offset: 1, color: bottomColor },
    ]
  }
}

function getTimeFormatter(): string {
  switch (duration.value) {
    case '1h': case '6h': return '{HH}:{mm}'
    case '1d': return '{HH}:{mm}'
    case '7d': return '{MM}/{dd} {HH}:{mm}'
    case '30d': return '{MM}/{dd}'
    default: return '{HH}:{mm}'
  }
}

function makeLineSeries(name: string, color: string, areaOpacity: number, yAxisIndex = 0, lineType?: string): any {
  const series: any = {
    name,
    type: 'line',
    smooth: 0.4,
    sampling: 'lttb',
    yAxisIndex,
    showSymbol: false,
    lineStyle: { width: 2.5, color, shadowBlur: 10, shadowColor: hexToRgba(color, 0.3) },
    itemStyle: { color },
    areaStyle: areaOpacity > 0 ? { color: createGradientColor(color, areaOpacity) } : undefined,
    data: [],
    emphasis: { focus: 'series', lineStyle: { width: 3 } },
    connectNulls: false,
  }
  if (lineType) {
    series.lineStyle.type = lineType
  }
  return series
}

function makeBaseOpt(): any {
  return {
    grid: { top: 35, right: 20, bottom: 30, left: 55 },
    legend: { top: 5, right: 10, textStyle: { color: '#8b949e', fontSize: 11 }, itemWidth: 16, itemHeight: 3 },
    tooltip: {
      trigger: 'axis',
      backgroundColor: '#1f242c',
      borderColor: '#30363d',
      borderWidth: 1,
      textStyle: { color: '#e6edf3', fontSize: 12 },
      axisPointer: { type: 'cross', lineStyle: { color: '#484f58' } },
    },
    animation: true,
    animationDuration: 500,
    animationEasing: 'cubicOut',
    backgroundColor: 'transparent',
  }
}

function makeTimeAxis(): any {
  return {
    type: 'time',
    boundaryGap: false,
    axisLine: { lineStyle: { color: '#30363d' } },
    axisLabel: { color: '#6e7681', fontSize: 10, formatter: getTimeFormatter() },
    splitLine: { show: true, lineStyle: { color: '#21262d', type: 'dashed' } },
  }
}

function makePercentYAxis(): any {
  return {
    type: 'value',
    min: 0,
    max: 100,
    axisLine: { lineStyle: { color: '#30363d' } },
    axisLabel: { color: '#6e7681', fontSize: 10, formatter: '{value}%' },
    splitLine: { lineStyle: { color: '#21262d', type: 'dashed' } },
  }
}

async function buildCharts() {
  disposeCharts()
  await nextTick()
  if (cpuRef.value) {
    cpuChart = echarts.init(cpuRef.value, 'dark')
    const opt = makeBaseOpt()
    opt.xAxis = makeTimeAxis()
    opt.yAxis = makePercentYAxis()
    opt.series = [makeLineSeries('CPU', '#3fb950', 0.2)]
    opt.series[0].markLine = {
      silent: true, symbol: 'none',
      lineStyle: { color: '#f8514966', type: 'dashed', width: 1 },
      data: [{ yAxis: 80, label: { formatter: '告警 80%', color: '#f85149', fontSize: 10 } }],
    }
    cpuChart.setOption(opt)
  }

  if (memRef.value) {
    memChart = echarts.init(memRef.value, 'dark')
    const opt = makeBaseOpt()
    opt.xAxis = makeTimeAxis()
    opt.yAxis = makePercentYAxis()
    opt.series = [makeLineSeries('内存', '#bc8cff', 0.2)]
    opt.series[0].markLine = {
      silent: true, symbol: 'none',
      lineStyle: { color: '#f8514966', type: 'dashed', width: 1 },
      data: [{ yAxis: 85, label: { formatter: '告警 85%', color: '#f85149', fontSize: 10 } }],
    }
    memChart.setOption(opt)
  }

  if (diskRef.value) {
    diskChart = echarts.init(diskRef.value, 'dark')
    const opt = makeBaseOpt()
    opt.grid = { top: 40, right: 55, bottom: 30, left: 55 }
    opt.xAxis = makeTimeAxis()
    opt.yAxis = [
      { type: 'value', min: 0, max: 100, position: 'left', axisLine: { lineStyle: { color: '#30363d' } }, axisLabel: { color: '#6e7681', fontSize: 10, formatter: '{value}%' }, splitLine: { lineStyle: { color: '#21262d', type: 'dashed' } } },
      { type: 'value', min: 0, position: 'right', axisLine: { show: false }, axisLabel: { color: '#6e7681', fontSize: 10, formatter: '{value} MB/s' }, splitLine: { show: false } },
    ]
    opt.series = [
      makeLineSeries('磁盘使用率', '#f0883e', 0.15, 0),
      makeLineSeries('读取速率', '#58a6ff', 0.08, 1),
      makeLineSeries('写入速率', '#d29922', 0.08, 1),
    ]
    diskChart.setOption(opt)
  }

  if (netRef.value) {
    netChart = echarts.init(netRef.value, 'dark')
    const opt = makeBaseOpt()
    opt.xAxis = makeTimeAxis()
    opt.yAxis = { type: 'value', min: 0, axisLine: { lineStyle: { color: '#30363d' } }, axisLabel: { color: '#6e7681', fontSize: 10, formatter: '{value} MB/s' }, splitLine: { lineStyle: { color: '#21262d', type: 'dashed' } } }
    opt.series = [
      makeLineSeries('\u2193 入流量', '#58a6ff', 0.12),
      makeLineSeries('\u2191 出流量', '#d29922', 0.12),
    ]
    netChart.setOption(opt)
  }

  if (compareCpuMemRef.value) {
    compareCpuMemChart = echarts.init(compareCpuMemRef.value, 'dark')
    const opt = makeBaseOpt()
    opt.xAxis = makeTimeAxis()
    opt.yAxis = makePercentYAxis()
    opt.legend = { textStyle: { color: '#8b949e', fontSize: 11 } }
    opt.series = [
      makeLineSeries('当前CPU', '#3fb950', 0.15),
      makeLineSeries('当前内存', '#bc8cff', 0.15),
      makeLineSeries('前期CPU', '#3fb95080', 0, undefined, 'dashed'),
      makeLineSeries('前期内存', '#bc8cff80', 0, undefined, 'dashed'),
    ]
    compareCpuMemChart.setOption(opt)
  }

  if (compareDiskNetRef.value) {
    compareDiskNetChart = echarts.init(compareDiskNetRef.value, 'dark')
    const opt = makeBaseOpt()
    opt.grid = { top: 40, right: 55, bottom: 30, left: 55 }
    opt.xAxis = makeTimeAxis()
    opt.yAxis = [
      { type: 'value', min: 0, max: 100, position: 'left', axisLine: { lineStyle: { color: '#30363d' } }, axisLabel: { color: '#6e7681', fontSize: 10, formatter: '{value}%' }, splitLine: { lineStyle: { color: '#21262d', type: 'dashed' } } },
      { type: 'value', min: 0, position: 'right', axisLine: { show: false }, axisLabel: { color: '#6e7681', fontSize: 10, formatter: '{value} MB/s' }, splitLine: { show: false } },
    ]
    opt.legend = { textStyle: { color: '#8b949e', fontSize: 11 } }
    opt.series = [
      makeLineSeries('当前磁盘', '#f0883e', 0.1, 0),
      makeLineSeries('当前↓入', '#58a6ff', 0.08, 1),
      makeLineSeries('当前↑出', '#d29922', 0.08, 1),
      makeLineSeries('前期磁盘', '#f0883e80', 0, 0, 'dashed'),
      makeLineSeries('前期↓入', '#58a6ff80', 0, 1, 'dashed'),
      makeLineSeries('前期↑出', '#d2992280', 0, 1, 'dashed'),
    ]
    compareDiskNetChart.setOption(opt)
  }

  if (anomalyCpuRef.value) {
    anomalyCpuChart = echarts.init(anomalyCpuRef.value, 'dark')
    const opt = makeBaseOpt()
    opt.xAxis = makeTimeAxis()
    opt.yAxis = makePercentYAxis()
    opt.series = [
      makeLineSeries('CPU', '#3fb950', 0.15),
      { name: '异常点', type: 'scatter', symbolSize: 10, itemStyle: { color: '#f85149' }, data: [] },
    ]
    anomalyCpuChart.setOption(opt)
  }

  if (anomalyMemRef.value) {
    anomalyMemChart = echarts.init(anomalyMemRef.value, 'dark')
    const opt = makeBaseOpt()
    opt.xAxis = makeTimeAxis()
    opt.yAxis = makePercentYAxis()
    opt.series = [
      makeLineSeries('内存', '#bc8cff', 0.15),
      { name: '异常点', type: 'scatter', symbolSize: 10, itemStyle: { color: '#f85149' }, data: [] },
    ]
    anomalyMemChart.setOption(opt)
  }
}

function disposeCharts() {
  [cpuChart, memChart, diskChart, netChart, compareCpuMemChart, compareDiskNetChart, anomalyCpuChart, anomalyMemChart].forEach(c => {
    try { c?.dispose() } catch {}
  })
  cpuChart = memChart = diskChart = netChart = compareCpuMemChart = compareDiskNetChart = anomalyCpuChart = anomalyMemChart = null
}

function computeAnomalyBand(data: number[]): { mean: number; upper: number; lower: number } {
  if (data.length < 3) return { mean: 0, upper: 100, lower: 0 }
  const mean = data.reduce((s, v) => s + v, 0) / data.length
  const std = Math.sqrt(data.reduce((s, v) => s + (v - mean) ** 2, 0) / data.length)
  return { mean, upper: Math.min(100, mean + 2 * std), lower: Math.max(0, mean - 2 * std) }
}

function pushData() {
  const cpuData = historyData.value.map((h: any) => [toTs(h.timestamp), getVal(h, ['cpu.usage_percent', 'cpu_usage'])])
  const memData = historyData.value.map((h: any) => [toTs(h.timestamp), getVal(h, ['memory.usage_percent', 'mem_usage_percent'])])

  const diskIOSeries = historyData.value.map((h: any, i: number) => {
    const readMB = getVal(h, ['disk_io.read_mb', 'read_mb'])
    const writeMB = getVal(h, ['disk_io.write_mb', 'write_mb'])
    const ts = toTs(h.timestamp)
    if (i === 0) return [ts, 0]
    const prevReadMB = getVal(historyData.value[i - 1], ['disk_io.read_mb', 'read_mb'])
    const prevWriteMB = getVal(historyData.value[i - 1], ['disk_io.write_mb', 'write_mb'])
    const prevTs = toTs(historyData.value[i - 1].timestamp)
    const dt = (ts - prevTs) / 1000
    if (dt <= 0) return [ts, 0]
    const rateMBps = ((readMB + writeMB - prevReadMB - prevWriteMB) / dt)
    return [ts, parseFloat(Math.max(0, rateMBps).toFixed(3))]
  })

  const diskUsageSeries = historyData.value.map((h: any) => [toTs(h.timestamp), getVal(h, ['disk.usage_percent', 'disk_usage_percent'])])

  const diskReadSeries = historyData.value.map((h: any, i: number) => {
    const readMB = getVal(h, ['disk_io.read_mb', 'read_mb'])
    const ts = toTs(h.timestamp)
    const readRateMB = getVal(h, ['disk_io.read_rate_mb', 'read_rate_mb'])
    if (readRateMB > 0) return [ts, readRateMB]
    if (i === 0) return [ts, 0]
    const prevReadMB = getVal(historyData.value[i - 1], ['disk_io.read_mb', 'read_mb'])
    const prevTs = toTs(historyData.value[i - 1].timestamp)
    const dt = (ts - prevTs) / 1000
    if (dt <= 0) return [ts, 0]
    return [ts, parseFloat(Math.max(0, (readMB - prevReadMB) / dt).toFixed(3))]
  })

  const diskWriteSeries = historyData.value.map((h: any, i: number) => {
    const writeMB = getVal(h, ['disk_io.write_mb', 'write_mb'])
    const ts = toTs(h.timestamp)
    const writeRateMB = getVal(h, ['disk_io.write_rate_mb', 'write_rate_mb'])
    if (writeRateMB > 0) return [ts, writeRateMB]
    if (i === 0) return [ts, 0]
    const prevWriteMB = getVal(historyData.value[i - 1], ['disk_io.write_mb', 'write_mb'])
    const prevTs = toTs(historyData.value[i - 1].timestamp)
    const dt = (ts - prevTs) / 1000
    if (dt <= 0) return [ts, 0]
    return [ts, parseFloat(Math.max(0, (writeMB - prevWriteMB) / dt).toFixed(3))]
  })

  const netRecvSeries = historyData.value.map((h: any, i: number) => {
    const ts = toTs(h.timestamp)
    const recvRateMB = getVal(h, ['network.recv_rate_mb', 'recv_rate_mb'])
    if (recvRateMB > 0) return [ts, recvRateMB]
    if (i === 0) return [ts, 0]
    const recv = getVal(h, ['network.bytes_recv', 'bytes_recv'])
    const prevRecv = getVal(historyData.value[i - 1], ['network.bytes_recv', 'bytes_recv'])
    const prevTs = toTs(historyData.value[i - 1].timestamp)
    const dt = (ts - prevTs) / 1000
    if (dt <= 0) return [ts, 0]
    const rateMBps = (recv - prevRecv) / 1024 / 1024 / dt
    return [ts, parseFloat(Math.max(0, rateMBps).toFixed(3))]
  })

  const netSentSeries = historyData.value.map((h: any, i: number) => {
    const ts = toTs(h.timestamp)
    const sentRateMB = getVal(h, ['network.sent_rate_mb', 'sent_rate_mb'])
    if (sentRateMB > 0) return [ts, sentRateMB]
    if (i === 0) return [ts, 0]
    const sent = getVal(h, ['network.bytes_sent', 'bytes_sent'])
    const prevSent = getVal(historyData.value[i - 1], ['network.bytes_sent', 'bytes_sent'])
    const prevTs = toTs(historyData.value[i - 1].timestamp)
    const dt = (ts - prevTs) / 1000
    if (dt <= 0) return [ts, 0]
    const rateMBps = (sent - prevSent) / 1024 / 1024 / dt
    return [ts, parseFloat(Math.max(0, rateMBps).toFixed(3))]
  })

  cpuChart?.setOption({ series: [{ data: cpuData }] })
  memChart?.setOption({ series: [{ data: memData }] })
  diskChart?.setOption({ series: [{ data: diskUsageSeries }, { data: diskReadSeries }, { data: diskWriteSeries }] })
  netChart?.setOption({ series: [{ data: netRecvSeries }, { data: netSentSeries }] })

  compareCpuMemChart?.setOption({ series: [{ data: cpuData }, { data: memData }, { data: [] }, { data: [] }] })
  compareDiskNetChart?.setOption({ series: [{ data: diskUsageSeries }, { data: netRecvSeries }, { data: netSentSeries }, { data: [] }, { data: [] }, { data: [] }] })

  if (compareData.value.current.length > 0 || compareData.value.previous.length > 0) {
    const curCpu = compareData.value.current.map((h: any) => [toTs(h.timestamp), getVal(h, ['cpu.usage_percent', 'cpu_usage'])])
    const curMem = compareData.value.current.map((h: any) => [toTs(h.timestamp), getVal(h, ['memory.usage_percent', 'mem_usage_percent'])])
    const prevCpu = compareData.value.previous.map((h: any) => [toTs(h.timestamp), getVal(h, ['cpu.usage_percent', 'cpu_usage'])])
    const prevMem = compareData.value.previous.map((h: any) => [toTs(h.timestamp), getVal(h, ['memory.usage_percent', 'mem_usage_percent'])])

    compareCpuMemChart?.setOption({
      legend: { data: ['当前CPU', '当前内存', '前期CPU', '前期内存'], textStyle: { color: '#8b949e', fontSize: 11 } },
      series: [
        { name: '当前CPU', data: curCpu },
        { name: '当前内存', data: curMem },
        { name: '前期CPU', data: prevCpu, lineStyle: { type: 'dashed' } },
        { name: '前期内存', data: prevMem, lineStyle: { type: 'dashed' } },
      ],
    })

    const curDiskUsage = compareData.value.current.map((h: any) => [toTs(h.timestamp), getVal(h, ['disk.usage_percent', 'disk_usage_percent'])])
    const curNetRecv = compareData.value.current.map((h: any) => [toTs(h.timestamp), getVal(h, ['network.recv_rate_mb', 'recv_rate_mb'])])
    const curNetSent = compareData.value.current.map((h: any) => [toTs(h.timestamp), getVal(h, ['network.sent_rate_mb', 'sent_rate_mb'])])
    const prevDiskUsage = compareData.value.previous.map((h: any) => [toTs(h.timestamp), getVal(h, ['disk.usage_percent', 'disk_usage_percent'])])
    const prevNetRecv = compareData.value.previous.map((h: any) => [toTs(h.timestamp), getVal(h, ['network.recv_rate_mb', 'recv_rate_mb'])])
    const prevNetSent = compareData.value.previous.map((h: any) => [toTs(h.timestamp), getVal(h, ['network.sent_rate_mb', 'sent_rate_mb'])])

    compareDiskNetChart?.setOption({
      legend: { data: ['当前磁盘', '当前↓入', '当前↑出', '前期磁盘', '前期↓入', '前期↑出'], textStyle: { color: '#8b949e', fontSize: 11 } },
      series: [
        { name: '当前磁盘', data: curDiskUsage },
        { name: '当前↓入', data: curNetRecv },
        { name: '当前↑出', data: curNetSent },
        { name: '前期磁盘', data: prevDiskUsage, lineStyle: { type: 'dashed' } },
        { name: '前期↓入', data: prevNetRecv, lineStyle: { type: 'dashed' } },
        { name: '前期↑出', data: prevNetSent, lineStyle: { type: 'dashed' } },
      ],
    })
  }

  const cpuVals = vals(['cpu.usage_percent', 'cpu_usage'])
  const memVals = vals(['memory.usage_percent', 'mem_usage_percent'])
  const cpuBand = computeAnomalyBand(cpuVals)
  const memBand = computeAnomalyBand(memVals)

  const cpuAnomalyScatter = cpuData.filter((d: number[]) => d[1] > cpuBand.upper || d[1] < cpuBand.lower)
  const memAnomalyScatter = memData.filter((d: number[]) => d[1] > memBand.upper || d[1] < memBand.lower)

  anomalyCpuChart?.setOption({
    series: [
      { data: cpuData },
      { data: cpuAnomalyScatter },
    ],
  })
  if (cpuData.length > 0) {
    anomalyCpuChart?.setOption({
      series: [{
        markArea: {
          silent: true,
          itemStyle: { color: 'rgba(63,185,80,0.08)' },
          data: [[{ yAxis: cpuBand.lower }, { yAxis: cpuBand.upper }]],
        },
        markLine: {
          silent: true, symbol: 'none',
          lineStyle: { color: '#3fb95066', type: 'dotted', width: 1 },
          data: [
            { yAxis: cpuBand.mean, label: { formatter: `均值 ${cpuBand.mean.toFixed(1)}%`, color: '#8b949e', fontSize: 10 } },
            { yAxis: cpuBand.upper, label: { formatter: `+2σ ${cpuBand.upper.toFixed(1)}%`, color: '#f85149', fontSize: 10 }, lineStyle: { color: '#f8514966' } },
            { yAxis: cpuBand.lower, label: { formatter: `-2σ ${cpuBand.lower.toFixed(1)}%`, color: '#f85149', fontSize: 10 }, lineStyle: { color: '#f8514966' } },
          ],
        },
      }],
    })
  }

  anomalyMemChart?.setOption({
    series: [
      { data: memData },
      { data: memAnomalyScatter },
    ],
  })
  if (memData.length > 0) {
    anomalyMemChart?.setOption({
      series: [{
        markArea: {
          silent: true,
          itemStyle: { color: 'rgba(188,140,255,0.08)' },
          data: [[{ yAxis: memBand.lower }, { yAxis: memBand.upper }]],
        },
        markLine: {
          silent: true, symbol: 'none',
          lineStyle: { color: '#bc8cff66', type: 'dotted', width: 1 },
          data: [
            { yAxis: memBand.mean, label: { formatter: `均值 ${memBand.mean.toFixed(1)}%`, color: '#8b949e', fontSize: 10 } },
            { yAxis: memBand.upper, label: { formatter: `+2σ ${memBand.upper.toFixed(1)}%`, color: '#f85149', fontSize: 10 }, lineStyle: { color: '#f8514966' } },
            { yAxis: memBand.lower, label: { formatter: `-2σ ${memBand.lower.toFixed(1)}%`, color: '#f85149', fontSize: 10 }, lineStyle: { color: '#f8514966' } },
          ],
        },
      }],
    })
  }

}

async function loadHistory() {
  loading.value = true
  try {
    const resp = await client.get('/history', { params: { duration: duration.value, limit: 500 } })
    const data = Array.isArray(resp.data) ? resp.data : []
    historyData.value = data
    if (data.length === 0) {
      console.warn('[trends] no history data')
    }
  } catch (e: unknown) {
    console.warn('[trends] load error:', (e as Error)?.message || e)
    historyData.value = []
  }
}

async function loadCompare() {
  try {
    const resp = await client.get('/trend/compare', { params: { period: duration.value } })
    if (resp.data && typeof resp.data === 'object') {
      compareData.value = {
        current: Array.isArray(resp.data.current) ? resp.data.current : [],
        previous: Array.isArray(resp.data.previous) ? resp.data.previous : [],
      }
    } else {
      compareData.value = { current: [], previous: [] }
    }
  } catch (e: unknown) {
    console.warn('[trends] compare load error:', (e as Error)?.message || e)
    compareData.value = { current: [], previous: [] }
  }
}

async function load() {
  if (!selectedNode.value) return
  loading.value = true
  try {
    await loadHistory()
    await buildCharts()
    pushData()
  } finally {
    loading.value = false
  }
}

function exportReport() {
  const text = [
    `DevDash 周报 - ${selectedNodeName.value}`,
    `时间范围: ${durationLabel.value}`,
    `--- 系统指标 ---`,
    `CPU:     均值${avgCpu.value}% 峰值${maxCpu.value}% 低谷${minCpu.value}%`,
    `内存:    均值${avgMem.value}% 峰值${maxMem.value}% 低谷${minMem.value}%`,
    `磁盘:    均值${avgDisk.value}% 峰值${maxDisk.value}% 低谷${minDisk.value}%`,
    `网络:    总计${totalNet.value}GB (入${totalIn.value} 出${totalOut.value})`,
    `--- 异常检测 ---`,
    anomalyPoints.value.length > 0 ? `发现 ${anomalyPoints.value.length} 个异常点` : '未发现异常',
  ].join('\n')
  const blob = new Blob([text], { type: 'text/plain;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = `report-${selectedNodeName.value}-${duration.value}.txt`
  a.click()
  URL.revokeObjectURL(url)
}

function handleResize() {
  [cpuChart, memChart, diskChart, netChart, compareCpuMemChart, compareDiskNetChart, anomalyCpuChart, anomalyMemChart].forEach(c => {
    try { c?.resize() } catch {}
  })
}

onMounted(async () => {
  window.addEventListener('resize', handleResize)
  await buildCharts()
  load()
})

watch(selectedNode, () => { load() })

async function onTabChange(name: string) {
  await buildCharts()
  if (name === 'compare') {
    await loadCompare()
  }
  if (historyData.value.length > 0) {
    pushData()
  }
}
onUnmounted(() => {
  window.removeEventListener('resize', handleResize)
  disposeCharts()
})
</script>

<style scoped>
.page { padding: 24px; }
.page-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 16px; flex-wrap: wrap; gap: 8px; }
h2 { font-size: 20px; font-weight: 600; margin: 0; }
.charts-row { display: grid; grid-template-columns: 1fr 1fr; gap: 16px; margin-bottom: 16px; }
.chart-box { background: #161b22; border: 1px solid #30363d; border-radius: 8px; padding: 16px; }
.chart-title { font-size: 13px; color: #8b949e; margin-bottom: 8px; }
.stats-row { display: grid; grid-template-columns: repeat(auto-fit, minmax(180px, 1fr)); gap: 12px; margin-bottom: 16px; }
.stat-box { background: #161b22; border: 1px solid #30363d; border-radius: 8px; padding: 16px; }
.stat-label { font-size: 12px; color: #8b949e; margin-bottom: 6px; }
.stat-val { font-size: 24px; font-weight: 700; }
.stat-sub { font-size: 11px; color: #6e7681; margin-top: 4px; }
.empty-state { text-align: center; padding: 60px 20px; color: #6e7681; }
.empty-icon { font-size: 48px; margin-bottom: 12px; }
.report-box { background: #161b22; border: 1px solid #30363d; border-radius: 8px; padding: 20px; }
.report-title { font-size: 14px; font-weight: 600; margin-bottom: 12px; }
.report-content p { margin: 0 0 6px; font-size: 13px; color: #8b949e; line-height: 1.8; }
.report-content strong { color: #e6edf3; }
.anomaly-summary { background: #161b22; border: 1px solid #30363d; border-radius: 8px; padding: 16px; margin-top: 16px; }
.anomaly-title { font-size: 13px; color: #f85149; margin-bottom: 12px; font-weight: 600; }
.anomaly-list { display: flex; flex-direction: column; gap: 4px; }
.anomaly-item { display: flex; gap: 12px; padding: 6px 10px; background: #f8514911; border-radius: 4px; font-size: 12px; }
.anomaly-time { color: #8b949e; min-width: 150px; }
.anomaly-metric { color: #e6edf3; font-weight: 500; min-width: 40px; }
.anomaly-val { color: #f85149; font-weight: 600; min-width: 60px; }
.anomaly-range { color: #6e7681; }

@media (max-width: 768px) {
  .charts-row { grid-template-columns: 1fr; }
}
</style>
