import { defineStore } from 'pinia'
import { systemApi } from '@/api'
import type { Overview } from '@/types'

export const usePlatformStore = defineStore('platform', {
  state: () => ({
    overview: null as Overview | null,
    loading: false,
    lastError: '',
    loadedAt: 0,
  }),
  getters: {
    engineConnected: (s) => !!s.overview?.engine?.connected,
    cpu: (s) => s.overview?.metrics?.cpu?.usage_percent ?? 0,
    memory: (s) => s.overview?.metrics?.memory?.usage_percent ?? 0,
    disk: (s) => s.overview?.metrics?.disk?.usage_percent ?? 0,
    gpu: (s) => s.overview?.metrics?.gpu ?? null,
    runningServices: (s) => s.overview?.services?.running ?? 0,
    totalServices: (s) => s.overview?.services?.total ?? 0,
  },
  actions: {
    async refresh() {
      if (this.loading) return
      this.loading = true
      try {
        this.overview = await systemApi.overview()
        this.loadedAt = Date.now()
        this.lastError = ''
      } catch (e: any) {
        this.lastError = e?.message || '无法获取平台状态'
      } finally {
        this.loading = false
      }
    },
  },
})
