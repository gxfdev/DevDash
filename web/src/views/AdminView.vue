<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useMessage, useDialog } from 'naive-ui'
import {
  PersonOutline, ShieldCheckmarkOutline, TrashOutline, AddOutline, SearchOutline,
  SettingsOutline, DocumentTextOutline, InformationCircleOutline, KeyOutline, RefreshOutline,
} from '@vicons/ionicons5'
import { adminApi } from '@/api'
import { useAuthStore } from '@/stores/auth'
import type { AuditLog, User } from '@/types'
import { fmtTime, roleLabel, timeAgo } from '@/utils/format'
import { usePlatformStore } from '@/stores/platform'

const auth = useAuthStore()
const platform = usePlatformStore()
const message = useMessage()
const dialog = useDialog()

const tab = ref('users')
const users = ref<User[]>([])
const logs = ref<AuditLog[]>([])
const auditQuery = ref('')
const settings = ref<Record<string, string>>({})
const savingSettings = ref(false)
const loading = ref(false)

const createOpen = ref(false)
const newUser = ref({ username: '', password: '', role: 'viewer' })
const creating = ref(false)

const editOpen = ref(false)
const editUser = ref<User | null>(null)
const editForm = ref({ role: 'viewer', password: '' })

const isAdmin = computed(() => auth.role === 'admin')

async function loadUsers() {
  if (!isAdmin.value) return
  try { users.value = (await adminApi.users()).users || [] } catch (e: any) { message.error(e.message) }
}
async function loadAudit() {
  try {
    const res = await adminApi.audit(auditQuery.value, 150)
    logs.value = res.logs || []
  } catch (e: any) { message.error(e.message) }
}
async function loadSettings() {
  try { settings.value = (await adminApi.settings()).settings || {} } catch (e: any) { message.error(e.message) }
}

async function loadAll() {
  loading.value = true
  await Promise.all([loadUsers(), loadAudit(), loadSettings()])
  loading.value = false
}

async function createUser() {
  if (!newUser.value.username.trim() || newUser.value.password.length < 8) {
    message.warning('用户名必填，密码至少 8 位')
    return
  }
  creating.value = true
  try {
    await adminApi.createUser(newUser.value)
    message.success('用户已创建')
    createOpen.value = false
    newUser.value = { username: '', password: '', role: 'viewer' }
    loadUsers()
  } catch (e: any) {
    message.error(e.message)
  } finally {
    creating.value = false
  }
}

function openEdit(u: User) {
  editUser.value = u
  editForm.value = { role: u.role, password: '' }
  editOpen.value = true
}

async function saveEdit() {
  if (!editUser.value) return
  try {
    const payload: any = { role: editForm.value.role }
    if (editForm.value.password) payload.password = editForm.value.password
    await adminApi.updateUser(editUser.value.id, payload)
    message.success('已更新')
    editOpen.value = false
    loadUsers()
  } catch (e: any) {
    message.error(e.message)
  }
}

function removeUser(u: User) {
  dialog.warning({
    title: '删除用户',
    content: `确定删除用户 ${u.username}？`,
    positiveText: '删除',
    negativeText: '取消',
    onPositiveClick: async () => {
      try {
        await adminApi.deleteUser(u.id)
        message.success('已删除')
        loadUsers()
      } catch (e: any) {
        message.error(e.message)
      }
    },
  })
}

async function saveSettings() {
  savingSettings.value = true
  try {
    const res = await adminApi.updateSettings(settings.value)
    settings.value = res.settings || settings.value
    message.success('设置已保存')
  } catch (e: any) {
    message.error(e.message)
  } finally {
    savingSettings.value = false
  }
}

const roleOptions = [
  { label: '超级管理员', value: 'admin' },
  { label: '运维操作员', value: 'operator' },
  { label: '只读观察者', value: 'viewer' },
]

onMounted(loadAll)
</script>

<template>
  <div class="page">
    <div class="page-head">
      <div>
        <h1 class="page-title">全局管理</h1>
        <p class="page-sub">用户与角色、审计日志与平台设置。所有变更操作都会记录到审计日志，可追溯。</p>
      </div>
      <n-button size="small" quaternary :loading="loading" @click="loadAll">刷新</n-button>
    </div>

    <n-tabs v-model:value="tab" type="line" animated>
      <n-tab-pane name="users" tab="用户与权限">
        <div class="panel-head">
          <div class="section-title"><span class="dot" />平台用户</div>
          <n-button v-if="isAdmin" size="small" type="primary" @click="createOpen = true">
            <template #icon><n-icon :component="AddOutline" /></template>新建用户
          </n-button>
        </div>
        <div class="glass table-wrap">
          <table class="tbl">
            <thead><tr><th>用户</th><th>角色</th><th>创建时间</th><th>最近登录</th><th>状态</th><th style="text-align:right">操作</th></tr></thead>
            <tbody>
              <tr v-for="u in users" :key="u.id">
                <td>
                  <div class="row gap-8">
                    <div class="avatar-sm">{{ u.username.slice(0, 1).toUpperCase() }}</div>
                    <b>{{ u.username }}</b>
                    <span v-if="u.username === auth.user?.username" class="badge info">当前</span>
                  </div>
                </td>
                <td><span class="badge" :class="u.role === 'admin' ? 'info' : u.role === 'operator' ? 'ok' : 'neutral'">{{ roleLabel(u.role) }}</span></td>
                <td class="muted">{{ fmtTime(u.created_at) }}</td>
                <td class="muted">{{ u.last_login_at ? timeAgo(u.last_login_at) : '从未' }}</td>
                <td><span v-if="u.must_change_pwd" class="badge warn">待改密</span><span v-else class="badge ok">正常</span></td>
                <td style="text-align:right">
                  <n-button v-if="isAdmin" size="tiny" tertiary @click="openEdit(u)">编辑</n-button>
                  <n-button v-if="isAdmin" size="tiny" quaternary type="error" :disabled="u.username === auth.user?.username" @click="removeUser(u)">
                    <template #icon><n-icon :component="TrashOutline" /></template>
                  </n-button>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
        <div v-if="!isAdmin" class="tip-box">只有超级管理员可以管理用户。</div>
      </n-tab-pane>

      <n-tab-pane name="audit" tab="审计日志">
        <div class="panel-head">
          <div class="section-title"><span class="dot" />操作审计</div>
          <div class="row gap-8">
            <n-input v-model:value="auditQuery" size="small" placeholder="搜索用户 / 动作 / 对象" clearable style="width: 260px" @keyup.enter="loadAudit">
              <template #prefix><n-icon :component="SearchOutline" /></template>
            </n-input>
            <n-button size="small" quaternary @click="loadAudit">查询</n-button>
          </div>
        </div>
        <div class="glass table-wrap">
          <table class="tbl">
            <thead><tr><th>时间</th><th>用户</th><th>角色</th><th>操作</th><th>对象</th><th>详情</th><th>来源 IP</th><th>结果</th></tr></thead>
            <tbody>
              <tr v-for="l in logs" :key="l.id">
                <td class="muted" style="white-space:nowrap">{{ fmtTime(l.created_at) }}</td>
                <td><b>{{ l.user || '-' }}</b></td>
                <td class="muted">{{ roleLabel(l.role) }}</td>
                <td>{{ l.action }}</td>
                <td class="mono">{{ l.target || '-' }}</td>
                <td class="muted detail">{{ l.detail || '-' }}</td>
                <td class="mono muted">{{ l.ip }}</td>
                <td><span class="badge" :class="l.result === 'ok' ? 'ok' : 'bad'">{{ l.result === 'ok' ? '成功' : '失败' }}</span></td>
              </tr>
              <tr v-if="!logs.length"><td colspan="8" class="muted" style="text-align:center; padding: 30px">暂无审计记录</td></tr>
            </tbody>
          </table>
        </div>
      </n-tab-pane>

      <n-tab-pane name="settings" tab="平台设置">
        <div class="settings-grid">
          <div class="glass panel">
            <div class="section-title"><span class="dot" />基础设置</div>
            <n-form label-placement="top" :disabled="!isAdmin">
              <n-form-item label="平台名称">
                <n-input v-model:value="settings.site_name" placeholder="AIDash 单机 AI 操作系统" />
              </n-form-item>
              <n-form-item label="Copilot 默认模型">
                <n-input v-model:value="settings.copilot_model" placeholder="留空则使用第一个运行中的模型" />
              </n-form-item>
              <n-form-item label="指标保留天数">
                <n-input v-model:value="settings.metrics_retention_days" placeholder="30" />
              </n-form-item>
              <n-form-item label="部署备注（显示在部署面板）">
                <n-input v-model:value="settings.deploy_host_note" type="textarea" :autosize="{ minRows: 2, maxRows: 4 }" placeholder="例如：本机仅限内网访问，模型端口请通过网关调用" />
              </n-form-item>
              <n-button type="primary" :loading="savingSettings" :disabled="!isAdmin" @click="saveSettings">保存设置</n-button>
            </n-form>
          </div>

          <div class="right-col">
            <div class="glass panel">
              <div class="section-title"><span class="dot" />运行环境</div>
              <div class="kv"><span class="k">版本</span><span class="v">{{ platform.overview?.edition }} v{{ platform.overview?.version }}</span></div>
              <div class="kv"><span class="k">Docker</span><span class="v">{{ platform.overview?.engine?.connected ? `v${platform.overview?.engine?.version}` : '未连接' }}</span></div>
              <div class="kv"><span class="k">引擎地址</span><span class="v mono">{{ platform.overview?.engine?.host }}</span></div>
              <div class="kv"><span class="k">操作系统</span><span class="v">{{ platform.overview?.host?.platform }} {{ platform.overview?.host?.platform_version }}</span></div>
              <div class="kv"><span class="k">容器运行时</span><span class="v">{{ platform.overview?.engine?.driver || '-' }}</span></div>
              <div class="kv"><span class="k">运行时长</span><span class="v">{{ Math.floor((platform.overview?.uptime_seconds || 0) / 3600) }} 小时</span></div>
            </div>
            <div class="glass panel">
              <div class="section-title"><span class="dot" />安全建议</div>
              <ul class="tips">
                <li>首次登录后立即修改默认密码，并删除或降级演示账号。</li>
                <li>宿主机权限容器仅授予可信的 AI 应用；不需要时请关闭。</li>
                <li>生产环境请通过反向代理启用 HTTPS，并限制 /api 访问来源。</li>
                <li>API Key 仅在创建时展示一次，按业务独立签发并设置配额。</li>
              </ul>
            </div>
          </div>
        </div>
      </n-tab-pane>

      <n-tab-pane name="about" tab="关于">
        <div class="about-grid">
          <div class="glass panel">
            <div class="about-logo">
              <svg viewBox="0 0 64 64"><path d="M18 46 L32 16 L46 46" fill="none" stroke="url(#ag)" stroke-width="5" stroke-linecap="round" stroke-linejoin="round" /><defs><linearGradient id="ag" x1="0" y1="0" x2="1" y2="1"><stop offset="0" stop-color="#2563eb" /><stop offset="1" stop-color="#14b8a6" /></linearGradient></defs><circle cx="32" cy="40" r="3.6" fill="#2563eb" /></svg>
            </div>
            <h2>AIDash · 单机 AI 操作系统</h2>
            <p>
              基于 DevDash 运维面板改造的 AI 操作系统。采用模块化架构（模型服务 / AI 应用 / 容器底座 /
              可观测 / 全局管理），不依赖 Kubernetes，直接用 Docker Engine API 实现单机编排。
            </p>
            <div class="arch">
              <div class="arch-row"><b>驾驶舱</b><span>资源趋势、服务状态、告警与用量</span></div>
              <div class="arch-row"><b>模型广场</b><span>Ollama / vLLM 模型一键部署</span></div>
              <div class="arch-row"><b>模型服务</b><span>生命周期、日志、控制台、终端</span></div>
              <div class="arch-row"><b>AI 应用</b><span>Open WebUI / n8n / 向量库 / 运维沙箱</span></div>
              <div class="arch-row"><b>容器引擎</b><span>容器、镜像、网络、数据卷</span></div>
              <div class="arch-row"><b>AI 网关</b><span>OpenAI 兼容路由、Key、Token 计量</span></div>
              <div class="arch-row"><b>全局管理</b><span>用户角色、审计、设置</span></div>
            </div>
          </div>
          <div class="right-col">
            <div class="glass panel">
              <div class="section-title"><span class="dot" />技术栈</div>
              <div class="kv"><span class="k">后端</span><span class="v">Go 1.26 · Gin · SQLite (WAL)</span></div>
              <div class="kv"><span class="k">前端</span><span class="v">Vue 3 · TypeScript · Naive UI · ECharts</span></div>
              <div class="kv"><span class="k">运行时</span><span class="v">Docker Engine API（unix socket / npipe）</span></div>
              <div class="kv"><span class="k">认证</span><span class="v">JWT + RBAC + API Key (SHA-256)</span></div>
              <div class="kv"><span class="k">终端</span><span class="v">xterm.js + PTY / exec hijack</span></div>
            </div>
            <div class="glass panel">
              <div class="section-title"><span class="dot" />部署方式</div>
              <p class="about-text">
                使用 <code>docker compose</code> 一键启动，挂载 docker.sock 与宿主机根目录即可获得完整能力。
                详见项目 <code>README.md</code> 与 <code>deploy/</code> 目录。
              </p>
            </div>
          </div>
        </div>
      </n-tab-pane>
    </n-tabs>

    <n-modal v-model:show="createOpen" preset="card" title="新建用户" style="width: 460px">
      <n-form label-placement="top">
        <n-form-item label="用户名"><n-input v-model:value="newUser.username" placeholder="例如 ops01" /></n-form-item>
        <n-form-item label="初始密码（至少 8 位）"><n-input v-model:value="newUser.password" type="password" show-password-on="click" /></n-form-item>
        <n-form-item label="角色"><n-select v-model:value="newUser.role" :options="roleOptions" /></n-form-item>
      </n-form>
      <template #footer>
        <div class="row gap-12" style="justify-content: flex-end">
          <n-button quaternary @click="createOpen = false">取消</n-button>
          <n-button type="primary" :loading="creating" @click="createUser">创建</n-button>
        </div>
      </template>
    </n-modal>

    <n-modal v-model:show="editOpen" preset="card" :title="`编辑用户 ${editUser?.username || ''}`" style="width: 460px">
      <n-form label-placement="top">
        <n-form-item label="角色"><n-select v-model:value="editForm.role" :options="roleOptions" /></n-form-item>
        <n-form-item label="重置密码（留空则不修改）"><n-input v-model:value="editForm.password" type="password" show-password-on="click" /></n-form-item>
      </n-form>
      <template #footer>
        <div class="row gap-12" style="justify-content: flex-end">
          <n-button quaternary @click="editOpen = false">取消</n-button>
          <n-button type="primary" @click="saveEdit">保存</n-button>
        </div>
      </template>
    </n-modal>
  </div>
</template>

<style scoped>
.panel-head { display: flex; align-items: center; justify-content: space-between; gap: 12px; margin-bottom: 12px; }
.table-wrap { overflow: auto; }
.tbl { width: 100%; border-collapse: collapse; font-size: 11.5px; }
.tbl th { text-align: left; font-size: 10px; color: #7d8bab; padding: 11px 14px; border-bottom: 1px solid var(--line); font-weight: 600; white-space: nowrap; }
.tbl td { padding: 11px 14px; border-bottom: 1px solid rgba(255,255,255,0.04); color: #c3cee6; }
.detail { max-width: 300px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.avatar-sm { width: 28px; height: 28px; border-radius: 8px; display: grid; place-items: center; font-size: 12px; font-weight: 700; color: #c4b5fd; background: rgba(59,130,246,0.15); border: 1px solid rgba(59,130,246,0.25); }
.tip-box { margin-top: 12px; font-size: 11.5px; color: var(--muted); }

.settings-grid { display: grid; grid-template-columns: 1.4fr 1fr; gap: 16px; }
.panel { padding: 18px 20px; }
.right-col { display: flex; flex-direction: column; gap: 16px; }
.tips { margin: 0; padding-left: 18px; font-size: 11.5px; color: #8b98b8; line-height: 2; }
.tips li::marker { color: #3b82f6; }

.about-grid { display: grid; grid-template-columns: 1.4fr 1fr; gap: 16px; }
.about-logo { width: 56px; height: 56px; border-radius: 16px; display: grid; place-items: center; background: rgba(59,130,246,0.08); border: 1px solid rgba(59,130,246,0.35); margin-bottom: 14px; }
.about-logo svg { width: 34px; height: 34px; }
.about-grid h2 { font-size: 19px; margin: 0 0 10px; letter-spacing: -0.4px; }
.about-grid p { font-size: 12px; color: #8b98b8; line-height: 1.9; margin: 0 0 16px; }
.arch { display: flex; flex-direction: column; gap: 7px; }
.arch-row { display: grid; grid-template-columns: 90px 1fr; gap: 12px; font-size: 11.5px; padding: 7px 10px; border-radius: 8px; background: rgba(255,255,255,0.025); border: 1px solid rgba(255,255,255,0.05); }
.arch-row b { color: #c4b5fd; font-weight: 600; }
.arch-row span { color: var(--muted); }
.about-text { font-size: 11.5px; color: #8b98b8; line-height: 1.9; }
.about-text code { color: #a5f3fc; font-size: 11px; }

@media (max-width: 1100px) {
  .settings-grid, .about-grid { grid-template-columns: 1fr; }
}
</style>
