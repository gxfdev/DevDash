<template>
  <app-layout>
    <div class="page">
      <div class="page-header">
        <h2>趋势分析</h2>
        <n-space>
          <n-select v-model:value="selectedNode" :options="nodeOptions" style="width:180px" />
          <n-select v-model:value="duration" :options="durationOptions" style="width:120px" />
          <n-button size="small" :loading="loading" @click="load">加载</n-button>
        </n-space>
      </div>

      <div class="charts-row">
        <div class="chart-box">
          <div class="chart-title">CPU 趋势</div>
          <div ref="cpuRef" style="height:200px" />
        </div>
        <div class="chart-box">
          <div class="chart-title">内存趋势</div>
          <div ref="memRef" style="height:200px" />
        </div>
        <div class="chart-box">
          <div class="chart-title">磁盘趋势</div>
          <div ref="diskRef" style="height:200px" />
        </div>
        <div class="chart-box">
          <div class="chart-title">网络趋势</div>
          <div ref="netRef" style="height:200px" />
        </div>
      </div>

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
          <div class="stat-val" :style="{color: avgDisk > 90 ? '#f85149' : '#d29922'}">{{ avgDisk }}%</div>
          <div class="stat-sub">峰值 {{ maxDisk }}% · 低谷 {{ minDisk }}%</div>
        </div>
        <div class="stat-box">
          <div class="stat-label">网络总量</div>
          <div class="stat-val" style="color:#58a6ff">{{ totalNet }} GB</div>
          <div class="stat-sub">入 {{ totalIn }} · 出 {{ totalOut }}</div>
        </div>
      </div>

      <div class="report-box">
        <div class="report-title">周报摘要</div>
        <div class="report-content">
          <p>节点：<strong>{{ selectedNodeName }}</strong> · 时间范围：{{ durationLabel }}</p>
          <p>CPU 平均负载 <strong>{{ avgCpu }}%</strong>，{{ avgCpu > 80 ? '⚠️ 建议关注' : '✅ 运行正常' }}</p>
          <p>内存平均使用 <strong>{{ avgMem }}%</strong>，{{ avgMem > 80 ? '⚠️ 建议关注' : '✅ 运行正常' }}</p>
          <p>磁盘平均使用 <strong>{{ avgDisk }}%</strong>，{{ avgDisk > 90 ? '⚠️ 磁盘即将满' : '✅ 空间充足' }}</p>
          <p>网络总计流量 <strong>{{ totalNet }} GB</strong></p>
        </div>
        <n-button size="small" style="margin-top:8px" @click="exportReport">导出报告</n-button>
      </div>
    </div>
  </app-layout>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, watch, onUnmounted, nextTick } from 'vue'
import * as echarts from 'echarts'
import AppLayout from '@/components/AppLayout.vue'
import { useNodesStore } from '@/stores/nodes'
import client from '@/api/client'

const nodesStore = useNodesStore()
const selectedNode = ref<string | null>(null)
const duration = ref('7d')
const historyData = ref<any[]>([])
const loading = ref(false)

const cpuRef = ref<HTMLDivElement>()
const memRef = ref<HTMLDivElement>()
const diskRef = ref<HTMLDivElement>()
const netRef = ref<HTMLDivElement>()
let cpuChart: echarts.ECharts | null = null
let memChart: echarts.ECharts | null = null
let diskChart: echarts.ECharts | null = null
let netChart: echarts.ECharts | null = null

const durationOptions = [
  { label: '1小时', value: '1h' },
  { label: '6小时', value: '6h' },
  { label: '24小时', value: '1d' },
  { label: '7天', value: '7d' },
  { label: '30天', value: '30d' },
]
const durationLabel = computed(() => durationOptions.find(d => d.value === duration.value)?.label || '')

const nodeOptions = computed(() => nodesStore.nodes.map((n: any) => ({ label: n.name || n.hostname || n.ip, value: n.id })))
const selectedNodeName = computed(() => nodesStore.nodes.find((n: any) => n.id === selectedNode.value)?.name || '--')

function toTs(val: any): number {
  if (!val) return Date.now()
  if (typeof val === 'number') return val > 9999999999 ? val : val * 1000
  return new Date(val).getTime()
}

function vals(key: string) {
  const fieldMap: Record<string, (h: any) => number> = {
    cpu: (h: any) => h.cpu?.usage_percent || 0,
    mem_pct: (h: any) => h.memory?.usage_percent || 0,
    memory_usage_percent: (h: any) => h.memory?.usage_percent || 0,
    disk_pct: (h: any) => h.disk?.usage_percent || 0,
    disk_usage_percent: (h: any) => h.disk?.usage_percent || 0,
    net_in: (h: any) => h.network?.bytes_recv || 0,
    bytes_recv: (h: any) => h.network?.bytes_recv || 0,
    net_out: (h: any) => h.network?.bytes_sent || 0,
    bytes_sent: (h: any) => h.network?.bytes_sent || 0,
  }
  const getter = fieldMap[key] || ((h: any) => h[key] || 0)
  return historyData.value.map(getter)
}
function avg(arr: number[]) { return arr.length ? Math.round(arr.reduce((s, v) => s + v, 0) / arr.length) : 0 }
function max(arr: number[]) { return arr.length ? Math.round(Math.max(...arr)) : 0 }
function min(arr: number[]) { return arr.length ? Math.round(Math.min(...arr)) : 0 }

const avgCpu = computed(() => avg(vals('cpu')))
const maxCpu = computed(() => max(vals('cpu')))
const minCpu = computed(() => min(vals('cpu')))
const avgMem = computed(() => avg(vals('mem_pct') || vals('memory_usage_percent')))
const maxMem = computed(() => max(vals('mem_pct') || vals('memory_usage_percent')))
const minMem = computed(() => min(vals('mem_pct') || vals('memory_usage_percent')))
const avgDisk = computed(() => avg(vals('disk_pct') || vals('disk_usage_percent')))
const maxDisk = computed(() => max(vals('disk_pct') || vals('disk_usage_percent')))
const minDisk = computed(() => min(vals('disk_pct') || vals('disk_usage_percent')))

const netInVals = computed(() => vals('net_in') || vals('bytes_recv') || [])
const netOutVals = computed(() => vals('net_out') || vals('bytes_sent') || [])

const totalNet = computed(() => ((sumArr(netInVals.value) + sumArr(netOutVals.value)) / 1024 / 1024 / 1024).toFixed(2))
const totalIn = computed(() => (sumArr(netInVals.value) / 1024 / 1024 / 1024).toFixed(2) + ' GB')
const totalOut = computed(() => (sumArr(netOutVals.value) / 1024 / 1024 / 1024).toFixed(2) + ' GB')

function sumArr(arr: number[]): number { return arr.reduce((s, v) => s + (v || 0), 0) }

function makeChartOpt(color: string) {
  return {
    grid: { top: 8, right: 20, bottom: 30, left: 50 },
    xAxis: { type: 'time', axisLine: { lineStyle: { color: '#30363d' } }, axisLabel: { color: '#6e7681', fontSize: 11 } },
    yAxis: { axisLabel: { color: '#6e7681', fontSize: 11 }, splitLine: { lineStyle: { color: '#21262d' } } },
    series: [{ type: 'line', smooth: true, data: [], lineStyle: { color }, itemStyle: { color }, symbol: 'none', areaStyle: { color: color + '22' } }],
    backgroundColor: 'transparent',
  }
}

function buildCharts() {
  disposeCharts()
  nextTick(() => {
    if (cpuRef.value) { cpuChart = echarts.init(cpuRef.value, 'dark'); cpuChart.setOption(makeChartOpt('#3fb950')) }
    if (memRef.value) { memChart = echarts.init(memRef.value, 'dark'); memChart.setOption(makeChartOpt('#bc8cff')) }
    if (diskRef.value) { diskChart = echarts.init(diskRef.value, 'dark'); diskChart.setOption(makeChartOpt('#d29922')) }
    if (netRef.value) { netChart = echarts.init(netRef.value, 'dark'); netChart.setOption(makeChartOpt('#58a6ff')) }
  })
}

function disposeCharts() {
  try { cpuChart?.dispose(); cpuChart = null } catch {}
  try { memChart?.dispose(); memChart = null } catch {}
  try { diskChart?.dispose(); diskChart = null } catch {}
  try { netChart?.dispose(); netChart = null } catch {}
}

function pushData() {
  cpuChart?.setOption({ series: [{ data: historyData.value.map((h: any) => [toTs(h.timestamp), h.cpu?.usage_percent || 0]) }] })
  memChart?.setOption({ series: [{ data: historyData.value.map((h: any) => [toTs(h.timestamp), h.memory?.usage_percent || 0]) }] })
  diskChart?.setOption({ series: [{ data: historyData.value.map((h: any) => [toTs(h.timestamp), h.disk?.usage_percent || 0]) }] })

  const netIn = historyData.value.map((h: any) => h.network?.bytes_recv || 0)
  const netOut = historyData.value.map((h: any) => h.network?.bytes_sent || 0)
  const netSeries = historyData.value.map((h: any, i: number) => [
    toTs(h.timestamp),
    ((netIn[i] || 0) + (netOut[i] || 0)) / 1048576,
  ])
  netChart?.setOption({ series: [{ data: netSeries }] })
}

async function load() {
  if (!selectedNode.value) return
  loading.value = true
  try {
    const { data } = await client.get(`/node/${selectedNode.value}/history`, { params: { duration: duration.value } })
    historyData.value = Array.isArray(data) ? data : []
    if (historyData.value.length === 0) {
      console.warn('[trends] no history data for', selectedNode.value)
    }
    buildCharts()
    await nextTick()
    pushData()
  } catch (e: any) {
    console.error('[trends] load error:', e)
    historyData.value = []
  } finally {
    loading.value = false
  }
}

function exportReport() {
  const text = [
    `DevDash 周报 - ${selectedNodeName.value}`,
    `时间范围: ${durationLabel.value}`,
    `CPU: 均值${avgCpu.value}% 峰值${maxCpu.value}% 低谷${minCpu.value}%`,
    `内存: 均值${avgMem.value}% 峰值${maxMem.value}% 低谷${minMem.value}%`,
    `磁盘: 均值${avgDisk.value}% 峰值${maxDisk.value}% 低谷${minDisk.value}%`,
    `网络: 总计${totalNet.value}GB (入${totalIn.value} 出${totalOut.value})`,
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
  try { cpuChart?.resize(); memChart?.resize(); diskChart?.resize(); netChart?.resize() } catch {}
}

onMounted(async () => {
  window.addEventListener('resize', handleResize)
  await nodesStore.fetchNodes()
  if (nodesStore.nodes.length) {
    selectedNode.value = nodesStore.nodes[0]?.id
    buildCharts()
    load()
  }
})

watch(selectedNode, () => { load() })
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
.stats-row { display: grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); gap: 12px; margin-bottom: 16px; }
.stat-box { background: #161b22; border: 1px solid #30363d; border-radius: 8px; padding: 16px; }
.stat-label { font-size: 12px; color: #8b949e; margin-bottom: 6px; }
.stat-val { font-size: 24px; font-weight: 700; }
.stat-sub { font-size: 11px; color: #6e7681; margin-top: 4px; }
.report-box { background: #161b22; border: 1px solid #30363d; border-radius: 8px; padding: 20px; }
.report-title { font-size: 14px; font-weight: 600; margin-bottom: 12px; }
.report-content p { margin: 0 0 6px; font-size: 13px; color: #8b949e; line-height: 1.8; }
.report-content strong { color: #e6edf3; }
</style>