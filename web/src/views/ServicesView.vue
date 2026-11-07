<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useMessage, useDialog } from 'naive-ui'
import {
  SearchOutline, PlayOutline, StopOutline, RefreshOutline, TrashOutline, CloudUploadOutline,
  LayersOutline, SparklesOutline, ShieldCheckmarkOutline, CubeOutline, ChevronForwardOutline,
} from '@vicons/ionicons5'
import { serviceApi } from '@/api'
import type { Service } from '@/types'
import { statusOf, timeAgo, shortId } from '@/utils/format'
import ServiceDrawer from '@/components/ServiceDrawer.vue'

const route = useRoute()
const router = useRouter()
const message = useMessage()
const dialog = useDialog()

const services = ref<Service[]>([])
const search = ref('')
const statusFilter = ref('all')
const kindFilter = ref('all')
const loading = ref(false)
const drawer = ref(false)
const activeId = ref<string | null>(null)
let timer: number | undefined

const counts = computed(() => ({
  total: services.value.length,
  running: services.value.filter((s) => s.status === 'running').length,
  models: services.value.filter((s) => s.kind === 'model').length,
  host: services.value.filter((s) => s.host_access).length,
  error: services.value.filter((s) => s.status === 'error' || s.health === 'unhealthy').length,
}))

const filtered = computed(() =>
  services.value.filter((s) => {
    if (statusFilter.value === 'running' && s.status !== 'running') return false
    if (statusFilter.value === 'stopped' && s.status !== 'stopped') return false
    if (statusFilter.value === 'error' && !(s.status === 'error' || s.health === 'unhealthy')) return false
    if (kindFilter.value !== 'all' && s.kind !== kindFilter.value) return false
    if (search.value) {
      const q = search.value.toLowerCase()
      return s.name.includes(q) || s.image.toLowerCase().includes(q) || (s.model_name || '').toLowerCase().includes(q)
    }
    return true
  }),
)

async function load() {
  loading.value = true
  try {
    const res = await serviceApi.list()
    services.value = res.services || []
    if (activeId.value) {
      const found = services.value.find((s) => s.id === activeId.value)
      if (!found) { drawer.value = false; activeId.value = null }
    }
  } catch (e: any) {
    message.error(e.message)
  } finally {
    loading.value = false
  }
}

function memLabel(mb: number): string {
  return mb >= 1024 ? `${(mb / 1024).toFixed(0)} GB` : `${mb} MB`
}

function publicUrl(ep: string) {
  return ep.replace('127.0.0.1', window.location.hostname)
}

function open(s: Service) {
  activeId.value = s.id
  drawer.value = true
  router.replace({ query: { ...route.query, focus: s.id } })
}

async function quickAction(s: Service, action: string) {
  try {
    await serviceApi.action(s.id, action)
    message.success('操作已提交')
    load()
  } catch (e: any) {
    message.error(e.message)
  }
}

function confirmRemove(s: Service) {
  dialog.warning({
    title: '移除服务',
    content: `确定移除「${s.name}」？容器将被删除，命名卷保留。`,
    positiveText: '移除',
    negativeText: '取消',
    onPositiveClick: async () => {
      try {
        await serviceApi.remove(s.id, true)
        message.success('已移除')
        if (activeId.value === s.id) { drawer.value = false; activeId.value = null }
        load()
      } catch (e: any) {
        message.error(e.message)
      }
    },
  })
}

onMounted(() => {
  load()
  const focus = route.query.focus as string | undefined
  if (focus) { activeId.value = focus; drawer.value = true }
  timer = window.setInterval(load, 8000)
})
onBeforeUnmount(() => { if (timer) window.clearInterval(timer) })
watch(() => route.query.focus, (v) => {
  if (typeof v === 'string' && v) { activeId.value = v; drawer.value = true }
})
</script>

<template>
  <div class="page">
    <div class="page-head">
      <div>
        <h1 class="page-title">模型服务</h1>
        <p class="page-sub">所有由 AIDash 托管的 AI 推理服务与应用实例：状态、日志、资源与终端集中管理。</p>
      </div>
      <div class="row gap-8">
        <n-button size="small" quaternary :loading="loading" @click="load">刷新</n-button>
        <n-button size="small" type="primary" @click="router.push('/models')">
          <template #icon><n-icon :component="SparklesOutline" /></template>部署模型
        </n-button>
      </div>
    </div>

    <div class="summary">
      <div class="sum-card glass"><b>{{ counts.total }}</b><span>全部服务</span></div>
      <div class="sum-card glass"><b class="ok">{{ counts.running }}</b><span>运行中</span></div>
      <div class="sum-card glass"><b>{{ counts.models }}</b><span>模型服务</span></div>
      <div class="sum-card glass"><b class="warn">{{ counts.host }}</b><span>宿主机权限</span></div>
      <div class="sum-card glass"><b class="bad">{{ counts.error }}</b><span>异常</span></div>
    </div>

    <div class="toolbar">
      <n-input v-model:value="search" placeholder="搜索服务名、镜像或模型" clearable style="max-width: 300px">
        <template #prefix><n-icon :component="SearchOutline" /></template>
      </n-input>
      <div class="chips">
        <button class="chip" :class="{ on: statusFilter === 'all' }" @click="statusFilter = 'all'">全部状态</button>
        <button class="chip" :class="{ on: statusFilter === 'running' }" @click="statusFilter = 'running'">运行中</button>
        <button class="chip" :class="{ on: statusFilter === 'stopped' }" @click="statusFilter = 'stopped'">已停止</button>
        <button class="chip" :class="{ on: statusFilter === 'error' }" @click="statusFilter = 'error'">异常</button>
      </div>
      <div class="spacer" />
      <n-select v-model:value="kindFilter" size="small" style="width: 130px" :options="[
        { label: '全部类型', value: 'all' },
        { label: '模型服务', value: 'model' },
        { label: 'AI 应用', value: 'app' },
        { label: '自定义', value: 'custom' },
      ]" />
    </div>

    <div v-if="filtered.length" class="svc-grid">
      <article v-for="s in filtered" :key="s.id" class="svc-card glass" @click="open(s)">
        <div class="sc-head">
          <div class="sc-icon" :class="statusOf(s.status).cls">
            <n-icon :component="s.kind === 'model' ? LayersOutline : CubeOutline" :size="20" />
          </div>
          <div class="sc-title-wrap">
            <div class="sc-title">{{ s.name }}</div>
            <div class="sc-sub">{{ s.model_name || s.catalog_id || s.kind }}</div>
          </div>
          <span class="badge" :class="statusOf(s.status).badge">{{ statusOf(s.status).label }}</span>
        </div>

        <div class="sc-tags">
          <span class="sc-tag">{{ s.kind === 'model' ? '模型' : s.kind === 'app' ? '应用' : '自定义' }}</span>
          <span v-if="s.host_access" class="sc-tag host"><n-icon :component="ShieldCheckmarkOutline" :size="11" /> 宿主机权限</span>
          <span v-if="s.health === 'unhealthy'" class="sc-tag bad">健康异常</span>
        </div>

        <div class="sc-meta">
          <div><span>镜像</span><b>{{ s.image }}</b></div>
          <div><span>资源</span><b>{{ s.spec?.resources?.cpus }}C / {{ memLabel(s.spec?.resources?.memory_mb || 0) }}</b></div>
          <div><span>端口</span><b>{{ s.spec?.ports?.[0]?.host_port || '-' }} → {{ s.spec?.ports?.[0]?.container_port || '-' }}</b></div>
          <div><span>部署于</span><b>{{ timeAgo(s.started_at || s.created_at) }}</b></div>
        </div>

        <div v-if="s.endpoints?.length" class="sc-endpoint mono">{{ publicUrl(s.endpoints[0]) }}</div>
        <div v-if="s.error" class="sc-error">{{ s.error }}</div>

        <footer class="sc-foot" @click.stop>
          <n-button size="tiny" tertiary @click="open(s)">详情</n-button>
          <n-button v-if="s.status !== 'running'" size="tiny" tertiary @click="quickAction(s, 'start')">
            <template #icon><n-icon :component="PlayOutline" /></template>启动
          </n-button>
          <n-button v-else size="tiny" tertiary @click="quickAction(s, 'stop')">
            <template #icon><n-icon :component="StopOutline" /></template>停止
          </n-button>
          <n-button size="tiny" tertiary @click="quickAction(s, 'restart')">
            <template #icon><n-icon :component="RefreshOutline" /></template>重启
          </n-button>
          <n-button size="tiny" tertiary @click="quickAction(s, 'upgrade')">
            <template #icon><n-icon :component="CloudUploadOutline" /></template>升级
          </n-button>
          <div class="spacer" />
          <n-button size="tiny" quaternary type="error" @click="confirmRemove(s)">
            <template #icon><n-icon :component="TrashOutline" /></template>
          </n-button>
          <n-icon :component="ChevronForwardOutline" :size="14" style="color: #4b5a7a" />
        </footer>
      </article>
    </div>

    <div v-else class="placeholder-panel glass">
      <n-icon :component="LayersOutline" :size="32" style="color: #4b5a7a" />
      <div>{{ services.length ? '没有符合条件的服务' : '还没有部署任何服务' }}</div>
      <n-button v-if="!services.length" size="small" type="primary" @click="router.push('/models')">前往模型广场</n-button>
    </div>

    <ServiceDrawer v-model:show="drawer" :service-id="activeId" @changed="load" />
  </div>
</template>

<style scoped>
.summary { display: grid; grid-template-columns: repeat(5, 1fr); gap: 12px; margin-bottom: 18px; }
.sum-card { padding: 14px 16px; }
.sum-card b { display: block; font-size: 24px; font-weight: 730; letter-spacing: -0.6px; }
.sum-card b.ok { color: #6ee7b7; }
.sum-card b.warn { color: #fcd34d; }
.sum-card b.bad { color: #fda4af; }
.sum-card span { font-size: 11px; color: var(--muted); }

.toolbar { display: flex; align-items: center; gap: 12px; margin-bottom: 16px; flex-wrap: wrap; }
.chips { display: flex; gap: 6px; }
.chip { font-size: 11.5px; padding: 5px 12px; border-radius: 999px; cursor: pointer; font-family: inherit; background: rgba(255,255,255,0.03); border: 1px solid var(--line); color: var(--muted); }
.chip.on { color: #ede9fe; background: rgba(124,92,255,0.2); border-color: rgba(124,92,255,0.45); }

.svc-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(360px, 1fr)); gap: 16px; }
.svc-card { padding: 16px 18px; cursor: pointer; transition: all 0.18s ease; display: flex; flex-direction: column; gap: 11px; }
.svc-card:hover { transform: translateY(-2px); border-color: rgba(124,92,255,0.35); }
.sc-head { display: flex; align-items: center; gap: 11px; }
.sc-icon { width: 42px; height: 42px; border-radius: 12px; display: grid; place-items: center; flex: none; color: #a5b4fc; background: rgba(124,92,255,0.12); border: 1px solid rgba(124,92,255,0.22); }
.sc-icon.running { color: #6ee7b7; background: rgba(52,211,153,0.1); border-color: rgba(52,211,153,0.26); }
.sc-icon.error { color: #fda4af; background: rgba(251,113,133,0.1); border-color: rgba(251,113,133,0.26); }
.sc-title-wrap { flex: 1; min-width: 0; }
.sc-title { font-size: 14px; font-weight: 680; }
.sc-sub { font-size: 11px; color: var(--muted); margin-top: 3px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.sc-tags { display: flex; gap: 6px; flex-wrap: wrap; }
.sc-tag { font-size: 10.5px; padding: 2px 8px; border-radius: 999px; border: 1px solid var(--line); background: rgba(255,255,255,0.03); color: #93a2c4; display: inline-flex; align-items: center; gap: 4px; }
.sc-tag.host { color: #fcd34d; border-color: rgba(251,191,36,0.28); background: rgba(251,191,36,0.08); }
.sc-tag.bad { color: #fda4af; border-color: rgba(251,113,133,0.28); background: rgba(251,113,133,0.08); }
.sc-meta { display: grid; grid-template-columns: 1fr 1fr; gap: 7px 12px; font-size: 11px; }
.sc-meta div { display: flex; flex-direction: column; gap: 2px; min-width: 0; }
.sc-meta span { color: #5d6a89; }
.sc-meta b { font-weight: 550; color: #b9c5dd; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.sc-endpoint { font-size: 11px; color: #a5f3fc; background: rgba(34,211,238,0.06); border: 1px solid rgba(34,211,238,0.16); padding: 6px 9px; border-radius: 8px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.sc-error { font-size: 11px; color: #fda4af; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.sc-foot { display: flex; align-items: center; gap: 4px; padding-top: 10px; border-top: 1px solid rgba(255,255,255,0.05); }

@media (max-width: 1100px) {
  .summary { grid-template-columns: repeat(3, 1fr); }
}
@media (max-width: 640px) {
  .summary { grid-template-columns: 1fr 1fr; }
}
</style>
