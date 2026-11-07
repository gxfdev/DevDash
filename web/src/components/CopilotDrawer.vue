<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import { useMessage } from 'naive-ui'
import { ChatbubbleEllipsesOutline, SendOutline, SparklesOutline, ConstructOutline } from '@vicons/ionicons5'
import { copilotApi, serviceApi } from '@/api'
import type { CopilotStep, Service } from '@/types'

const props = defineProps<{ show: boolean }>()
const emit = defineEmits<{ (e: 'update:show', v: boolean): void }>()

interface Msg {
  role: 'user' | 'assistant'
  content: string
  steps?: CopilotStep[]
  error?: string
}

const message = useMessage()
const listEl = ref<HTMLDivElement>()
const input = ref('')
const loading = ref(false)
const allowExec = ref(false)
const model = ref('')
const sandboxId = ref('')
const messages = ref<Msg[]>([
  { role: 'assistant', content: '你好，我是 AIDash 运维助手。我可以读取实时指标、服务状态、容器日志，并在授权沙箱中执行诊断命令。\n\n试着问我：这台服务器的负载瓶颈在哪里？' },
])
const services = ref<Service[]>([])

const modelOptions = computed(() =>
  services.value
    .filter((s) => s.kind === 'model' && s.status === 'running')
    .sort((a, b) => (b.health === 'healthy' ? 1 : 0) - (a.health === 'healthy' ? 1 : 0))
    .map((s) => ({
      label: `${s.model_name || s.name}${s.health === 'healthy' ? '' : '（健康未知）'}`,
      value: s.model_name || s.name,
    })),
)
const sandboxOptions = computed(() =>
  services.value.filter((s) => s.spec?.host_access?.enabled && s.status === 'running')
    .map((s) => ({ label: `${s.name}（宿主机权限）`, value: s.id })),
)

async function loadServices() {
  try {
    const res = await serviceApi.list()
    services.value = res.services || []
    if (!model.value && modelOptions.value.length) {
      const healthy = services.value.find((s) => s.kind === 'model' && s.status === 'running' && s.health === 'healthy')
      model.value = healthy ? (healthy.model_name || healthy.name) : modelOptions.value[0].value
    }
    if (!sandboxId.value && sandboxOptions.value.length) sandboxId.value = sandboxOptions.value[0].value
  } catch { /* ignore */ }
}

watch(() => props.show, (v) => {
  if (v) {
    loadServices()
    nextTick(scrollBottom)
  }
})

function scrollBottom() {
  nextTick(() => {
    if (listEl.value) listEl.value.scrollTop = listEl.value.scrollHeight
  })
}

async function send() {
  const text = input.value.trim()
  if (!text || loading.value) return
  if (!modelOptions.value.length) {
    message.warning('还没有运行中的模型服务，请先在「模型广场」一键部署模型')
    return
  }
  messages.value.push({ role: 'user', content: text })
  input.value = ''
  loading.value = true
  scrollBottom()
  try {
    const history = messages.value
      .filter((m) => m.role === 'user' || (m.role === 'assistant' && m.content))
      .slice(-10)
      .map((m) => ({ role: m.role, content: m.content }))
    const resp = await copilotApi.chat({
      model: model.value,
      messages: history,
      allow_exec: allowExec.value,
      sandbox_id: sandboxId.value || undefined,
    })
    messages.value.push({
      role: 'assistant',
      content: resp.reply || resp.error || '模型没有返回内容',
      steps: resp.steps,
      error: resp.error,
    })
  } catch (e: any) {
    messages.value.push({ role: 'assistant', content: e.message, error: e.message })
  } finally {
    loading.value = false
    scrollBottom()
  }
}
</script>

<template>
  <n-drawer :show="show" :width="540" placement="right" @update:show="emit('update:show', $event)">
    <div class="copilot">
      <header class="cp-head">
        <div class="cp-avatar"><n-icon :component="SparklesOutline" :size="19" /></div>
        <div>
          <div class="cp-title">AI 运维助手</div>
          <div class="cp-sub">基于已部署模型 · 可读写平台状态</div>
        </div>
        <div class="spacer" />
        <n-button quaternary circle size="small" @click="emit('update:show', false)">✕</n-button>
      </header>

      <div class="cp-controls">
        <div class="cp-field">
          <label>模型</label>
          <n-select v-model:value="model" size="small" :options="modelOptions" placeholder="暂无可用模型" />
        </div>
        <div class="cp-field">
          <label>命令沙箱</label>
          <n-select v-model:value="sandboxId" size="small" :options="sandboxOptions" placeholder="无宿主机权限服务" clearable />
        </div>
        <div class="cp-switch">
          <n-switch v-model:value="allowExec" size="small" />
          <span>允许执行命令</span>
        </div>
      </div>

      <div ref="listEl" class="cp-list">
        <div v-for="(m, i) in messages" :key="i" class="cp-msg" :class="m.role">
          <div class="bubble">
            <div class="content">{{ m.content }}</div>
            <div v-if="m.steps && m.steps.length" class="steps">
              <div class="steps-title"><n-icon :component="ConstructOutline" :size="13" /> 工具调用 {{ m.steps.length }} 次</div>
              <div v-for="(s, j) in m.steps" :key="j" class="step">
                <div class="step-head"><code>{{ s.tool }}</code></div>
                <pre class="step-body">{{ s.result }}</pre>
              </div>
            </div>
          </div>
        </div>
        <div v-if="loading" class="cp-msg assistant">
          <div class="bubble typing"><span /><span /><span /></div>
        </div>
      </div>

      <footer class="cp-input">
        <n-input
          v-model:value="input"
          type="textarea"
          :autosize="{ minRows: 2, maxRows: 5 }"
          placeholder="描述你的问题，例如：查看最近 1 小时主机负载，并诊断哪个容器最占内存"
          @keydown.enter.exact.prevent="send"
        />
        <n-button type="primary" :loading="loading" :disabled="!input.trim()" @click="send">
          <template #icon><n-icon :component="SendOutline" /></template>
          发送
        </n-button>
      </footer>
    </div>
  </n-drawer>
</template>

<style scoped>
.copilot { display: flex; flex-direction: column; height: 100%; }
.cp-head { display: flex; align-items: center; gap: 12px; padding: 20px 20px 14px; border-bottom: 1px solid var(--line); }
.cp-avatar {
  width: 40px; height: 40px; border-radius: 12px; display: grid; place-items: center; color: #fff;
  background: #2563eb; box-shadow: 0 8px 20px rgba(59,130,246,0.35);
}
.cp-title { font-weight: 700; font-size: 15px; }
.cp-sub { font-size: 11.5px; color: var(--muted); margin-top: 2px; }
.cp-controls { display: grid; grid-template-columns: 1fr 1fr auto; gap: 10px; padding: 12px 20px; border-bottom: 1px solid var(--line); align-items: end; }
.cp-field label { display: block; font-size: 10.5px; color: var(--muted); margin-bottom: 4px; }
.cp-switch { display: flex; align-items: center; gap: 7px; font-size: 11.5px; color: var(--text-dim); padding-bottom: 4px; white-space: nowrap; }
.cp-list { flex: 1; overflow-y: auto; padding: 18px 20px; display: flex; flex-direction: column; gap: 14px; }
.cp-msg { display: flex; }
.cp-msg.user { justify-content: flex-end; }
.bubble { max-width: 92%; border-radius: 14px; padding: 11px 14px; font-size: 13px; line-height: 1.7; white-space: pre-wrap; word-break: break-word; }
.cp-msg.user .bubble { background: #2563eb; color: #fff; border-bottom-right-radius: 4px; }
.cp-msg.assistant .bubble { background: rgba(255,255,255,0.045); border: 1px solid var(--line); border-bottom-left-radius: 4px; }
.steps { margin-top: 10px; border-top: 1px dashed rgba(255,255,255,0.1); padding-top: 8px; }
.steps-title { display: flex; align-items: center; gap: 5px; font-size: 11px; color: var(--muted); margin-bottom: 6px; }
.step { background: rgba(0,0,0,0.25); border: 1px solid var(--line); border-radius: 8px; padding: 7px 9px; margin-bottom: 6px; }
.step-head code { font-size: 11px; color: #a5b4fc; }
.step-body { margin: 5px 0 0; font-size: 11px; color: #9aa7c4; max-height: 130px; overflow: auto; white-space: pre-wrap; font-family: var(--mono); }
.typing { display: flex; gap: 4px; align-items: center; }
.typing span { width: 6px; height: 6px; border-radius: 50%; background: #8b98b8; animation: blink 1.2s infinite; }
.typing span:nth-child(2) { animation-delay: 0.15s; }
.typing span:nth-child(3) { animation-delay: 0.3s; }
@keyframes blink { 0%, 100% { opacity: 0.25; } 50% { opacity: 1; } }
.cp-input { padding: 12px 20px 20px; border-top: 1px solid var(--line); display: flex; gap: 10px; align-items: flex-end; }
</style>
