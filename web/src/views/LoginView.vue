<script setup lang="ts">
import { ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useMessage } from 'naive-ui'
import { useAuthStore } from '@/stores/auth'
import { LayersOutline, CubeOutline, KeyOutline, ShieldCheckmarkOutline } from '@vicons/ionicons5'

const auth = useAuthStore()
const router = useRouter()
const route = useRoute()
const message = useMessage()

const username = ref('')
const password = ref('')
const loading = ref(false)

const features = [
  { icon: LayersOutline, title: '模型部署', desc: '从魔搭 / HuggingFace 下载权重，llama.cpp、vLLM、Ollama 一键运行' },
  { icon: CubeOutline, title: '单机 Docker', desc: '无需 Kubernetes，容器、镜像、数据卷与网络统一管理' },
  { icon: KeyOutline, title: '统一网关', desc: 'OpenAI 兼容接口、API Key、Token 计量与配额' },
  { icon: ShieldCheckmarkOutline, title: '权限与审计', desc: 'RBAC、全量操作审计、按需授予宿主机权限' },
]

async function submit() {
  if (!username.value || !password.value) {
    message.warning('请输入用户名和密码')
    return
  }
  loading.value = true
  try {
    await auth.login(username.value.trim(), password.value)
    message.success('登录成功')
    const redirect = (route.query.redirect as string) || '/'
    router.replace(redirect)
  } catch (e: any) {
    message.error(e.message || '登录失败')
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="login">
    <section class="intro">
      <div class="brand-row">
        <div class="logo">
          <svg viewBox="0 0 64 64">
            <path d="M18 46 L32 16 L46 46" fill="none" stroke="#60a5fa" stroke-width="5" stroke-linecap="round" stroke-linejoin="round" />
            <circle cx="32" cy="40" r="3.6" fill="#60a5fa" />
          </svg>
        </div>
        <span class="brand">AIDash</span>
        <span class="tag">单机 AI 操作系统</span>
      </div>

      <h1>在一台 Linux 服务器上<br />运行与管理 AI 模型</h1>
      <p class="lead">
        模型文件真实下载到服务器，以容器方式运行，通过统一网关提供 OpenAI 兼容接口。
        同时提供容器管理、监控、账号权限和操作审计。
      </p>

      <div class="feature-grid">
        <div v-for="f in features" :key="f.title" class="feature">
          <div class="f-icon"><n-icon :component="f.icon" :size="17" /></div>
          <div>
            <div class="f-title">{{ f.title }}</div>
            <div class="f-desc">{{ f.desc }}</div>
          </div>
        </div>
      </div>

      <div class="intro-foot">
        <span>Docker Engine API</span>
        <span>OpenAI 兼容</span>
        <span>RBAC · 审计</span>
        <span>私有部署</span>
      </div>
    </section>

    <section class="panel">
      <div class="panel-inner">
        <div class="panel-head">
          <h2>登录控制台</h2>
          <p>首次部署默认账号 admin / admin123，登录后请立即修改密码。</p>
        </div>

        <n-form size="large" @keyup.enter="submit">
          <n-form-item label="用户名">
            <n-input v-model:value="username" placeholder="admin" autofocus />
          </n-form-item>
          <n-form-item label="密码">
            <n-input v-model:value="password" type="password" show-password-on="click" placeholder="请输入密码" />
          </n-form-item>
          <n-button type="primary" size="large" block :loading="loading" @click="submit">进入控制台</n-button>
        </n-form>

        <div class="panel-tip">
          没有账号？请联系平台管理员在「全局管理 → 用户」中创建。
        </div>
      </div>
      <div class="panel-foot">AIDash v2.0 · Docker Edition</div>
    </section>
  </div>
</template>

<style scoped>
.login { min-height: 100vh; display: grid; grid-template-columns: 1.15fr 1fr; }

.intro { padding: 48px 56px; display: flex; flex-direction: column; justify-content: center; border-right: 1px solid var(--line); }
.brand-row { display: flex; align-items: center; gap: 10px; margin-bottom: 42px; }
.logo {
  width: 40px; height: 40px; border-radius: 10px; display: grid; place-items: center;
  background: rgba(59, 130, 246, 0.08); border: 1px solid rgba(59, 130, 246, 0.25);
}
.logo svg { width: 25px; height: 25px; }
.brand { font-size: 18px; font-weight: 700; letter-spacing: 0.2px; }
.tag {
  font-size: 11px; color: var(--text-dim); border: 1px solid var(--line);
  padding: 3px 9px; border-radius: 5px;
}

h1 { font-size: 34px; line-height: 1.35; margin: 0 0 16px; letter-spacing: -0.6px; font-weight: 650; }
.lead { color: var(--muted); font-size: 13.5px; line-height: 1.9; max-width: 560px; margin: 0 0 34px; }

.feature-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 16px 24px; max-width: 600px; }
.feature { display: flex; gap: 12px; align-items: flex-start; }
.f-icon {
  width: 32px; height: 32px; border-radius: 8px; flex: none; display: grid; place-items: center;
  color: #93c5fd; background: rgba(59, 130, 246, 0.08); border: 1px solid rgba(59, 130, 246, 0.18);
}
.f-title { font-size: 13px; font-weight: 600; margin-bottom: 3px; }
.f-desc { font-size: 11.5px; color: var(--muted); line-height: 1.65; }

.intro-foot { display: flex; gap: 18px; margin-top: 42px; font-size: 11px; color: #6b7890; flex-wrap: wrap; }

.panel { display: flex; flex-direction: column; justify-content: center; padding: 40px 56px; }
.panel-inner { width: 100%; max-width: 360px; margin: 0 auto; }
.panel-head h2 { font-size: 22px; margin: 0 0 8px; letter-spacing: -0.3px; }
.panel-head p { font-size: 12.5px; color: var(--muted); line-height: 1.7; margin: 0 0 26px; }
.panel-tip { margin-top: 22px; font-size: 11.5px; color: #6b7890; text-align: center; line-height: 1.7; }
.panel-foot { margin-top: auto; padding-top: 40px; text-align: center; font-size: 10.5px; color: #566176; letter-spacing: 0.3px; }

@media (max-width: 1000px) {
  .login { grid-template-columns: 1fr; }
  .intro { display: none; }
  .panel { border-left: none; }
}
</style>
