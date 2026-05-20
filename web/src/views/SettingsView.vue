<template>
  <app-layout>
    <div class="page">
      <h2>全局设置</h2>

      <n-tabs type="line" animated>
        <!-- 个人设置 -->
        <n-tab-pane name="profile" tab="👤 个人设置">
          <div class="section-card">
            <div class="section-title">修改密码</div>
            <n-form :model="pwdForm" label-placement="top" style="max-width:400px">
              <n-form-item label="当前密码"><n-input v-model:value="pwdForm.old" type="password" show-password-on="mousedown" /></n-form-item>
              <n-form-item label="新密码"><n-input v-model:value="pwdForm.new" type="password" show-password-on="mousedown" /></n-form-item>
              <n-form-item label="确认新密码"><n-input v-model:value="pwdForm.confirm" type="password" show-password-on="mousedown" /></n-form-item>
              <n-button type="primary" :loading="pwdLoading" @click="changePwd">保存</n-button>
            </n-form>
          </div>
        </n-tab-pane>

        <!-- 告警设置 -->
        <n-tab-pane name="alert" tab="🔔 告警设置">
          <div class="section-card">
            <div class="section-title">通知渠道</div>
            <n-space vertical>
              <n-checkbox v-model:checked="alertSettings.browser">浏览器通知</n-checkbox>
              <n-checkbox v-model:checked="alertSettings.feishu">飞书机器人</n-checkbox>
            </n-space>
            <div v-if="alertSettings.feishu" style="margin-top:16px;max-width:500px">
              <n-form-item label="飞书 Webhook URL">
                <n-input v-model:value="alertSettings.feishuUrl" placeholder="https://open.feishu.cn/open-apis/bot/v2/hook/xxx" />
              </n-form-item>
              <n-button type="primary" size="small" @click="testFeishu">测试</n-button>
            </div>
          </div>

          <div class="section-card" style="margin-top:16px">
            <div class="section-title">告警阈值（默认）</div>
            <n-space vertical style="max-width:500px">
              <div class="threshold-row">
                <span>CPU 告警阈值</span>
                <n-input-number v-model:value="alertSettings.cpuThreshold" :min="0" :max="100" /> %
              </div>
              <div class="threshold-row">
                <span>内存 告警阈值</span>
                <n-input-number v-model:value="alertSettings.memThreshold" :min="0" :max="100" /> %
              </div>
              <div class="threshold-row">
                <span>磁盘 告警阈值</span>
                <n-input-number v-model:value="alertSettings.diskThreshold" :min="0" :max="100" /> %
              </div>
              <div class="threshold-row">
                <span>告警冷却时间</span>
                <n-input-number v-model:value="alertSettings.cooldownMin" :min="1" :max="60" /> 分钟
              </div>
            </n-space>
            <n-button type="primary" style="margin-top:16px" :loading="saving" @click="saveAlertSettings">保存设置</n-button>
          </div>
        </n-tab-pane>

        <!-- 主题设置 -->
        <n-tab-pane name="theme" tab="🎨 界面">
          <div class="section-card">
            <div class="section-title">主题色</div>
            <div class="color-swatches">
              <div v-for="c in colors" :key="c.value" class="swatch" :style="{ background: c.value }" :class="{ active: themeColor === c.value }" @click="setColor(c.value)" />
            </div>
          </div>

          <div class="section-card" style="margin-top:16px">
            <div class="section-title">显示密度</div>
            <n-radio-group v-model:value="density">
              <n-space><n-radio value="compact">紧凑</n-radio><n-radio value="default">默认</n-radio><n-radio value="comfortable">宽松</n-radio></n-space>
            </n-radio-group>
          </div>
        </n-tab-pane>

        <!-- 系统信息 -->
        <n-tab-pane name="about" tab="ℹ️ 关于">
          <div class="section-card">
            <div class="section-title">DevDash</div>
            <div class="about-info">
              <p>版本：<strong>1.0.0</strong></p>
              <p>前端：Vue 3 + Vite + TypeScript + Naive UI + ECharts</p>
              <p>后端：Go + Gin + WebSocket + SQLite/PostgreSQL</p>
              <p>终端：xterm.js</p>
            </div>
          </div>
        </n-tab-pane>
      </n-tabs>
    </div>
  </app-layout>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { useMessage } from 'naive-ui'
import AppLayout from '@/components/AppLayout.vue'
import client, { authClient, getErrorMessage } from '@/api/client'

const message = useMessage()

const pwdLoading = ref(false)
const pwdForm = reactive({ old: '', new: '', confirm: '' })
const saving = ref(false)
const themeColor = ref('#58a6ff')
const density = ref('default')

const alertSettings = reactive({
  browser: true,
  feishu: false,
  feishuUrl: '',
  cpuThreshold: 90,
  memThreshold: 90,
  diskThreshold: 90,
  cooldownMin: 5,
})

const colors = [
  { label: '蓝色', value: '#58a6ff' },
  { label: '绿色', value: '#3fb950' },
  { label: '紫色', value: '#bc8cff' },
  { label: '橙色', value: '#f0883e' },
  { label: '红色', value: '#f85149' },
  { label: '金色', value: '#d29922' },
]

async function changePwd() {
  if (!pwdForm.new || pwdForm.new !== pwdForm.confirm) { message.warning('两次密码不一致'); return }
  pwdLoading.value = true
  try {
    await authClient.put('/auth/password', pwdForm)
    message.success('密码已修改')
    Object.assign(pwdForm, { old: '', new: '', confirm: '' })
  } catch (e: unknown) { message.error(getErrorMessage(e, '修改失败')) }
  finally { pwdLoading.value = false }
}

async function saveAlertSettings() {
  saving.value = true
  try {
    await client.put('/alert-settings', alertSettings)
    message.success('已保存')
  } catch { message.error('保存失败') }
  finally { saving.value = false }
}

async function testFeishu() {
  if (!alertSettings.feishuUrl) { message.warning('请先填写飞书 Webhook URL'); return }
  try {
    await client.post('/alert/test-feishu', { url: alertSettings.feishuUrl })
    message.success('测试消息已发送')
  } catch { message.error('发送失败，请检查 URL') }
}

function setColor(c: string) {
  themeColor.value = c
  document.documentElement.style.setProperty('--primary-color', c)
  localStorage.setItem('theme-color', c)
}

onMounted(() => {
  const saved = localStorage.getItem('theme-color')
  if (saved) { themeColor.value = saved; setColor(saved) }
})
</script>

<style scoped>
.page { padding: 24px; }
h2 { font-size: 20px; font-weight: 600; margin: 0 0 20px; }
.section-card { background: #161b22; border: 1px solid #30363d; border-radius: 8px; padding: 20px; }
.section-title { font-size: 14px; font-weight: 600; margin-bottom: 16px; color: #e6edf3; }
.threshold-row { display: flex; align-items: center; justify-content: space-between; max-width: 300px; }
.color-swatches { display: flex; gap: 12px; }
.swatch { width: 32px; height: 32px; border-radius: 50%; cursor: pointer; border: 2px solid transparent; transition: border-color 0.2s; }
.swatch:hover, .swatch.active { border-color: #fff; }
.about-info p { margin: 0 0 6px; font-size: 13px; color: #8b949e; }
.about-info strong { color: #e6edf3; }
</style>