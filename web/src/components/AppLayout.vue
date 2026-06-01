<template>
  <n-layout has-sider style="height: 100vh">
    <n-layout-sider bordered :width="200" :collapsed-width="64" collapse-mode="width" :collapsed="collapsed" show-trigger @collapse="collapsed = true" @expand="collapsed = false">
      <div class="logo" @click="router.push('/')">
        <span v-if="!collapsed" class="logo-text">WebShell</span>
        <span v-else class="logo-icon">$_</span>
      </div>
      <n-menu :options="menuOptions" :value="currentRoute" :collapsed="collapsed" :collapsed-width="64" :collapsed-icon-size="22" @update:value="onMenu" />
    </n-layout-sider>
    <n-layout>
      <n-layout-header bordered style="height: 48px; display: flex; align-items: center; justify-content: space-between; padding: 0 20px">
        <span style="font-weight: 600">{{ pageTitle }}</span>
        <n-space align="center">
          <n-tag v-if="auth.user" size="small" :type="auth.user.role === 'admin' ? 'success' : 'info'">{{ auth.user.username }} ({{ auth.user.role }})</n-tag>
          <n-button quaternary size="small" @click="showPasswordModal = true">修改密码</n-button>
          <n-button quaternary size="small" type="error" @click="handleLogout">退出</n-button>
        </n-space>
      </n-layout-header>
      <n-layout-content style="padding: 16px; overflow: auto">
        <router-view />
      </n-layout-content>
    </n-layout>

    <n-modal v-model:show="showPasswordModal" title="修改密码" preset="dialog" positive-text="确认" negative-text="取消" @positive-click="handleChangePassword">
      <n-form :model="pwdForm">
        <n-form-item label="旧密码">
          <n-input v-model:value="pwdForm.oldPassword" type="password" />
        </n-form-item>
        <n-form-item label="新密码">
          <n-input v-model:value="pwdForm.newPassword" type="password" />
        </n-form-item>
      </n-form>
    </n-modal>
  </n-layout>
</template>

<script setup lang="ts">
import { ref, computed, reactive, h } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { NIcon, useMessage } from 'naive-ui'
import { useAuthStore } from '../stores/auth'

const router = useRouter()
const route = useRoute()
const message = useMessage()
const auth = useAuthStore()
const collapsed = ref(false)
const showPasswordModal = ref(false)

const pwdForm = reactive({ oldPassword: '', newPassword: '' })

const currentRoute = computed(() => route.name as string)

const pageTitle = computed(() => {
  const map: Record<string, string> = {
    dashboard: '系统监控',
    terminal: 'Web 终端',
    cron: '定时任务',
    files: '文件管理',
    scripts: '脚本管理',
  }
  return map[route.name as string] || 'WebShell'
})

function renderIcon(icon: string) {
  return () => h('span', { style: 'font-size: 18px' }, icon)
}

const menuOptions = [
  { label: '系统监控', key: 'dashboard', icon: renderIcon('📊') },
  { label: 'Web 终端', key: 'terminal', icon: renderIcon('💻') },
  { label: '定时任务', key: 'cron', icon: renderIcon('⏰') },
  { label: '文件管理', key: 'files', icon: renderIcon('📁') },
  { label: '脚本管理', key: 'scripts', icon: renderIcon('📜') },
]

function onMenu(key: string) {
  router.push({ name: key })
}

function handleLogout() {
  auth.logout()
  router.push('/login')
}

async function handleChangePassword() {
  if (!pwdForm.oldPassword || !pwdForm.newPassword) {
    message.warning('请填写旧密码和新密码')
    return false
  }
  try {
    await auth.changePassword(pwdForm.oldPassword, pwdForm.newPassword)
    message.success('密码已修改，请重新登录')
    auth.logout()
    router.push('/login')
  } catch (err: any) {
    message.error(err.response?.data?.error || '修改失败')
    return false
  }
  pwdForm.oldPassword = ''
  pwdForm.newPassword = ''
  return true
}
</script>

<style scoped>
.logo {
  height: 48px;
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  border-bottom: 1px solid var(--n-border-color);
}
.logo-text {
  font-family: 'Courier New', monospace;
  font-size: 20px;
  font-weight: bold;
  color: #0f0;
}
.logo-icon {
  font-family: 'Courier New', monospace;
  font-size: 18px;
  color: #0f0;
}
</style>
