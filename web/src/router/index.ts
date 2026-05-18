import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from '@/stores/auth'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/login', name: 'login', component: () => import('@/views/LoginView.vue') },
    { path: '/', name: 'dashboard', component: () => import('@/views/DashboardView.vue'), meta: { requiresAuth: true } },
    { path: '/hosts', name: 'hosts', component: () => import('@/views/HostListView.vue'), meta: { requiresAuth: true } },
    { path: '/hosts/:id', name: 'host-detail', component: () => import('@/views/HostDetailView.vue'), meta: { requiresAuth: true } },
    { path: '/store', name: 'store', component: () => import('@/views/StoreView.vue'), meta: { requiresAuth: true } },
    { path: '/files/:hostId?', name: 'files', component: () => import('@/views/FileMgrView.vue'), meta: { requiresAuth: true } },
    { path: '/firewall/:hostId?', name: 'firewall', component: () => import('@/views/FirewallView.vue'), meta: { requiresAuth: true } },
    { path: '/cron/:hostId?', name: 'cron', component: () => import('@/views/CronView.vue'), meta: { requiresAuth: true } },
    { path: '/database/:hostId?', name: 'database', component: () => import('@/views/DatabaseView.vue'), meta: { requiresAuth: true } },
    { path: '/terminal/:hostId?', name: 'terminal', component: () => import('@/views/TerminalView.vue'), meta: { requiresAuth: true } },
    { path: '/alerts', name: 'alerts', component: () => import('@/views/AlertView.vue'), meta: { requiresAuth: true } },
    { path: '/trends', name: 'trends', component: () => import('@/views/TrendsView.vue'), meta: { requiresAuth: true } },
    { path: '/settings', name: 'settings', component: () => import('@/views/SettingsView.vue'), meta: { requiresAuth: true } },
  ],
})

router.beforeEach((to, from, next) => {
  const auth = useAuthStore()
  if (to.meta.requiresAuth && !auth.isLoggedIn) {
    next('/login')
  } else {
    next()
  }
})

export default router