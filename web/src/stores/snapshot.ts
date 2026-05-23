import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import client from '@/api/client'
import type { Snapshot } from '@/types'

export const useSnapshotStore = defineStore('snapshot', () => {
  const current = ref<Snapshot | null>(null)
  const history = ref<Snapshot[]>([])
  const loading = ref(false)

  async function fetchLatest() {
    loading.value = true
    try {
      const { data } = await client.get<Snapshot>('/latest')
      current.value = data
    } finally {
      loading.value = false
    }
  }

  async function fetchHistory(limit = 60) {
    try {
      const { data } = await client.get<Snapshot[]>(`/history?limit=${limit}`)
      history.value = Array.isArray(data) ? data : []
    } catch {
      history.value = []
    }
  }

  async function triggerCollect() {
    const { data } = await client.post<{ snapshot: Snapshot }>('/collect')
    current.value = data.snapshot
    return data
  }

  const cpuPercent = computed(() => current.value?.cpu.usage_percent ?? 0)
  const memPercent = computed(() => current.value?.memory.usage_percent ?? 0)
  const diskPercent = computed(() => current.value?.disk.usage_percent ?? 0)
  const cpuCores = computed(() => current.value?.cpu.cores ?? 0)
  const hostname = computed(() => current.value?.host?.hostname ?? '--')
  const uptimeSeconds = computed(() => current.value?.host?.uptime_seconds ?? 0)
  const load1 = computed(() => current.value?.load?.load1 ?? 0)
  const netRecvMB = computed(() => ((current.value?.network?.bytes_recv ?? 0) / 1024 / 1024).toFixed(2))
  const netSentMB = computed(() => ((current.value?.network?.bytes_sent ?? 0) / 1024 / 1024).toFixed(2))

  return {
    current, history, loading,
    fetchLatest, fetchHistory, triggerCollect,
    cpuPercent, memPercent, diskPercent,
    cpuCores, hostname, uptimeSeconds, load1, netRecvMB, netSentMB,
  }
})
