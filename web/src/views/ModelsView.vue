<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useMessage } from 'naive-ui'
import { SearchOutline, SparklesOutline, FlashOutline, HardwareChipOutline, ArrowForwardOutline } from '@vicons/ionicons5'
import { catalogApi, serviceApi } from '@/api'
import type { CatalogItem, Service } from '@/types'
import CatalogCard from '@/components/CatalogCard.vue'
import DeployDrawer from '@/components/DeployDrawer.vue'

const router = useRouter()
const message = useMessage()
const models = ref<CatalogItem[]>([])
const categories = ref<string[]>([])
const services = ref<Service[]>([])
const search = ref('')
const category = ref('全部')
const gpuOnly = ref(false)
const loading = ref(false)
const drawer = ref(false)
const selected = ref<CatalogItem | null>(null)

const filtered = computed(() =>
  models.value.filter((m) => {
    if (category.value !== '全部' && m.category !== category.value) return false
    if (gpuOnly.value && !m.requires_gpu) return false
    if (search.value) {
      const q = search.value.toLowerCase()
      return (
        m.name.toLowerCase().includes(q) || m.id.includes(q) || m.vendor.toLowerCase().includes(q) ||
        m.tags.some((t) => t.toLowerCase().includes(q)) || (m.model_ref || '').includes(q)
      )
    }
    return true
  }),
)

const deployedMap = computed(() => {
  const map: Record<string, Service> = {}
  for (const s of services.value) {
    if (s.catalog_id && s.kind === 'model') {
      if (!map[s.catalog_id] || s.status === 'running') map[s.catalog_id] = s
    }
  }
  return map
})

const runningModels = computed(() => services.value.filter((s) => s.kind === 'model' && s.status === 'running').length)

async function load() {
  loading.value = true
  try {
    const [cat, svc] = await Promise.all([catalogApi.list(), serviceApi.list()])
    models.value = cat.models || []
    categories.value = cat.model_categories || []
    services.value = svc.services || []
  } catch (e: any) {
    message.error(e.message)
  } finally {
    loading.value = false
  }
}

function openDeploy(item: CatalogItem) {
  selected.value = item
  drawer.value = true
}

function onDeployed(svc: Service) {
  message.success('部署任务已启动，正在拉取镜像与模型')
  router.push({ name: 'services', query: { focus: svc.id } })
}

onMounted(load)
</script>

<template>
  <div class="page">
    <div class="hero glass">
      <div class="hero-left">
        <div class="hero-chip"><n-icon :component="SparklesOutline" :size="14" /> 模型广场 · Model Plaza</div>
        <h1>开源大模型，<span class="grad-text">一键变成 API</span></h1>
        <p>
          选择模型后在任意单机 Docker 环境分钟级拉起。自动完成镜像拉取、模型下载、端口发布与
          OpenAI 兼容网关注册，无需 Kubernetes、无需手工编排。
        </p>
        <div class="hero-stats">
          <div><b>{{ models.length }}</b><span>可部署模型</span></div>
          <div><b>{{ runningModels }}</b><span>运行中</span></div>
          <div><b>{{ services.length }}</b><span>总服务</span></div>
        </div>
      </div>
      <div class="hero-right">
        <div class="hero-card">
          <div class="hc-title">推荐组合</div>
          <div class="hc-row"><span>日常中文</span><b>Qwen2.5 7B</b></div>
          <div class="hc-row"><span>深度推理</span><b>DeepSeek R1 7B</b></div>
          <div class="hc-row"><span>高并发</span><b>vLLM + GPU</b></div>
          <n-button size="small" type="primary" block style="margin-top: 12px" @click="router.push('/services')">
            查看已部署服务
            <template #icon><n-icon :component="ArrowForwardOutline" /></template>
          </n-button>
        </div>
        <div class="float-badge b1"><n-icon :component="FlashOutline" :size="13" /> Ollama 运行时</div>
        <div class="float-badge b2"><n-icon :component="HardwareChipOutline" :size="13" /> vLLM · GPU 加速</div>
      </div>
    </div>

    <div class="toolbar">
      <n-input v-model:value="search" placeholder="搜索模型、厂商或能力标签" clearable style="max-width: 320px">
        <template #prefix><n-icon :component="SearchOutline" /></template>
      </n-input>
      <div class="chips">
        <button class="chip" :class="{ on: category === '全部' }" @click="category = '全部'">全部</button>
        <button v-for="c in categories" :key="c" class="chip" :class="{ on: category === c }" @click="category = c">{{ c }}</button>
      </div>
      <div class="spacer" />
      <n-checkbox v-model:checked="gpuOnly">仅 GPU</n-checkbox>
      <n-button size="small" quaternary :loading="loading" @click="load">刷新</n-button>
    </div>

    <div v-if="filtered.length" class="grid-cards">
      <CatalogCard
        v-for="item in filtered"
        :key="item.id"
        :item="item"
        :service="deployedMap[item.id]"
        @deploy="openDeploy(item)"
        @view="router.push({ name: 'services', query: { focus: deployedMap[item.id]?.id } })"
      />
    </div>
    <div v-else class="placeholder-panel glass">
      <n-icon :component="SearchOutline" :size="28" style="color: #4b5a7a" />
      <div>没有匹配的模型，换个关键词试试</div>
    </div>

    <DeployDrawer v-model:show="drawer" :item="selected" @deployed="onDeployed" />
  </div>
</template>

<style scoped>
.hero { position: relative; display: flex; gap: 30px; padding: 30px 32px; overflow: hidden; margin-bottom: 22px; }
.hero::before {
  content: ''; position: absolute; inset: 0;
  background: rgba(59,130,246,0.05);
}
.hero-left { flex: 1.4; position: relative; }
.hero-chip {
  display: inline-flex; align-items: center; gap: 6px; font-size: 11px; color: #c4b5fd;
  border: 1px solid rgba(59,130,246,0.32); background: rgba(59,130,246,0.12);
  padding: 4px 10px; border-radius: 999px; margin-bottom: 14px;
}
.hero h1 { font-size: 30px; margin: 0 0 12px; letter-spacing: -0.8px; font-weight: 740; }
.hero p { font-size: 13px; color: #9aa7c4; line-height: 1.85; max-width: 640px; margin: 0 0 20px; }
.hero-stats { display: flex; gap: 38px; }
.hero-stats b { display: block; font-size: 24px; font-weight: 730; letter-spacing: -0.6px; }
.hero-stats span { font-size: 11px; color: var(--muted); }
.hero-right { position: relative; flex: 0.85; min-width: 260px; }
.hero-card { position: relative; border-radius: 16px; border: 1px solid var(--line); background: rgba(9,13,24,0.7); padding: 16px; z-index: 2; }
.hc-title { font-size: 11px; color: var(--muted); letter-spacing: 0.8px; margin-bottom: 10px; }
.hc-row { display: flex; justify-content: space-between; font-size: 12px; padding: 7px 0; border-bottom: 1px dashed rgba(255,255,255,0.06); }
.hc-row span { color: var(--muted); }
.hc-row b { font-weight: 620; }
.float-badge { display: none; }

.toolbar { display: flex; align-items: center; gap: 14px; margin-bottom: 18px; flex-wrap: wrap; }
.chips { display: flex; gap: 6px; flex-wrap: wrap; }
.chip {
  font-size: 11.5px; padding: 5px 12px; border-radius: 999px; cursor: pointer; font-family: inherit;
  background: rgba(255,255,255,0.03); border: 1px solid var(--line); color: var(--muted); transition: all 0.15s;
}
.chip:hover { color: var(--text); border-color: var(--line-strong); }
.chip.on { color: #ede9fe; background: rgba(59,130,246,0.2); border-color: rgba(59,130,246,0.45); }

@media (max-width: 900px) {
  .hero { flex-direction: column; }
}
</style>
