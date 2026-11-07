<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useMessage } from 'naive-ui'
import {
  CubeOutline, ShieldCheckmarkOutline, ChatbubblesOutline, ArrowForwardOutline,
  WarningOutline, LayersOutline,
} from '@vicons/ionicons5'
import { catalogApi, serviceApi } from '@/api'
import type { CatalogItem, Service } from '@/types'
import CatalogCard from '@/components/CatalogCard.vue'
import DeployDrawer from '@/components/DeployDrawer.vue'

const router = useRouter()
const message = useMessage()
const apps = ref<CatalogItem[]>([])
const services = ref<Service[]>([])
const category = ref('全部')
const categories = ref<string[]>([])
const loading = ref(false)
const drawer = ref(false)
const selected = ref<CatalogItem | null>(null)

const filtered = computed(() =>
  apps.value.filter((a) => category.value === '全部' || a.category === category.value),
)
const deployedMap = computed(() => {
  const map: Record<string, Service> = {}
  for (const s of services.value) {
    if (s.catalog_id && s.kind !== 'model') map[s.catalog_id] = s
  }
  return map
})
const hostApps = computed(() => services.value.filter((s) => s.host_access && s.kind !== 'model').length)

async function load() {
  loading.value = true
  try {
    const [cat, svc] = await Promise.all([catalogApi.list(), serviceApi.list()])
    apps.value = cat.apps || []
    categories.value = ['全部', ...(cat.app_categories || [])]
    services.value = svc.services || []
  } catch (e: any) {
    message.error(e.message)
  } finally {
    loading.value = false
  }
}

function open(item: CatalogItem) {
  selected.value = item
  drawer.value = true
}

function onDeployed(svc: Service) {
  message.success('部署任务已启动')
  router.push({ name: 'services', query: { focus: svc.id } })
}

onMounted(load)
</script>

<template>
  <div class="page">
    <div class="page-head">
      <div>
        <h1 class="page-title">AI 应用</h1>
        <p class="page-sub">一键部署对话门户、自动化编排、向量库与数据工具；其中「AI 运维智能体」自带宿主机权限，可作为 Copilot 的执行沙箱。</p>
      </div>
      <n-button size="small" quaternary :loading="loading" @click="load">刷新</n-button>
    </div>

    <div class="banner glass">
      <div class="banner-icon"><n-icon :component="ShieldCheckmarkOutline" :size="24" /></div>
      <div class="banner-body">
        <div class="banner-title">容器化，但拥有服务器权限</div>
        <p>
          AIDash 的「运维模式」在保持容器隔离的同时，按需授予特权模式、宿主机根目录挂载、host PID 与 Docker Socket。
          AI 应用仍以容器交付，却能读取系统日志、执行命令、管理容器——与 d.run 平台组件在 K8s 上获得的权限等价，但不需要 Kubernetes。
        </p>
      </div>
      <div class="banner-stats">
        <div><b>{{ apps.length }}</b><span>可部署应用</span></div>
        <div><b>{{ hostApps }}</b><span>宿主机权限实例</span></div>
        <n-button size="small" type="primary" ghost @click="router.push('/services')">
          管理实例<n-icon :component="ArrowForwardOutline" />
        </n-button>
      </div>
    </div>

    <div class="toolbar">
      <div class="chips">
        <button v-for="c in categories" :key="c" class="chip" :class="{ on: category === c }" @click="category = c">{{ c }}</button>
      </div>
      <div class="spacer" />
      <div class="row gap-8 tiny muted">
        <n-icon :component="WarningOutline" :size="14" style="color: #fbbf24" />
        宿主机权限仅授予可信应用
      </div>
    </div>

    <div class="grid-cards">
      <CatalogCard
        v-for="item in filtered"
        :key="item.id"
        :item="item"
        :service="deployedMap[item.id]"
        @deploy="open(item)"
        @view="router.push({ name: 'services', query: { focus: deployedMap[item.id]?.id } })"
      />
    </div>

    <div class="hint-grid">
      <div class="hint glass-soft">
        <n-icon :component="ChatbubblesOutline" :size="16" />
        <div><b>接入网关</b><span>应用通过 http://host.docker.internal:9090/v1 调用本机所有模型。</span></div>
      </div>
      <div class="hint glass-soft">
        <n-icon :component="CubeOutline" :size="16" />
        <div><b>数据持久化</b><span>命名卷自动创建，删除服务时默认保留，可在容器引擎中清理。</span></div>
      </div>
      <div class="hint glass-soft">
        <n-icon :component="LayersOutline" :size="16" />
        <div><b>统一观测</b><span>所有实例的日志、资源曲线与事件在「模型服务」「可观测」中汇总。</span></div>
      </div>
    </div>

    <DeployDrawer v-model:show="drawer" :item="selected" @deployed="onDeployed" />
  </div>
</template>

<style scoped>
.banner { display: flex; align-items: center; gap: 18px; padding: 22px 26px; margin-bottom: 20px; position: relative; overflow: hidden; }
.banner::before { content: ''; position: absolute; inset: 0; background: radial-gradient(500px 200px at 0% 0%, rgba(251,191,36,0.12), transparent 60%); }
.banner-icon { width: 50px; height: 50px; border-radius: 14px; display: grid; place-items: center; color: #fcd34d; background: rgba(251,191,36,0.1); border: 1px solid rgba(251,191,36,0.28); flex: none; }
.banner-body { flex: 1; min-width: 0; position: relative; }
.banner-title { font-size: 16px; font-weight: 700; margin-bottom: 6px; }
.banner-body p { margin: 0; font-size: 12px; color: #9aa7c4; line-height: 1.8; max-width: 780px; }
.banner-stats { display: flex; align-items: center; gap: 30px; position: relative; }
.banner-stats b { display: block; font-size: 22px; font-weight: 720; }
.banner-stats span { font-size: 10.5px; color: var(--muted); }
.banner-stats .n-button { margin-top: 4px; }

.toolbar { display: flex; align-items: center; gap: 12px; margin-bottom: 16px; flex-wrap: wrap; }
.chips { display: flex; gap: 6px; flex-wrap: wrap; }
.chip { font-size: 11.5px; padding: 5px 12px; border-radius: 999px; cursor: pointer; font-family: inherit; background: rgba(255,255,255,0.03); border: 1px solid var(--line); color: var(--muted); }
.chip.on { color: #ede9fe; background: rgba(124,92,255,0.2); border-color: rgba(124,92,255,0.45); }

.hint-grid { display: grid; grid-template-columns: repeat(3, 1fr); gap: 12px; margin-top: 18px; }
.hint { display: flex; gap: 10px; padding: 13px 15px; align-items: flex-start; color: #a5b4fc; }
.hint b { display: block; font-size: 12px; color: var(--text); margin-bottom: 3px; }
.hint span { font-size: 11px; color: var(--muted); line-height: 1.7; }

@media (max-width: 1100px) {
  .banner { flex-direction: column; align-items: flex-start; }
  .hint-grid { grid-template-columns: 1fr; }
}
</style>
