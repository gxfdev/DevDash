<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useMessage } from 'naive-ui'
import { PulseOutline, ServerOutline, HardwareChipOutline, CubeOutline, SpeedometerOutline, CloudDownloadOutline } from '@vicons/ionicons5'
import { systemApi, serviceApi } from '@/api'
import type { MetricPoint, Service } from '@/types'
import { fmtGB, fmtPct, fmtMB, fmtUptime, statusOf } from '@/utils/format'
import TrendChart from '@/components/TrendChart.vue'
import { usePlatformStore } from '@/stores/platform'

const platform = usePlatformStore()
const message = useMessage()
const range = ref('6h')
const points = ref<MetricPoint[]>([])
const services = ref<Service[]>([])
const disk = ref<any>(null)
const loading = ref(false)
let timer: number | undefined

const labels = computed(() =>
  points.value.map((p) => {
    const d = new Date(p.ts * 1000)
    return range.value === '7d' ? `${d.getMonth() + 1}/${d.getDate()} ${d.getHours()}h` : `${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`
  }),
)
const ov = computed(() => platform.overview)
const metrics = computed(() => ov.value?.metrics)
const gpu = computed(() => ov.value?.metrics?.gpu)

const cpuSeries = computed(() => [{ name: 'CPU', values: points.value.map((p) => p.cpu), color: '#3b82f6' }])
const memSeries = computed(() => [{ name: '内存', values: points.value.map((p) => p.memory), color: '#14b8a6' }])
const diskSeries = computed(() => [{ name: '磁盘', values: points.value.map((p) => p.disk), color: '#f59e0b' }])
const netSeries = computed(() => [
  { name: '入站', values: points.value.map((p) => p.net_rx), color: '#34d399' },
  { name: '出站', values: points.value.map((p) => p.net_tx), color: '#f472b6' },
])
const gpuSeries = computed(() => [
  { name: 'GPU 利用率', values: points.value.map((p) => p.gpu), color: '#60a5fa' },
  { name: '显存利用率', values: points.value.map((p) => p.gpu_mem), color: '#f472b6' },
])

const unhealthy = computed(() => services.value.filter((s) => s.status === 'error' || s.health === 'unhealthy'))

async function load() {
  loading.value = true
  try {
    const [his, svc, df] = await Promise.all([
      systemApi.metrics(range.value),
      serviceApi.list(),
      systemApi.disk().catch(() => null),
    ])
    points.value = his.points || []
    services.value = svc.services || []
    disk.value = df
  } catch (e: any) {
    message.error(e.message)
  } finally {
    loading.value = false
  }
}

function setRange(r: string) {
  range.value = r
  load()
}

onMounted(() => {
  platform.refresh()
  load()
  timer = window.setInterval(() => { platform.refresh(); load() }, 15000)
})
onBeforeUnmount(() => { if (timer) window.clearInterval(timer) })
</script>

<template>
  <div class="page">
    <div class="page-head">
      <div>
        <h1 class="page-title">可观测</h1>
        <p class="page-sub">主机、容器与 AI 服务的统一观测视图：指标趋势、磁盘占用、服务健康与运行环境。</p>
      </div>
      <div class="row gap-8">
        <div class="range-tabs">
          <button v-for="r in ['1h', '6h', '24h', '7d']" :key="r" :class="{ on: range === r }" @click="setRange(r)">{{ r }}</button>
        </div>
        <n-button size="small" quaternary :loading="loading" @click="load">刷新</n-button>
      </div>
    </div>

    <div class="mini-kpis">
      <div class="mk glass"><n-icon :component="SpeedometerOutline" :size="16" class="ic cpu" /><div><b>{{ fmtPct(metrics?.cpu?.usage_percent) }}</b><span>CPU · {{ metrics?.cpu?.cores || 0 }} 核</span></div></div>
      <div class="mk glass"><n-icon :component="HardwareChipOutline" :size="16" class="ic mem" /><div><b>{{ fmtPct(metrics?.memory?.usage_percent) }}</b><span>内存 · {{ metrics?.memory?.total_gb?.toFixed(0) || 0 }} GB</span></div></div>
      <div class="mk glass"><n-icon :component="ServerOutline" :size="16" class="ic disk" /><div><b>{{ fmtPct(metrics?.disk?.usage_percent) }}</b><span>磁盘 · 可用 {{ fmtGB(metrics?.disk?.free_gb) }}</span></div></div>
      <div class="mk glass"><n-icon :component="CubeOutline" :size="16" class="ic svc" /><div><b>{{ ov?.containers?.running || 0 }}/{{ ov?.containers?.total || 0 }}</b><span>容器运行中</span></div></div>
      <div class="mk glass"><n-icon :component="PulseOutline" :size="16" class="ic bad" /><div><b :class="{ bad: unhealthy.length }">{{ unhealthy.length }}</b><span>异常服务</span></div></div>
    </div>

    <div class="chart-grid">
      <div class="glass panel">
        <div class="panel-title">CPU 使用率</div>
        <TrendChart :labels="labels" :series="cpuSeries" height="170px" :y-max="100" />
        <div class="panel-foot">负载 {{ metrics?.load?.load1?.toFixed(2) }} / {{ metrics?.load?.load5?.toFixed(2) }} / {{ metrics?.load?.load15?.toFixed(2) }}</div>
      </div>
      <div class="glass panel">
        <div class="panel-title">内存使用率</div>
        <TrendChart :labels="labels" :series="memSeries" height="170px" :y-max="100" />
        <div class="panel-foot">{{ metrics?.memory?.used_gb?.toFixed(1) }} / {{ metrics?.memory?.total_gb?.toFixed(1) }} GB</div>
      </div>
      <div class="glass panel">
        <div class="panel-title">磁盘使用率</div>
        <TrendChart :labels="labels" :series="diskSeries" height="170px" :y-max="100" />
        <div class="panel-foot">I/O 读 {{ fmtMB(metrics?.disk?.read_rate_mb) }}/s · 写 {{ fmtMB(metrics?.disk?.write_rate_mb) }}/s</div>
      </div>
      <div class="glass panel">
        <div class="panel-title">网络吞吐</div>
        <TrendChart :labels="labels" :series="netSeries" height="170px" :show-legend="true" :digits="2" />
        <div class="panel-foot">↓ {{ metrics?.network?.recv_rate_mb?.toFixed(2) }} MB/s · ↑ {{ metrics?.network?.sent_rate_mb?.toFixed(2) }} MB/s</div>
      </div>
    </div>

    <div v-if="gpu" class="glass panel" style="margin-top: 16px">
      <div class="panel-title">GPU · {{ gpu.name }}</div>
      <TrendChart :labels="labels" :series="gpuSeries" height="170px" :show-legend="true" :y-max="100" />
      <div class="panel-foot">显存 {{ gpu.mem_used_mb?.toFixed(0) }} / {{ gpu.mem_total_mb?.toFixed(0) }} MB · 温度 {{ gpu.temperature?.toFixed(0) }}°C · {{ gpu.devices?.length }} 张卡</div>
    </div>

    <div class="split-2" style="margin-top: 16px">
      <div class="glass panel">
        <div class="panel-title">服务健康</div>
        <table class="tbl">
          <thead><tr><th>服务</th><th>状态</th><th>健康</th><th>端口</th><th>类型</th></tr></thead>
          <tbody>
            <tr v-for="s in services" :key="s.id">
              <td><b>{{ s.name }}</b><div class="tiny muted">{{ s.image }}</div></td>
              <td><span class="badge" :class="statusOf(s.status).badge">{{ statusOf(s.status).label }}</span></td>
              <td>
                <span class="badge" :class="s.health === 'healthy' ? 'ok' : s.health === 'unhealthy' ? 'bad' : 'neutral'">
                  {{ s.health === 'healthy' ? '正常' : s.health === 'unhealthy' ? '异常' : '未知' }}
                </span>
              </td>
              <td class="mono">{{ s.spec?.ports?.[0]?.host_port || '-' }}</td>
              <td>{{ s.kind === 'model' ? '模型' : '应用' }}</td>
            </tr>
            <tr v-if="!services.length"><td colspan="5" class="muted" style="text-align:center; padding: 28px">暂无服务</td></tr>
          </tbody>
        </table>
      </div>

      <div class="right-col">
        <div class="glass panel">
          <div class="panel-title">主机信息</div>
          <div class="kv"><span class="k">主机名</span><span class="v">{{ ov?.host?.hostname }}</span></div>
          <div class="kv"><span class="k">系统</span><span class="v">{{ ov?.host?.platform }} {{ ov?.host?.platform_version }}</span></div>
          <div class="kv"><span class="k">内核</span><span class="v">{{ ov?.host?.kernel_version }}</span></div>
          <div class="kv"><span class="k">架构</span><span class="v">{{ ov?.host?.arch }}</span></div>
          <div class="kv"><span class="k">运行时长</span><span class="v">{{ fmtUptime(ov?.host?.uptime_seconds) }}</span></div>
          <div class="kv"><span class="k">Docker</span><span class="v">{{ ov?.engine?.connected ? `v${ov?.engine?.version}` : '未连接' }}</span></div>
        </div>

        <div v-if="disk" class="glass panel">
          <div class="panel-title">Docker 存储占用</div>
          <div class="kv"><span class="k">镜像</span><span class="v">{{ fmtGB((disk.images_size || 0) / 1e9) }} · {{ disk.images }} 个</span></div>
          <div class="kv"><span class="k">容器可写层</span><span class="v">{{ fmtGB((disk.containers_size || 0) / 1e9) }}</span></div>
          <div class="kv"><span class="k">数据卷</span><span class="v">{{ fmtGB((disk.volumes_size || 0) / 1e9) }} · {{ disk.volumes }} 个</span></div>
          <div class="kv"><span class="k">镜像层缓存</span><span class="v">{{ fmtGB((disk.layers_size || 0) / 1e9) }}</span></div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.range-tabs { display: flex; gap: 4px; background: rgba(255,255,255,0.04); border: 1px solid var(--line); border-radius: 8px; padding: 2px; }
.range-tabs button { background: none; border: none; color: var(--muted); font-size: 11px; padding: 4px 10px; border-radius: 6px; cursor: pointer; font-family: inherit; }
.range-tabs button.on { background: rgba(59,130,246,0.25); color: #ddd6fe; }

.mini-kpis { display: grid; grid-template-columns: repeat(5, 1fr); gap: 12px; margin-bottom: 16px; }
.mk { display: flex; align-items: center; gap: 11px; padding: 13px 15px; }
.mk .ic { padding: 8px; border-radius: 9px; box-sizing: content-box; }
.ic.cpu { color: #60a5fa; background: rgba(59,130,246,0.12); }
.ic.mem { color: #67e8f9; background: rgba(20,184,166,0.12); }
.ic.disk { color: #fcd34d; background: rgba(251,191,36,0.12); }
.ic.svc { color: #6ee7b7; background: rgba(52,211,153,0.12); }
.ic.bad { color: #fda4af; background: rgba(251,113,133,0.12); }
.mk b { display: block; font-size: 18px; font-weight: 700; letter-spacing: -0.3px; }
.mk b.bad { color: #fda4af; }
.mk span { font-size: 10.5px; color: var(--muted); }

.chart-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 16px; }
.panel { padding: 16px 18px 12px; }
.panel-title { font-size: 12.5px; font-weight: 650; color: #b9c5dd; margin-bottom: 6px; }
.panel-foot { font-size: 10.5px; color: #5d6a89; margin-top: 4px; padding-bottom: 4px; }
.right-col { display: flex; flex-direction: column; gap: 16px; }
.tbl { width: 100%; border-collapse: collapse; font-size: 11.5px; }
.tbl th { text-align: left; font-size: 10px; color: #7d8bab; padding: 8px 8px; border-bottom: 1px solid var(--line); font-weight: 600; }
.tbl td { padding: 9px 8px; border-bottom: 1px solid rgba(255,255,255,0.04); color: #c3cee6; }
.tbl td b { font-weight: 600; }

@media (max-width: 1100px) {
  .mini-kpis { grid-template-columns: 1fr 1fr; }
  .chart-grid { grid-template-columns: 1fr; }
}
</style>
