<template>
  <app-layout>
    <div class="page">
      <div class="page-header">
        <h2>{{ nodeName }}</h2>
        <n-space>
          <n-tag :type="statusType">{{ statusText }}</n-tag>
          <n-button size="small" @click="load">刷新</n-button>
          <n-button size="small" type="error" @click="router.push('/hosts')">返回</n-button>
        </n-space>
      </div>

      <!-- 实时指标卡片 -->
      <div class="summary-row">
        <metric-card label="CPU" :value="(snap.cpu || 0) + '%'" :sub="`核心数 ${info.cpu_cores || '--'}`" color="#3fb950" />
        <metric-card label="内存" :value="(snap.mem_pct || 0) + '%'" :sub="`${memUsed} / ${memTotal} GB`" color="#bc8cff" />
        <metric-card label="磁盘" :value="(snap.disk_pct || 0) + '%'" :sub="`${diskUsed} / ${diskTotal} GB`" color="#d29922" />
        <metric-card label="网络" :value="netRate" :sub="`↓ ${netIn}  ↑ ${netOut}`" color="#58a6ff" />
        <metric-card v-if="!isWindowsNode" label="负载" :value="snap.load_1 || '--'" :sub="`1m / ${snap.load_5 || '--'} / ${snap.load_15 || '--'}`" color="#f85149" />
        <metric-card v-else label="负载" value="--" sub="Windows 不支持此指标" color="#6e7681" />
        <metric-card label="进程数" :value="snap.procs || '--'" sub="运行中" color="#f0883e" />
      </div>

      <!-- 图表区 -->
      <div class="charts-row">
        <div class="chart-box" style="grid-column: span 2">
          <div class="chart-title">CPU & 内存 实时趋势</div>
          <div ref="chartRef" style="height:200px"></div>
        </div>
        <div class="chart-box" style="grid-column: span 2">
          <div class="chart-title">网络 实时趋势</div>
          <div ref="chart2Ref" style="height:200px"></div>
        </div>
      </div>

      <!-- 进程 Top10 + 容器 -->
      <div class="detail-row">
        <div class="detail-box">
          <div class="detail-title">进程 Top10 (CPU)</div>
          <n-data-table :columns="procColumns" :data="topProcs" size="small" :bordered="false" :loading="procLoading" />
        </div>
        <div class="detail-box">
          <div class="detail-title">容器列表</div>
          <n-data-table :columns="containerColumns" :data="containers" size="small" :bordered="false" :loading="containerLoading" />
        </div>
      </div>
    </div>
  </app-layout>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, h } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import * as echarts from 'echarts'
import { NTag, NProgress } from 'naive-ui'
import AppLayout from '@/components/AppLayout.vue'
import MetricCard from '@/components/MetricCard.vue'
import { useMetricsStore } from '@/stores/metrics'
import client from '@/api/client'

const route = useRoute()
const router = useRouter()
const metricsStore = useMetricsStore()

const nodeId = route.params.id as string
const snap = ref<any>({})
const info = ref<any>({})
const topProcs = ref<Record<string, unknown>[]>([])
const containers = ref<Record<string, unknown>[]>([])
const procLoading = ref(false)
const containerLoading = ref(false)

const chartRef = ref<HTMLDivElement>()
const chart2Ref = ref<HTMLDivElement>()
let chart: echarts.ECharts
let chart2: echarts.ECharts
let timer: ReturnType<typeof setInterval>

const nodeName = computed(() => info.value?.name || info.value?.hostname || '主机详情')
const isWindowsNode = computed(() => {
  const os = info.value?.os || ''
  return os.toLowerCase().includes('windows')
})
const statusText = computed(() => snap.value?.status === 'online' ? '在线' : '离线')
const statusType = computed(() => snap.value?.status === 'online' ? 'success' : 'error')
const memUsed = computed(() => ((snap.value?.mem_used || 0) / 1024).toFixed(1))
const memTotal = computed(() => ((snap.value?.mem_total || 0) / 1024).toFixed(1))
const diskUsed = computed(() => ((snap.value?.disk_used || 0) / 1024).toFixed(1))
const diskTotal = computed(() => ((snap.value?.disk_total || 0) / 1024).toFixed(1))
const netIn = computed(() => ((snap.value?.net_in || 0) / 1024 / 1024).toFixed(2))
const netOut = computed(() => ((snap.value?.net_out || 0) / 1024 / 1024).toFixed(2))
const netRate = computed(() => {
  const total = ((snap.value?.net_in || 0) + (snap.value?.net_out || 0)) / 1024 / 1024
  return total.toFixed(2) + ' MB/s'
})

const procColumns = [
  { title: 'PID', key: 'pid', width: 80 },
  { title: '名称', key: 'name' },
  { title: 'CPU%', key: 'cpu_pct', render: (r: any) => r.cpu_pct?.toFixed(1) + '%' },
  { title: '内存%', key: 'mem_pct', render: (r: any) => r.mem_pct?.toFixed(1) + '%' },
]
const containerColumns = [
  { title: '名称', key: 'name' },
  { title: '镜像', key: 'image', ellipsis: true },
  { title: '状态', key: 'status', render: (r: any) => h(NTag, { type: r.status === 'running' ? 'success' : 'default', size: 'small' }, () => r.status) },
  { title: 'CPU%', key: 'cpu_pct', render: (r: any) => r.cpu_pct?.toFixed(1) + '%' || '--' },
  { title: '内存', key: 'mem_usage', render: (r: any) => r.mem_usage || '--' },
]

function initCharts() {
  if (chartRef.value) {
    chart = echarts.init(chartRef.value, 'dark')
    chart.setOption({
      grid: { top: 10, right: 20, bottom: 30, left: 50 },
      legend: { textStyle: { color: '#8b949e' } },
      xAxis: { type: 'time', axisLine: { lineStyle: { color: '#30363d' } }, axisLabel: { color: '#6e7681' } },
      yAxis: { type: 'value', max: 100, axisLabel: { color: '#6e7681', formatter: '{value}%' }, splitLine: { lineStyle: { color: '#21262d' } } },
      series: [
        { name: 'CPU', type: 'line', smooth: true, data: [], lineStyle: { color: '#3fb950' }, itemStyle: { color: '#3fb950' } },
        { name: '内存', type: 'line', smooth: true, data: [], lineStyle: { color: '#bc8cff' }, itemStyle: { color: '#bc8cff' } },
      ],
      backgroundColor: 'transparent',
    })
  }
  if (chart2Ref.value) {
    chart2 = echarts.init(chart2Ref.value, 'dark')
    chart2.setOption({
      grid: { top: 10, right: 20, bottom: 30, left: 50 },
      legend: { textStyle: { color: '#8b949e' } },
      xAxis: { type: 'time', axisLine: { lineStyle: { color: '#30363d' } }, axisLabel: { color: '#6e7681' } },
      yAxis: { axisLabel: { color: '#6e7681', formatter: (v: number) => (v / 1024 / 1024).toFixed(1) + 'MB/s' }, splitLine: { lineStyle: { color: '#21262d' } } },
      series: [
        { name: '入', type: 'line', smooth: true, data: [], lineStyle: { color: '#58a6ff' }, itemStyle: { color: '#58a6ff' } },
        { name: '出', type: 'line', smooth: true, data: [], lineStyle: { color: '#f0883e' }, itemStyle: { color: '#f0883e' } },
      ],
      backgroundColor: 'transparent',
    })
  }
}

function pushData() {
  const now = new Date()
  const cpu = snap.value?.cpu || 0
  const mem = snap.value?.mem_pct || 0
  const netInVal = snap.value?.net_in || 0
  const netOutVal = snap.value?.net_out || 0

  if (chart) {
    const opt = chart.getOption() as any
    const d0 = [...(opt.series[0].data || [])]
    const d1 = [...(opt.series[1].data || [])]
    d0.push([now, cpu])
    d1.push([now, mem])
    if (d0.length > 60) { d0.shift(); d1.shift() }
    chart.setOption({ series: [{ data: d0 }, { data: d1 }] })
  }
  if (chart2) {
    const opt = chart2.getOption() as any
    const d0 = [...(opt.series[0].data || [])]
    const d1 = [...(opt.series[1].data || [])]
    d0.push([now, netInVal])
    d1.push([now, netOutVal])
    if (d0.length > 60) { d0.shift(); d1.shift() }
    chart2.setOption({ series: [{ data: d0 }, { data: d1 }] })
  }
}

async function load() {
  try {
    const { data: nodeData } = await client.get(`/node/${nodeId}`)
    info.value = nodeData
  } catch {}
  try {
    const { data: metricsData } = await client.get(`/node/${nodeId}/metrics`)
    const m = metricsData
    snap.value = {
      status: info.value?.status,
      cpu: m.cpu?.usage_percent || 0,
      cpu_cores: m.cpu?.cores || 0,
      mem_pct: m.memory?.usage_percent || 0,
      mem_used: m.memory?.used_gb || 0,
      mem_total: m.memory?.total_gb || 0,
      disk_pct: m.disk?.usage_percent || 0,
      disk_used: m.disk?.used_gb || 0,
      disk_total: m.disk?.total_gb || 0,
      net_in: m.network?.bytes_recv || 0,
      net_out: m.network?.bytes_sent || 0,
      load_1: m.load?.load1 || 0,
      load_5: m.load?.load5 || 0,
      load_15: m.load?.load15 || 0,
      procs: m.processes?.length || 0,
    }
  } catch {}
  try {
    procLoading.value = true
    const { data } = await client.get(`/node/${nodeId}/procs`)
    topProcs.value = (data || []).sort((a: { cpu_percent?: number }, b: { cpu_percent?: number }) => (b.cpu_percent || 0) - (a.cpu_percent || 0)).slice(0, 10)
  } catch {} finally { procLoading.value = false }
  try {
    containerLoading.value = true
    const { data } = await client.get(`/node/${nodeId}/containers`)
    containers.value = data || []
  } catch {} finally { containerLoading.value = false }
  pushData()
}

onMounted(() => {
  initCharts()
  load()
  timer = setInterval(load, 30000)
  metricsStore.connectWS(nodeId)
})

onUnmounted(() => {
  clearInterval(timer)
  metricsStore.disconnectWS()
  chart?.dispose()
  chart2?.dispose()
})
</script>

<style scoped>
.page { padding: 24px; }
.page-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 20px; }
h2 { font-size: 20px; font-weight: 600; margin: 0; }
.summary-row { display: grid; grid-template-columns: repeat(auto-fit, minmax(150px, 1fr)); gap: 12px; margin-bottom: 16px; }
.charts-row { display: grid; grid-template-columns: 1fr 1fr; gap: 16px; margin-bottom: 16px; }
.chart-box { background: #161b22; border: 1px solid #30363d; border-radius: 8px; padding: 16px; }
.chart-title { font-size: 13px; color: #8b949e; margin-bottom: 8px; }
.detail-row { display: grid; grid-template-columns: 1fr 1fr; gap: 16px; }
.detail-box { background: #161b22; border: 1px solid #30363d; border-radius: 8px; padding: 16px; }
.detail-title { font-size: 13px; color: #8b949e; margin-bottom: 12px; }
</style>