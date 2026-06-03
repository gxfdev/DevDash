import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from '@/stores/auth'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/login', name: 'login', component: () => import('@/views/LoginView.vue') },
    { path: '/force-change-password', name: 'force-change-password', component: () => import('@/views/ForceChangePasswordView.vue') },
    { path: '/', name: 'dashboard', component: () => import('@/views/DashboardView.vue'), meta: { requiresAuth: true } },
    { path: '/files', name: 'files', component: () => import('@/views/FileMgrView.vue'), meta: { requiresAuth: true } },
    { path: '/cron', name: 'cron', component: () => import('@/views/CronView.vue'), meta: { requiresAuth: true } },
    { path: '/scripts', name: 'scripts', component: () => import('@/views/ScriptView.vue'), meta: { requiresAuth: true } },
    { path: '/terminal', name: 'terminal', component: () => import('@/views/TerminalView.vue'), meta: { requiresAuth: true } },
    { path: '/alerts', name: 'alerts', component: () => import('@/views/AlertView.vue'), meta: { requiresAuth: true } },
    { path: '/trends', name: 'trends', component: () => import('@/views/TrendsView.vue'), meta: { requiresAuth: true } },
    { path: '/settings', name: 'settings', component: () => import('@/views/SettingsView.vue'), meta: { requiresAuth: true } },
  ],
})

router.beforeEach(async (to, from, next) => {
  const auth = useAuthStore()

  if (to.meta.requiresAuth && !auth.isLoggedIn) {
    next('/login')
    return
  }

  if (auth.isLoggedIn && !auth.username && to.name !== 'force-change-password') {
    await auth.fetchMe()
    if (!auth.isLoggedIn) {
      next('/login')
      return
    }
  }

  if (auth.isLoggedIn && auth.mustChangePwd && to.name !== 'force-change-password') {
    next('/force-change-password')
    return
  }

  if (to.path === '/login' && auth.isLoggedIn && !auth.mustChangePwd) {
    next('/')
    return
  }

  next()
})

export default router
