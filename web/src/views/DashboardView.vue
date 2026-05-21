<template>
  <app-layout>
    <div class="page">
      <div class="page-header">
        <h2>全局仪表板</h2>
        <div style="display:flex;gap:8px;align-items:center">
          <span v-if="snap.current" class="last-update">更新于 {{ formatTime(snap.current.timestamp) }}</span>
          <n-button size="small" @click="refresh" :loading="snap.loading">
            <template #icon><refresh-icon /></template>刷新
          </n-button>
        </div>
      </div>

      <div class="summary-row">
        <metric-card label="主机数" :value="String(nodesStore.nodes.length)" :sub="onlineSummary" color="#58a6ff" />
        <metric-card label="CPU 使用率" :value="cpuLabel" :sub="coresLabel" color="#3fb950" />
        <metric-card label="内存使用率" :value="memLabel" :sub="memSubLabel" color="#bc8cff" />
        <metric-card label="磁盘使用率" :value="diskLabel" :sub="diskSubLabel" color="#d29922" />
        <metric-card label="网络收发" :value="netLabel" :sub="netSubLabel" color="#f0883e" />
        <metric-card label="负载(1m)" :value="loadLabel" :sub="loadSubLabel" color="#f85149" />
      </div>

      <div class="charts-row">
        <div class="chart-box">
          <div class="chart-title">CPU & 内存 实时趋势 <span class="chart-hint">每30s自动刷新</span></div>
          <div ref="chartRef" style="height:220px;width:100%" />
        </div>
        <div class="chart-box">
          <div class="chart-title">磁盘 & 网络 实时趋势</div>
          <div ref="chart2Ref" style="height:220px;width:100%" />
        </div>
      </div>

      <div v-if="snap.current" class="info-grid">
        <div class="info-card">
          <div class="info-title">系统信息</div>
          <div class="info-row"><span>主机名</span><span>{{ snap.current.host?.hostname }}</span></div>
          <div class="info-row"><span>操作系统</span><span>{{ snap.current.host?.platform }} {{ snap.current.host?.platform_version }}</span></div>
          <div class="info-row"><span>运行时长</span><span>{{ uptimeLabel }}</span></div>
          <div class="info-row"><span>CPU 核心</span><span>{{ snap.cpuCores }} 核</span></div>
        </div>
        <div class="info-card">
          <div class="info-title">网络连接</div>
          <div class="info-row"><span>TCP 已建立</span><span class="ok">{{ snap.current.tcp_conns?.established ?? 0 }}</span></div>
          <div class="info-row"><span>LISTEN</span><span>{{ snap.current.tcp_conns?.listen ?? 0 }}</span></div>
          <div class="info-row"><span>TIME_WAIT</span><span>{{ snap.current.tcp_conns?.time_wait ?? 0 }}</span></div>
          <div class="info-row"><span>CLOSE_WAIT</span><span>{{ snap.current.tcp_conns?.close_wait ?? 0 }}</span></div>
        </div>
        <div class="info-card">
          <div class="info-title">磁盘 IO</div>
          <div class="info-row"><span>累计读取</span><span>{{ snap.current.disk_io?.read_mb ? (snap.current.disk_io.read_mb / 1024).toFixed(1) + ' GB' : '--' }}</span></div>
          <div class="info-row"><span>累计写入</span><span>{{ snap.current.disk_io?.write_mb ? (snap.current.disk_io.write_mb / 1024).toFixed(1) + ' GB' : '--' }}</span></div>
        </div>
      </div>

      <div v-if="alertItems.length" class="alert-section">
        <div class="alert-title">⚠️ 触发告警</div>
        <div class="alert-list">
          <div v-for="a in alertItems" :key="a.id" class="alert-item">
            <span class="alert-node">{{ a.node }}</span>
            <span class="alert-reason">{{ a.msg }}</span>
          </div>
        </div>
      </div>
    </div>
  </app-layout>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, nextTick } from 'vue'
import { NButton } from 'naive-ui'
import { Refresh as RefreshIcon } from '@vicons/ionicons5'
import * as echarts from 'echarts/core'
import { LineChart } from 'echarts/charts'
import { GridComponent, TooltipComponent, LegendComponent } from 'echarts/components'
import { UniversalTransition } from 'echarts/features'
import { CanvasRenderer } from 'echarts/renderers'
echarts.use([LineChart, GridComponent, TooltipComponent, LegendComponent, UniversalTransition, CanvasRenderer])
import AppLayout from '@/components/AppLayout.vue'
import MetricCard from '@/components/MetricCard.vue'
import { useSnapshotStore } from '@/stores/snapshot'
import { useNodesStore } from '@/stores/nodes'

const snap = useSnapshotStore()
const nodesStore = useNodesStore()

const chartRef = ref<HTMLDivElement>()
const chart2Ref = ref<HTMLDivElement>()
let chart: echarts.ECharts | null = null
let chart2: echarts.ECharts | null = null
let pollTimer: ReturnType<typeof setInterval>

const MAX_POINTS = 60
const cpuData: [number, number][] = []
const memData: [number, number][] = []
const diskData: [number, number][] = []
const netRecvData: [number, number][] = []
const netSentData: [number, number][] = []

const onlineCount = computed(() => nodesStore.nodes.filter((n: any) => n.status === 'online').length)
const onlineSummary = computed(() => `在线 ${onlineCount.value} / 离线 ${nodesStore.nodes.length - onlineCount.value}`)

const cpuLabel = computed(() => snap.cpuPercent.toFixed(1) + '%')
const coresLabel = computed(() => `核心数 ${snap.cpuCores}`)
const memLabel = computed(() => snap.memPercent.toFixed(1) + '%')
const memSubLabel = computed(() => {
  const m = snap.current?.memory
  if (!m) return '--'
  return `${m.used_gb.toFixed(1)} / ${m.total_gb.toFixed(1)} GB`
})
const diskLabel = computed(() => snap.diskPercent.toFixed(1) + '%')
const diskSubLabel = computed(() => {
  const d = snap.current?.disk
  if (!d) return '--'
  return `${d.used_gb.toFixed(1)} / ${d.total_gb.toFixed(1)} GB`
})
const netLabel = computed(() => {
  const r = parseFloat(snap.netRecvMB)
  const s = parseFloat(snap.netSentMB)
  return `${(r + s).toFixed(1)} MB/s`
})
const netSubLabel = computed(() => `\u2193 ${snap.netRecvMB}  \u2191 ${snap.netSentMB}`)
const loadLabel = computed(() => snap.load1.toFixed(2))
const loadSubLabel = computed(() => {
  const l = snap.current?.load
  if (!l) return '--'
  return `5m ${l.load5.toFixed(2)} / 15m ${l.load15.toFixed(2)}`
})
const uptimeLabel = computed(() => {
  const s = snap.uptimeSeconds
  if (!s) return '--'
  const d = Math.floor(s / 86400)
  const h = Math.floor((s % 86400) / 3600)
  return d > 0 ? `${d} 天 ${h} 小时` : `${h} 小时`
})

const alertItems = computed(() => {
  const cur = snap.current
  if (!cur) return []
  const items: { id: number; node: string; msg: string }[] = []
  if (cur.cpu.usage_percent > 80) items.push({ id: 1, node: cur.host?.hostname || '本地', msg: `CPU ${cur.cpu.usage_percent.toFixed(1)}%` })
  if (cur.memory.usage_percent > 85) items.push({ id: 2, node: cur.host?.hostname || '本地', msg: `内存 ${cur.memory.usage_percent.toFixed(1)}%` })
  if (cur.disk.usage_percent > 90) items.push({ id: 3, node: cur.host?.hostname || '本地', msg: `磁盘 ${cur.disk.usage_percent.toFixed(1)}%` })
  return items
})

function formatTime(ts: string) {
  try { return new Date(ts).toLocaleTimeString('zh-CN', { hour12: false }) }
  catch { return ts }
}

function hexToRgba(hex: string, alpha: number): string {
  const h = hex.replace('#', '')
  const r = parseInt(h.substring(0, 2), 16)
  const g = parseInt(h.substring(2, 4), 16)
  const b = parseInt(h.substring(4, 6), 16)
  return `rgba(${r},${g},${b},${alpha})`
}

function createGradientColor(color: string, opacity: number = 0.15): any {
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

function makeBaseOpt(): Record<string, unknown> {
  return {
    grid: { top: 30, right: 20, bottom: 30, left: 50 },
    legend: { top: 5, right: 10, textStyle: { color: '#8b949e', fontSize: 11 }, itemWidth: 16, itemHeight: 3 },
    tooltip: {
      trigger: 'axis',
      backgroundColor: '#1f242c',
      borderColor: '#30363d',
      borderWidth: 1,
      textStyle: { color: '#e6edf3', fontSize: 12 },
      axisPointer: { type: 'cross', lineStyle: { color: '#484f58', width: 1 } },
    },
    animation: true,
    animationDuration: 300,
    animationEasing: 'cubicOut',
    backgroundColor: 'transparent',
  }
}

function initCharts() {
  disposeCharts()
  nextTick(() => {
    if (chartRef.value) {
      chart = echarts.init(chartRef.value, 'dark')
      const opt = makeBaseOpt()
      opt.xAxis = {
        type: 'time',
        boundaryGap: false,
        axisLine: { lineStyle: { color: '#30363d' } },
        axisLabel: { color: '#6e7681', fontSize: 10, formatter: '{HH}:{mm}' },
        splitLine: { show: true, lineStyle: { color: '#21262d', type: 'dashed' } }
      }
      opt.yAxis = {
        type: 'value',
        min: 0,
        max: 100,
        axisLine: { lineStyle: { color: '#30363d' } },
        axisLabel: { color: '#6e7681', fontSize: 10, formatter: '{value}%' },
        splitLine: { lineStyle: { color: '#21262d', type: 'dashed' } }
      }
      opt.series = [
        {
          name: 'CPU',
          type: 'line',
          smooth: 0.4,
          showSymbol: false,
          sampling: 'lttb',
          lineStyle: { width: 2.5, color: '#3fb950', shadowBlur: 10, shadowColor: 'rgba(63,185,80,0.3)' },
          itemStyle: { color: '#3fb950' },
          areaStyle: { color: createGradientColor('rgb(63,185,80)', 0.2) },
          data: [],
          emphasis: { focus: 'series', lineStyle: { width: 3 } }
        },
        {
          name: '内存',
          type: 'line',
          smooth: 0.4,
          showSymbol: false,
          sampling: 'lttb',
          lineStyle: { width: 2.5, color: '#bc8cff', shadowBlur: 10, shadowColor: 'rgba(188,140,255,0.3)' },
          itemStyle: { color: '#bc8cff' },
          areaStyle: { color: createGradientColor('rgb(188,140,255)', 0.2) },
          data: [],
          emphasis: { focus: 'series', lineStyle: { width: 3 } }
        },
      ]
      chart.setOption(opt)
    }

    if (chart2Ref.value) {
      chart2 = echarts.init(chart2Ref.value, 'dark')
      const opt = makeBaseOpt()
      opt.xAxis = {
        type: 'time',
        boundaryGap: false,
        axisLine: { lineStyle: { color: '#30363d' } },
        axisLabel: { color: '#6e7681', fontSize: 10, formatter: '{HH}:{mm}' },
        splitLine: { show: true, lineStyle: { color: '#21262d', type: 'dashed' } }
      }
      opt.yAxis = [
        {
          type: 'value',
          min: 0,
          max: 100,
          position: 'left',
          axisLine: { lineStyle: { color: '#30363d' } },
          axisLabel: { color: '#6e7681', fontSize: 10, formatter: '{value}%' },
          splitLine: { lineStyle: { color: '#21262d', type: 'dashed' } }
        },
        {
          type: 'value',
          min: 0,
          position: 'right',
          axisLine: { show: false },
          axisLabel: { color: '#6e7681', fontSize: 10, formatter: '{value}' },
          splitLine: { show: false }
        },
      ]
      opt.series = [
        {
          name: '磁盘',
          type: 'line',
          smooth: 0.4,
          yAxisIndex: 0,
          showSymbol: false,
          sampling: 'lttb',
          lineStyle: { width: 2.5, color: '#d29922', shadowBlur: 10, shadowColor: 'rgba(210,153,34,0.3)' },
          itemStyle: { color: '#d29922' },
          areaStyle: { color: createGradientColor('rgb(210,153,34)', 0.15) },
          data: [],
          emphasis: { focus: 'series', lineStyle: { width: 3 } }
        },
        {
          name: '\u2193入流量',
          type: 'line',
          smooth: 0.4,
          yAxisIndex: 1,
          showSymbol: false,
          sampling: 'lttb',
          lineStyle: { width: 2.5, color: '#58a6ff', shadowBlur: 10, shadowColor: 'rgba(88,166,255,0.3)' },
          itemStyle: { color: '#58a6ff' },
          areaStyle: { color: createGradientColor('rgb(88,166,255)', 0.12) },
          data: [],
          emphasis: { focus: 'series', lineStyle: { width: 3 } }
        },
        {
          name: '\u2191出流量',
          type: 'line',
          smooth: 0.4,
          yAxisIndex: 1,
          showSymbol: false,
          sampling: 'lttb',
          lineStyle: { width: 2.5, color: '#f0883e', shadowBlur: 10, shadowColor: 'rgba(240,136,62,0.3)' },
          itemStyle: { color: '#f0883e' },
          areaStyle: { color: createGradientColor('rgb(240,136,62)', 0.12) },
          data: [],
          emphasis: { focus: 'series', lineStyle: { width: 3 } }
        },
      ]
      chart2.setOption(opt)
    }

    pushData()
  })
}

function disposeCharts() {
  try { chart?.dispose(); chart = null } catch {}
  try { chart2?.dispose(); chart2 = null } catch {}
}

function pushData() {
  const cur = snap.current
  if (!cur) return

  const now = Date.now()
  const cpuVal = typeof cur.cpu?.usage_percent === 'number' ? cur.cpu.usage_percent : 0
  const memVal = typeof cur.memory?.usage_percent === 'number' ? cur.memory.usage_percent : 0
  const diskVal = typeof cur.disk?.usage_percent === 'number' ? cur.disk.usage_percent : 0
  const netIn = parseFloat(snap.netRecvMB) || 0
  const netOut = parseFloat(snap.netSentMB) || 0

  cpuData.push([now, cpuVal])
  memData.push([now, memVal])
  diskData.push([now, diskVal])
  netRecvData.push([now, netIn])
  netSentData.push([now, netOut])

  while (cpuData.length > MAX_POINTS) { cpuData.shift(); memData.shift(); diskData.shift(); netRecvData.shift(); netSentData.shift() }

  if (chart) {
    try {
      chart.setOption({
        series: [
          { data: cpuData.map(d => [...d]) },
          { data: memData.map(d => [...d]) },
        ],
      })
    } catch (e) {
      console.warn('[dashboard] chart1 error:', e)
    }
  }

  if (chart2) {
    try {
      chart2.setOption({
        series: [
          { data: diskData.map(d => [...d]) },
          { data: netRecvData.map(d => [...d]) },
          { data: netSentData.map(d => [...d]) },
        ],
      })
    } catch (e) {
      console.warn('[dashboard] chart2 error:', e)
    }
  }
}

async function refresh() {
  try {
    await snap.triggerCollect()
    await nodesStore.fetchNodes()
    pushData()
  } catch (e) {
    console.warn('[dashboard] refresh error:', (e as Error)?.message || e)
  }
}

function handleResize() {
  try { chart?.resize(); chart2?.resize() } catch {}
}

onMounted(async () => {
  window.addEventListener('resize', handleResize)
  await nodesStore.fetchNodes()
  initCharts()

  try {
    await snap.fetchHistory(60)
    const hist = snap.history
    if (hist && hist.length > 0) {
      for (const h of hist) {
        const t = new Date(h.timestamp).getTime()
        if (isNaN(t)) continue
        cpuData.push([t, typeof h.cpu?.usage_percent === 'number' ? h.cpu.usage_percent : 0])
        memData.push([t, typeof h.memory?.usage_percent === 'number' ? h.memory.usage_percent : 0])
        diskData.push([t, typeof h.disk?.usage_percent === 'number' ? h.disk.usage_percent : 0])
        const nr = (h.network?.bytes_recv ?? 0) / 1024 / 1024
        const ns = (h.network?.bytes_sent ?? 0) / 1024 / 1024
        netRecvData.push([t, parseFloat(nr.toFixed(2))])
        netSentData.push([t, parseFloat(ns.toFixed(2))])
      }
      while (cpuData.length > MAX_POINTS) { cpuData.shift(); memData.shift(); diskData.shift(); netRecvData.shift(); netSentData.shift() }
      if (chart) {
        try { chart.setOption({ series: [{ data: cpuData.map(d => [...d]) }, { data: memData.map(d => [...d]) }] }) } catch {}
      }
      if (chart2) {
        try { chart2.setOption({ series: [{ data: diskData.map(d => [...d]) }, { data: netRecvData.map(d => [...d]) }, { data: netSentData.map(d => [...d]) }] }) } catch {}
      }
    }
  } catch (e) {
    console.warn('[dashboard] load history failed:', e)
  }

  await snap.fetchLatest()
  pushData()
  pollTimer = setInterval(refresh, 5000)
})

onUnmounted(() => {
  clearInterval(pollTimer)
  window.removeEventListener('resize', handleResize)
  disposeCharts()
})
</script>

<style scoped>
.page { padding: 24px; }
.page-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 20px; flex-wrap: wrap; gap: 8px; }
h2 { font-size: 20px; font-weight: 600; margin: 0; }
.last-update { font-size: 12px; color: #6e7681; }

.summary-row { display: grid; grid-template-columns: repeat(auto-fit, minmax(170px, 1fr)); gap: 12px; margin-bottom: 20px; }
.charts-row { display: grid; grid-template-columns: 1fr 1fr; gap: 16px; margin-bottom: 20px; }
.chart-box { background: #161b22; border: 1px solid #30363d; border-radius: 8px; padding: 16px; overflow: hidden; }
.chart-title { font-size: 13px; color: #8b949e; margin-bottom: 8px; }
.chart-hint { font-size: 11px; color: #484f58; margin-left: 6px; }

.info-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(240px, 1fr)); gap: 12px; margin-bottom: 20px; }
.info-card { background: #161b22; border: 1px solid #30363d; border-radius: 8px; padding: 14px 16px; }
.info-title { font-size: 13px; color: #8b949e; margin-bottom: 10px; font-weight: 600; }
.info-row { display: flex; justify-content: space-between; align-items: center; font-size: 13px; padding: 3px 0; border-bottom: 1px solid #21262d; }
.info-row:last-child { border-bottom: none; }
.info-row span:first-child { color: #6e7681; }
.info-row span:last-child { color: #e6edf3; font-weight: 500; }
.info-row .ok { color: #3fb950; }

.alert-section { background: #161b22; border: 1px solid #f8514966; border-radius: 8px; padding: 16px; margin-bottom: 0; }
.alert-title { font-size: 13px; color: #f85149; margin-bottom: 12px; font-weight: 600; }
.alert-list { display: flex; flex-direction: column; gap: 6px; }
.alert-item { display: flex; justify-content: space-between; padding: 8px 12px; background: #f8514911; border-radius: 6px; }
.alert-node { color: #e6edf3; font-weight: 500; font-size: 13px; }
.alert-reason { color: #f85149; font-size: 13px; }
</style>