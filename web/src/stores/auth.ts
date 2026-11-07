import { defineStore } from 'pinia'
import { authApi } from '@/api'
import type { User } from '@/types'

export const useAuthStore = defineStore('auth', {
  state: () => ({
    token: localStorage.getItem('aidash_token') || '',
    user: JSON.parse(localStorage.getItem('aidash_user') || 'null') as User | null,
  }),
  getters: {
    isLoggedIn: (s) => !!s.token,
    role: (s) => s.user?.role || 'viewer',
    canWrite: (s) => s.user?.role === 'admin' || s.user?.role === 'operator',
    isAdmin: (s) => s.user?.role === 'admin',
  },
  actions: {
    async login(username: string, password: string) {
      const res = await authApi.login(username, password)
      this.token = res.token
      this.user = res.user
      localStorage.setItem('aidash_token', res.token)
      localStorage.setItem('aidash_user', JSON.stringify(res.user))
      return res.user
    },
    async refresh() {
      if (!this.token) return
      const res = await authApi.me()
      this.user = res.user
      localStorage.setItem('aidash_user', JSON.stringify(res.user))
    },
    logout() {
      this.token = ''
      this.user = null
      localStorage.removeItem('aidash_token')
      localStorage.removeItem('aidash_user')
    },
  },
})
