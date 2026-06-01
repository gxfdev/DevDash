<template>
  <div class="dashboard">
    <n-grid :cols="4" :x-gap="12" :y-gap="12">
      <n-gi>
        <n-card size="small">
          <n-statistic label="主机名" :value="status?.system?.hostname || '-'">
            <template #suffix>
              <n-text depth="3" style="font-size: 12px">{{ status?.system?.platform }}</n-text>
            </template>
          </n-statistic>
        </n-card>
      </n-gi>
      <n-gi>
        <n-card size="small">
          <n-statistic label="运行时间" :value="formatUptime(status?.system?.uptime || 0)" />
        </n-card>
      </n-gi>
      <n-gi>
        <n-card size="small">
          <n-statistic label="CPU 使用率">
            <template #default>
              <n-text :type="cpuPercent > 80 ? 'error' : cpuPercent > 50 ? 'warning' : 'success'">
                {{ cpuPercent.toFixed(1) }}%
              </n-text>
            </template>
          </n-statistic>
        </n-card>
      </n-gi>
      <n-gi>
        <n-card size="small">
          <n-statistic label="内存使用率">
            <template #default>
              <n-text :type="memPercent > 80 ? 'error' : memPercent > 50 ? 'warning' : 'success'">
                {{ memPercent.toFixed(1) }}%
              </n-text>
            </template>
          </n-statistic>
        </n-card>
      </n-gi>
    </n-grid>

    <n-grid :cols="2" :x-gap="12" :y-gap="12" style="margin-top: 12px">
      <n-gi>
        <n-card title="CPU 使用率" size="small">
          <div ref="cpuChartRef" style="height: 250px"></div>
        </n-card>
      </n-gi>
      <n-gi>
        <n-card title="内存使用" size="small">
          <div ref="memChartRef" style="height: 250px"></div>
        </n-card>
      </n-gi>
    </n-grid>

    <n-grid :cols="2" :x-gap="12" :y-gap="12" style="margin-top: 12px">
      <n-gi>
        <n-card title="磁盘使用" size="small">
          <div ref="diskChartRef" style="height: 250px"></div>
        </n-card>
      </n-gi>
      <n-gi>
        <n-card title="系统负载" size="small">
          <n-descriptions bordered :column="1" size="small">
            <n-descriptions-item label="1 分钟">{{ status?.cpu?.load1?.toFixed(2) || '-' }}</n-descriptions-item>
            <n-descriptions-item label="5 分钟">{{ status?.cpu?.load5?.toFixed(2) || '-' }}</n-descriptions-item>
            <n-descriptions-item label="15 分钟">{{ status?.cpu?.load15?.toFixed(2) || '-' }}</n-descriptions-item>
            <n-descriptions-item label="CPU 核数">{{ status?.cpu?.count || '-' }}</n-descriptions-item>
            <n-descriptions-item label="磁盘总量">{{ formatBytes(status?.disk?.total || 0) }}</n-descriptions-item>
            <n-descriptions-item label="磁盘已用">{{ formatBytes(status?.disk?.used || 0) }}</n-descriptions-item>
            <n-descriptions-item label="网络发送">{{ formatBytes(status?.network?.bytesSent || 0) }}</n-descriptions-item>
            <n-descriptions-item label="网络接收">{{ formatBytes(status?.network?.bytesRecv || 0) }}</n-descriptions-item>
          </n-descriptions>
        </n-card>
      </n-gi>
    </n-grid>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted, nextTick } from 'vue'
import * as echarts from 'echarts'
import { monitorApi } from '../api/client'

interface SystemStatus {
  system: { hostname: string; platform: string; os: string; uptime: number }
  cpu: { count: number; usage: number[]; load1: number; load5: number; load15: number }
  memory: { total: number; used: number; available: number; usedPercent: number; swapTotal: number; swapUsed: number }
  disk: { total: number; used: number; free: number; usedPercent: number }
  network: { bytesSent: number; bytesRecv: number }
}

const status = ref<SystemStatus | null>(null)
const cpuChartRef = ref<HTMLElement>()
const memChartRef = ref<HTMLElement>()
const diskChartRef = ref<HTMLElement>()
let cpuChart: echarts.ECharts | null = null
let memChart: echarts.ECharts | null = null
let diskChart: echarts.ECharts | null = null
let timer: number | null = null

const cpuHistory: number[] = []
const timeLabels: string[] = []

const cpuPercent = ref(0)
const memPercent = ref(0)

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return (bytes / Math.pow(k, i)).toFixed(1) + ' ' + sizes[i]
}

function formatUptime(seconds: number): string {
  const d = Math.floor(seconds / 86400)
  const h = Math.floor((seconds % 86400) / 3600)
  const m = Math.floor((seconds % 3600) / 60)
  if (d > 0) return `${d}天 ${h}时`
  if (h > 0) return `${h}时 ${m}分`
  return `${m}分`
}

async function fetchData() {
  try {
    const res = await monitorApi.getStatus()
    status.value = res.data
    cpuPercent.value = res.data.cpu?.usage?.reduce((a: number, b: number) => a + b, 0) / (res.data.cpu?.usage?.length || 1)
    memPercent.value = res.data.memory?.usedPercent || 0
    updateCharts()
  } catch {}
}

function updateCharts() {
  if (!status.value) return

  const now = new Date().toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit', second: '2-digit' })
  cpuHistory.push(cpuPercent.value)
  timeLabels.push(now)
  if (cpuHistory.length > 30) {
    cpuHistory.shift()
    timeLabels.shift()
  }

  if (cpuChart) {
    cpuChart.setOption({
      xAxis: { data: timeLabels },
      series: [{ data: cpuHistory }],
    })
  }

  if (memChart) {
    const m = status.value.memory
    memChart.setOption({
      series: [{ data: [{ value: m.used, name: '已用' }, { value: m.available, name: '可用' }] }],
    })
  }

  if (diskChart) {
    const d = status.value.disk
    diskChart.setOption({
      series: [{ data: [{ value: d.used, name: '已用' }, { value: d.free, name: '可用' }] }],
    })
  }
}

function initCharts() {
  if (cpuChartRef.value) {
    cpuChart = echarts.init(cpuChartRef.value)
    cpuChart.setOption({
      grid: { top: 10, right: 15, bottom: 25, left: 45 },
      xAxis: { type: 'category', data: [], axisLabel: { fontSize: 10 } },
      yAxis: { type: 'value', max: 100, axisLabel: { formatter: '{value}%', fontSize: 10 } },
      series: [{ type: 'line', data: [], smooth: true, areaStyle: { opacity: 0.3 }, itemStyle: { color: '#18a058' } }],
      tooltip: { trigger: 'axis', formatter: '{b}<br/>CPU: {c}%' },
    })
  }

  if (memChartRef.value) {
    memChart = echarts.init(memChartRef.value)
    memChart.setOption({
      tooltip: { trigger: 'item', formatter: '{b}: ' + formatBytes(0) },
      series: [{ type: 'pie', radius: ['40%', '70%'], data: [], label: { formatter: '{b}\n{d}%' }, emphasis: { focus: 'self' } }],
      color: ['#e88080', '#63e2b7'],
    })
  }

  if (diskChartRef.value) {
    diskChart = echarts.init(diskChartRef.value)
    diskChart.setOption({
      tooltip: { trigger: 'item', formatter: '{b}: ' + formatBytes(0) },
      series: [{ type: 'pie', radius: ['40%', '70%'], data: [], label: { formatter: '{b}\n{d}%' }, emphasis: { focus: 'self' } }],
      color: ['#f2c97d', '#63e2b7'],
    })
  }
}

onMounted(async () => {
  await nextTick()
  initCharts()
  await fetchData()
  timer = window.setInterval(fetchData, 3000)
})

onUnmounted(() => {
  if (timer) clearInterval(timer)
  cpuChart?.dispose()
  memChart?.dispose()
  diskChart?.dispose()
})
</script>
