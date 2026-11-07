<script setup lang="ts">
import { ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useMessage } from 'naive-ui'
import { useAuthStore } from '@/stores/auth'
import { SparklesOutline, CubeOutline, FlashOutline, ShieldCheckmarkOutline } from '@vicons/ionicons5'

const auth = useAuthStore()
const router = useRouter()
const route = useRoute()
const message = useMessage()

const username = ref('')
const password = ref('')
const loading = ref(false)

const features = [
  { icon: SparklesOutline, title: '一键部署开源大模型', desc: 'Ollama / vLLM 运行时，分钟级拉起 OpenAI 兼容接口' },
  { icon: CubeOutline, title: '纯 Docker 单机架构', desc: '无需 K8s，容器编排、镜像、卷与网络统一管理' },
  { icon: FlashOutline, title: 'AI 网关与 Token 计量', desc: 'API Key、限流配额、调用统计与模型路由' },
  { icon: ShieldCheckmarkOutline, title: '容器化运维模式', desc: '按需授权宿主机权限，AI 助手可在沙箱内排障' },
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
    <section class="hero">
      <div class="hero-top">
        <div class="logo">
          <svg viewBox="0 0 64 64">
            <defs>
              <linearGradient id="g1" x1="0" y1="0" x2="1" y2="1">
                <stop offset="0" stop-color="#a78bfa" /><stop offset="1" stop-color="#22d3ee" />
              </linearGradient>
            </defs>
            <path d="M18 46 L32 16 L46 46" fill="none" stroke="url(#g1)" stroke-width="5" stroke-linecap="round" stroke-linejoin="round" />
            <circle cx="32" cy="40" r="3.6" fill="url(#g1)" />
          </svg>
        </div>
        <span class="brand">AIDash</span>
        <span class="tag">单机 AI 操作系统</span>
      </div>

      <h1>把一台服务器<br /><span class="grad-text">变成 AI 生产力平台</span></h1>
      <p class="lead">
        对标 d.run 的云原生 AI 架构，但只需要一台 Linux + Docker。
        模型广场、模型服务、AI 应用、网关计量、可观测与权限体系，全部收敛到一个面板。
      </p>

      <div class="feature-grid">
        <div v-for="f in features" :key="f.title" class="feature">
          <div class="f-icon"><n-icon :component="f.icon" :size="18" /></div>
          <div>
            <div class="f-title">{{ f.title }}</div>
            <div class="f-desc">{{ f.desc }}</div>
          </div>
        </div>
      </div>

      <div class="hero-foot">
        <span class="pill">Docker Engine API</span>
        <span class="pill">OpenAI Compatible</span>
        <span class="pill">RBAC · Audit</span>
        <span class="pill">Self-hosted</span>
      </div>

      <div class="orbit">
        <div class="ring r1" /><div class="ring r2" /><div class="ring r3" />
        <div class="core">AI</div>
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
            <n-input v-model:value="username" placeholder="admin" autofocus>
              <template #prefix><n-icon :component="SparklesOutline" /></template>
            </n-input>
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
      <div class="panel-foot">AIDash v2.0 · Single-host AI OS · Docker Edition</div>
    </section>
  </div>
</template>

<style scoped>
.login { min-height: 100vh; display: grid; grid-template-columns: 1.25fr 1fr; }

.hero { position: relative; padding: 46px 58px; display: flex; flex-direction: column; justify-content: center; overflow: hidden; }
.hero::before {
  content: ''; position: absolute; width: 620px; height: 620px; right: -160px; top: -180px;
  background: radial-gradient(circle, rgba(124,92,255,0.22), transparent 65%);
  filter: blur(10px);
}
.hero-top { display: flex; align-items: center; gap: 10px; margin-bottom: 46px; }
.logo {
  width: 44px; height: 44px; border-radius: 13px; display: grid; place-items: center;
  background: radial-gradient(120% 120% at 0% 0%, rgba(124,92,255,0.4), rgba(15,21,38,0.9));
  border: 1px solid rgba(124,92,255,0.4);
}
.logo svg { width: 27px; height: 27px; }
.brand { font-size: 19px; font-weight: 750; letter-spacing: 0.3px; }
.tag {
  font-size: 11px; color: #c4b5fd; border: 1px solid rgba(124,92,255,0.35);
  background: rgba(124,92,255,0.12); padding: 3px 9px; border-radius: 999px;
}

h1 { font-size: 46px; line-height: 1.18; margin: 0 0 18px; letter-spacing: -1.2px; font-weight: 760; }
.lead { color: #9aa7c4; font-size: 14px; line-height: 1.85; max-width: 560px; margin: 0 0 36px; }

.feature-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 14px 22px; max-width: 620px; }
.feature { display: flex; gap: 12px; align-items: flex-start; }
.f-icon {
  width: 34px; height: 34px; border-radius: 10px; flex: none; display: grid; place-items: center; color: #c4b5fd;
  background: rgba(124,92,255,0.13); border: 1px solid rgba(124,92,255,0.25);
}
.f-title { font-size: 13px; font-weight: 650; margin-bottom: 3px; }
.f-desc { font-size: 11.5px; color: #7d8bab; line-height: 1.6; }

.hero-foot { display: flex; gap: 8px; flex-wrap: wrap; margin-top: 40px; }
.pill {
  font-size: 11px; color: #8b98b8; border: 1px solid var(--line); border-radius: 999px; padding: 4px 11px;
  background: rgba(255,255,255,0.03);
}

.orbit { position: absolute; right: 6%; bottom: 12%; width: 210px; height: 210px; opacity: 0.8; }
.ring { position: absolute; inset: 0; border-radius: 50%; border: 1px solid rgba(124,92,255,0.18); }
.ring.r2 { inset: 26px; border-color: rgba(34,211,238,0.18); }
.ring.r3 { inset: 52px; border-color: rgba(255,255,255,0.1); }
.core {
  position: absolute; inset: 78px; border-radius: 50%; display: grid; place-items: center;
  background: linear-gradient(135deg, rgba(124,92,255,0.35), rgba(34,211,238,0.22));
  border: 1px solid rgba(255,255,255,0.16); font-weight: 800; font-size: 20px; color: #e9eefc;
  box-shadow: 0 0 60px rgba(124,92,255,0.35);
}

.panel { display: flex; flex-direction: column; justify-content: center; padding: 40px 56px; border-left: 1px solid var(--line); background: rgba(9, 13, 24, 0.55); backdrop-filter: blur(20px); }
.panel-inner { width: 100%; max-width: 380px; margin: 0 auto; }
.panel-head h2 { font-size: 24px; margin: 0 0 8px; letter-spacing: -0.4px; }
.panel-head p { font-size: 12.5px; color: var(--muted); line-height: 1.7; margin: 0 0 28px; }
.panel-tip { margin-top: 22px; font-size: 11.5px; color: #5d6a89; text-align: center; line-height: 1.7; }
.panel-foot { margin-top: auto; padding-top: 40px; text-align: center; font-size: 10.5px; color: #3f4a66; letter-spacing: 0.3px; }

@media (max-width: 1000px) {
  .login { grid-template-columns: 1fr; }
  .hero { display: none; }
  .panel { border-left: none; }
}
</style>
