<template>
  <app-layout>
    <div class="page">
      <div class="page-header">
        <h2>告警中心</h2>
        <n-space>
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
                <div class="alert-name">{{ a.node_name || '本机' }} · {{ a.metric || '--' }}</div>
                <div class="alert-detail">{{ a.message || `${a.value?.toFixed(1) || '?'}% 超过阈值` }} · {{ formatTime(a.created_at) }}</div>
              </div>
              <n-button size="tiny" :loading="silencingId === a.id" @click="silenceAlert(a)">静默</n-button>
            </div>
          </div>
        </n-tab-pane>

        <n-tab-pane name="history" tab="📋 历史记录">
          <n-data-table :columns="historyColumns" :data="historyList" :bordered="false" :loading="loading" size="small" :row-key="(r:any) => r?.id ?? String(Math.random())" />
        </n-tab-pane>

        <n-tab-pane name="rules" tab="⚙️ 告警规则">
          <n-data-table :columns="ruleColumns" :data="rules" :bordered="false" size="small" :row-key="(r:any) => r?.id ?? String(Math.random())" />
        </n-tab-pane>

        <n-tab-pane name="notify" tab="🔔 通知配置">
          <div class="notify-config">
            <div class="section-card">
              <div class="section-title">📡 通知渠道配置</div>
              <p class="section-desc">配置告警消息推送到飞书、钉钉、邮件或自定义Webhook</p>

              <n-form label-placement="top" style="max-width:600px">
                <!-- 飞书 -->
                <div class="channel-block">
                  <div class="channel-header">
                    <n-switch v-model:value="notifyConfig.feishu" />
                    <span class="channel-name">飞书机器人</span>
                  </div>
                  <n-form-item label="飞书 Webhook URL" v-if="notifyConfig.feishu">
                    <n-input v-model:value="notifyConfig.feishu_url" placeholder="https://open.feishu.cn/open-apis/bot/v2/hook/xxxxx" />
                  </n-form-item>
                </div>

                <!-- 钉钉 -->
                <div class="channel-block">
                  <div class="channel-header">
                    <n-switch v-model:value="notifyConfig.dingtalk" />
                    <span class="channel-name">钉钉机器人</span>
                  </div>
                  <template v-if="notifyConfig.dingtalk">
                    <n-form-item label="钉钉 Webhook URL">
                      <n-input v-model:value="notifyConfig.dingtalk_url" placeholder="https://oapi.dingtalk.com/robot/send?access_token=xxxxx" />
                    </n-form-item>
                    <n-form-item label="加签密钥 (可选)">
                      <n-input v-model:value="notifyConfig.dingtalk_secret" placeholder="SEC..." type="password" show-password-on="mousedown" />
                    </n-form-item>
                  </template>
                </div>

                <!-- Webhook -->
                <div class="channel-block">
                  <div class="channel-header">
                    <n-switch v-model:value="notifyConfig.webhook_enabled" />
                    <span class="channel-name">自定义 Webhook</span>
                  </div>
                  <template v-if="notifyConfig.webhook_enabled">
                    <n-form-item label="Webhook URL">
                      <n-input v-model:value="notifyConfig.webhook_url" placeholder="https://your-webhook.com/alert" />
                    </n-form-item>
                    <n-form-item label="Webhook Secret (可选)">
                      <n-input v-model:value="notifyConfig.webhook_secret" placeholder="用于验证请求来源" type="password" show-password-on="mousedown" />
                    </n-form-item>
                  </template>
                </div>

                <!-- 邮件 -->
                <div class="channel-block">
                  <div class="channel-header">
                    <n-switch v-model:value="notifyConfig.email_enabled" />
                    <span class="channel-name">邮件通知</span>
                  </div>
                  <template v-if="notifyConfig.email_enabled">
                    <n-space>
                      <n-form-item label="SMTP 服务器"><n-input v-model:value="notifyConfig.email_smtp" placeholder="smtp.gmail.com" style="width:200px" /></n-form-item>
                      <n-form-item label="端口"><n-input-number v-model:value="notifyConfig.email_port" :min="1" :max="65535" style="width:100px" /></n-form-item>
                    </n-space>
                    <n-form-item label="用户名"><n-input v-model:value="notifyConfig.email_user" placeholder="user@gmail.com" /></n-form-item>
                    <n-form-item label="密码/授权码"><n-input v-model:value="notifyConfig.email_password" type="password" show-password-on="mousedown" /></n-form-item>
                    <n-form-item label="发件人"><n-input v-model:value="notifyConfig.email_from" placeholder="devdash@gmail.com" /></n-form-item>
                    <n-form-item label="收件人"><n-input v-model:value="notifyConfig.email_to" placeholder="admin@example.com" /></n-form-item>
                  </template>
                </div>

                <!-- 浏览器 -->
                <div class="channel-block">
                  <div class="channel-header">
                    <n-switch v-model:value="notifyConfig.browser" />
                    <span class="channel-name">浏览器通知</span>
                  </div>
                </div>
              </n-form>

              <n-space style="margin-top:16px">
                <n-button type="primary" :loading="notifySaving" @click="saveNotifyConfig">保存配置</n-button>
                <n-button :loading="notifyTesting" @click="testNotify">发送测试告警</n-button>
              </n-space>
            </div>

            <div class="section-card" style="margin-top:16px">
              <div class="section-title">📖 接入指南</div>
              <div class="guide-content">
                <h4>飞书机器人接入</h4>
                <ol>
                  <li>打开飞书群 → 群设置 → 群机器人 → 添加机器人 → 自定义机器人</li>
                  <li>复制 Webhook URL 填入上方配置</li>
                  <li>保存后点击"发送测试告警"验证</li>
                </ol>
                <h4>钉钉机器人接入</h4>
                <ol>
                  <li>打开钉钉群 → 群设置 → 智能群助手 → 添加机器人 → 自定义</li>
                  <li>安全设置选择"加签"，复制密钥填入上方配置</li>
                  <li>复制 Webhook URL 填入上方配置</li>
                  <li>保存后点击"发送测试告警"验证</li>
                </ol>
              </div>
            </div>
          </div>
        </n-tab-pane>
      </n-tabs>

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
import { ref, onMounted, onUnmounted, h } from 'vue'
import { NButton, NTag, useMessage, useDialog } from 'naive-ui'
import AppLayout from '@/components/AppLayout.vue'
import { alertAPI } from '@/api'
import { getErrorMessage } from '@/api/client'

const message = useMessage()
const dialog = useDialog()

const activeAlerts = ref<any[]>([])
const historyList = ref<any[]>([])
const rules = ref<any[]>([])
const loading = ref(false)
const showRule = ref(false)
const saving = ref(false)
const silencingId = ref<number | null>(null)
const ruleForm = ref({ metric: 'cpu', op: '>', threshold: 90, level: 'warning' as string, channels: ['browser'], enabled: true })

const notifyConfig = ref({
  browser: true,
  email_enabled: false,
  email_smtp: '',
  email_port: 587,
  email_user: '',
  email_password: '',
  email_from: '',
  email_to: '',
  webhook_enabled: false,
  webhook_url: '',
  webhook_secret: '',
  feishu: false,
  feishu_url: '',
  dingtalk: false,
  dingtalk_url: '',
  dingtalk_secret: '',
})
const notifySaving = ref(false)
const notifyTesting = ref(false)

let pollTimer: ReturnType<typeof setInterval> | null = null
const POLL_INTERVAL = 15000

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
  // 如果是数字（Unix时间戳，秒或毫秒）
  if (typeof ts === 'number') {
    let num = ts
    if (num > 9999999999) num = Math.floor(num / 1000)
    try { return new Date(num * 1000).toLocaleString('zh-CN', { hour12: false }) } catch { return String(num) }
  }
  // 如果是字符串，先尝试解析为ISO日期（后端time.Time序列化格式）
  if (typeof ts === 'string') {
    const parsed = Date.parse(ts)
    if (!isNaN(parsed)) {
      try { return new Date(parsed).toLocaleString('zh-CN', { hour12: false }) } catch { return ts }
    }
    // 尝试解析为纯数字字符串（Unix时间戳）
    const num = parseInt(ts, 10)
    if (!isNaN(num)) {
      let n = num
      if (n > 9999999999) n = Math.floor(n / 1000)
      try { return new Date(n * 1000).toLocaleString('zh-CN', { hour12: false }) } catch { return ts }
    }
    return ts
  }
  return String(ts)
}

function showBrowserNotification(alert: { metric?: string; node_name?: string; value?: number; id?: string | number; message?: string }) {
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

  try {
    const { data: activeData } = await alertAPI.active()
    const newAlerts = Array.isArray(activeData) ? activeData : []

    const prevIds = new Set(activeAlerts.value.map((a: any) => a.id))
    const trulyNew = newAlerts.filter((a: any) => !prevIds.has(a.id))

    if (trulyNew.length > 0 && document.hidden) {
      trulyNew.forEach((a: any) => showBrowserNotification(a))
    }

    activeAlerts.value = newAlerts
  } catch (e: unknown) {
    console.warn('Alerts service unavailable:', (e as Error)?.message || e)
  }

  try {
    const { data } = await alertAPI.history()
    historyList.value = Array.isArray(data) ? data : []
  } catch (e: unknown) {
    console.warn('Alert history unavailable:', (e as Error)?.message || e)
  }

  try {
    const { data } = await alertAPI.rules()
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
  { title: '时间', key: 'time', render: (r: any) => formatTime(r.time || r.created_at) },
  { title: '节点', key: 'node_name', render: (r: any) => r.node_name || '本机' },
  { title: '指标', key: 'type', render: (r: any) => metricOptions.find(m => m.value === (r.type || r.metric))?.label || r.type || r.metric || '--' },
  { title: '值', key: 'value', render: (r: any) => typeof r.value === 'number' ? r.value.toFixed(1) + '%' : '--' },
  {
    title: '级别', key: 'level',
    render: (r: any) => h(NTag, { type: (r.level || '') === 'critical' ? 'error' : 'warning', size: 'small' }, () => (r.level || '') === 'critical' ? '严重' : '警告'),
  },
  { title: '状态', key: 'status', render: (r: any) => h(NTag, { size: 'small', type: r.status === 'firing' ? 'error' : r.status === 'silenced' ? 'default' : 'info' }, () => r.status === 'firing' ? '触发中' : r.status === 'silenced' ? '已静默' : r.status || '--') },
]

const ruleColumns = [
  { title: '指标', key: 'metric', render: (r: any) => metricOptions.find(m => m.value === r.metric)?.label || r.metric || '--' },
  { title: '条件', key: 'condition', render: (r: any) => `${r.op || '>'} ${typeof r.threshold === 'number' ? r.threshold : '--'}%` },
  { title: '级别', key: 'level', render: (r: any) => h(NTag, { type: (r.level || '') === 'critical' ? 'error' : 'warning', size: 'small' }, () => (r.level || '') === 'critical' ? '严重' : '警告') },
  { title: '渠道', key: 'channels', render: (r: any) => Array.isArray(r.channels) ? r.channels.join(', ') : 'browser' },
  { title: '状态', key: 'enabled', render: (r: any) => h(NTag, { size: 'small', type: r.enabled ? 'success' : 'default' }, () => r.enabled ? '启用' : '禁用') },
  {
    title: '操作', key: 'actions', width: 120,
    render: (r: any) => h('div', { style: 'display:flex; gap:8px' }, [
      h(NButton, { size: 'small', quaternary: true, type: 'error', onClick: () => confirmDeleteRule(r) }, () => '删除'),
    ]),
  },
]

async function addRule() {
  saving.value = true
  try {
    await alertAPI.createRule(ruleForm.value)
    message.success('规则已创建')
    showRule.value = false
    ruleForm.value = { metric: 'cpu', op: '>', threshold: 90, level: 'warning', channels: ['browser'], enabled: true }
    await fetchAlerts()
  } catch (e: unknown) {
    message.error(getErrorMessage(e, '创建失败'))
  } finally { saving.value = false }
}

function confirmDeleteRule(rule: any) {
  dialog.warning({
    title: '删除告警规则',
    content: `确定要删除该规则吗？指标：${metricOptions.find(m => m.value === rule.metric)?.label || rule.metric}`,
    positiveText: '删除',
    negativeText: '取消',
    onPositiveClick: async () => {
      try {
        await alertAPI.deleteRule(String(rule.id))
        message.success('规则已删除')
        await fetchAlerts()
      } catch (e: unknown) {
        message.error(getErrorMessage(e, '删除失败'))
      }
    },
  })
}

async function silenceAlert(a: any) {
  if (!a.id) return

  dialog.warning({
    title: '确认静默',
    content: `确定要静默此告警吗？${a.node_name || ''} - ${a.metric || ''}`,
    positiveText: '确认',
    negativeText: '取消',
    onPositiveClick: async () => {
      silencingId.value = a.id ?? null
      try {
        await alertAPI.silence(String(a.id))
        message.success('已静默')
        await fetchAlerts()
      } catch (e: unknown) {
        message.error(getErrorMessage(e, '静默失败'))
      } finally { silencingId.value = null }
    }
  })
}

onMounted(async () => {
  await fetchAlerts()
  startPolling()

  if ('Notification' in window && Notification.permission === 'default') {
    Notification.requestPermission()
  }

  // 加载通知配置
  try {
    const { data } = await alertAPI.getNotifyConfig()
    if (data) Object.assign(notifyConfig.value, data)
  } catch (e: unknown) {
    console.warn('Failed to load notify config:', (e as Error)?.message)
  }
})

onUnmounted(() => {
  stopPolling()
})

async function saveNotifyConfig() {
  notifySaving.value = true
  try {
    await alertAPI.updateNotifyConfig(notifyConfig.value)
    message.success('通知配置已保存')
  } catch (e: unknown) {
    message.error(getErrorMessage(e, '保存失败'))
  } finally { notifySaving.value = false }
}

async function testNotify() {
  notifyTesting.value = true
  try {
    await alertAPI.testNotify()
    message.success('测试告警已发送，请检查各通知渠道')
  } catch (e: unknown) {
    message.error(getErrorMessage(e, '发送失败'))
  } finally { notifyTesting.value = false }
}
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
.notify-config { max-width: 800px; }
.section-card { background: rgba(255,255,255,0.03); border: 1px solid rgba(255,255,255,0.08); border-radius: 8px; padding: 20px; }
.section-title { font-size: 16px; font-weight: 600; margin-bottom: 8px; }
.section-desc { font-size: 13px; color: #8b949e; margin-bottom: 16px; }
.channel-block { padding: 12px 0; border-bottom: 1px solid rgba(255,255,255,0.06); }
.channel-header { display: flex; align-items: center; gap: 12px; margin-bottom: 8px; }
.channel-name { font-weight: 500; }
.guide-content { color: #8b949e; font-size: 13px; line-height: 1.8; }
.guide-content h4 { color: #e6edf3; margin: 12px 0 4px; }
.guide-content ol { padding-left: 20px; }
</style>
