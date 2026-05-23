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
        <metric-card label="磁盘使用率" :value="diskLabel" :sub="diskSubLabel" color="#f0883e" />
        <metric-card label="网络收发" :value="netLabel" :sub="netSubLabel" color="#d29922" />
        <metric-card v-if="!isWindows" label="负载(1m)" :value="loadLabel" :sub="loadSubLabel" color="#f85149" />
        <metric-card v-else label="负载(1m)" value="--" sub="Windows 不支持此指标" color="#6e7681" />
      </div>

      <div class="monitor-grid">
        <div class="monitor-panel">
          <div class="panel-header">
            <span class="panel-dot" style="background:#3fb950"></span>
            <span class="panel-title">CPU 监控</span>
            <span class="panel-value" style="color:#3fb950">{{ cpuLabel }}</span>
          </div>
          <div class="panel-sub">{{ coresLabel }}</div>
          <div ref="cpuChartRef" class="panel-chart" />
        </div>

        <div class="monitor-panel">
          <div class="panel-header">
            <span class="panel-dot" style="background:#bc8cff"></span>
            <span class="panel-title">内存使用</span>
            <span class="panel-value" style="color:#bc8cff">{{ memLabel }}</span>
          </div>
          <div class="panel-sub">{{ memSubLabel }}</div>
          <div ref="memChartRef" class="panel-chart" />
        </div>

        <div class="monitor-panel">
          <div class="panel-header">
            <span class="panel-dot" style="background:#f0883e"></span>
            <span class="panel-title">磁盘状态</span>
            <span class="panel-value" style="color:#f0883e">{{ diskLabel }}</span>
          </div>
          <div class="panel-sub">{{ diskSubLabel }}</div>
          <div ref="diskChartRef" class="panel-chart" />
        </div>

        <div class="monitor-panel">
          <div class="panel-header">
            <span class="panel-dot" style="background:#d29922"></span>
            <span class="panel-title">网络流量</span>
            <span class="panel-value" style="color:#d29922">{{ netLabel }}</span>
          </div>
          <div class="panel-sub">{{ netSubLabel }}</div>
          <div ref="netChartRef" class="panel-chart" />
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
import { GridComponent, TooltipComponent, LegendComponent, MarkLineComponent, DataZoomComponent } from 'echarts/components'
import { UniversalTransition } from 'echarts/features'
import { CanvasRenderer } from 'echarts/renderers'
echarts.use([LineChart, GridComponent, TooltipComponent, LegendComponent, MarkLineComponent, DataZoomComponent, UniversalTransition, CanvasRenderer])
import AppLayout from '@/components/AppLayout.vue'
import MetricCard from '@/components/MetricCard.vue'
import { useSnapshotStore } from '@/stores/snapshot'
import { useNodesStore } from '@/stores/nodes'

const snap = useSnapshotStore()
const nodesStore = useNodesStore()

const cpuChartRef = ref<HTMLDivElement>()
const memChartRef = ref<HTMLDivElement>()
const diskChartRef = ref<HTMLDivElement>()
const netChartRef = ref<HTMLDivElement>()
let cpuChart: echarts.ECharts | null = null
let memChart: echarts.ECharts | null = null
let diskChart: echarts.ECharts | null = null
let netChart: echarts.ECharts | null = null
let pollTimer: ReturnType<typeof setInterval>

const MAX_POINTS = 120
const cpuData: [number, number][] = []
const memData: [number, number][] = []
const diskData: [number, number][] = []
const diskReadData: [number, number][] = []
const diskWriteData: [number, number][] = []
const netRecvData: [number, number][] = []
const netSentData: [number, number][] = []

const onlineCount = computed(() => nodesStore.nodes.filter((n: any) => n.status === 'online').length)
const onlineSummary = computed(() => `在线 ${onlineCount.value} / 离线 ${nodesStore.nodes.length - onlineCount.value}`)
const isWindows = computed(() => {
  const platform = snap.current?.host?.platform || ''
  return platform.toLowerCase().includes('windows')
})

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
  const cur = snap.current
  if (!cur?.network) return '-- MB/s'
  const r = typeof cur.network.recv_rate_mb === 'number' ? cur.network.recv_rate_mb : 0
  const s = typeof cur.network.sent_rate_mb === 'number' ? cur.network.sent_rate_mb : 0
  return `${(r + s).toFixed(3)} MB/s`
})
const netSubLabel = computed(() => {
  const cur = snap.current
  if (!cur?.network) return '\u2193 0  \u2191 0 MB/s'
  const r = typeof cur.network.recv_rate_mb === 'number' ? cur.network.recv_rate_mb : 0
  const s = typeof cur.network.sent_rate_mb === 'number' ? cur.network.sent_rate_mb : 0
  return `\u2193 ${r.toFixed(3)}  \u2191 ${s.toFixed(3)} MB/s`
})
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

function makeBaseOpt(): any {
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

function makeTimeAxis(): any {
  return {
    type: 'time',
    boundaryGap: false,
    axisLine: { lineStyle: { color: '#30363d' } },
    axisLabel: { color: '#6e7681', fontSize: 10, formatter: '{HH}:{mm}' },
    splitLine: { show: true, lineStyle: { color: '#21262d', type: 'dashed' } },
  }
}

function makeLineSeries(name: string, color: string, areaOpacity: number, yAxisIndex = 0): any {
  return {
    name,
    type: 'line',
    smooth: 0.4,
    showSymbol: false,
    sampling: 'lttb',
    yAxisIndex,
    lineStyle: { width: 2.5, color, shadowBlur: 10, shadowColor: hexToRgba(color, 0.3) },
    itemStyle: { color },
    areaStyle: { color: createGradientColor(color, areaOpacity) },
    data: [],
    emphasis: { focus: 'series', lineStyle: { width: 3 } },
    connectNulls: true,
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

function initCharts(): Promise<void> {
  disposeCharts()
  return new Promise(resolve => {
    nextTick(() => {
      if (cpuChartRef.value) {
        cpuChart = echarts.init(cpuChartRef.value, 'dark')
        const opt = makeBaseOpt()
        opt.grid = { top: 30, right: 20, bottom: 30, left: 50 }
        opt.xAxis = makeTimeAxis()
        opt.yAxis = makePercentYAxis()
        opt.series = [
          makeLineSeries('CPU', '#3fb950', 0.2),
        ]
        opt.series[0].markLine = {
          silent: true,
          symbol: 'none',
          lineStyle: { color: '#f8514966', type: 'dashed', width: 1 },
          data: [{ yAxis: 80, label: { formatter: '告警线 80%', color: '#f85149', fontSize: 10 } }],
        }
        cpuChart.setOption(opt)
      }

      if (memChartRef.value) {
        memChart = echarts.init(memChartRef.value, 'dark')
        const opt = makeBaseOpt()
        opt.grid = { top: 30, right: 20, bottom: 30, left: 50 }
        opt.xAxis = makeTimeAxis()
        opt.yAxis = makePercentYAxis()
        opt.series = [
          makeLineSeries('内存', '#bc8cff', 0.2),
        ]
        opt.series[0].markLine = {
          silent: true,
          symbol: 'none',
          lineStyle: { color: '#f8514966', type: 'dashed', width: 1 },
          data: [{ yAxis: 85, label: { formatter: '告警线 85%', color: '#f85149', fontSize: 10 } }],
        }
        memChart.setOption(opt)
      }

      if (diskChartRef.value) {
        diskChart = echarts.init(diskChartRef.value, 'dark')
        const opt = makeBaseOpt()
        opt.grid = { top: 40, right: 20, bottom: 30, left: 50 }
        opt.xAxis = makeTimeAxis()
        opt.yAxis = [
          {
            type: 'value',
            min: 0,
            max: 100,
            position: 'left',
            axisLine: { lineStyle: { color: '#30363d' } },
            axisLabel: { color: '#6e7681', fontSize: 10, formatter: '{value}%' },
            splitLine: { lineStyle: { color: '#21262d', type: 'dashed' } },
          },
          {
            type: 'value',
            min: 0,
            position: 'right',
            axisLine: { show: false },
            axisLabel: { color: '#6e7681', fontSize: 10, formatter: '{value} MB/s' },
            splitLine: { show: false },
          },
        ]
        opt.series = [
          makeLineSeries('磁盘使用', '#f0883e', 0.15, 0),
          makeLineSeries('读取速率', '#58a6ff', 0.08, 1),
          makeLineSeries('写入速率', '#d29922', 0.08, 1),
        ]
        opt.series[0].markLine = {
          silent: true,
          symbol: 'none',
          lineStyle: { color: '#f8514966', type: 'dashed', width: 1 },
          data: [{ yAxis: 90, label: { formatter: '告警线 90%', color: '#f85149', fontSize: 10 } }],
        }
        diskChart.setOption(opt)
      }

      if (netChartRef.value) {
        netChart = echarts.init(netChartRef.value, 'dark')
        const opt = makeBaseOpt()
        opt.grid = { top: 30, right: 20, bottom: 30, left: 50 }
        opt.xAxis = makeTimeAxis()
        opt.yAxis = {
          type: 'value',
          min: 0,
          axisLine: { lineStyle: { color: '#30363d' } },
          axisLabel: { color: '#6e7681', fontSize: 10, formatter: '{value} MB/s' },
          splitLine: { lineStyle: { color: '#21262d', type: 'dashed' } },
        }
        opt.series = [
          makeLineSeries('\u2193 入流量', '#58a6ff', 0.12),
          makeLineSeries('\u2191 出流量', '#d29922', 0.12),
        ]
        netChart.setOption(opt)
      }

      resolve()
    })
  })
}

function disposeCharts() {
  try { cpuChart?.dispose(); cpuChart = null } catch {}
  try { memChart?.dispose(); memChart = null } catch {}
  try { diskChart?.dispose(); diskChart = null } catch {}
  try { netChart?.dispose(); netChart = null } catch {}
}

function pushData() {
  const cur = snap.current
  if (!cur) return

  const now = cur.timestamp ? new Date(cur.timestamp).getTime() : Date.now()
  if (cpuData.length > 0 && Math.abs(now - cpuData[cpuData.length - 1][0]) < 3000) return

  const cpuVal = typeof cur.cpu?.usage_percent === 'number' ? cur.cpu.usage_percent : 0
  const memVal = typeof cur.memory?.usage_percent === 'number' ? cur.memory.usage_percent : 0
  const diskVal = typeof cur.disk?.usage_percent === 'number' ? cur.disk.usage_percent : 0

  let diskReadRate = 0
  let diskWriteRate = 0
  if (typeof cur.disk_io?.read_rate_mb === 'number' && cur.disk_io.read_rate_mb > 0) {
    diskReadRate = cur.disk_io.read_rate_mb
    diskWriteRate = cur.disk_io.write_rate_mb ?? 0
  } else {
    diskReadRate = computeDiskRate(cur, 'read_mb')
    diskWriteRate = computeDiskRate(cur, 'write_mb')
  }

  let netRecvRate = 0
  let netSentRate = 0
  if (typeof cur.network?.recv_rate_mb === 'number' && cur.network.recv_rate_mb > 0) {
    netRecvRate = cur.network.recv_rate_mb
    netSentRate = cur.network.sent_rate_mb ?? 0
  } else {
    netRecvRate = computeNetRate(cur, 'recv')
    netSentRate = computeNetRate(cur, 'sent')
  }

  cpuData.push([now, cpuVal])
  memData.push([now, memVal])
  diskData.push([now, diskVal])
  diskReadData.push([now, diskReadRate])
  diskWriteData.push([now, diskWriteRate])
  netRecvData.push([now, netRecvRate])
  netSentData.push([now, netSentRate])

  while (cpuData.length > MAX_POINTS) { cpuData.shift(); memData.shift(); diskData.shift(); diskReadData.shift(); diskWriteData.shift(); netRecvData.shift(); netSentData.shift() }

  updateCharts()
}

let prevDiskRead = 0
let prevDiskWrite = 0
let prevDiskTs = 0
let prevNetRecv = 0
let prevNetSent = 0
let prevNetTs = 0

function computeDiskRate(cur: any, field: string): number {
  const val = cur.disk_io?.[field] ?? 0
  const ts = Date.now()
  if (prevDiskTs === 0) {
    prevDiskRead = cur.disk_io?.read_mb ?? 0
    prevDiskWrite = cur.disk_io?.write_mb ?? 0
    prevDiskTs = ts
    return 0
  }
  const dt = (ts - prevDiskTs) / 1000
  if (dt <= 0) return 0
  const prev = field === 'read_mb' ? prevDiskRead : prevDiskWrite
  const rate = Math.max(0, (val - prev) / dt)
  if (field === 'read_mb') { prevDiskRead = val; prevDiskWrite = cur.disk_io?.write_mb ?? prevDiskWrite }
  else { prevDiskWrite = val; prevDiskRead = cur.disk_io?.read_mb ?? prevDiskRead }
  prevDiskTs = ts
  return parseFloat(rate.toFixed(2))
}

function computeNetRate(cur: any, field: string): number {
  const bytesField = field === 'recv' ? 'bytes_recv' : 'bytes_sent'
  const val = cur.network?.[bytesField] ?? 0
  const ts = Date.now()
  if (prevNetTs === 0) {
    prevNetRecv = cur.network?.bytes_recv ?? 0
    prevNetSent = cur.network?.bytes_sent ?? 0
    prevNetTs = ts
    return 0
  }
  const dt = (ts - prevNetTs) / 1000
  if (dt <= 0) return 0
  const prev = field === 'recv' ? prevNetRecv : prevNetSent
  const rateMBps = Math.max(0, (val - prev) / 1024 / 1024 / dt)
  if (field === 'recv') { prevNetRecv = val }
  else { prevNetSent = val }
  if (field === 'sent') {
    prevNetRecv = cur.network?.bytes_recv ?? prevNetRecv
    prevNetTs = ts
  }
  return parseFloat(rateMBps.toFixed(3))
}

function updateCharts() {
  if (cpuChart) {
    try { cpuChart.setOption({ series: [{ data: cpuData.map(d => [...d]) }] }) } catch {}
  }
  if (memChart) {
    try { memChart.setOption({ series: [{ data: memData.map(d => [...d]) }] }) } catch {}
  }
  if (diskChart) {
    try {
      diskChart.setOption({
        series: [
          { data: diskData.map(d => [...d]) },
          { data: diskReadData.map(d => [...d]) },
          { data: diskWriteData.map(d => [...d]) },
        ],
      })
    } catch {}
  }
  if (netChart) {
    try {
      netChart.setOption({
        series: [
          { data: netRecvData.map(d => [...d]) },
          { data: netSentData.map(d => [...d]) },
        ],
      })
    } catch {}
  }
}

async function refresh() {
  try {
    await snap.fetchLatest()
    pushData()
  } catch (e: unknown) {
    const status = (e as any)?.response?.status
    if (status === 429) {
      console.warn('[dashboard] rate limited, will retry next cycle')
    } else {
      console.warn('[dashboard] refresh error:', (e as Error)?.message || e)
    }
  }
  try {
    await nodesStore.fetchNodes()
  } catch {}
}

function handleResize() {
  try { cpuChart?.resize(); memChart?.resize(); diskChart?.resize(); netChart?.resize() } catch {}
}

onMounted(async () => {
  window.addEventListener('resize', handleResize)
  await nodesStore.fetchNodes()
  await initCharts()

  try {
    await snap.fetchHistory(200)
    const hist = snap.history
    if (hist && hist.length > 0) {
      for (let i = 0; i < hist.length; i++) {
        const h = hist[i]
        const t = new Date(h.timestamp).getTime()
        if (isNaN(t)) continue
        cpuData.push([t, typeof h.cpu?.usage_percent === 'number' ? h.cpu.usage_percent : 0])
        memData.push([t, typeof h.memory?.usage_percent === 'number' ? h.memory.usage_percent : 0])
        diskData.push([t, typeof h.disk?.usage_percent === 'number' ? h.disk.usage_percent : 0])

        if (typeof h.disk_io?.read_rate_mb === 'number' && h.disk_io.read_rate_mb > 0) {
          diskReadData.push([t, h.disk_io.read_rate_mb])
          diskWriteData.push([t, h.disk_io.write_rate_mb ?? 0])
        } else if (i > 0) {
          const prev = hist[i - 1]
          const prevT = new Date(prev.timestamp).getTime()
          const dt = (t - prevT) / 1000
          if (dt > 0) {
            const readRate = Math.max(0, ((h.disk_io?.read_mb ?? 0) - (prev.disk_io?.read_mb ?? 0)) / dt)
            const writeRate = Math.max(0, ((h.disk_io?.write_mb ?? 0) - (prev.disk_io?.write_mb ?? 0)) / dt)
            diskReadData.push([t, parseFloat(readRate.toFixed(2))])
            diskWriteData.push([t, parseFloat(writeRate.toFixed(2))])
          } else {
            diskReadData.push([t, 0])
            diskWriteData.push([t, 0])
          }
        } else {
          diskReadData.push([t, 0])
          diskWriteData.push([t, 0])
        }

        if (typeof h.network?.recv_rate_mb === 'number' && h.network.recv_rate_mb > 0) {
          netRecvData.push([t, h.network.recv_rate_mb])
          netSentData.push([t, h.network.sent_rate_mb ?? 0])
        } else if (i > 0) {
          const prev = hist[i - 1]
          const prevT = new Date(prev.timestamp).getTime()
          const dtSec = (t - prevT) / 1000
          if (dtSec > 0) {
            const curRecv = (h.network?.bytes_recv ?? 0) / 1024 / 1024
            const curSent = (h.network?.bytes_sent ?? 0) / 1024 / 1024
            const prevRecv = (prev.network?.bytes_recv ?? 0) / 1024 / 1024
            const prevSent = (prev.network?.bytes_sent ?? 0) / 1024 / 1024
            netRecvData.push([t, parseFloat(Math.max(0, (curRecv - prevRecv) / dtSec).toFixed(3))])
            netSentData.push([t, parseFloat(Math.max(0, (curSent - prevSent) / dtSec).toFixed(3))])
          } else {
            netRecvData.push([t, 0])
            netSentData.push([t, 0])
          }
        } else {
          netRecvData.push([t, 0])
          netSentData.push([t, 0])
        }
      }
      while (cpuData.length > MAX_POINTS) { cpuData.shift(); memData.shift(); diskData.shift(); diskReadData.shift(); diskWriteData.shift(); netRecvData.shift(); netSentData.shift() }
      updateCharts()
      prevDiskTs = 0
      prevNetTs = 0
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

.monitor-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 16px; margin-bottom: 20px; }
.monitor-panel { background: #161b22; border: 1px solid #30363d; border-radius: 8px; padding: 16px; overflow: hidden; }
.panel-header { display: flex; align-items: center; gap: 8px; margin-bottom: 2px; }
.panel-dot { width: 8px; height: 8px; border-radius: 50%; flex-shrink: 0; }
.panel-title { font-size: 13px; color: #8b949e; font-weight: 500; }
.panel-value { font-size: 18px; font-weight: 700; margin-left: auto; }
.panel-sub { font-size: 11px; color: #6e7681; margin-bottom: 8px; }
.panel-chart { height: 200px; width: 100%; }

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

@media (max-width: 768px) {
  .monitor-grid { grid-template-columns: 1fr; }
  .summary-row { grid-template-columns: repeat(auto-fit, minmax(140px, 1fr)); }
}
</style>
