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

      <n-tabs type="line" animated>
        <n-tab-pane name="system" tab="系统指标">
          <div v-if="historyData.length === 0 && !loading" class="empty-state">
            <div class="empty-icon">📊</div>
            <div>暂无历史数据，请等待数据采集或调整时间范围</div>
          </div>
          <div v-else class="charts-row">
            <div class="chart-box">
              <div class="chart-title">CPU 趋势</div>
              <div ref="cpuRef" style="height:220px" />
            </div>
            <div class="chart-box">
              <div class="chart-title">内存趋势</div>
              <div ref="memRef" style="height:220px" />
            </div>
            <div class="chart-box">
              <div class="chart-title">磁盘趋势</div>
              <div ref="diskRef" style="height:220px" />
            </div>
            <div class="chart-box">
              <div class="chart-title">网络趋势 (MB/s)</div>
              <div ref="netRef" style="height:220px" />
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
        </n-tab-pane>

        <n-tab-pane name="files" tab="文件统计">
          <div v-if="fileStats.length === 0 && !loading" class="empty-state">
            <div class="empty-icon">📁</div>
            <div>暂无文件操作数据，请先使用文件管理功能进行操作</div>
          </div>
          <div v-else class="charts-row">
            <div class="chart-box">
              <div class="chart-title">文件操作趋势</div>
              <div ref="fileOpRef" style="height:220px" />
            </div>
            <div class="chart-box">
              <div class="chart-title">文件类型分布</div>
              <div ref="fileTypeRef" style="height:220px" />
            </div>
            <div class="chart-box">
              <div class="chart-title">存储空间使用趋势</div>
              <div ref="storageRef" style="height:220px" />
            </div>
            <div class="chart-box">
              <div class="chart-title">目录文件数统计</div>
              <div ref="dirCountRef" style="height:220px" />
            </div>
          </div>

          <div class="stats-row">
            <div class="stat-box">
              <div class="stat-label">文件总数</div>
              <div class="stat-val" style="color:#58a6ff">{{ totalFiles }}</div>
              <div class="stat-sub">目录 {{ totalDirs }} · 文件 {{ totalRegular }}</div>
            </div>
            <div class="stat-box">
              <div class="stat-label">总大小</div>
              <div class="stat-val" style="color:#3fb950">{{ totalSize }}</div>
            </div>
            <div class="stat-box">
              <div class="stat-label">今日操作</div>
              <div class="stat-val" style="color:#d29922">{{ todayOps }}</div>
              <div class="stat-sub">创建 {{ todayCreated }} · 删除 {{ todayDeleted }}</div>
            </div>
            <div class="stat-box">
              <div class="stat-label">最活跃目录</div>
              <div class="stat-val" style="color:#bc8cff">{{ topDir }}</div>
            </div>
          </div>
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
          <p v-if="fileStats.length > 0">文件操作统计：总计 <strong>{{ totalFiles }}</strong> 个文件，大小 <strong>{{ totalSize }}</strong></p>
        </div>
        <n-button size="small" style="margin-top:8px" @click="exportReport">导出报告</n-button>
      </div>
    </div>
  </app-layout>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, watch, onUnmounted, nextTick } from 'vue'
import * as echarts from 'echarts/core'
import { LineChart, BarChart, PieChart } from 'echarts/charts'
import { GridComponent, TooltipComponent, LegendComponent } from 'echarts/components'
import { CanvasRenderer } from 'echarts/renderers'
echarts.use([LineChart, BarChart, PieChart, GridComponent, TooltipComponent, LegendComponent, CanvasRenderer])
import AppLayout from '@/components/AppLayout.vue'
import { useNodesStore } from '@/stores/nodes'
import client from '@/api/client'
import type { Snapshot } from '@/types'

const nodesStore = useNodesStore()
const selectedNode = ref<string | null>(null)
const duration = ref('7d')
const historyData = ref<Snapshot[]>([])
const fileStats = ref<any[]>([])
const loading = ref(false)

const cpuRef = ref<HTMLDivElement>()
const memRef = ref<HTMLDivElement>()
const diskRef = ref<HTMLDivElement>()
const netRef = ref<HTMLDivElement>()
const fileOpRef = ref<HTMLDivElement>()
const fileTypeRef = ref<HTMLDivElement>()
const storageRef = ref<HTMLDivElement>()
const dirCountRef = ref<HTMLDivElement>()

let cpuChart: echarts.ECharts | null = null
let memChart: echarts.ECharts | null = null
let diskChart: echarts.ECharts | null = null
let netChart: echarts.ECharts | null = null
let fileOpChart: echarts.ECharts | null = null
let fileTypeChart: echarts.ECharts | null = null
let storageChart: echarts.ECharts | null = null
let dirCountChart: echarts.ECharts | null = null

const durationOptions = [
  { label: '1小时', value: '1h' },
  { label: '6小时', value: '6h' },
  { label: '24小时', value: '1d' },
  { label: '7天', value: '7d' },
  { label: '30天', value: '30d' },
]
const durationLabel = computed(() => durationOptions.find(d => d.value === duration.value)?.label || '')

const nodeOptions = computed(() => nodesStore.nodes.map((n: { name: string; hostname?: string; ip: string; id: string }) => ({ label: n.name || n.hostname || n.ip, value: n.id })))
const selectedNodeName = computed(() => nodesStore.nodes.find((n: { name: string; hostname?: string; ip: string; id: string }) => n.id === selectedNode.value)?.name || '--')

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

const totalFiles = computed(() => fileStats.value.reduce((s, f) => s + (f.total || 0), 0))
const totalDirs = computed(() => fileStats.value.reduce((s, f) => s + (f.dirs || 0), 0))
const totalRegular = computed(() => totalFiles.value - totalDirs.value)
const totalSize = computed(() => {
  const bytes = fileStats.value.reduce((s, f) => s + (f.size || 0), 0)
  if (bytes < 1024) return bytes + ' B'
  if (bytes < 1048576) return (bytes / 1024).toFixed(1) + ' KB'
  if (bytes < 1073741824) return (bytes / 1048576).toFixed(1) + ' MB'
  return (bytes / 1073741824).toFixed(2) + ' GB'
})
const todayOps = computed(() => fileStats.value.reduce((s, f) => s + (f.today_ops || 0), 0))
const todayCreated = computed(() => fileStats.value.reduce((s, f) => s + (f.today_created || 0), 0))
const todayDeleted = computed(() => fileStats.value.reduce((s, f) => s + (f.today_deleted || 0), 0))
const topDir = computed(() => {
  if (fileStats.value.length === 0) return '--'
  const sorted = [...fileStats.value].sort((a, b) => (b.total || 0) - (a.total || 0))
  return (sorted[0]?.path || '--').split(/[/\\]/).pop() || '--'
})

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

function makeChartOpt(color: string) {
  return {
    grid: { top: 20, right: 20, bottom: 30, left: 50 },
    xAxis: { type: 'time', boundaryGap: false, axisLine: { lineStyle: { color: '#30363d' } }, axisLabel: { color: '#6e7681', fontSize: 11, formatter: '{HH}:{mm}' }, splitLine: { show: true, lineStyle: { color: '#21262d', type: 'dashed' } } },
    yAxis: { axisLabel: { color: '#6e7681', fontSize: 11 }, splitLine: { lineStyle: { color: '#21262d', type: 'dashed' } } },
    tooltip: {
      trigger: 'axis',
      backgroundColor: '#1f242c',
      borderColor: '#30363d',
      borderWidth: 1,
      textStyle: { color: '#e6edf3', fontSize: 12 },
      axisPointer: { type: 'cross', lineStyle: { color: '#484f58' } }
    },
    series: [{
      type: 'line',
      smooth: 0.4,
      sampling: 'lttb',
      data: [],
      lineStyle: { width: 2.5, color, shadowBlur: 10, shadowColor: hexToRgba(color, 0.3) },
      itemStyle: { color },
      symbol: 'none',
      areaStyle: { color: createGradientColor(color) },
      emphasis: { focus: 'series', lineStyle: { width: 3 } }
    }],
    animation: true,
    animationDuration: 500,
    animationEasing: 'cubicOut',
    backgroundColor: 'transparent',
  }
}

function makeBarChartOpt(color: string) {
  return {
    grid: { top: 20, right: 20, bottom: 40, left: 50 },
    xAxis: { type: 'category', axisLine: { lineStyle: { color: '#30363d' } }, axisLabel: { color: '#6e7681', fontSize: 10, rotate: 30 } },
    yAxis: { axisLabel: { color: '#6e7681', fontSize: 11 }, splitLine: { lineStyle: { color: '#21262d', type: 'dashed' } } },
    tooltip: { trigger: 'axis', backgroundColor: '#1f242c', borderColor: '#30363d', textStyle: { color: '#e6edf3' } },
    series: [{ type: 'bar', data: [], itemStyle: { color: { type: 'linear', x: 0, y: 0, x2: 0, y2: 1, colorStops: [{offset: 0, color}, {offset: 1, color: hexToRgba(color, 0.2)}] }, borderRadius: [4, 4, 0, 0] } }],
    animation: true,
    animationDuration: 500,
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
    if (fileOpRef.value) { fileOpChart = echarts.init(fileOpRef.value, 'dark'); fileOpChart.setOption({
      grid: { top: 30, right: 20, bottom: 30, left: 50 },
      xAxis: { type: 'category', boundaryGap: false, axisLine: { lineStyle: { color: '#30363d' } }, axisLabel: { color: '#6e7681', fontSize: 10 }, splitLine: { show: true, lineStyle: { color: '#21262d', type: 'dashed' } } },
      yAxis: { axisLabel: { color: '#6e7681', fontSize: 11 }, splitLine: { lineStyle: { color: '#21262d', type: 'dashed' } } },
      tooltip: { trigger: 'axis', backgroundColor: '#1f242c', borderColor: '#30363d', textStyle: { color: '#e6edf3' } },
      legend: { data: ['创建', '删除'], textStyle: { color: '#8b949e' }, top: 5, right: 10 },
      series: [
        { type: 'bar', data: [], name: '创建', itemStyle: { color: '#3fb950', borderRadius: [4, 4, 0, 0] } },
        { type: 'bar', data: [], name: '删除', itemStyle: { color: '#f85149', borderRadius: [4, 4, 0, 0] } },
      ],
      animation: true,
      animationDuration: 500,
      backgroundColor: 'transparent',
    }) }
    if (fileTypeRef.value) { fileTypeChart = echarts.init(fileTypeRef.value, 'dark'); fileTypeChart.setOption({
      tooltip: { trigger: 'item', backgroundColor: '#1f242c', borderColor: '#30363d', textStyle: { color: '#e6edf3' } },
      legend: { orient: 'vertical', right: 10, top: 'center', textStyle: { color: '#8b949e' } },
      backgroundColor: 'transparent',
    }) }
    if (storageRef.value) { storageChart = echarts.init(storageRef.value, 'dark'); storageChart.setOption(makeBarChartOpt('#d29922')) }
    if (dirCountRef.value) { dirCountChart = echarts.init(dirCountRef.value, 'dark'); dirCountChart.setOption(makeBarChartOpt('#bc8cff')) }
  })
}

function disposeCharts() {
  [cpuChart, memChart, diskChart, netChart, fileOpChart, fileTypeChart, storageChart, dirCountChart].forEach(c => {
    try { c?.dispose() } catch {}
  })
  cpuChart = memChart = diskChart = netChart = fileOpChart = fileTypeChart = storageChart = dirCountChart = null
}

function pushData() {
  const cpuData = historyData.value.map((h: any) => [toTs(h.timestamp), getVal(h, ['cpu.usage_percent', 'cpu_usage'])])
  const memData = historyData.value.map((h: any) => [toTs(h.timestamp), getVal(h, ['memory.usage_percent', 'mem_usage_percent'])])
  const diskData = historyData.value.map((h: any) => [toTs(h.timestamp), getVal(h, ['disk.usage_percent', 'disk_usage_percent'])])

  const netSeries = historyData.value.map((h: any, i: number) => {
    const recv = getVal(h, ['network.bytes_recv', 'bytes_recv'])
    const sent = getVal(h, ['network.bytes_sent', 'bytes_sent'])
    const prevRecv = i > 0 ? getVal(historyData.value[i - 1], ['network.bytes_recv', 'bytes_recv']) : recv
    const prevSent = i > 0 ? getVal(historyData.value[i - 1], ['network.bytes_sent', 'bytes_sent']) : sent
    const rate = ((recv + sent - prevRecv - prevSent) / 1048576)
    return [toTs(h.timestamp), Math.max(0, rate)]
  })

  cpuChart?.setOption({ series: [{ data: cpuData }] })
  memChart?.setOption({ series: [{ data: memData }] })
  diskChart?.setOption({ series: [{ data: diskData }] })
  netChart?.setOption({ series: [{ data: netSeries }] })

  pushFileStats()
}

function pushFileStats() {
  if (fileStats.value.length === 0) return

  const sorted = [...fileStats.value].sort((a, b) => (b.total || 0) - (a.total || 0)).slice(0, 10)

  dirCountChart?.setOption({
    xAxis: { data: sorted.map((f: any) => (f.path || '').split(/[/\\]/).pop() || f.path || '?') },
    series: [{ data: sorted.map((f: any) => f.total || 0) }],
  })

  const typeCount: Record<string, number> = {}
  fileStats.value.forEach((f: any) => {
    const types = f.types || {}
    Object.entries(types).forEach(([ext, count]) => {
      typeCount[ext] = (typeCount[ext] || 0) + (count as number)
    })
  })
  const pieData = Object.entries(typeCount)
    .sort((a, b) => b[1] - a[1])
    .slice(0, 8)
    .map(([name, value]) => ({ name, value }))

  fileTypeChart?.setOption({
    series: [{
      type: 'pie',
      radius: ['30%', '65%'],
      center: ['40%', '50%'],
      data: pieData,
      label: { color: '#8b949e', fontSize: 11 },
      emphasis: { itemStyle: { shadowBlur: 10, shadowColor: 'rgba(0,0,0,0.5)' } },
    }],
  })

  const storageData = sorted.map((f: any) => ({
    name: (f.path || '').split(/[/\\]/).pop() || f.path || '?',
    value: f.size || 0,
  }))
  storageChart?.setOption({
    series: [{ data: storageData.map((d, i) => [i, d.value ? +(d.value / 1048576).toFixed(1) : 0]) }],
    xAxis: { type: 'category', data: storageData.map(d => d.name) },
  })

  const opData = fileStats.value.slice(0, 10).map((f: any) => ({
    name: (f.path || '').split(/[/\\]/).pop() || f.path || '?',
    created: f.today_created || 0,
    deleted: f.today_deleted || 0,
  }))
  fileOpChart?.setOption({
    xAxis: { type: 'category', data: opData.map(d => d.name) },
    series: [
      { data: opData.map(d => d.created), name: '创建', type: 'bar', itemStyle: { color: '#3fb950' } },
      { data: opData.map(d => d.deleted), name: '删除', type: 'bar', itemStyle: { color: '#f85149' } },
    ],
  } as any)
}

async function loadHistory() {
  if (!selectedNode.value) return
  loading.value = true
  try {
    const { data } = await client.get(`/node/${selectedNode.value}/history`, { params: { duration: duration.value, limit: 500 } })
    historyData.value = Array.isArray(data) ? data : []
    if (historyData.value.length === 0) {
      console.warn('[trends] no history data for', selectedNode.value)
    }
  } catch (e: unknown) {
    console.warn('[trends] load error:', (e as Error)?.message || e)
    historyData.value = []
  }
}

async function loadFileStats() {
  if (!selectedNode.value) return
  try {
    const { data } = await client.get(`/node/${selectedNode.value}/fs/stats`, { params: { duration: duration.value } })
    fileStats.value = Array.isArray(data) ? data : []
  } catch {
    fileStats.value = []
  }
}

async function load() {
  if (!selectedNode.value) return
  loading.value = true
  try {
    await Promise.all([loadHistory(), loadFileStats()])
    buildCharts()
    await nextTick()
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
    `--- 文件统计 ---`,
    `文件总数: ${totalFiles.value} (目录${totalDirs.value} 文件${totalRegular.value})`,
    `总大小:   ${totalSize.value}`,
    `今日操作: ${todayOps.value} (创建${todayCreated.value} 删除${todayDeleted.value})`,
    `最活跃目录: ${topDir.value}`,
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
  [cpuChart, memChart, diskChart, netChart, fileOpChart, fileTypeChart, storageChart, dirCountChart].forEach(c => {
    try { c?.resize() } catch {}
  })
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
</style>