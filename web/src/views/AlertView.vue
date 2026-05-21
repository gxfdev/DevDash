<template>
  <app-layout>
    <div class="page">
      <div class="page-header">
        <h2>告警中心</h2>
        <n-space>
          <n-select v-model:value="selectedNode" :options="nodeOptions" style="width:180px" placeholder="全部节点" clearable />
          <n-button type="primary" size="small" @click="showRule = true">添加规则</n-button>
        </n-space>
      </div>

      <n-tabs type="line">
        <n-tab-pane name="active" tab="⚠️ 活跃告警">
          <div v-if="!loading && activeAlerts.length === 0" class="empty-tip">✅ 暂无活跃告警</div>
          <div v-else class="alert-list">
            <div v-for="a in activeAlerts" :key="a.id" class="alert-item" :class="a.level || 'warning'">
              <div class="alert-icon">{{ (a.level || 'warning') === 'critical' ? '🔴' : '🟡' }}</div>
              <div class="alert-body">
                <div class="alert-name">{{ a.node_name || a.node_id || 'unknown' }} · {{ a.metric || a.type || '--' }}</div>
                <div class="alert-detail">{{ a.message || `${a.value?.toFixed(1) || '?'}% 超过阈值` }} · {{ formatTime(a.created_at) }}</div>
              </div>
              <n-button size="tiny" :loading="silencingId === a.id" @click="silenceAlert(a)">静默</n-button>
            </div>
          </div>
        </n-tab-pane>

        <n-tab-pane name="history" tab="📋 历史记录">
          <n-data-table :columns="historyColumns" :data="historyList" :bordered="false" :loading="loading" size="small" :row-key="(r:any) => r.id" />
        </n-tab-pane>

        <n-tab-pane name="rules" tab="⚙️ 告警规则">
          <n-data-table :columns="ruleColumns" :data="rules" :bordered="false" size="small" :row-key="(r:any) => r.id" />
        </n-tab-pane>
      </n-tabs>

      <!-- 添加规则弹窗 -->
      <n-modal v-model:show="showRule" preset="card" title="添加告警规则" style="width:440px">
        <n-form :model="ruleForm" label-placement="top">
          <n-form-item label="指标">
            <n-select v-model:value="ruleForm.metric" :options="metricOptions" />
          </n-form-item>
          <n-form-item label="条件">
            <n-space>
              <n-select v-model:value="ruleForm.op" :options="opOptions" style="width:80px" />
              <n-input-number v-model:value="ruleForm.threshold" :min="0" :max="100" :step="1" />
            </n-space>
          </n-form-item>
          <n-form-item label="告警级别">
            <n-radio-group v-model:value="ruleForm.level">
              <n-radio value="warning">警告 (黄色)</n-radio>
              <n-radio value="critical">严重 (红色)</n-radio>
            </n-radio-group>
          </n-form-item>
          <n-form-item label="通知渠道">
            <n-checkbox-group v-model:value="ruleForm.channels">
              <n-space><n-checkbox value="browser">浏览器</n-checkbox><n-checkbox value="feishu">飞书</n-checkbox></n-space>
            </n-checkbox-group>
          </n-form-item>
        </n-form>
        <template #footer>
          <n-button @click="showRule = false">取消</n-button>
          <n-button type="primary" style="margin-left:8px" :loading="saving" @click="addRule">保存</n-button>
        </template>
      </n-modal>
    </div>
  </app-layout>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, h } from 'vue'
import { NButton, NTag, useMessage, useDialog } from 'naive-ui'
import AppLayout from '@/components/AppLayout.vue'
import { useNodesStore } from '@/stores/nodes'
import client, { getErrorMessage } from '@/api/client'

const nodesStore = useNodesStore()
const message = useMessage()
const dialog = useDialog()

const selectedNode = ref<string | null>(null)
const activeAlerts = ref<any[]>([])
const historyList = ref<any[]>([])
const rules = ref<any[]>([])
const loading = ref(false)
const showRule = ref(false)
const saving = ref(false)
const silencingId = ref<number | null>(null)
const ruleForm = ref({ metric: 'cpu', op: '>', threshold: 90, level: 'warning' as string, channels: ['browser'], enabled: true })

let pollTimer: ReturnType<typeof setInterval> | null = null
const POLL_INTERVAL = 15000

const nodeOptions = computed(() => [
  { label: '全部节点', value: null },
  ...nodesStore.nodes.map((n: { name: string; hostname?: string; ip: string; id: string }) => ({ label: n.name || n.hostname || n.id, value: n.id })),
])

const metricOptions = [
  { label: 'CPU 使用率', value: 'cpu' },
  { label: '内存使用率', value: 'mem' },
  { label: '磁盘使用率', value: 'disk' },
  { label: '系统负载', value: 'load' },
]

const opOptions = [
  { label: '>', value: '>' },
  { label: '<', value: '<' },
  { label: '>=', value: '>=' },
  { label: '<=', value: '<=' },
]

function formatTime(ts: string | number): string {
  if (!ts) return '--'
  let num: number
  if (typeof ts === 'number') {
    num = ts
  } else if (typeof ts === 'string') {
    num = parseInt(ts, 10)
    if (isNaN(num)) {
      try { return new Date(ts).toLocaleString('zh-CN', { hour12: false }) } catch { return ts }
    }
  } else {
    return String(ts)
  }
  if (num > 9999999999) num = Math.floor(num / 1000)
  try { return new Date(num * 1000).toLocaleString('zh-CN', { hour12: false }) } catch { return String(num) }
}

function showBrowserNotification(alert: { metric?: string; node_name?: string; value?: number; id?: string; message?: string; level?: string }) {
  if (!('Notification' in window)) return
  if (Notification.permission === 'granted') {
    const notification = new Notification(`DevDash 告警: ${alert.metric}`, {
      body: (alert.message || '') + ' ' + (alert.node_name || '') + ' ' + (alert.value ?? '') + '%',
      icon: '/favicon.ico',
      tag: `devdash-alert-${alert.id}`,
    })
    notification.onclick = () => window.focus()
  } else if (Notification.permission !== 'denied') {
    Notification.requestPermission().then(permission => {
      if (permission === 'granted') showBrowserNotification(alert)
    })
  }
}

async function fetchAlerts() {
  loading.value = true
  const params: Record<string, unknown> = {}
  if (selectedNode.value) params.node_id = selectedNode.value
  
  try {
    const { data: activeData } = await client.get('/alerts/active', { params })
    const newAlerts = Array.isArray(activeData) ? activeData : []
    
    const prevIds = new Set(activeAlerts.value.map((a: Record<string, unknown>) => a.id))
    const trulyNew = newAlerts.filter((a: Record<string, unknown>) => !prevIds.has(a.id))
    
    if (trulyNew.length > 0 && document.hidden) {
      trulyNew.forEach((a: Record<string, unknown>) => showBrowserNotification(a))
    }
    
    activeAlerts.value = newAlerts
  } catch (e: unknown) {
    console.warn('Alerts service unavailable:', (e as Error)?.message || e)
    message.error('加载活跃告警失败')
  }

  try {
    const { data } = await client.get('/alerts/history', { params })
    historyList.value = Array.isArray(data) ? data : []
  } catch (e: unknown) {
    console.warn('Alert history unavailable:', (e as Error)?.message || e)
  }

  try {
    const { data } = await client.get('/alert-rules')
    rules.value = Array.isArray(data) ? data : []
  } catch (e: unknown) {
    console.warn('Alert rules unavailable:', (e as Error)?.message || e)
  } finally { loading.value = false }
}

function startPolling() {
  stopPolling()
  pollTimer = setInterval(fetchAlerts, POLL_INTERVAL)
}

function stopPolling() {
  if (pollTimer) {
    clearInterval(pollTimer)
    pollTimer = null
  }
}

const historyColumns = [
  { title: '时间', key: 'time', render: (r: Record<string, unknown>) => formatTime(String(r.time || r.created_at || Date.now())) },
  { title: '节点', key: 'node_name', render: (r: Record<string, unknown>) => r.node_name || r.node_id || '--' },
  { title: '指标', key: 'metric', render: (r: Record<string, unknown>) => r.metric || r.type || '--' },
  { title: '值', key: 'value', render: (r: Record<string, unknown>) => typeof r.value === 'number' ? r.value.toFixed(1) + '%' : '--' },
  {
    title: '级别', key: 'level',
    render: (r: Record<string, unknown>) => h(NTag, { type: (r.level || '') === 'critical' ? 'error' : 'warning', size: 'small' }, () => (r.level || '') === 'critical' ? '严重' : '警告'),
  },
  { title: '状态', key: 'status', render: (r: Record<string, unknown>) => h(NTag, { size: 'small', type: r.status === 'firing' ? 'error' : r.status === 'silenced' ? 'default' : 'info' }, () => r.status === 'firing' ? '触发中' : r.status === 'silenced' ? '已静默' : r.status || '--') },
]

const ruleColumns = [
  { title: '指标', key: 'metric', render: (r: Record<string, unknown>) => metricOptions.find(m => m.value === r.metric)?.label || r.metric || '--' },
  { title: '条件', key: 'condition', render: (r: Record<string, unknown>) => `${r.op || '>'} ${typeof r.threshold === 'number' ? r.threshold : '--'}%` },
  { title: '级别', key: 'level', render: (r: Record<string, unknown>) => h(NTag, { type: (r.level || '') === 'critical' ? 'error' : 'warning', size: 'small' }, () => (r.level || '') === 'critical' ? '严重' : '警告') },
  { title: '渠道', key: 'channels', render: (r: Record<string, unknown>) => Array.isArray(r.channels) ? r.channels.join(', ') : 'browser' },
  { title: '状态', key: 'enabled', render: (r: Record<string, unknown>) => h(NTag, { size: 'small', type: r.enabled ? 'success' : 'default' }, () => r.enabled ? '启用' : '禁用') },
]

async function addRule() {
  saving.value = true
  try {
    await client.post('/alert-rules', ruleForm.value)
    message.success('规则已创建')
    showRule.value = false
    ruleForm.value = { metric: 'cpu', op: '>', threshold: 90, level: 'warning', channels: ['browser'], enabled: true }
    await fetchAlerts()
  } catch (e: unknown) {
    message.error(getErrorMessage(e, '创建失败'))
  } finally { saving.value = false }
}

async function silenceAlert(a: { id?: number; node_name?: string; metric?: string; [k: string]: unknown }) {
  if (!a.id) return
  
  dialog.warning({
    title: '确认静默',
    content: `确定要静默此告警吗？${a.node_name || ''} - ${a.metric || ''}`,
    positiveText: '确认',
    negativeText: '取消',
    onPositiveClick: async () => {
      silencingId.value = a.id ?? null
      try {
        await client.post(`/alerts/${a.id}/silence`)
        message.success('已静默')
        await fetchAlerts()
      } catch (e: unknown) {
        message.error(getErrorMessage(e, '静默失败'))
      } finally { silencingId.value = null }
    }
  })
}

onMounted(async () => {
  await nodesStore.fetchNodes()
  if (nodesStore.nodes.length) {
    await fetchAlerts()
    startPolling()
  }
  
  if ('Notification' in window && Notification.permission === 'default') {
    Notification.requestPermission()
  }
})

onUnmounted(() => {
  stopPolling()
})
</script>

<style scoped>
.page { padding: 24px; }
.page-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 16px; flex-wrap: wrap; gap: 8px; }
h2 { font-size: 20px; font-weight: 600; margin: 0; }
.empty-tip { text-align: center; padding: 60px; color: #3fb950; font-size: 16px; }
.alert-list { display: flex; flex-direction: column; gap: 8px; }
.alert-item { display: flex; align-items: center; gap: 12px; padding: 12px 16px; border-radius: 8px; }
.alert-item.critical { background: #f8514911; border: 1px solid #f8514944; }
.alert-item.warning { background: #d2992211; border: 1px solid #d2992244; }
.alert-icon { font-size: 20px; flex-shrink: 0; }
.alert-body { flex: 1; min-width: 0; }
.alert-name { font-weight: 600; color: #e6edf3; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.alert-detail { font-size: 12px; color: #8b949e; margin-top: 2px; }
</style>