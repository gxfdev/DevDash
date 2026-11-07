<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import { useMessage, useDialog } from 'naive-ui'
import {
  PlayOutline, StopOutline, RefreshOutline, TrashOutline, CopyOutline, PulseOutline,
  TerminalOutline, DocumentTextOutline, ChatbubbleEllipsesOutline, CloudUploadOutline,
  ShieldCheckmarkOutline, HardwareChipOutline,
} from '@vicons/ionicons5'
import { serviceApi, copilotApi } from '@/api'
import type { ContainerStats, Service, ServiceEvent } from '@/types'
import { statusOf, fmtTime, timeAgo, copyText, fmtMB, fmtPct, shortId } from '@/utils/format'
import TerminalPane from '@/components/TerminalPane.vue'

const props = defineProps<{ show: boolean; serviceId: string | null }>()
const emit = defineEmits<{ (e: 'update:show', v: boolean): void; (e: 'changed'): void }>()

const message = useMessage()
const dialog = useDialog()

const service = ref<Service | null>(null)
const events = ref<ServiceEvent[]>([])
const stats = ref<ContainerStats | null>(null)
const logs = ref('')
const tab = ref('overview')
const busy = ref(false)
const logsLoading = ref(false)
const autoLog = ref(true)
const command = ref('uname -a && df -h / | tail -2')
const commandOut = ref('')
const runningCmd = ref(false)

const chatMessages = ref<{ role: string; content: string }[]>([])
const chatInput = ref('')
const chatting = ref(false)

let logTimer: number | undefined
let statsTimer: number | undefined
let eventTimer: number | undefined
let statusTimer: number | undefined

const status = computed(() => (service.value ? statusOf(service.value.status) : null))
const isModel = computed(() => service.value?.kind === 'model')
const canTerminal = computed(() => service.value?.status === 'running')
const modelName = computed(() => service.value?.model_name || service.value?.name || '')

async function loadDetail() {
  if (!props.serviceId) return
  try {
    const res = await serviceApi.get(props.serviceId)
    service.value = res.service
    events.value = res.events || []
    stats.value = res.stats || null
  } catch (e: any) {
    message.error(e.message)
    emit('update:show', false)
  }
}

async function loadLogs() {
  if (!props.serviceId) return
  logsLoading.value = true
  try {
    const res = await serviceApi.logs(props.serviceId, 400)
    logs.value = res.logs || '（暂无日志）'
  } catch (e: any) {
    logs.value = `获取日志失败：${e.message}`
  } finally {
    logsLoading.value = false
  }
}

function startTimers() {
  stopTimers()
  logTimer = window.setInterval(() => { if (autoLog.value && tab.value === 'logs') loadLogs() }, 5000)
  statsTimer = window.setInterval(async () => {
    if (tab.value !== 'overview' || !props.serviceId) return
    try { stats.value = await serviceApi.stats(props.serviceId) } catch { /* ignore */ }
  }, 5000)
  eventTimer = window.setInterval(async () => {
    if (!props.serviceId) return
    try {
      const res = await serviceApi.events(props.serviceId)
      events.value = res.events || []
    } catch { /* ignore */ }
  }, 6000)
  statusTimer = window.setInterval(() => {
    if (props.show && props.serviceId) loadDetail()
  }, 4000)
}
function stopTimers() {
  if (logTimer) window.clearInterval(logTimer)
  if (statsTimer) window.clearInterval(statsTimer)
  if (eventTimer) window.clearInterval(eventTimer)
  if (statusTimer) window.clearInterval(statusTimer)
}

watch(() => props.show, async (v) => {
  if (v) {
    tab.value = 'overview'
    service.value = null
    logs.value = ''
    commandOut.value = ''
    chatMessages.value = []
    await loadDetail()
    startTimers()
  } else {
    stopTimers()
  }
})
watch(() => props.serviceId, async (v) => {
  if (props.show && v) {
    await loadDetail()
  }
})
watch(tab, (t) => {
  if (t === 'logs' && !logs.value) loadLogs()
})

async function doAction(action: string) {
  if (!service.value) return
  busy.value = true
  try {
    await serviceApi.action(service.value.id, action)
    message.success('操作已提交')
    await loadDetail()
    emit('changed')
  } catch (e: any) {
    message.error(e.message)
  } finally {
    busy.value = false
  }
}

function confirmRemove() {
  if (!service.value) return
  dialog.warning({
    title: '移除服务',
    content: `确定要移除「${service.value.name}」吗？容器将被强制删除，命名卷默认保留。`,
    positiveText: '确认移除',
    negativeText: '取消',
    onPositiveClick: async () => {
      try {
        await serviceApi.remove(service.value!.id, true)
        message.success('服务已移除')
        emit('update:show', false)
        emit('changed')
      } catch (e: any) {
        message.error(e.message)
      }
    },
  })
}

async function runCommand() {
  if (!service.value || !command.value.trim()) return
  runningCmd.value = true
  commandOut.value = ''
  try {
    const res = await serviceApi.exec(service.value.id, command.value, 30)
    commandOut.value = res.output || '（无输出）'
    if (res.error) commandOut.value += `\n[错误] ${res.error}`
  } catch (e: any) {
    commandOut.value = `[错误] ${e.message}`
  } finally {
    runningCmd.value = false
  }
}

async function sendChat() {
  const text = chatInput.value.trim()
  if (!text || chatting.value || !service.value) return
  const list = [...chatMessages.value, { role: 'user', content: text }]
  chatMessages.value = list
  chatInput.value = ''
  chatting.value = true
  try {
    const res = await copilotApi.playground({ model: modelName.value, messages: list.map((m) => ({ role: m.role, content: m.content })) })
    const content = res?.choices?.[0]?.message?.content || res?.error?.message || '模型未返回内容'
    chatMessages.value = [...list, { role: 'assistant', content }]
    nextTick(() => {
      const el = document.querySelector('.chat-list')
      if (el) el.scrollTop = el.scrollHeight
    })
  } catch (e: any) {
    chatMessages.value = [...list, { role: 'assistant', content: `调用失败：${e.message}` }]
  } finally {
    chatting.value = false
  }
}

function publicUrl(ep: string) {
  return ep.replace('127.0.0.1', window.location.hostname)
}

async function copy(v: string) {
  await copyText(v)
  message.success('已复制')
}
</script>

<template>
  <n-drawer :show="show" :width="760" placement="right" @update:show="emit('update:show', $event)">
    <div v-if="service" class="sd">
      <header class="sd-head">
        <div class="sd-icon" :class="status?.cls">
          <n-icon :component="isModel ? HardwareChipOutline : CloudUploadOutline" :size="21" />
        </div>
        <div style="flex: 1; min-width: 0">
          <div class="sd-title">
            {{ service.name }}
            <span class="badge" :class="status?.badge" style="margin-left: 8px">{{ status?.label }}</span>
            <span v-if="service.host_access" class="badge warn" style="margin-left: 6px">
              <n-icon :component="ShieldCheckmarkOutline" :size="11" /> 宿主机权限
            </span>
          </div>
          <div class="sd-sub">{{ service.image }} · 容器 {{ shortId(service.container_id) }} · 由 {{ service.owner }} 部署</div>
        </div>
        <n-button-group size="small">
          <n-button v-if="service.status !== 'running'" :loading="busy" @click="doAction('start')">
            <template #icon><n-icon :component="PlayOutline" /></template>启动
          </n-button>
          <n-button v-else :loading="busy" @click="doAction('stop')">
            <template #icon><n-icon :component="StopOutline" /></template>停止
          </n-button>
          <n-button :loading="busy" @click="doAction('restart')">
            <template #icon><n-icon :component="RefreshOutline" /></template>重启
          </n-button>
          <n-button :loading="busy" @click="doAction('upgrade')">
            <template #icon><n-icon :component="CloudUploadOutline" /></template>升级
          </n-button>
          <n-button type="error" ghost @click="confirmRemove">
            <template #icon><n-icon :component="TrashOutline" /></template>
          </n-button>
        </n-button-group>
      </header>

      <n-tabs v-model:value="tab" type="line" animated class="sd-tabs">
        <n-tab-pane name="overview" tab="概览">
          <div class="ov-grid">
            <div class="ov-left">
              <div class="card soft">
                <div class="card-title">访问地址</div>
                <div v-if="service.endpoints?.length" class="ep-list">
                  <div v-for="ep in service.endpoints" :key="ep" class="ep">
                    <code>{{ publicUrl(ep) }}</code>
                    <n-button text size="tiny" @click="copy(publicUrl(ep))">
                      <n-icon :component="CopyOutline" />
                    </n-button>
                  </div>
                </div>
                <div v-else class="muted tiny">暂无已发布的访问地址</div>
                <div v-if="isModel" class="ep-note">
                  网关模型名 <code>{{ modelName }}</code> · 通过 <code>/v1/chat/completions</code> 调用
                </div>
              </div>

              <div class="card soft">
                <div class="card-title">实时资源</div>
                <div class="stat-line">
                  <span>CPU</span>
                  <div class="bar"><i :style="{ width: Math.min(100, stats?.cpu_percent || 0) + '%', background: 'linear-gradient(90deg,#7c5cff,#a78bfa)' }" /></div>
                  <b>{{ fmtPct(stats?.cpu_percent) }}</b>
                </div>
                <div class="stat-line">
                  <span>内存</span>
                  <div class="bar"><i :style="{ width: Math.min(100, stats?.memory_percent || 0) + '%', background: 'linear-gradient(90deg,#22d3ee,#34d399)' }" /></div>
                  <b>{{ fmtMB(stats?.memory_mb) }} / {{ fmtMB(stats?.memory_limit_mb) }}</b>
                </div>
                <div class="mini-grid">
                  <div><span>网络入/出</span><b>{{ fmtMB(stats?.net_rx_mb) }} / {{ fmtMB(stats?.net_tx_mb) }}</b></div>
                  <div><span>磁盘读/写</span><b>{{ fmtMB(stats?.blk_read_mb) }} / {{ fmtMB(stats?.blk_write_mb) }}</b></div>
                  <div><span>进程数</span><b>{{ stats?.pids || 0 }}</b></div>
                  <div><span>运行时长</span><b>{{ timeAgo(service.started_at) }}</b></div>
                </div>
              </div>

              <div class="card soft">
                <div class="card-title">部署配置</div>
                <div class="kv"><span class="k">运行框架</span><span class="v">{{ service.spec?.labels?.['aios.runtime'] || 'Docker' }}</span></div>
                <div class="kv"><span class="k">CPU / 内存</span><span class="v">{{ service.spec?.resources?.cpus }} 核 / {{ ((service.spec?.resources?.memory_mb || 0) / 1024).toFixed(0) }} GB</span></div>
                <div class="kv"><span class="k">GPU</span><span class="v">{{ service.spec?.resources?.gpu_count || 0 }} 张</span></div>
                <div class="kv"><span class="k">端口映射</span><span class="v">
                  <template v-if="service.spec?.ports?.length">
                    <span v-for="(p, i) in service.spec.ports" :key="p.container_port"><span v-if="i > 0"> · </span>{{ p.host_port }} → {{ p.container_port }}</span>
                  </template>
                  <template v-else>无</template>
                </span></div>
                <div class="kv"><span class="k">自动重启</span><span class="v">{{ service.spec?.restart || 'unless-stopped' }}</span></div>
                <div class="kv"><span class="k">创建时间</span><span class="v">{{ fmtTime(service.created_at) }}</span></div>
                <div v-if="service.error" class="err-box">{{ service.error }}</div>
              </div>
            </div>

            <div class="ov-right">
              <div class="card soft">
                <div class="card-title">事件时间线</div>
                <div class="timeline">
                  <div v-for="ev in events.slice(-12).reverse()" :key="ev.id" class="tl-item">
                    <span class="tl-dot" :class="ev.level" />
                    <div>
                      <div class="tl-msg">{{ ev.message }}</div>
                      <div class="tl-time">{{ timeAgo(ev.created_at) }}</div>
                    </div>
                  </div>
                  <div v-if="!events.length" class="muted tiny">暂无事件</div>
                </div>
              </div>
            </div>
          </div>
        </n-tab-pane>

        <n-tab-pane name="logs" tab="日志">
          <div class="row gap-12" style="margin-bottom: 10px">
            <n-button size="small" :loading="logsLoading" @click="loadLogs">
              <template #icon><n-icon :component="DocumentTextOutline" /></template>刷新
            </n-button>
            <n-checkbox v-model:checked="autoLog">自动刷新（5s）</n-checkbox>
            <div class="spacer" />
            <span class="muted tiny">最后 {{ logs.length }} 字符</span>
          </div>
          <pre class="log-view">{{ logs || '正在加载日志…' }}</pre>
        </n-tab-pane>

        <n-tab-pane name="events" tab="事件">
          <div class="timeline full">
            <div v-for="ev in events.slice().reverse()" :key="ev.id" class="tl-item">
              <span class="tl-dot" :class="ev.level" />
              <div>
                <div class="tl-msg">{{ ev.message }}</div>
                <div class="tl-time">{{ fmtTime(ev.created_at) }}</div>
              </div>
            </div>
          </div>
        </n-tab-pane>

        <n-tab-pane name="console" tab="命令控制台">
          <div class="console">
            <div class="muted tiny" style="margin-bottom: 8px">
              在容器 {{ shortId(service.container_id) }} 内执行一次性命令{{ service.host_access ? '（该容器具备宿主机权限，可操作宿主机）' : '' }}。
            </div>
            <div class="row gap-8">
              <n-input v-model:value="command" placeholder="输入 shell 命令，例如 docker ps / df -h / cat /host/etc/os-release" @keyup.enter="runCommand" />
              <n-button type="primary" :loading="runningCmd" @click="runCommand">执行</n-button>
            </div>
            <pre class="log-view" style="height: 320px; margin-top: 12px">{{ commandOut || '等待执行…' }}</pre>
          </div>
        </n-tab-pane>

        <n-tab-pane v-if="isModel" name="chat" tab="模型体验">
          <div class="chat">
            <div class="chat-list">
              <div v-if="!chatMessages.length" class="chat-empty">
                <n-icon :component="ChatbubbleEllipsesOutline" :size="30" style="color: #4b5a7a" />
                <div>直接与 <code>{{ modelName }}</code> 对话，验证部署是否可用。</div>
              </div>
              <div v-for="(m, i) in chatMessages" :key="i" class="chat-msg" :class="m.role">
                <div class="bubble">{{ m.content }}</div>
              </div>
            </div>
            <div class="row gap-8">
              <n-input v-model:value="chatInput" placeholder="输入消息…" @keyup.enter="sendChat" />
              <n-button type="primary" :loading="chatting" @click="sendChat">发送</n-button>
            </div>
          </div>
        </n-tab-pane>

        <n-tab-pane name="terminal" tab="容器终端" :disabled="!canTerminal">
          <TerminalPane
            v-if="show && tab === 'terminal' && canTerminal"
            :path="`/ws/services/${service.id}/terminal`"
            height="440px"
            :notice="service.host_access ? '该容器具备宿主机权限：/host 可访问宿主机根目录' : undefined"
          />
          <div v-else class="placeholder-panel">容器未运行，启动后可使用终端</div>
        </n-tab-pane>
      </n-tabs>
    </div>
  </n-drawer>
</template>

<style scoped>
.sd { display: flex; flex-direction: column; height: 100%; }
.sd-head { display: flex; align-items: center; gap: 14px; padding: 20px 24px 14px; border-bottom: 1px solid var(--line); flex-wrap: wrap; }
.sd-icon { width: 46px; height: 46px; border-radius: 13px; display: grid; place-items: center; flex: none; border: 1px solid var(--line); background: rgba(255,255,255,0.04); color: #c4b5fd; }
.sd-icon.running { color: #6ee7b7; border-color: rgba(52,211,153,0.3); background: rgba(52,211,153,0.1); }
.sd-icon.error { color: #fda4af; border-color: rgba(251,113,133,0.3); background: rgba(251,113,133,0.1); }
.sd-title { font-size: 17px; font-weight: 700; display: flex; align-items: center; flex-wrap: wrap; }
.sd-sub { font-size: 11px; color: var(--muted); margin-top: 4px; }
.sd-tabs { flex: 1; overflow: hidden; display: flex; flex-direction: column; padding: 0 24px; }
.sd-tabs :deep(.n-tabs-pane-wrapper) { flex: 1; overflow-y: auto; }
.sd-tabs :deep(.n-tab-pane) { padding: 16px 0 24px; }

.ov-grid { display: grid; grid-template-columns: 1.35fr 1fr; gap: 14px; }
.ov-left, .ov-right { display: flex; flex-direction: column; gap: 14px; }
.card { padding: 14px 16px; }
.card.soft { background: rgba(255,255,255,0.028); border: 1px solid var(--line); border-radius: 12px; }
.card-title { font-size: 11.5px; font-weight: 650; color: #9aa7c4; letter-spacing: 0.3px; margin-bottom: 10px; }
.ep { display: flex; align-items: center; gap: 8px; justify-content: space-between; padding: 6px 0; }
.ep code { font-family: var(--mono); font-size: 12px; color: #a5f3fc; }
.ep-note { margin-top: 8px; font-size: 11px; color: #6e7c9c; }
.ep-note code { color: #c4b5fd; }
.stat-line { display: grid; grid-template-columns: 44px 1fr 116px; align-items: center; gap: 10px; font-size: 11.5px; color: var(--muted); margin-bottom: 10px; }
.stat-line b { text-align: right; color: #c3cee6; font-weight: 600; }
.bar { height: 5px; border-radius: 99px; background: rgba(255,255,255,0.06); overflow: hidden; }
.bar i { display: block; height: 100%; border-radius: 99px; transition: width 0.4s ease; }
.mini-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 10px; margin-top: 6px; }
.mini-grid div { background: rgba(255,255,255,0.025); border: 1px solid var(--line); border-radius: 9px; padding: 8px 10px; }
.mini-grid span { display: block; font-size: 10px; color: var(--muted); }
.mini-grid b { font-size: 12px; font-weight: 620; }
.err-box { margin-top: 10px; font-size: 11.5px; color: #fda4af; background: rgba(251,113,133,0.08); border: 1px solid rgba(251,113,133,0.2); border-radius: 8px; padding: 8px 10px; word-break: break-all; }

.timeline { display: flex; flex-direction: column; gap: 12px; max-height: 340px; overflow-y: auto; }
.timeline.full { max-height: none; }
.tl-item { display: flex; gap: 10px; position: relative; }
.tl-dot { width: 7px; height: 7px; border-radius: 50%; margin-top: 5px; flex: none; background: #64748b; }
.tl-dot.success { background: #34d399; }
.tl-dot.error { background: #fb7185; }
.tl-dot.warn { background: #fbbf24; }
.tl-dot.info { background: #60a5fa; }
.tl-msg { font-size: 12px; color: #c3cee6; line-height: 1.6; word-break: break-word; }
.tl-time { font-size: 10px; color: #5d6a89; margin-top: 2px; }

.log-view {
  background: #05080f; border: 1px solid var(--line); border-radius: 12px; padding: 14px;
  font-family: var(--mono); font-size: 11.5px; line-height: 1.7; color: #b9c5dd;
  height: 460px; overflow: auto; white-space: pre-wrap; word-break: break-all; margin: 0;
}
.chat { display: flex; flex-direction: column; height: 460px; gap: 10px; }
.chat-list { flex: 1; overflow-y: auto; display: flex; flex-direction: column; gap: 10px; padding: 4px; }
.chat-empty { margin: auto; text-align: center; color: var(--muted); font-size: 12px; display: flex; flex-direction: column; gap: 10px; align-items: center; }
.chat-msg { display: flex; }
.chat-msg.user { justify-content: flex-end; }
.bubble { max-width: 82%; padding: 10px 13px; border-radius: 12px; font-size: 12.5px; line-height: 1.7; white-space: pre-wrap; }
.chat-msg.user .bubble { background: linear-gradient(135deg, rgba(124,92,255,0.9), rgba(91,111,240,0.85)); color: #fff; }
.chat-msg.assistant .bubble { background: rgba(255,255,255,0.045); border: 1px solid var(--line); }
</style>
