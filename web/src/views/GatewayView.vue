<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useMessage, useDialog } from 'naive-ui'
import {
  KeyOutline, CopyOutline, AddOutline, TrashOutline, CloudOutline, CubeOutline,
  AnalyticsOutline, TerminalOutline, LayersOutline,
} from '@vicons/ionicons5'
import { gatewayApi } from '@/api'
import type { APIKey } from '@/types'
import { copyText, fmtTime, fmtTime as ft, timeAgo } from '@/utils/format'
import TrendChart from '@/components/TrendChart.vue'

const router = useRouter()
const message = useMessage()
const dialog = useDialog()

const info = ref<any>(null)
const keys = ref<APIKey[]>([])
const loading = ref(false)
const createOpen = ref(false)
const form = ref({ name: '', token_limit: 0 })
const creating = ref(false)
const secretOpen = ref(false)
const newSecret = ref('')
const snippet = ref('curl')

const labels = computed(() => (info.value?.series || []).map((p: any) => {
  const d = new Date(p.ts * 1000)
  return `${String(d.getHours()).padStart(2, '0')}:00`
}))
const tokenSeries = computed(() => [
  { name: 'Token', values: (info.value?.series || []).map((p: any) => p.tokens), color: '#3b82f6' },
])
const requestSeries = computed(() => [
  { name: '请求数', values: (info.value?.series || []).map((p: any) => p.requests), color: '#14b8a6' },
])
const baseURL = computed(() => info.value?.base_url || `${window.location.origin}/v1`)

const snippets = computed<Record<string, string>>(() => ({
  curl: `curl ${baseURL.value}/chat/completions \\
  -H "Content-Type: application/json" \\
  -H "Authorization: Bearer <你的API_KEY>" \\
  -d '{
    "model": "${info.value?.models?.[0]?.id || 'qwen2.5-7b'}",
    "messages": [{"role": "user", "content": "你好"}],
    "stream": true
  }'`,
  python: `from openai import OpenAI

client = OpenAI(
    base_url="${baseURL.value}",
    api_key="<你的API_KEY>",
)

resp = client.chat.completions.create(
    model="${info.value?.models?.[0]?.id || 'qwen2.5-7b'}",
    messages=[{"role": "user", "content": "你好"}],
)
print(resp.choices[0].message.content)`,
  node: `import OpenAI from "openai";

const client = new OpenAI({
  baseURL: "${baseURL.value}",
  apiKey: "<你的API_KEY>",
});

const chat = await client.chat.completions.create({
  model: "${info.value?.models?.[0]?.id || 'qwen2.5-7b'}",
  messages: [{ role: "user", content: "你好" }],
});
console.log(chat.choices[0].message.content);`,
}))

async function load() {
  loading.value = true
  try {
    const [g, k] = await Promise.all([gatewayApi.info(), gatewayApi.keys()])
    info.value = g
    keys.value = k.keys || []
  } catch (e: any) {
    message.error(e.message)
  } finally {
    loading.value = false
  }
}

async function createKey() {
  if (!form.value.name.trim()) {
    message.warning('请输入密钥名称')
    return
  }
  creating.value = true
  try {
    const res = await gatewayApi.createKey({ name: form.value.name.trim(), token_limit: form.value.token_limit })
    newSecret.value = res.secret
    createOpen.value = false
    secretOpen.value = true
    form.value = { name: '', token_limit: 0 }
    await load()
  } catch (e: any) {
    message.error(e.message)
  } finally {
    creating.value = false
  }
}

function removeKey(k: APIKey) {
  dialog.warning({
    title: '删除 API Key',
    content: `删除后使用该密钥的客户端将立即失效：${k.name}`,
    positiveText: '删除',
    negativeText: '取消',
    onPositiveClick: async () => {
      try {
        await gatewayApi.deleteKey(k.id)
        message.success('已删除')
        load()
      } catch (e: any) {
        message.error(e.message)
      }
    },
  })
}

async function copy(v: string) {
  await copyText(v)
  message.success('已复制')
}

onMounted(load)
</script>

<template>
  <div class="page">
    <div class="page-head">
      <div>
        <h1 class="page-title">AI 网关</h1>
        <p class="page-sub">统一 OpenAI 兼容入口：模型路由、API Key、Token 计量与配额。任何已部署模型都会自动出现在这里。</p>
      </div>
      <div class="row gap-8">
        <n-button size="small" quaternary :loading="loading" @click="load">刷新</n-button>
        <n-button size="small" type="primary" @click="createOpen = true">
          <template #icon><n-icon :component="AddOutline" /></template>新建 API Key
        </n-button>
      </div>
    </div>

    <div class="hero glass">
      <div class="hero-left">
        <div class="hero-label">BASE URL</div>
        <div class="hero-url mono">{{ baseURL }}</div>
        <div class="hero-sub">OpenAI 兼容 · 自动路由到运行中的模型服务 · 支持流式输出</div>
        <div class="row gap-8" style="margin-top: 14px">
          <n-button size="small" @click="copy(baseURL)">
            <template #icon><n-icon :component="CopyOutline" /></template>复制地址
          </n-button>
          <n-button size="small" quaternary @click="copy(snippets[snippet])">
            <template #icon><n-icon :component="TerminalOutline" /></template>复制示例
          </n-button>
        </div>
      </div>
      <div class="hero-stats">
        <div class="hs"><span>运行中模型</span><b>{{ info?.models?.length || 0 }}</b></div>
        <div class="hs"><span>今日调用</span><b>{{ info?.usage?.today?.requests || 0 }}</b></div>
        <div class="hs"><span>今日 Token</span><b>{{ (info?.usage?.today?.total_tokens || 0).toLocaleString() }}</b></div>
        <div class="hs"><span>API Key</span><b>{{ keys.length }}</b></div>
      </div>
    </div>

    <div class="split-2" style="margin-top: 16px">
      <div class="glass panel">
        <div class="panel-title"><n-icon :component="AnalyticsOutline" :size="15" /> 24 小时用量</div>
        <TrendChart :labels="labels" :series="tokenSeries" height="180px" />
        <TrendChart :labels="labels" :series="requestSeries" height="120px" :digits="0" />
      </div>

      <div class="glass panel">
        <div class="panel-title"><n-icon :component="CubeOutline" :size="15" /> 模型路由</div>
        <div v-if="info?.models?.length" class="routing">
          <div v-for="m in info.models" :key="m.id" class="route-row">
            <div class="route-icon"><n-icon :component="LayersOutline" :size="15" /></div>
            <div class="route-main">
              <div class="route-name mono">{{ m.id }}</div>
              <div class="tiny muted">owned_by {{ m.owned_by }}</div>
            </div>
            <span class="badge ok">{{ m.status === 'running' ? '已就绪' : m.status }}</span>
          </div>
        </div>
        <div v-else class="placeholder-panel" style="min-height: 160px">
          <n-icon :component="CloudOutline" :size="26" style="color: #4b5a7a" />
          <div>还没有运行中的模型</div>
          <n-button size="tiny" type="primary" @click="router.push('/models')">去部署</n-button>
        </div>
      </div>
    </div>

    <div class="glass panel" style="margin-top: 16px">
      <div class="panel-title"><n-icon :component="TerminalOutline" :size="15" /> 接入示例</div>
      <div class="snippet-tabs">
        <button v-for="(_, k) in snippets" :key="k" :class="{ on: snippet === k }" @click="snippet = k">{{ k }}</button>
      </div>
      <pre class="code-block">{{ snippets[snippet] }}</pre>
    </div>

    <div class="glass panel" style="margin-top: 16px">
      <div class="panel-head">
        <div class="panel-title" style="margin: 0"><n-icon :component="KeyOutline" :size="15" /> API Key</div>
        <n-button size="tiny" tertiary @click="createOpen = true">新建</n-button>
      </div>
      <table class="tbl">
        <thead><tr><th>名称</th><th>前缀</th><th>配额</th><th>已用 Token</th><th>调用次数</th><th>最近使用</th><th>创建时间</th><th></th></tr></thead>
        <tbody>
          <tr v-for="k in keys" :key="k.id">
            <td><b>{{ k.name }}</b></td>
            <td class="mono muted">{{ k.prefix }}…</td>
            <td>{{ k.token_limit ? k.token_limit.toLocaleString() : '不限' }}</td>
            <td>{{ k.tokens_used.toLocaleString() }}</td>
            <td>{{ k.requests_used }}</td>
            <td class="muted">{{ k.last_used_at ? timeAgo(k.last_used_at) : '从未' }}</td>
            <td class="muted">{{ fmtTime(k.created_at) }}</td>
            <td style="text-align:right">
              <n-button size="tiny" quaternary type="error" @click="removeKey(k)">
                <template #icon><n-icon :component="TrashOutline" /></template>
              </n-button>
            </td>
          </tr>
          <tr v-if="!keys.length"><td colspan="8" class="muted" style="text-align:center; padding: 26px">还没有 API Key，新建一个开始调用</td></tr>
        </tbody>
      </table>
    </div>

    <div v-if="info?.by_model?.length" class="glass panel" style="margin-top: 16px">
      <div class="panel-title">按模型统计</div>
      <div class="model-usage">
        <div v-for="m in info.by_model" :key="m.model" class="mu-row">
          <span class="mono">{{ m.model }}</span>
          <div class="mu-bar"><i :style="{ width: Math.min(100, (m.tokens / Math.max(1, info.by_model[0]?.tokens || 1)) * 100) + '%' }" /></div>
          <span class="muted">{{ m.requests }} 次 · {{ m.tokens.toLocaleString() }} tokens</span>
        </div>
      </div>
    </div>

    <n-modal v-model:show="createOpen" preset="card" title="新建 API Key" style="width: 460px">
      <n-form label-placement="top">
        <n-form-item label="名称"><n-input v-model:value="form.name" placeholder="例如 team-a / ci-pipeline" /></n-form-item>
        <n-form-item label="Token 配额（0 表示不限）">
          <n-input-number v-model:value="form.token_limit" :min="0" :step="100000" style="width: 100%" />
        </n-form-item>
      </n-form>
      <template #footer>
        <div class="row gap-12" style="justify-content: flex-end">
          <n-button quaternary @click="createOpen = false">取消</n-button>
          <n-button type="primary" :loading="creating" @click="createKey">创建</n-button>
        </div>
      </template>
    </n-modal>

    <n-modal v-model:show="secretOpen" preset="card" title="API Key 已创建" style="width: 560px">
      <div class="warn-box">密钥只会展示这一次，请立即复制保存。</div>
      <div class="secret mono">
        <span>{{ newSecret }}</span>
        <n-button size="tiny" tertiary @click="copy(newSecret)">复制</n-button>
      </div>
      <template #footer>
        <n-button type="primary" @click="secretOpen = false">我已保存</n-button>
      </template>
    </n-modal>
  </div>
</template>

<style scoped>
.hero { display: flex; gap: 24px; align-items: center; padding: 24px 26px; position: relative; overflow: hidden; }
.hero-left { flex: 1; position: relative; }
.hero-label { font-size: 10px; color: var(--muted); letter-spacing: 1.4px; margin-bottom: 8px; }
.hero-url { font-size: 20px; color: #a5f3fc; letter-spacing: -0.2px; }
.hero-sub { font-size: 11.5px; color: var(--muted); margin-top: 8px; }
.hero-stats { display: flex; gap: 30px; position: relative; }
.hs { text-align: right; }
.hs span { display: block; font-size: 10.5px; color: var(--muted); }
.hs b { font-size: 22px; font-weight: 730; letter-spacing: -0.5px; }

.panel { padding: 16px 18px; }
.panel-title { display: flex; align-items: center; gap: 7px; font-size: 12.5px; font-weight: 650; color: #b9c5dd; margin-bottom: 12px; }
.panel-head { display: flex; align-items: center; justify-content: space-between; margin-bottom: 8px; }
.routing { display: flex; flex-direction: column; gap: 8px; }
.route-row { display: flex; align-items: center; gap: 11px; padding: 9px 11px; border: 1px solid var(--line); border-radius: 10px; background: rgba(255,255,255,0.025); }
.route-icon { width: 30px; height: 30px; border-radius: 8px; display: grid; place-items: center; color: #a5b4fc; background: rgba(59,130,246,0.12); }
.route-main { flex: 1; min-width: 0; }
.route-name { font-size: 12px; font-weight: 600; }
.snippet-tabs { display: flex; gap: 4px; margin-bottom: 10px; }
.snippet-tabs button { background: rgba(255,255,255,0.03); border: 1px solid var(--line); color: var(--muted); font-size: 11px; padding: 4px 11px; border-radius: 7px; cursor: pointer; font-family: inherit; }
.snippet-tabs button.on { color: #ddd6fe; background: rgba(59,130,246,0.2); border-color: rgba(59,130,246,0.4); }
.tbl { width: 100%; border-collapse: collapse; font-size: 11.5px; }
.tbl th { text-align: left; font-size: 10px; color: #7d8bab; padding: 9px 10px; border-bottom: 1px solid var(--line); font-weight: 600; white-space: nowrap; }
.tbl td { padding: 10px; border-bottom: 1px solid rgba(255,255,255,0.04); color: #c3cee6; }
.model-usage { display: flex; flex-direction: column; gap: 9px; }
.mu-row { display: grid; grid-template-columns: 170px 1fr 190px; align-items: center; gap: 12px; font-size: 11.5px; }
.mu-bar { height: 6px; border-radius: 99px; background: rgba(255,255,255,0.05); overflow: hidden; }
.mu-bar i { display: block; height: 100%; background: #3b82f6; border-radius: 99px; }
.warn-box { font-size: 11.5px; color: #fcd34d; background: rgba(251,191,36,0.08); border: 1px solid rgba(251,191,36,0.25); border-radius: 9px; padding: 9px 12px; margin-bottom: 12px; }
.secret { display: flex; align-items: center; justify-content: space-between; gap: 10px; background: #05080f; border: 1px solid var(--line); border-radius: 10px; padding: 12px 14px; font-size: 12px; color: #a5f3fc; word-break: break-all; }
</style>
