<script setup lang="ts">
import { computed, h, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useMessage } from 'naive-ui'
import {
  GridOutline, SparklesOutline, LayersOutline, RocketOutline, CubeOutline,
  PulseOutline, KeyOutline, SettingsOutline, ChatbubbleEllipsesOutline,
  ServerOutline, LogOutOutline, PersonCircleOutline, AddCircleOutline, LockClosedOutline,
  HardwareChipOutline, CloudDownloadOutline,
} from '@vicons/ionicons5'
import { useAuthStore } from '@/stores/auth'
import { usePlatformStore } from '@/stores/platform'
import { authApi } from '@/api'
import { fmtPct, roleLabel, fmtUptime } from '@/utils/format'
import CopilotDrawer from '@/components/CopilotDrawer.vue'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()
const platform = usePlatformStore()
const message = useMessage()

const navGroups = [
  {
    label: 'AI 能力',
    items: [
      { to: '/models', label: '模型广场', icon: SparklesOutline },
      { to: '/hub', label: '模型仓库', icon: CloudDownloadOutline },
      { to: '/services', label: '模型服务', icon: LayersOutline },
      { to: '/apps', label: 'AI 应用', icon: RocketOutline },
    ],
  },
  {
    label: '基础设施',
    items: [
      { to: '/containers', label: '容器引擎', icon: CubeOutline },
      { to: '/observability', label: '可观测', icon: PulseOutline },
      { to: '/gateway', label: 'AI 网关', icon: KeyOutline },
    ],
  },
  {
    label: '平台',
    items: [{ to: '/admin', label: '全局管理', icon: SettingsOutline }],
  },
]

const title = computed(() => (route.meta.title as string) || '驾驶舱')
const isActive = (to: string) => route.path === to || route.path.startsWith(to + '/')
const copilotOpen = ref(false)
const pwdOpen = ref(false)
const pwdForm = ref({ old: '', next: '', confirm: '' })
const savingPwd = ref(false)

const cpu = computed(() => platform.overview?.metrics?.cpu?.usage_percent ?? 0)
const mem = computed(() => platform.overview?.metrics?.memory?.usage_percent ?? 0)
const engineConnected = computed(() => platform.overview?.engine?.connected)
const hostName = computed(() => platform.overview?.host?.hostname || '本机')
const uptime = computed(() => fmtUptime(platform.overview?.uptime_seconds))

const userOptions = computed(() => [
  { label: `${auth.user?.username} · ${roleLabel(auth.user?.role)}`, key: 'profile', icon: () => h(PersonCircleOutline) },
  { label: '修改密码', key: 'password', icon: () => h(LockClosedOutline) },
  { type: 'divider', key: 'd1' },
  { label: '退出登录', key: 'logout', icon: () => h(LogOutOutline) },
])

function onUserSelect(key: string) {
  if (key === 'logout') {
    auth.logout()
    router.push({ name: 'login' })
  } else if (key === 'password') {
    pwdForm.value = { old: '', next: '', confirm: '' }
    pwdOpen.value = true
  }
}

async function submitPassword() {
  if (!pwdForm.value.old || !pwdForm.value.next) {
    message.warning('请填写完整')
    return
  }
  if (pwdForm.value.next.length < 8) {
    message.warning('新密码至少 8 位')
    return
  }
  if (pwdForm.value.next !== pwdForm.value.confirm) {
    message.warning('两次输入的新密码不一致')
    return
  }
  savingPwd.value = true
  try {
    await authApi.changePassword(pwdForm.value.old, pwdForm.value.next)
    message.success('密码已更新')
    pwdOpen.value = false
  } catch (e: any) {
    message.error(e.message)
  } finally {
    savingPwd.value = false
  }
}

let timer: number | undefined
onMounted(() => {
  platform.refresh()
  timer = window.setInterval(() => platform.refresh(), 15000)
})
onBeforeUnmount(() => {
  if (timer) window.clearInterval(timer)
})
watch(() => route.path, () => {
  if (!platform.overview) platform.refresh()
})
</script>

<template>
  <div class="shell">
    <aside class="sidebar">
      <div class="brand">
        <div class="logo">
          <svg viewBox="0 0 64 64" aria-hidden="true">
            <defs>
              <linearGradient id="lg" x1="0" y1="0" x2="1" y2="1">
                <stop offset="0" stop-color="#8b6dff" />
                <stop offset="1" stop-color="#22d3ee" />
              </linearGradient>
            </defs>
            <path d="M18 46 L32 16 L46 46" fill="none" stroke="url(#lg)" stroke-width="5" stroke-linecap="round" stroke-linejoin="round" />
            <circle cx="32" cy="40" r="3.6" fill="url(#lg)" />
          </svg>
        </div>
        <div class="brand-text">
          <div class="brand-name">AIDash</div>
          <div class="brand-sub">单机 AI 操作系统</div>
        </div>
      </div>

      <nav class="nav">
        <div class="nav-single">
          <router-link to="/" class="nav-item" :class="{ active: route.path === '/' }">
            <n-icon :component="GridOutline" :size="18" />
            <span>驾驶舱</span>
          </router-link>
        </div>
        <div v-for="group in navGroups" :key="group.label" class="nav-group">
          <div class="nav-label">{{ group.label }}</div>
          <router-link
            v-for="item in group.items"
            :key="item.to"
            :to="item.to"
            class="nav-item"
            :class="{ active: isActive(item.to) }"
          >
            <n-icon :component="item.icon" :size="18" />
            <span>{{ item.label }}</span>
            <span v-if="item.to === '/services' && platform.overview?.services?.unhealthy" class="nav-badge">
              {{ platform.overview?.services?.unhealthy }}
            </span>
          </router-link>
        </div>
      </nav>

      <div class="host-card">
        <div class="row gap-8" style="margin-bottom: 10px">
          <n-icon :component="ServerOutline" :size="15" style="color: #8b98b8" />
          <span class="host-name">{{ hostName }}</span>
          <span class="spacer" />
          <span class="dot" :class="engineConnected ? 'on' : 'off'" />
        </div>
        <div class="meter">
          <div class="meter-row"><span>CPU</span><b>{{ fmtPct(cpu) }}</b></div>
          <div class="meter-track"><i :style="{ width: Math.min(100, cpu) + '%', background: 'linear-gradient(90deg,#7c5cff,#a78bfa)' }" /></div>
        </div>
        <div class="meter">
          <div class="meter-row"><span>内存</span><b>{{ fmtPct(mem) }}</b></div>
          <div class="meter-track"><i :style="{ width: Math.min(100, mem) + '%', background: 'linear-gradient(90deg,#22d3ee,#34d399)' }" /></div>
        </div>
        <div class="host-foot">
          <span>运行 {{ uptime }}</span>
          <span>v{{ platform.overview?.version || '2.0.0' }}</span>
        </div>
      </div>
    </aside>

    <main class="main">
      <header class="topbar">
        <div class="crumb">
          <span class="crumb-root">AIDash</span>
          <span class="crumb-sep">/</span>
          <span class="crumb-cur">{{ title }}</span>
        </div>
        <div class="spacer" />
        <div class="engine-chip">
          <span class="dot" :class="engineConnected ? 'on' : 'off'" />
          <span class="engine-text">{{ engineConnected ? `Docker ${platform.overview?.engine?.version || ''}` : 'Docker 未连接' }}</span>
        </div>
        <button class="copilot-btn" @click="copilotOpen = true">
          <n-icon :component="ChatbubbleEllipsesOutline" :size="16" />
          <span>AI 运维助手</span>
        </button>
        <button class="deploy-btn" @click="router.push('/models')">
          <n-icon :component="AddCircleOutline" :size="16" />
          <span>一键部署</span>
        </button>
        <n-dropdown trigger="click" :options="userOptions" @select="onUserSelect">
          <div class="avatar">
            <n-icon :component="PersonCircleOutline" :size="20" />
          </div>
        </n-dropdown>
      </header>

      <div class="content-scroll">
        <router-view v-slot="{ Component }">
          <transition name="fade" mode="out-in">
            <component :is="Component" />
          </transition>
        </router-view>
      </div>
    </main>

    <CopilotDrawer v-model:show="copilotOpen" />

    <n-modal v-model:show="pwdOpen" preset="card" title="修改密码" style="width: 420px">
      <n-form label-placement="top">
        <n-form-item label="原密码"><n-input v-model:value="pwdForm.old" type="password" show-password-on="click" /></n-form-item>
        <n-form-item label="新密码（至少 8 位）"><n-input v-model:value="pwdForm.next" type="password" show-password-on="click" /></n-form-item>
        <n-form-item label="确认新密码"><n-input v-model:value="pwdForm.confirm" type="password" show-password-on="click" /></n-form-item>
      </n-form>
      <template #footer>
        <div class="row gap-12" style="justify-content: flex-end">
          <n-button quaternary @click="pwdOpen = false">取消</n-button>
          <n-button type="primary" :loading="savingPwd" @click="submitPassword">保存</n-button>
        </div>
      </template>
    </n-modal>
  </div>
</template>

<style scoped>
.shell { display: flex; height: 100vh; overflow: hidden; }

.sidebar {
  width: 242px; flex: none; display: flex; flex-direction: column;
  padding: 20px 14px 16px; gap: 18px;
  background: linear-gradient(180deg, rgba(14, 19, 36, 0.92), rgba(8, 11, 22, 0.96));
  border-right: 1px solid var(--line);
  backdrop-filter: blur(20px);
}

.brand { display: flex; align-items: center; gap: 12px; padding: 4px 8px 0; }
.logo {
  width: 42px; height: 42px; border-radius: 13px; display: grid; place-items: center;
  background: radial-gradient(120% 120% at 0% 0%, rgba(124,92,255,0.35), rgba(15,21,38,0.9));
  border: 1px solid rgba(124,92,255,0.35);
  box-shadow: 0 8px 24px rgba(124,92,255,0.25);
}
.logo svg { width: 26px; height: 26px; }
.brand-name { font-size: 17px; font-weight: 750; letter-spacing: 0.2px; }
.brand-sub { font-size: 10.5px; color: var(--muted); letter-spacing: 0.4px; margin-top: 1px; }

.nav { flex: 1; overflow-y: auto; display: flex; flex-direction: column; gap: 16px; padding: 4px 2px; }
.nav-group { display: flex; flex-direction: column; gap: 3px; }
.nav-label { font-size: 10.5px; color: #5d6a89; letter-spacing: 1.2px; padding: 0 12px 6px; text-transform: uppercase; }
.nav-item {
  display: flex; align-items: center; gap: 11px; padding: 10px 12px; border-radius: 10px;
  color: #9aa7c4; font-size: 13.5px; font-weight: 550; transition: all 0.16s ease; position: relative;
}
.nav-item:hover { color: #e9eefc; background: rgba(255,255,255,0.045); }
.nav-item.active {
  color: #fff;
  background: linear-gradient(90deg, rgba(124,92,255,0.24), rgba(124,92,255,0.06));
  box-shadow: inset 0 0 0 1px rgba(124,92,255,0.22);
}
.nav-item.active::before {
  content: ''; position: absolute; left: -2px; top: 9px; bottom: 9px; width: 3px; border-radius: 3px;
  background: linear-gradient(180deg, #a78bfa, #22d3ee);
}
.nav-badge {
  margin-left: auto; background: rgba(251,113,133,0.16); color: #fda4af; font-size: 10.5px;
  padding: 1px 7px; border-radius: 999px; border: 1px solid rgba(251,113,133,0.3);
}

.host-card {
  border: 1px solid var(--line); border-radius: 14px; padding: 13px 14px 11px;
  background: linear-gradient(180deg, rgba(255,255,255,0.045), rgba(255,255,255,0.015));
}
.host-name { font-size: 12.5px; color: var(--text-dim); font-weight: 600; max-width: 120px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.dot { width: 7px; height: 7px; border-radius: 50%; background: #475569; }
.dot.on { background: #34d399; box-shadow: 0 0 8px rgba(52,211,153,0.8); }
.dot.off { background: #fb7185; box-shadow: 0 0 8px rgba(251,113,133,0.6); }
.meter { margin-top: 9px; }
.meter-row { display: flex; justify-content: space-between; font-size: 11px; color: var(--muted); margin-bottom: 4px; }
.meter-row b { color: #b9c5dd; font-weight: 600; }
.meter-track { height: 4px; border-radius: 99px; background: rgba(255,255,255,0.07); overflow: hidden; }
.meter-track i { display: block; height: 100%; border-radius: 99px; transition: width 0.6s ease; }
.host-foot { display: flex; justify-content: space-between; margin-top: 11px; font-size: 10.5px; color: #5d6a89; }

.main { flex: 1; display: flex; flex-direction: column; min-width: 0; }
.topbar {
  height: 62px; flex: none; display: flex; align-items: center; gap: 12px;
  padding: 0 26px; border-bottom: 1px solid var(--line);
  background: rgba(8, 12, 22, 0.55); backdrop-filter: blur(16px);
}
.crumb { display: flex; align-items: center; gap: 8px; font-size: 13px; }
.crumb-root { color: var(--muted); }
.crumb-sep { color: #3d4964; }
.crumb-cur { color: var(--text); font-weight: 650; }
.engine-chip {
  display: flex; align-items: center; gap: 7px; padding: 6px 11px; border-radius: 999px;
  border: 1px solid var(--line); background: rgba(255,255,255,0.03); font-size: 11.5px; color: var(--text-dim);
}
.copilot-btn, .deploy-btn {
  display: inline-flex; align-items: center; gap: 7px; padding: 8px 14px; border-radius: 9px;
  font-size: 12.5px; font-weight: 600; cursor: pointer; font-family: inherit;
  border: 1px solid var(--line); background: rgba(255,255,255,0.04); color: var(--text); transition: all 0.16s ease;
}
.copilot-btn:hover { border-color: rgba(124,92,255,0.45); background: rgba(124,92,255,0.12); }
.deploy-btn { border: none; color: #fff; background: linear-gradient(135deg, #7c5cff, #5b6ff0 55%, #22b8d4); box-shadow: 0 8px 22px rgba(92,111,240,0.32); }
.deploy-btn:hover { filter: brightness(1.08); transform: translateY(-1px); }
.avatar {
  width: 34px; height: 34px; border-radius: 10px; display: grid; place-items: center; cursor: pointer;
  color: #c4b5fd; border: 1px solid var(--line); background: rgba(124,92,255,0.1);
}
.content-scroll { flex: 1; overflow-y: auto; overflow-x: hidden; }

@media (max-width: 900px) {
  .sidebar { display: none; }
}
</style>
