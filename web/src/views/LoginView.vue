<template>
  <div class="login-page">
    <div class="login-card">
      <div class="logo">
        <div class="logo-icon">🦞</div>
        <h1>DevDash</h1>
        <p>轻量级运维面板</p>
      </div>

      <n-form ref="formRef" :model="form" :rules="rules" @submit.prevent="handleLogin">
        <n-form-item path="username" label="用户名">
          <n-input v-model:value="form.username" placeholder="admin" @keydown.enter="handleLogin" />
        </n-form-item>
        <n-form-item path="password" label="密码">
          <n-input
            v-model:value="form.password"
            type="password"
            placeholder="••••••••"
            show-password-on="mousedown"
            @keydown.enter="handleLogin"
          />
        </n-form-item>

        <n-button
          type="primary"
          block
          :loading="loading"
          :disabled="loading"
          @click="handleLogin"
        >
          登录
        </n-button>

        <div v-if="errorMsg" class="error-msg">
          <n-alert type="error" :show-icon="false">{{ errorMsg }}</n-alert>
        </div>
      </n-form>

      <div class="hint">默认账号：admin / admin123</div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'

const router = useRouter()
const authStore = useAuthStore()

const formRef = ref()
const loading = ref(false)
const errorMsg = ref('')

const form = reactive({ username: 'admin', password: '' })
const rules = {
  username: [{ required: true, message: '请输入用户名', trigger: 'blur' }],
  password: [{ required: true, message: '请输入密码', trigger: 'blur' }],
}

async function handleLogin() {
  errorMsg.value = ''
  try {
    await formRef.value?.validate()
  } catch {
    return
  }
  loading.value = true
  try {
    await authStore.login(form.username, form.password)
    router.push('/')
  } catch (e: any) {
    errorMsg.value = e?.response?.data?.error || '登录失败，请检查用户名和密码'
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.login-page {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: #0d1117;
}
.login-card {
  width: 360px;
  background: #161b22;
  border: 1px solid #30363d;
  border-radius: 12px;
  padding: 40px 32px;
}
.logo {
  text-align: center;
  margin-bottom: 32px;
}
.logo-icon {
  font-size: 48px;
  line-height: 1;
  margin-bottom: 8px;
}
.logo h1 {
  font-size: 24px;
  font-weight: 700;
  color: #e6edf3;
  margin: 0 0 4px;
}
.logo p {
  font-size: 13px;
  color: #8b949e;
  margin: 0;
}
.error-msg {
  margin-top: 12px;
}
.hint {
  text-align: center;
  font-size: 12px;
  color: #6e7681;
  margin-top: 16px;
}
</style>