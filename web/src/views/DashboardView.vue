<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import {
  SpeedometerOutline, HardwareChipOutline, ServerOutline, LayersOutline,
  CloudDownloadOutline, CubeOutline, SparklesOutline, WarningOutline,
  CheckmarkCircleOutline, ArrowForwardOutline,
} from '@vicons/ionicons5'
import { systemApi, serviceApi } from '@/api'
import { usePlatformStore } from '@/stores/platform'
import { fmtGB, fmtPct, timeAgo, statusOf, fmtUptime } from '@/utils/format'
import type { MetricPoint, Service } from '@/types'
import StatCard from '@/components/StatCard.vue'
import TrendChart from '@/components/TrendChart.vue'

const platform = usePlatformStore()
const router = useRouter()
const points = ref<MetricPoint[]>([])
const services = ref<Service[]>([])
const range = ref('1h')
const loading = ref(false)
let timer: number | undefined

const labels = computed(() =>
  points.value.map((p) => {
    const d = new Date(p.ts * 1000)
    return `${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`
  }),
)
const cpuSeries = computed(() => [{ name: 'CPU', values: points.value.map((p) => p.cpu), color: '#7c5cff' }])
const memSeries = computed(() => [
  { name: '内存', values: points.value.map((p) => p.memory), color: '#22d3ee' },
  { name: '磁盘', values: points.value.map((p) => p.disk), color: '#f59e0b' },
])
const netSeries = computed(() => [
  { name: '入站', values: points.value.map((p) => p.net_rx), color: '#34d399' },
  { name: '出站', values: points.value.map((p) => p.net_tx), color: '#f472b6' },
])

const ov = computed(() => platform.overview)
const metrics = computed(() => ov.value?.metrics)
const gpu = computed(() => ov.value?.metrics?.gpu)
const host = computed(() => ov.value?.host)
const engine = computed(() => ov.value?.engine)
const alerts = computed(() => ov.value?.alerts || [])
const runningServices = computed(() =>
  services.value.filter((s) => s.status === 'running').slice(0, 5),
)
const usageToday = computed(() => ov.value?.usage?.today)

async function loadHistory() {
  try {
    const res = await systemApi.metrics(range.value)
    points.value = res.points || []
  } catch { /* ignore */ }
}
async function loadServices() {
  try {
    const res = await serviceApi.list()
    services.value = res.services || []
  } catch { /* ignore */ }
}
async function refreshAll() {
  loading.value = true
  await Promise.all([platform.refresh(), loadHistory(), loadServices()])
  loading.value = false
}

onMounted(() => {
  refreshAll()
  timer = window.setInterval(() => {
    platform.refresh()
    loadHistory()
    loadServices()
  }, 10000)
})
onBeforeUnmount(() => {
  if (timer) window.clearInterval(timer)
})

function setRange(r: string) {
  range.value = r
  loadHistory()
}
</script>

<template>
  <div class="page">
    <div class="page-head">
      <div>
        <h1 class="page-title">驾驶舱 <span class="live-dot" /></h1>
        <p class="page-sub">
          {{ ov?.edition || 'AIDash 单机 AI 操作系统' }} ·
          {{ host?.hostname || '本机' }} · {{ host?.platform }} {{ host?.platform_version }} ·
          已运行 {{ fmtUptime(ov?.uptime_seconds) }}
        </p>
      </div>
      <div class="row gap-8">
        <n-button size="small" quaternary :loading="loading" @click="refreshAll">刷新</n-button>
        <n-button size="small" type="primary" ghost @click="router.push('/models')">
          <template #icon><n-icon :component="SparklesOutline" /></template>
          部署模型
        </n-button>
      </div>
    </div>

    <div v-if="alerts.length" class="alert-strip glass-soft">
      <n-icon :component="WarningOutline" :size="18" style="color: #fbbf24" />
      <div class="alert-items">
        <div v-for="(a, i) in alerts" :key="i" class="alert-item" :class="a.level">
          <b>{{ a.title }}</b><span>{{ a.detail }}</span>
        </div>
      </div>
    </div>

    <div class="kpis">
      <StatCard label="CPU 使用率" :value="fmtPct(metrics?.cpu?.usage_percent)" :percent="metrics?.cpu?.usage_percent"
        :hint="`${metrics?.cpu?.cores || 0} 核 · 负载 ${metrics?.load?.load1?.toFixed(2) || '-'}`" :icon="SpeedometerOutline" tone="#7c5cff" />
      <StatCard label="内存使用" :value="fmtPct(metrics?.memory?.usage_percent)" :percent="metrics?.memory?.usage_percent"
        :hint="`${metrics?.memory?.used_gb?.toFixed(1) || 0} / ${metrics?.memory?.total_gb?.toFixed(1) || 0} GB`" :icon="HardwareChipOutline" tone="#22d3ee" />
      <StatCard label="磁盘占用" :value="fmtPct(metrics?.disk?.usage_percent)" :percent="metrics?.disk?.usage_percent"
        :hint="`可用 ${fmtGB(metrics?.disk?.free_gb)} / 共 ${fmtGB(metrics?.disk?.total_gb)}`" :icon="ServerOutline" tone="#f59e0b" />
      <StatCard label="运行中的 AI 服务" :value="`${ov?.services?.running || 0}`" :suffix="`/ ${ov?.services?.total || 0}`"
        :percent="ov?.services?.total ? ((ov?.services?.running || 0) / ov.services.total) * 100 : 0"
        :hint="`模型 ${ov?.services?.models_running || 0}/${ov?.services?.models || 0} · 宿主机权限 ${ov?.services?.host_access || 0}`" :icon="LayersOutline" tone="#34d399" />
    </div>

    <div v-if="gpu" class="gpu-strip glass">
      <div class="gpu-main">
        <div class="gpu-label"><n-icon :component="HardwareChipOutline" :size="16" /> GPU · {{ gpu.name }}</div>
        <div class="gpu-metric"><span>{{ fmtPct(gpu.usage_percent) }}</span><small>利用率</small></div>
        <div class="gpu-metric"><span>{{ gpu.mem_used_mb?.toFixed(0) || 0 }}</span><small>/ {{ gpu.mem_total_mb?.toFixed(0) }} MB 显存</small></div>
        <div class="gpu-metric"><span>{{ gpu.temperature?.toFixed(0) }}°C</span><small>温度</small></div>
      </div>
      <div class="gpu-track"><i :style="{ width: Math.min(100, gpu.usage_percent) + '%' }" /></div>
    </div>

    <div class="split-2" style="margin-top: 16px">
      <div class="glass panel">
        <div class="panel-head">
          <div class="section-title"><span class="dot" />资源趋势</div>
          <div class="range-tabs">
            <button v-for="r in ['1h', '6h', '24h', '7d']" :key="r" :class="{ on: range === r }" @click="setRange(r)">{{ r }}</button>
          </div>
        </div>
        <TrendChart :labels="labels" :series="cpuSeries" height="180px" :y-max="100" :show-legend="false" />
        <TrendChart :labels="labels" :series="memSeries" height="140px" :y-max="100" :show-legend="true" />
        <TrendChart :labels="labels" :series="netSeries" height="120px" :show-legend="true" :digits="2" />
        <div class="chart-foot">
          <span>数据点 {{ points.length }} 个 · 采样间隔 5s</span>
          <span class="row gap-8">
            <span class="legend"><i style="background:#7c5cff" />CPU</span>
            <span class="legend"><i style="background:#22d3ee" />内存</span>
            <span class="legend"><i style="background:#f59e0b" />磁盘</span>
          </span>
        </div>
      </div>

      <div class="right-col">
        <div class="glass panel">
          <div class="section-title"><span class="dot" />主机与引擎</div>
          <div class="kv"><span class="k">操作系统</span><span class="v">{{ host?.platform }} {{ host?.platform_version }}</span></div>
          <div class="kv"><span class="k">内核 / 架构</span><span class="v">{{ host?.kernel_version }} · {{ host?.arch }}</span></div>
          <div class="kv"><span class="k">运行时长</span><span class="v">{{ fmtUptime(host?.uptime_seconds) }}</span></div>
          <div class="kv">
            <span class="k">Docker 引擎</span>
            <span class="v">
              <span class="badge" :class="engine?.connected ? 'ok' : 'bad'">
                {{ engine?.connected ? `已连接 v${engine.version}` : '未连接' }}
              </span>
            </span>
          </div>
          <div class="kv"><span class="k">容器 / 镜像</span><span class="v">{{ engine?.containers || 0 }} / {{ engine?.images || 0 }}</span></div>
          <div class="kv"><span class="k">引擎资源配置</span><span class="v">{{ engine?.cpus || 0 }} 核 · {{ engine?.memory_gb || 0 }} GB</span></div>
          <div v-if="!engine?.connected" class="tip bad-tip">{{ engine?.error }}</div>
        </div>

        <div class="glass panel">
          <div class="section-title"><span class="dot" />今日 AI 用量</div>
          <div class="usage-row">
            <div><div class="u-val">{{ usageToday?.requests || 0 }}</div><div class="u-label">调用次数</div></div>
            <div><div class="u-val">{{ (usageToday?.total_tokens || 0).toLocaleString() }}</div><div class="u-label">Token 总量</div></div>
          </div>
          <div class="usage-mini">
            <span>输入 {{ (usageToday?.prompt_tokens || 0).toLocaleString() }}</span>
            <span>输出 {{ (usageToday?.completion_tokens || 0).toLocaleString() }}</span>
          </div>
          <n-button size="small" tertiary block style="margin-top: 14px" @click="router.push('/gateway')">
            查看 AI 网关
            <template #icon><n-icon :component="ArrowForwardOutline" /></template>
          </n-button>
        </div>
      </div>
    </div>

    <div class="split-2" style="margin-top: 16px">
      <div class="glass panel">
        <div class="panel-head">
          <div class="section-title"><span class="dot" />AI 服务</div>
          <n-button text size="small" @click="router.push('/services')">全部服务 →</n-button>
        </div>
        <div v-if="!runningServices.length" class="placeholder-panel">
          <n-icon :component="CloudDownloadOutline" :size="30" style="color: #4b5a7a" />
          <div>还没有运行中的 AI 服务</div>
          <n-button size="small" type="primary" @click="router.push('/models')">去模型广场一键部署</n-button>
        </div>
        <div v-for="s in runningServices" :key="s.id" class="svc-row" @click="router.push('/services')">
          <div class="icon-chip svc-chip">
            <n-icon :component="s.kind === 'model' ? SparklesOutline : CubeOutline" :size="18" />
          </div>
          <div class="svc-main">
            <div class="svc-name">{{ s.name }}
              <span v-if="s.host_access" class="badge warn" style="margin-left: 8px">宿主机权限</span>
            </div>
            <div class="svc-sub">{{ s.image }} · {{ s.model_name || '应用' }}</div>
          </div>
          <div class="svc-right">
            <span class="badge" :class="statusOf(s.status).badge">{{ statusOf(s.status).label }}</span>
            <div class="svc-time">{{ timeAgo(s.started_at || s.created_at) }}</div>
          </div>
        </div>
      </div>

      <div class="glass panel">
        <div class="panel-head">
          <div class="section-title"><span class="dot" />快捷操作</div>
        </div>
        <div class="quick-grid">
          <button class="quick" @click="router.push('/models')">
            <n-icon :component="SparklesOutline" :size="20" /><span>部署模型</span><small>Qwen / DeepSeek / vLLM</small>
          </button>
          <button class="quick" @click="router.push('/apps')">
            <n-icon :component="CubeOutline" :size="20" /><span>部署 AI 应用</span><small>Open WebUI / n8n</small>
          </button>
          <button class="quick" @click="router.push('/containers')">
            <n-icon :component="LayersOutline" :size="20" /><span>容器管理</span><small>日志 / 启停 / 镜像</small>
          </button>
          <button class="quick" @click="router.push('/gateway')">
            <n-icon :component="CheckmarkCircleOutline" :size="20" /><span>API Key</span><small>网关与用量</small>
          </button>
        </div>
        <div class="tip">
          提示：AI 服务默认发布到宿主机端口；如需容器内访问服务器权限，请在部署时打开「宿主机权限（运维模式）」。
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.live-dot { display: inline-block; width: 8px; height: 8px; border-radius: 50%; background: #34d399; margin-left: 8px; box-shadow: 0 0 0 4px rgba(52,211,153,0.15); animation: pulse 2s infinite; }
.alert-strip { display: flex; align-items: flex-start; gap: 12px; padding: 13px 16px; margin-bottom: 16px; border-color: rgba(251,191,36,0.25); background: rgba(251,191,36,0.07); }
.alert-items { display: flex; flex-wrap: wrap; gap: 8px 22px; }
.alert-item { font-size: 12.5px; color: #d7deee; display: flex; gap: 6px; }
.alert-item b { color: #fcd34d; font-weight: 600; }
.alert-item.critical b { color: #fda4af; }

.kpis { display: grid; grid-template-columns: repeat(4, 1fr); gap: 16px; }

.gpu-strip { margin-top: 16px; padding: 16px 20px; }
.gpu-main { display: flex; align-items: center; gap: 34px; flex-wrap: wrap; }
.gpu-label { display: flex; align-items: center; gap: 8px; font-size: 13px; font-weight: 650; color: #c4b5fd; }
.gpu-metric { display: flex; align-items: baseline; gap: 6px; }
.gpu-metric span { font-size: 20px; font-weight: 700; }
.gpu-metric small { font-size: 11px; color: var(--muted); }
.gpu-track { margin-top: 12px; height: 4px; border-radius: 99px; background: rgba(255,255,255,0.06); overflow: hidden; }
.gpu-track i { display: block; height: 100%; background: linear-gradient(90deg, #7c5cff, #22d3ee); }

.panel { padding: 18px 20px 16px; }
.panel-head { display: flex; align-items: center; justify-content: space-between; margin-bottom: 10px; }
.panel-head .section-title { margin: 0; }
.range-tabs { display: flex; gap: 4px; background: rgba(255,255,255,0.04); border: 1px solid var(--line); border-radius: 8px; padding: 2px; }
.range-tabs button { background: none; border: none; color: var(--muted); font-size: 11px; padding: 3px 9px; border-radius: 6px; cursor: pointer; font-family: inherit; }
.range-tabs button.on { background: rgba(124,92,255,0.25); color: #ddd6fe; }

.right-col { display: flex; flex-direction: column; gap: 16px; }
.chart-foot { display: flex; justify-content: space-between; align-items: center; margin-top: 6px; font-size: 10.5px; color: #5d6a89; }
.legend { display: inline-flex; align-items: center; gap: 5px; }
.legend i { width: 8px; height: 8px; border-radius: 2px; display: inline-block; }

.svc-row { display: flex; align-items: center; gap: 12px; padding: 11px 6px; border-bottom: 1px solid rgba(255,255,255,0.05); cursor: pointer; border-radius: 8px; }
.svc-row:hover { background: rgba(255,255,255,0.03); }
.svc-chip { width: 38px; height: 38px; }
.svc-main { flex: 1; min-width: 0; }
.svc-name { font-size: 13px; font-weight: 600; display: flex; align-items: center; }
.svc-sub { font-size: 11px; color: var(--muted); margin-top: 2px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.svc-right { text-align: right; }
.svc-time { font-size: 10.5px; color: #5d6a89; margin-top: 4px; }

.usage-row { display: flex; gap: 40px; margin: 6px 0 10px; }
.u-val { font-size: 26px; font-weight: 720; letter-spacing: -0.6px; }
.u-label { font-size: 11px; color: var(--muted); margin-top: 2px; }
.usage-mini { display: flex; gap: 16px; font-size: 11px; color: #7d8bab; }

.quick-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 10px; }
.quick {
  display: flex; flex-direction: column; align-items: flex-start; gap: 5px; padding: 14px;
  border-radius: 12px; border: 1px solid var(--line); background: rgba(255,255,255,0.03);
  color: var(--text); cursor: pointer; font-family: inherit; text-align: left; transition: all 0.16s ease;
}
.quick:hover { border-color: rgba(124,92,255,0.4); background: rgba(124,92,255,0.09); transform: translateY(-1px); }
.quick span { font-size: 13px; font-weight: 600; }
.quick small { font-size: 10.5px; color: var(--muted); }
.tip { margin-top: 12px; font-size: 11px; color: #5d6a89; line-height: 1.7; }
.bad-tip { color: #fda4af; word-break: break-all; }

@media (max-width: 1100px) {
  .kpis { grid-template-columns: 1fr 1fr; }
}
@media (max-width: 640px) {
  .kpis { grid-template-columns: 1fr; }
}
</style>
