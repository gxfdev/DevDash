<template>
  <div class="force-page">
    <div class="force-card">
      <div class="logo">
        <span class="logo-icon">🦞</span>
        <h1>修改初始密码</h1>
      </div>
      <p class="desc">首次登录需要设置新的密码才能继续使用系统</p>

      <n-form ref="formRef" :model="form" :rules="rules" @submit.prevent="handleSubmit">
        <n-form-item path="newPassword" label="新密码">
          <n-input
            v-model:value="form.newPassword"
            type="password"
            placeholder="请输入新密码"
            show-password-on="mousedown"
            @keydown.enter="handleSubmit"
          />
        </n-form-item>
        <n-form-item path="confirmPassword" label="确认新密码">
          <n-input
            v-model:value="form.confirmPassword"
            type="password"
            placeholder="请再次输入新密码"
            show-password-on="mousedown"
            @keydown.enter="handleSubmit"
          />
        </n-form-item>

        <div class="complexity-hint">
          <div :class="['hint-item', { met: hints.minLength }]">8位以上</div>
          <div :class="['hint-item', { met: hints.upper }]">大写字母</div>
          <div :class="['hint-item', { met: hints.lower }]">小写字母</div>
          <div :class="['hint-item', { met: hints.digit }]">数字</div>
          <div :class="['hint-item', { met: hints.special }]">特殊符号</div>
        </div>

        <n-button
          type="primary"
          block
          :loading="loading"
          :disabled="loading"
          @click="handleSubmit"
        >
          确认修改
        </n-button>

        <div v-if="errorMsg" class="error-msg">
          <n-alert type="error" :show-icon="false">{{ errorMsg }}</n-alert>
        </div>
      </n-form>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { authClient, getErrorMessage } from '@/api/client'

const router = useRouter()
const authStore = useAuthStore()

const formRef = ref()
const loading = ref(false)
const errorMsg = ref('')

const form = reactive({
  newPassword: '',
  confirmPassword: '',
})

const hints = computed(() => ({
  minLength: form.newPassword.length >= 8,
  upper: /[A-Z]/.test(form.newPassword),
  lower: /[a-z]/.test(form.newPassword),
  digit: /[0-9]/.test(form.newPassword),
  special: /[!@#$%^&*()_+\-=\[\]{};':"\\|,.<>\/?`~]/.test(form.newPassword),
}))

const rules = {
  newPassword: [
    { required: true, message: '请输入新密码', trigger: 'blur' },
  ],
  confirmPassword: [
    { required: true, message: '请确认新密码', trigger: 'blur' },
    {
      validator: (_rule: any, value: string) => {
        return value === form.newPassword
      },
      message: '两次输入的密码不一致',
      trigger: 'blur',
    },
  ],
}

async function handleSubmit() {
  errorMsg.value = ''
  try {
    await formRef.value?.validate()
  } catch {
    return
  }
  if (!hints.value.minLength || !hints.value.upper || !hints.value.lower || !hints.value.digit || !hints.value.special) {
    errorMsg.value = '密码不满足复杂度要求'
    return
  }
  loading.value = true
  try {
    await authClient.put('/auth/password', {
      old: '',
      new: form.newPassword,
    })
    authStore.mustChangePwd = false
    authStore.fetchMe()
    router.push('/')
  } catch (e: unknown) {
    errorMsg.value = getErrorMessage(e, '修改密码失败')
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  if (!authStore.isLoggedIn) {
    router.push('/login')
  }
})
</script>

<style scoped>
.force-page {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: #0d1117;
}
.force-card {
  width: 400px;
  background: #161b22;
  border: 1px solid #30363d;
  border-radius: 12px;
  padding: 40px 32px;
}
.logo {
  text-align: center;
  margin-bottom: 24px;
}
.logo-icon {
  font-size: 40px;
  line-height: 1;
  margin-bottom: 8px;
}
.logo h1 {
  font-size: 20px;
  font-weight: 700;
  color: #e6edf3;
  margin: 0;
}
.desc {
  text-align: center;
  font-size: 13px;
  color: #8b949e;
  margin-bottom: 24px;
}
.complexity-hint {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin-bottom: 16px;
}
.hint-item {
  font-size: 11px;
  padding: 2px 8px;
  border-radius: 4px;
  background: #21262d;
  color: #6e7681;
}
.hint-item.met {
  background: #1a3a2a;
  color: #3fb950;
}
.error-msg {
  margin-top: 12px;
}
</style>
