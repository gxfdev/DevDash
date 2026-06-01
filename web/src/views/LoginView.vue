<template>
  <div class="login-container">
    <div class="login-card">
      <div class="login-header">
        <h1>WebShell</h1>
        <p>Linux Web 终端管理工具</p>
      </div>
      <n-form ref="formRef" :model="form" :rules="rules" @submit.prevent="handleLogin">
        <n-form-item path="username" label="用户名">
          <n-input v-model:value="form.username" placeholder="请输入用户名" @keydown.enter="handleLogin" />
        </n-form-item>
        <n-form-item path="password" label="密码">
          <n-input v-model:value="form.password" type="password" show-password-on="click" placeholder="请输入密码" @keydown.enter="handleLogin" />
        </n-form-item>
        <n-button type="primary" block :loading="loading" @click="handleLogin">登 录</n-button>
      </n-form>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive } from 'vue'
import { useRouter } from 'vue-router'
import { useMessage } from 'naive-ui'
import { useAuthStore } from '../stores/auth'

const router = useRouter()
const message = useMessage()
const authStore = useAuthStore()
const loading = ref(false)

const form = reactive({ username: '', password: '' })
const rules = {
  username: { required: true, message: '请输入用户名', trigger: 'blur' },
  password: { required: true, message: '请输入密码', trigger: 'blur' },
}

async function handleLogin() {
  if (!form.username || !form.password) {
    message.warning('请输入用户名和密码')
    return
  }
  loading.value = true
  try {
    await authStore.login(form.username, form.password)
    message.success('登录成功')
    router.push('/')
  } catch (err: any) {
    message.error(err.response?.data?.error || '登录失败')
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.login-container {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 100vh;
  background: linear-gradient(135deg, #0c0c1d 0%, #1a1a2e 50%, #16213e 100%);
}
.login-card {
  width: 380px;
  padding: 40px;
  background: rgba(255, 255, 255, 0.05);
  border: 1px solid rgba(255, 255, 255, 0.1);
  border-radius: 16px;
  backdrop-filter: blur(20px);
}
.login-header {
  text-align: center;
  margin-bottom: 32px;
}
.login-header h1 {
  font-family: 'Courier New', monospace;
  font-size: 32px;
  color: #0f0;
  margin: 0 0 8px;
}
.login-header p {
  color: rgba(255, 255, 255, 0.5);
  font-size: 14px;
  margin: 0;
}
</style>
