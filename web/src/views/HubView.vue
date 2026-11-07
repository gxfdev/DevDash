<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useMessage, useDialog } from 'naive-ui'
import {
  CloudDownloadOutline, RefreshOutline, TrashOutline, DocumentTextOutline, RocketOutline,
  CubeOutline, ServerOutline, WarningOutline, CheckmarkCircleOutline, CloseCircleOutline,
  SyncOutline, FolderOpenOutline, LinkOutline, LayersOutline,
} from '@vicons/ionicons5'
import { assetApi, serviceApi } from '@/api'
import type { AssetSource, ModelAsset } from '@/types'
import { fmtMB, timeAgo, statusOf, fmtTime } from '@/utils/format'

const message = useMessage()
const dialog = useDialog()

const sources = ref<AssetSource[]>([])
const assets = ref<ModelAsset[]>([])
const loading = ref(false)
const createOpen = ref(false)
const creating = ref(false)
const logOpen = ref(false)
const logText = ref('')
const logTitle = ref('')
const activeLogId = ref('')
const deploying = ref<ModelAsset | null>(null)
const deployOpen = ref(false)
const deploySaving = ref(false)
const deployForm = ref({
  name: '', model_name: '', runtime: 'llamacpp', cpus: 4, memory_mb: 8192,
  gpu_count: 0, host_access: false,
})
const volume = ref('aidash-models')

const form = ref({
  source: 'modelscope', name: '', repo: '', revision: '', file: '',
  mirror: '', token: '', url: '', host_path: '', runtime: '',
})

const selectedSource = computed(() => sources.value.find((s) => s.id === form.value.source))

const sourceIcon: Record<string, any> = {
  modelscope: LayersOutline, huggingface: CloudDownloadOutline, ollama: CubeOutline,
  local: FolderOpenOutline, url: LinkOutline,
}

const statusMeta: Record<string, { label: string; badge: string }> = {
  queued: { label: '排队中', badge: 'info' },
  downloading: { label: '下载中', badge: 'warn' },
  completed: { label: '已就绪', badge: 'ok' },
  registry: { label: '仓库直拉', badge: 'ok' },
  failed: { label: '失败', badge: 'bad' },
  canceled: { label: '已取消', badge: 'neutral' },
}

function meta(a: ModelAsset) { return statusMeta[a.status] || { label: a.status, badge: 'neutral' } }

let timer: number | undefined
let logTimer: number | undefined

async function load() {
  loading.value = true
  try {
    const [s, a] = await Promise.all([assetApi.sources(), assetApi.list()])
    sources.value = s.sources || []
    volume.value = s.volume || 'aidash-models'
    assets.value = a.assets || []
  } catch (e: any) {
    message.error(e.message)
  } finally {
    loading.value = false
  }
}

function openCreate(sourceId?: string) {
  form.value = {
    source: sourceId || 'modelscope', name: '', repo: '', revision: '', file: '',
    mirror: '', token: '', url: '', host_path: '', runtime: '',
  }
  createOpen.value = true
}

function applySourceDefaults() {
  const src = selectedSource.value
  if (!src) return
  if (src.id === 'huggingface' && !form.value.mirror) form.value.mirror = src.default_mirror
  if (src.id === 'modelscope' && !form.value.revision) form.value.revision = 'master'
  if (src.id === 'huggingface' && !form.value.revision) form.value.revision = 'main'
}

async function submitCreate() {
  if (!form.value.name && !form.value.repo && !form.value.host_path) {
    message.warning('请填写名称和模型来源')
    return
  }
  creating.value = true
  try {
    const payload: any = {
      name: form.value.name || form.value.repo || 'model',
      source: form.value.source,
      repo: form.value.repo.trim(),
      revision: form.value.revision.trim(),
      file: form.value.file.trim(),
      mirror: form.value.mirror.trim(),
      token: form.value.token.trim(),
      url: form.value.url.trim(),
      host_path: form.value.host_path.trim(),
      runtime: form.value.runtime,
    }
    await assetApi.create(payload)
    message.success('下载任务已创建，正在拉取模型')
    createOpen.value = false
    await load()
  } catch (e: any) {
    message.error(e.message)
  } finally {
    creating.value = false
  }
}

async function showLogs(a: ModelAsset) {
  logOpen.value = true
  logTitle.value = `${a.name} · 下载日志`
  activeLogId.value = a.id
  logText.value = '加载中…'
  await refreshLog(a.id)
  if (logTimer) window.clearInterval(logTimer)
  logTimer = window.setInterval(() => refreshLog(a.id), 2500)
}

function stopLogTimer() {
  if (logTimer) window.clearInterval(logTimer)
  logTimer = undefined
}

async function refreshLog(id: string) {
  try {
    const res = await assetApi.logs(id, 400)
    logText.value = res.logs || '（暂无日志）'
    const el = document.querySelector('.hub-log')
    if (el) el.scrollTop = el.scrollHeight
  } catch { /* ignore */ }
}

async function cancel(a: ModelAsset) {
  try {
    await assetApi.cancel(a.id)
    message.success('已取消')
    load()
  } catch (e: any) { message.error(e.message) }
}

async function scan(a: ModelAsset) {
  try {
    const res = await assetApi.scan(a.id)
    message.success(`扫描完成：${res.asset.file_count} 个文件`)
    load()
  } catch (e: any) { message.error(e.message) }
}

function remove(a: ModelAsset) {
  dialog.warning({
    title: '删除模型',
    content: `删除「${a.name}」及其下载文件？此操作不可恢复。`,
    positiveText: '删除',
    negativeText: '取消',
    onPositiveClick: async () => {
      try {
        await assetApi.remove(a.id, true)
        message.success('已删除')
        load()
      } catch (e: any) { message.error(e.message) }
    },
  })
}

function openDeploy(a: ModelAsset) {
  deploying.value = a
  const base = a.name.toLowerCase().replace(/[^a-z0-9-]+/g, '-').replace(/^-+|-+$/g, '').slice(0, 30)
  deployForm.value = {
    name: `${base || 'model'}-serve`,
    model_name: base || 'local-model',
    runtime: a.runtime || 'llamacpp',
    cpus: a.runtime === 'vllm' ? 8 : 4,
    memory_mb: a.runtime === 'vllm' ? 16384 : 8192,
    gpu_count: a.runtime === 'vllm' ? 1 : 0,
    host_access: false,
  }
  deployOpen.value = true
}

async function submitDeploy() {
  if (!deploying.value) return
  deploySaving.value = true
  try {
    const svc = await serviceApi.deploy({
      name: deployForm.value.name,
      kind: 'model',
      model_name: deployForm.value.model_name,
      model_asset_id: deploying.value.id,
      model_runtime: deployForm.value.runtime,
      resources: {
        cpus: deployForm.value.cpus, memory_mb: deployForm.value.memory_mb,
        shm_mb: deployForm.value.runtime === 'vllm' ? 2048 : 512,
        gpu_count: deployForm.value.gpu_count, gpu_kind: '',
      },
      host_access: { enabled: deployForm.value.host_access, privileged: false, host_pid: false, mount_host_fs: false, mount_docker_socket: false },
      auto_start: true,
    } as any)
    deployOpen.value = false
    message.success('部署任务已创建')
    window.location.href = `/services?focus=${svc.id}`
  } catch (e: any) {
    message.error(e.message)
  } finally {
    deploySaving.value = false
  }
}

onMounted(() => {
  load()
  timer = window.setInterval(load, 6000)
})
onBeforeUnmount(() => {
  if (timer) window.clearInterval(timer)
  if (logTimer) window.clearInterval(logTimer)
})
</script>

<template>
  <div class="page">
    <div class="page-head">
      <div>
        <h1 class="page-title">模型仓库</h1>
        <p class="page-sub">
          先把真实模型文件下载到服务器（魔搭 / HuggingFace / 直链 / 本地目录），再用 llama.cpp 或 vLLM 容器加载。
          下载产物存放在 Docker 数据卷 <code>{{ volume }}</code>，推理容器直接挂载同一卷，不存在假数据。
        </p>
      </div>
      <div class="row gap-8">
        <n-button size="small" quaternary :loading="loading" @click="load">刷新</n-button>
        <n-button size="small" type="primary" @click="openCreate()">
          <template #icon><n-icon :component="CloudDownloadOutline" /></template>下载模型
        </n-button>
      </div>
    </div>

    <div class="source-grid">
      <button v-for="s in sources" :key="s.id" class="source-card glass" @click="openCreate(s.id)">
        <div class="src-icon"><n-icon :component="sourceIcon[s.id] || CloudDownloadOutline" :size="20" /></div>
        <div class="src-main">
          <div class="src-name">{{ s.name }} <span class="src-badge">{{ s.badge }}</span></div>
          <div class="src-desc">{{ s.description }}</div>
        </div>
        <n-icon :component="CloudDownloadOutline" :size="15" class="src-arrow" />
      </button>
    </div>

    <div v-if="assets.length" class="asset-list">
      <article v-for="a in assets" :key="a.id" class="asset glass">
        <div class="asset-head">
          <div class="asset-icon" :class="meta(a).badge">
            <n-icon :component="a.status === 'completed' || a.status === 'registry' ? CheckmarkCircleOutline : a.status === 'failed' ? CloseCircleOutline : CloudDownloadOutline" :size="20" />
          </div>
          <div class="asset-title">
            <div class="asset-name">{{ a.name }}</div>
            <div class="asset-sub">
              <span class="badge neutral">{{ a.source }}</span>
              <span v-if="a.repo" class="mono">{{ a.repo }}<template v-if="a.revision">@{{ a.revision }}</template></span>
              <span v-if="a.runtime" class="badge info">{{ a.runtime }}</span>
            </div>
          </div>
          <span class="badge" :class="meta(a).badge">{{ meta(a).label }}</span>
        </div>

        <div class="asset-meta">
          <div><span>大小</span><b>{{ a.size_bytes ? fmtMB(a.size_bytes / 1e6) : '-' }}</b></div>
          <div><span>文件数</span><b>{{ a.file_count || '-' }}</b></div>
          <div class="wide"><span>路径</span><b class="mono">{{ a.host_path || a.local_path }}</b></div>
          <div><span>创建</span><b>{{ timeAgo(a.created_at) }}</b></div>
        </div>

        <div v-if="a.gguf" class="asset-gguf mono"><n-icon :component="CubeOutline" :size="13" /> {{ a.gguf }}</div>
        <div v-if="a.error" class="asset-error">{{ a.error.split('\n')[0] }}</div>

        <footer class="asset-foot">
          <n-button v-if="a.status === 'completed' || a.status === 'registry'" size="tiny" type="primary" secondary @click="openDeploy(a)">
            <template #icon><n-icon :component="RocketOutline" /></template>部署推理服务
          </n-button>
          <n-button size="tiny" tertiary @click="showLogs(a)">
            <template #icon><n-icon :component="DocumentTextOutline" /></template>日志
          </n-button>
          <n-button v-if="a.status === 'downloading' || a.status === 'queued'" size="tiny" tertiary @click="cancel(a)">
            <template #icon><n-icon :component="CloseCircleOutline" /></template>取消
          </n-button>
          <n-button v-if="a.status === 'completed'" size="tiny" tertiary @click="scan(a)">
            <template #icon><n-icon :component="SyncOutline" /></template>重新扫描
          </n-button>
          <div class="spacer" />
          <n-button size="tiny" quaternary type="error" @click="remove(a)">
            <template #icon><n-icon :component="TrashOutline" /></template>
          </n-button>
        </footer>
      </article>
    </div>

    <div v-else class="placeholder-panel glass">
      <n-icon :component="CloudDownloadOutline" :size="32" style="color: #4b5a7a" />
      <div>还没有本地模型。点击上方任一来源下载真实模型文件。</div>
      <n-button size="small" type="primary" @click="openCreate()">开始下载</n-button>
    </div>

    <!-- 下载弹窗 -->
    <n-modal v-model:show="createOpen" preset="card" title="下载模型文件" style="width: 720px">
      <div class="src-tabs">
        <button v-for="s in sources" :key="s.id" :class="{ on: form.source === s.id }" @click="form.source = s.id; applySourceDefaults()">
          <n-icon :component="sourceIcon[s.id] || CloudDownloadOutline" :size="14" />
          {{ s.name }}
        </button>
      </div>

      <n-form label-placement="top" size="small">
        <div class="form-grid">
          <n-form-item label="模型名称（用于资产与服务名）">
            <n-input v-model:value="form.name" placeholder="例如 qwen2.5-0.5b-gguf" />
          </n-form-item>
          <n-form-item label="推理引擎">
            <n-select v-model:value="form.runtime" :options="[
              { label: '自动识别（GGUF→llama.cpp，其余→vLLM）', value: '' },
              { label: 'llama.cpp（CPU / GGUF）', value: 'llamacpp' },
              { label: 'vLLM（GPU / HuggingFace 权重）', value: 'vllm' },
              { label: 'Ollama 仓库直拉', value: 'ollama' },
            ]" />
          </n-form-item>
        </div>

        <template v-if="form.source === 'modelscope' || form.source === 'huggingface'">
          <n-form-item :label="form.source === 'modelscope' ? '魔搭模型 ID' : 'HuggingFace 模型 ID'">
            <n-input v-model:value="form.repo" :placeholder="form.source === 'modelscope' ? 'Qwen/Qwen2.5-0.5B-Instruct-GGUF' : 'Qwen/Qwen2.5-0.5B-Instruct-GGUF'" />
          </n-form-item>
          <div class="form-grid">
            <n-form-item :label="form.source === 'modelscope' ? '分支（默认 master）' : 'Revision（默认 main）'">
              <n-input v-model:value="form.revision" :placeholder="form.source === 'modelscope' ? 'master' : 'main'" />
            </n-form-item>
            <n-form-item label="只下载文件（可选，支持通配）">
              <n-input v-model:value="form.file" placeholder="*.gguf 或 qwen2.5-0.5b-instruct-q4_k_m.gguf" />
            </n-form-item>
          </div>
          <n-form-item v-if="form.source === 'huggingface'" label="镜像地址">
            <n-input v-model:value="form.mirror" placeholder="https://hf-mirror.com" />
          </n-form-item>
          <n-form-item label="访问令牌（私有/受限仓库，可选）">
            <n-input v-model:value="form.token" type="password" show-password-on="click" placeholder="HF_TOKEN / MODELSCOPE_API_TOKEN" />
          </n-form-item>
        </template>

        <template v-else-if="form.source === 'ollama'">
          <n-form-item label="Ollama 模型">
            <n-input v-model:value="form.repo" placeholder="qwen2.5:0.5b / deepseek-r1:7b" />
          </n-form-item>
          <div class="note">Ollama 会直接从官方仓库拉取，无需本地文件；部署时选择 ollama 引擎即可。</div>
        </template>

        <template v-else-if="form.source === 'local'">
          <n-form-item label="宿主机模型目录（绝对路径）">
            <n-input v-model:value="form.host_path" placeholder="/opt/models/qwen2.5-0.5b-gguf" />
          </n-form-item>
          <div class="note">目录会以只读方式挂载给推理容器（llama.cpp 需包含 .gguf 文件）。</div>
        </template>

        <template v-else>
          <n-form-item label="模型文件直链">
            <n-input v-model:value="form.url" placeholder="https://内网仓库/models/qwen2.5-0.5b-q4_k_m.gguf" />
          </n-form-item>
          <n-form-item label="保存文件名（可选，识别 .tar.gz 自动解压）">
            <n-input v-model:value="form.file" placeholder="model.gguf" />
          </n-form-item>
        </template>
      </n-form>

      <template #footer>
        <div class="row gap-12" style="justify-content: flex-end">
          <span class="muted tiny">下载容器：python:3.12-slim / alpine，挂载数据卷 {{ volume }} → /models</span>
          <div class="spacer" />
          <n-button quaternary @click="createOpen = false">取消</n-button>
          <n-button type="primary" :loading="creating" @click="submitCreate">开始下载</n-button>
        </div>
      </template>
    </n-modal>

    <!-- 日志 -->
    <n-drawer v-model:show="logOpen" :width="700" placement="right" @after-leave="stopLogTimer">
      <div class="log-panel">
        <header class="log-head">
          <div>
            <div class="log-title">{{ logTitle }}</div>
            <div class="muted tiny">每 2.5 秒刷新，显示下载容器实时输出</div>
          </div>
          <n-button quaternary circle size="small" @click="logOpen = false">✕</n-button>
        </header>
        <pre class="hub-log">{{ logText }}</pre>
        <footer class="log-foot">
          <n-button size="small" @click="refreshLog(activeLogId)"><n-icon :component="RefreshOutline" />刷新</n-button>
        </footer>
      </div>
    </n-drawer>

    <!-- 部署 -->
    <n-modal v-model:show="deployOpen" preset="card" :title="`部署 ${deploying?.name || ''}`" style="width: 560px">
      <n-form label-placement="top" size="small">
        <div class="form-grid">
          <n-form-item label="服务名称">
            <n-input v-model:value="deployForm.name" />
          </n-form-item>
          <n-form-item label="网关模型别名">
            <n-input v-model:value="deployForm.model_name" />
          </n-form-item>
        </div>
        <n-form-item label="推理引擎">
          <n-select v-model:value="deployForm.runtime" :options="[
            { label: 'llama.cpp（GGUF，CPU 可用）', value: 'llamacpp' },
            { label: 'vLLM（需要 NVIDIA GPU）', value: 'vllm' },
            { label: 'Ollama', value: 'ollama' },
          ]" />
        </n-form-item>
        <div class="form-grid">
          <n-form-item label="CPU 核数"><n-input-number v-model:value="deployForm.cpus" :min="1" style="width: 100%" /></n-form-item>
          <n-form-item label="内存 (MB)"><n-input-number v-model:value="deployForm.memory_mb" :min="512" :step="1024" style="width: 100%" /></n-form-item>
        </div>
        <n-form-item :label="deployForm.runtime === 'vllm' ? 'GPU 数量（vLLM 必填）' : 'GPU 数量（可选，CUDA 版 llama.cpp）'">
          <n-input-number v-model:value="deployForm.gpu_count" :min="0" :max="8" style="width: 100%" />
        </n-form-item>
        <n-checkbox v-model:checked="deployForm.host_access">授予宿主机权限（不推荐用于模型推理）</n-checkbox>
      </n-form>
      <div class="note" style="margin-top: 12px">
        部署将创建 {{ deployForm.runtime === 'vllm' ? 'vllm/vllm-openai' : deployForm.runtime === 'ollama' ? 'ollama/ollama' : 'llama.cpp server' }} 容器，
        挂载 {{ deploying?.host_path || '模型数据卷' }}，启动后通过 AI 网关以 <code>{{ deployForm.model_name }}</code> 调用。
      </div>
      <template #footer>
        <div class="row gap-12" style="justify-content: flex-end">
          <n-button quaternary @click="deployOpen = false">取消</n-button>
          <n-button type="primary" :loading="deploySaving" @click="submitDeploy">部署</n-button>
        </div>
      </template>
    </n-modal>
  </div>
</template>

<style scoped>
.source-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(300px, 1fr)); gap: 12px; margin-bottom: 20px; }
.source-card { display: flex; align-items: flex-start; gap: 12px; padding: 15px 16px; cursor: pointer; text-align: left; color: var(--text); font-family: inherit; transition: all 0.16s; }
.source-card:hover { border-color: rgba(59,130,246,0.4); transform: translateY(-2px); }
.src-icon { width: 38px; height: 38px; border-radius: 11px; display: grid; place-items: center; color: #c4b5fd; background: rgba(59,130,246,0.12); border: 1px solid rgba(59,130,246,0.25); flex: none; }
.src-main { flex: 1; min-width: 0; }
.src-name { font-size: 13px; font-weight: 650; display: flex; align-items: center; gap: 8px; }
.src-badge { font-size: 10px; color: #6ee7b7; border: 1px solid rgba(52,211,153,0.3); background: rgba(52,211,153,0.1); padding: 1px 6px; border-radius: 999px; }
.src-desc { font-size: 11px; color: #8b98b8; line-height: 1.65; margin-top: 4px; }
.src-arrow { color: #4b5a7a; margin-top: 4px; }

.asset-list { display: flex; flex-direction: column; gap: 12px; }
.asset { padding: 16px 18px; display: flex; flex-direction: column; gap: 11px; }
.asset-head { display: flex; align-items: center; gap: 12px; }
.asset-icon { width: 40px; height: 40px; border-radius: 12px; display: grid; place-items: center; background: rgba(255,255,255,0.04); border: 1px solid var(--line); color: #93a2c4; }
.asset-icon.ok { color: #6ee7b7; background: rgba(52,211,153,0.1); border-color: rgba(52,211,153,0.25); }
.asset-icon.bad { color: #fda4af; background: rgba(251,113,133,0.1); border-color: rgba(251,113,133,0.25); }
.asset-icon.warn { color: #fcd34d; background: rgba(251,191,36,0.1); border-color: rgba(251,191,36,0.25); }
.asset-title { flex: 1; min-width: 0; }
.asset-name { font-size: 14px; font-weight: 680; }
.asset-sub { display: flex; align-items: center; gap: 8px; margin-top: 4px; font-size: 11px; color: var(--muted); flex-wrap: wrap; }
.asset-meta { display: grid; grid-template-columns: 90px 80px 1fr 110px; gap: 12px; font-size: 11px; }
.asset-meta div { display: flex; flex-direction: column; gap: 2px; min-width: 0; }
.asset-meta span { color: #5d6a89; }
.asset-meta b { font-weight: 550; color: #b9c5dd; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.asset-gguf { font-size: 11px; color: #a5f3fc; background: rgba(20,184,166,0.06); border: 1px solid rgba(20,184,166,0.15); border-radius: 8px; padding: 6px 9px; display: flex; align-items: center; gap: 6px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.asset-error { font-size: 11px; color: #fda4af; background: rgba(251,113,133,0.07); border: 1px solid rgba(251,113,133,0.18); border-radius: 8px; padding: 7px 9px; }
.asset-foot { display: flex; align-items: center; gap: 6px; border-top: 1px solid rgba(255,255,255,0.05); padding-top: 10px; }

.src-tabs { display: flex; gap: 6px; flex-wrap: wrap; margin-bottom: 16px; }
.src-tabs button { display: inline-flex; align-items: center; gap: 6px; font-size: 11.5px; padding: 6px 12px; border-radius: 9px; border: 1px solid var(--line); background: rgba(255,255,255,0.03); color: var(--muted); cursor: pointer; font-family: inherit; }
.src-tabs button.on { color: #ede9fe; background: rgba(59,130,246,0.2); border-color: rgba(59,130,246,0.45); }
.form-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 12px; }
.note { font-size: 11px; color: #6e7c9c; line-height: 1.7; }
.note code { color: #a5f3fc; }

.log-panel { display: flex; flex-direction: column; height: 100%; }
.log-head { display: flex; align-items: flex-start; justify-content: space-between; padding: 20px 22px 14px; border-bottom: 1px solid var(--line); }
.log-title { font-size: 15px; font-weight: 650; margin-bottom: 4px; }
.hub-log { flex: 1; margin: 0; overflow: auto; background: #05080f; color: #b9c5dd; font-family: var(--mono); font-size: 11.5px; line-height: 1.7; padding: 16px 18px; white-space: pre-wrap; word-break: break-all; }
.log-foot { padding: 12px 18px; border-top: 1px solid var(--line); display: flex; justify-content: flex-end; }
@media (max-width: 900px) {
  .asset-meta { grid-template-columns: 1fr 1fr; }
  .form-grid { grid-template-columns: 1fr; }
}
</style>
