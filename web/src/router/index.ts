import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from '@/stores/auth'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/login', name: 'login', component: () => import('@/views/LoginView.vue'), meta: { public: true } },
    {
      path: '/',
      component: () => import('@/components/AppShell.vue'),
      children: [
        { path: '', name: 'dashboard', component: () => import('@/views/DashboardView.vue'), meta: { title: '驾驶舱' } },
        { path: 'models', name: 'models', component: () => import('@/views/ModelsView.vue'), meta: { title: '模型广场' } },
        { path: 'hub', name: 'hub', component: () => import('@/views/HubView.vue'), meta: { title: '模型仓库' } },
        { path: 'services', name: 'services', component: () => import('@/views/ServicesView.vue'), meta: { title: '模型服务' } },
        { path: 'apps', name: 'apps', component: () => import('@/views/AppsView.vue'), meta: { title: 'AI 应用' } },
        { path: 'containers', name: 'containers', component: () => import('@/views/ContainersView.vue'), meta: { title: '容器引擎' } },
        { path: 'observability', name: 'observability', component: () => import('@/views/ObservabilityView.vue'), meta: { title: '可观测' } },
        { path: 'gateway', name: 'gateway', component: () => import('@/views/GatewayView.vue'), meta: { title: 'AI 网关' } },
        { path: 'admin', name: 'admin', component: () => import('@/views/AdminView.vue'), meta: { title: '全局管理' } },
      ],
    },
    { path: '/:pathMatch(.*)*', redirect: '/' },
  ],
})

router.beforeEach((to) => {
  const auth = useAuthStore()
  if (!to.meta.public && !auth.isLoggedIn) {
    return { name: 'login', query: { redirect: to.fullPath } }
  }
  if (to.name === 'login' && auth.isLoggedIn) {
    return { name: 'dashboard' }
  }
  return true
})

export default router
