<template>
  <div class="layout">
    <!-- 侧边栏 -->
    <aside class="sidebar">
      <div class="logo">
        <span class="logo-icon">🦞</span>
        <span class="logo-text">DevDash</span>
      </div>

      <n-menu
        v-model:value="activeKey"
        :options="menuOptions"
        :collapsed="sidebarCollapsed"
        :collapsed-width="64"
        :collapsed-icon-size="22"
        @update:value="onMenu"
      />

      <div class="sidebar-footer">
        <n-button text style="width:100%;color:#6e7681;font-size:12px" @click="sidebarCollapsed = !sidebarCollapsed">
          {{ sidebarCollapsed ? '→' : '← 收起' }}
        </n-button>
      </div>
    </aside>

    <!-- 主内容 -->
    <div class="main-wrapper">
      <!-- 顶栏 -->
      <header class="topbar">
        <div class="topbar-left">
          <n-breadcrumb>
            <n-breadcrumb-item>{{ activeGroup }}</n-breadcrumb-item>
            <n-breadcrumb-item>{{ activeLabel }}</n-breadcrumb-item>
          </n-breadcrumb>
        </div>
        <div class="topbar-right">
          <n-tooltip>
            <template #trigger>
              <n-button text @click="refresh">
                <template #icon><refresh-icon /></template>
              </n-button>
            </template>
            刷新数据
          </n-tooltip>
          <n-dropdown :options="userMenuOpts" @select="onUserMenu">
            <div class="user-avatar">{{ userInitial }}</div>
          </n-dropdown>
        </div>
      </header>

      <!-- 页面内容 -->
      <main class="main-content">
        <slot />
      </main>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, h } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { NButton, NTooltip, NDropdown, useMessage, useDialog } from 'naive-ui'
import { Refresh as RefreshIcon } from '@vicons/ionicons5'
import { useAuthStore } from '@/stores/auth'
import { useNodesStore } from '@/stores/nodes'

const router = useRouter()
const route = useRoute()
const authStore = useAuthStore()
const nodesStore = useNodesStore()
const message = useMessage()
const dialog = useDialog()

const sidebarCollapsed = ref(false)
const activeKey = ref(route.name as string)

const userInitial = computed(() => (authStore.username || 'A')[0].toUpperCase())

const userMenuOpts = [
  { label: '个人设置', key: 'profile' },
  { label: '系统设置', key: 'settings' },
  { type: 'divider', key: 'd1' },
  { label: '退出登录', key: 'logout' },
]

function renderIcon(icon: string) {
  return () => h('span', { style: 'font-size:16px' }, icon)
}

const menuOptions = [
  {
    label: '仪表板',
    key: 'dashboard',
    icon: renderIcon('📊'),
  },
  {
    label: '主机管理',
    key: 'host-group',
    icon: renderIcon('🖥️'),
    children: [
      { label: '主机列表', key: 'hosts', icon: renderIcon('📋') },
      { label: '软件商店', key: 'store', icon: renderIcon('🏪') },
    ],
  },
  {
    label: '文件管理',
    key: 'files',
    icon: renderIcon('📁'),
  },
  {
    label: '防火墙',
    key: 'firewall',
    icon: renderIcon('🔥'),
  },
  {
    label: '计划任务',
    key: 'cron',
    icon: renderIcon('⏰'),
  },
  {
    label: '数据库',
    key: 'database',
    icon: renderIcon('🗄️'),
  },
  {
    label: 'Web终端',
    key: 'terminal',
    icon: renderIcon('💻'),
  },
  {
    label: '告警中心',
    key: 'alerts',
    icon: renderIcon('🔔'),
  },
  {
    label: '趋势分析',
    key: 'trends',
    icon: renderIcon('📈'),
  },
  { type: 'divider', key: 'd2' },
  {
    label: 'Docker管理',
    key: 'docker',
    icon: renderIcon('🐳'),
  },
  {
    label: '系统设置',
    key: 'settings',
    icon: renderIcon('⚙️'),
  },
]

const labelMap: Record<string, string> = {
  dashboard: '仪表板',
  hosts: '主机列表',
  'host-detail': '主机详情',
  store: '软件商店',
  files: '文件管理',
  firewall: '防火墙',
  cron: '计划任务',
  database: '数据库',
  terminal: 'Web终端',
  alerts: '告警中心',
  trends: '趋势分析',
  settings: '系统设置',
}

const groupMap: Record<string, string> = {
  dashboard: '概览',
  hosts: '主机管理',
  'host-detail': '主机管理',
  store: '主机管理',
  files: '工具',
  firewall: '工具',
  cron: '工具',
  database: '工具',
  terminal: '工具',
  alerts: '运维',
  trends: '运维',
  settings: '系统',
}

const activeLabel = computed(() => labelMap[activeKey.value] || activeKey.value)
const activeGroup = computed(() => groupMap[activeKey.value] || '')

function onMenu(key: string) {
  activeKey.value = key
  router.push({ name: key })
}

function refresh() {
  nodesStore.fetchNodes()
  message.info('数据已刷新')
}

function onUserMenu(key: string) {
  if (key === 'logout') {
    dialog.warning({
      title: '确认退出',
      content: '确定要退出登录吗？',
      positiveText: '退出',
      negativeText: '取消',
      onPositiveClick: () => {
        authStore.logout()
        router.push('/login')
      },
    })
  } else if (key === 'profile' || key === 'settings') {
    router.push({ name: key })
  }
}
</script>

<style scoped>
.layout { display: flex; height: 100vh; background: #0d1117; color: #e6edf3; overflow: hidden; }

.sidebar {
  width: 220px;
  background: #161b22;
  border-right: 1px solid #30363d;
  display: flex;
  flex-direction: column;
  flex-shrink: 0;
  overflow-y: auto;
  transition: width 0.2s;
}
.sidebar::-webkit-scrollbar { width: 4px; }
.sidebar::-webkit-scrollbar-thumb { background: #30363d; border-radius: 2px; }

.logo {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 16px 20px 20px;
  border-bottom: 1px solid #21262d;
  margin-bottom: 8px;
}
.logo-icon { font-size: 24px; }
.logo-text { font-size: 18px; font-weight: 700; color: #3fb950; }

.sidebar-footer {
  margin-top: auto;
  padding: 12px 16px;
  border-top: 1px solid #21262d;
}

.main-wrapper { flex: 1; display: flex; flex-direction: column; overflow: hidden; }

.topbar {
  height: 48px;
  background: #161b22;
  border-bottom: 1px solid #30363d;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 20px;
  flex-shrink: 0;
}
.topbar-left { display: flex; align-items: center; }
.topbar-right { display: flex; align-items: center; gap: 12px; }

.user-avatar {
  width: 32px;
  height: 32px;
  border-radius: 50%;
  background: #388bfd;
  color: #fff;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 14px;
  font-weight: 600;
  cursor: pointer;
}

.main-content { flex: 1; overflow-y: auto; }
.main-content::-webkit-scrollbar { width: 8px; }
.main-content::-webkit-scrollbar-thumb { background: #30363d; border-radius: 4px; }
</style>