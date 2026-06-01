import { createRouter, createWebHistory } from 'vue-router'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/login', name: 'login', component: () => import('../views/LoginView.vue') },
    {
      path: '/',
      component: () => import('../components/AppLayout.vue'),
      children: [
        { path: '', name: 'dashboard', component: () => import('../views/DashboardView.vue') },
        { path: 'terminal', name: 'terminal', component: () => import('../views/TerminalView.vue') },
        { path: 'cron', name: 'cron', component: () => import('../views/CronView.vue') },
        { path: 'files', name: 'files', component: () => import('../views/FileView.vue') },
        { path: 'scripts', name: 'scripts', component: () => import('../views/ScriptView.vue') },
      ],
    },
  ],
})

router.beforeEach((to) => {
  const token = localStorage.getItem('token')
  if (to.name !== 'login' && !token) {
    return { name: 'login' }
  }
  if (to.name === 'login' && token) {
    return { name: 'dashboard' }
  }
})

export default router
