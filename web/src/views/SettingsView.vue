<template>
  <app-layout>
    <div class="page">
      <h2>{{ activeTab === 'profile' ? '个人设置' : '系统设置' }}</h2>

      <n-tabs v-model:value="activeTab" type="line" animated @update:value="onTabSwitch">
        <n-tab-pane name="profile" tab="👤 个人设置">
          <div class="section-card">
            <div class="section-title">修改用户名</div>
            <n-form :model="usernameForm" label-placement="top" style="max-width:400px">
              <n-form-item label="当前用户名"><n-input :value="authStore.username" disabled /></n-form-item>
              <n-form-item label="新用户名"><n-input v-model:value="usernameForm.newUsername" placeholder="输入新用户名（2-32位）" /></n-form-item>
              <n-button type="primary" :loading="usernameLoading" @click="changeUsername">保存</n-button>
            </n-form>
          </div>

          <div class="section-card" style="margin-top:16px">
            <div class="section-title">修改密码</div>
            <n-form :model="pwdForm" label-placement="top" style="max-width:400px">
              <n-form-item label="当前密码"><n-input v-model:value="pwdForm.old" type="password" show-password-on="mousedown" /></n-form-item>
              <n-form-item label="新密码"><n-input v-model:value="pwdForm.new" type="password" show-password-on="mousedown" /></n-form-item>
              <n-form-item label="确认新密码"><n-input v-model:value="pwdForm.confirm" type="password" show-password-on="mousedown" /></n-form-item>
              <n-button type="primary" :loading="pwdLoading" @click="changePwd">保存</n-button>
            </n-form>
          </div>

          <div class="section-card" style="margin-top:16px">
            <div class="section-title">个人偏好</div>
            <n-space vertical style="max-width:500px">
              <div class="threshold-row">
                <span>主题色</span>
                <div class="color-swatches">
                  <div v-for="c in colors" :key="c.value" class="swatch" :style="{ background: c.value }" :class="{ active: themeColor === c.value }" @click="setColor(c.value)" />
                </div>
              </div>
              <div class="threshold-row">
                <span>显示密度</span>
                <n-radio-group v-model:value="density" size="small">
                  <n-radio value="compact">紧凑</n-radio>
                  <n-radio value="default">默认</n-radio>
                  <n-radio value="comfortable">宽松</n-radio>
                </n-radio-group>
              </div>
            </n-space>
          </div>
        </n-tab-pane>

        <n-tab-pane name="system" tab="⚙️ 系统设置">
          <div class="section-card">
            <div class="section-title">告警规则</div>
            <n-space vertical style="max-width:500px">
              <div v-if="alertRules.length === 0" style="color:#8b949e;font-size:13px">暂无告警规则</div>
              <div v-for="rule in alertRules" :key="rule.id" class="rule-item">
                <div class="rule-info">
                  <n-tag :type="rule.enabled ? 'success' : 'default'" size="small">{{ rule.enabled ? '启用' : '禁用' }}</n-tag>
                  <span class="rule-metric">{{ rule.metric }} {{ rule.op }} {{ rule.threshold }}%</span>
                  <n-tag size="small" :type="rule.level === 'critical' ? 'error' : rule.level === 'warning' ? 'warning' : 'info'">{{ rule.level }}</n-tag>
                </div>
                <n-switch size="small" :value="rule.enabled" @update:value="(v: boolean) => toggleRule(rule, v)" />
              </div>
            </n-space>
          </div>

          <div class="section-card" style="margin-top:16px">
            <div class="section-title">采集配置</div>
            <n-space vertical style="max-width:500px">
              <div class="threshold-row">
                <span>采集间隔</span>
                <n-input-number v-model:value="systemSettings.collectInterval" :min="3" :max="60" size="small" style="width:120px" />
                <span>秒</span>
              </div>
              <div class="threshold-row">
                <span>数据保留天数</span>
                <n-input-number v-model:value="systemSettings.retentionDays" :min="1" :max="365" size="small" style="width:120px" />
                <span>天</span>
              </div>
            </n-space>
            <n-button type="primary" style="margin-top:16px" @click="saveSystemSettingsLocal">保存采集配置</n-button>
          </div>
        </n-tab-pane>

        <n-tab-pane name="about" tab="ℹ️ 关于">
          <div class="section-card">
            <div class="section-title">DevDash</div>
            <div class="about-info">
              <p>版本：<strong>1.0.0</strong></p>
              <p>前端：Vue 3 + Vite + TypeScript + Naive UI + ECharts</p>
              <p>后端：Go + Gin + WebSocket + SQLite</p>
              <p>终端：xterm.js</p>
            </div>
          </div>
        </n-tab-pane>
      </n-tabs>
    </div>
  </app-layout>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useMessage } from 'naive-ui'
import AppLayout from '@/components/AppLayout.vue'
import { authClient, getErrorMessage } from '@/api/client'
import { authAPI, alertAPI } from '@/api'
import { useAuthStore } from '@/stores/auth'
import { useTheme } from '@/composables/useTheme'

const route = useRoute()
const router = useRouter()
const message = useMessage()
const authStore = useAuthStore()
const { themeColor: currentThemeColor, setColor: setThemeColor, densityMode, setDensity } = useTheme()

const activeTab = ref((route.query.tab as string) || 'profile')

watch(() => route.query.tab, (tab) => {
  if (tab === 'system' || tab === 'profile' || tab === 'about') {
    activeTab.value = tab
  }
})

const usernameLoading = ref(false)
const usernameForm = reactive({ newUsername: '' })
const pwdLoading = ref(false)
const pwdForm = reactive({ old: '', new: '', confirm: '' })
const themeColor = ref(currentThemeColor.value)
const density = ref(densityMode.value)
const alertRules = ref<any[]>([])

watch(density, (v) => { setDensity(v) })

const systemSettings = reactive({
  collectInterval: 5,
  retentionDays: 30,
})

const colors = [
  { label: '蓝色', value: '#58a6ff' },
  { label: '绿色', value: '#3fb950' },
  { label: '紫色', value: '#bc8cff' },
  { label: '橙色', value: '#f0883e' },
  { label: '红色', value: '#f85149' },
  { label: '金色', value: '#d29922' },
]

function onTabSwitch(tab: string) {
  router.replace({ name: 'settings', query: { tab } })
}

async function changeUsername() {
  const name = usernameForm.newUsername.trim()
  if (!name || name.length < 2 || name.length > 32) {
    message.warning('用户名长度需在2-32位之间')
    return
  }
  usernameLoading.value = true
  try {
    await authAPI.changeUsername(name)
    authStore.username = name
    message.success('用户名已修改')
    usernameForm.newUsername = ''
  } catch (e: unknown) {
    message.error(getErrorMessage(e, '修改失败，用户名可能已存在'))
  } finally {
    usernameLoading.value = false
  }
}

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

async function fetchAlertRules() {
  try {
    const { data } = await alertAPI.rules()
    alertRules.value = Array.isArray(data) ? data : []
  } catch {
    alertRules.value = []
  }
}

async function toggleRule(rule: any, enabled: boolean) {
  try {
    await alertAPI.updateRule(String(rule.id), { enabled })
    rule.enabled = enabled
    message.success(enabled ? '已启用' : '已禁用')
  } catch {
    message.error('操作失败')
  }
}

function saveSystemSettingsLocal() {
  localStorage.setItem('devdash_collect_interval', String(systemSettings.collectInterval))
  localStorage.setItem('devdash_retention_days', String(systemSettings.retentionDays))
  message.success('已保存')
}

function setColor(c: string) {
  themeColor.value = c
  setThemeColor(c)
}

onMounted(async () => {
  fetchAlertRules()
  const ci = localStorage.getItem('devdash_collect_interval')
  const rd = localStorage.getItem('devdash_retention_days')
  if (ci) systemSettings.collectInterval = Number(ci)
  if (rd) systemSettings.retentionDays = Number(rd)
})
</script>

<style scoped>
.page { padding: 24px; }
h2 { font-size: 20px; font-weight: 600; margin: 0 0 20px; }
.section-card { background: #161b22; border: 1px solid #30363d; border-radius: 8px; padding: 20px; }
.section-title { font-size: 14px; font-weight: 600; margin-bottom: 16px; color: #e6edf3; }
.section-subtitle { font-size: 13px; font-weight: 500; margin-bottom: 8px; color: #8b949e; }
.threshold-row { display: flex; align-items: center; gap: 12px; max-width: 400px; }
.threshold-row > span:first-child { min-width: 100px; color: #8b949e; font-size: 13px; }
.color-swatches { display: flex; gap: 8px; }
.swatch { width: 28px; height: 28px; border-radius: 50%; cursor: pointer; border: 2px solid transparent; transition: border-color 0.2s; }
.swatch:hover, .swatch.active { border-color: #fff; }
.about-info p { margin: 0 0 6px; font-size: 13px; color: #8b949e; }
.about-info strong { color: #e6edf3; }
.rule-item { display: flex; justify-content: space-between; align-items: center; padding: 8px 0; border-bottom: 1px solid #21262d; }
.rule-item:last-child { border-bottom: none; }
.rule-info { display: flex; align-items: center; gap: 8px; }
.rule-metric { font-size: 13px; color: #e6edf3; }
</style>
