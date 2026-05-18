import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import client from '@/api/client'

export interface Snapshot {
  node_id: string
  timestamp: string
  cpu: { cores: number; usage_percent: number; per_core_usage?: number[] }
  memory: { total_gb: number; used_gb: number; available_gb: number; usage_percent: number }
  disk: { total_gb: number; used_gb: number; free_gb: number; usage_percent: number }
  network: { bytes_recv: number; bytes_sent: number; recv_rate_mb: number; sent_rate_mb: number }
  load: { load1: number; load5: number; load15: number }
  host: { hostname: string; os: string; platform: string; platform_version: string; uptime_seconds: number }
  processes?: any[]
  tcp_conns?: { established: number; time_wait: number; close_wait: number; listen: number }
  disk_io?: { read_mb: number; write_mb: number }
}

export const useSnapshotStore = defineStore('snapshot', () => {
  const current = ref<Snapshot | null>(null)
  const history = ref<Snapshot[]>([])
  const loading = ref(false)

  async function fetchLatest() {
    loading.value = true
    try {
      const { data } = await client.get('/latest')
      current.value = data
    } finally {
      loading.value = false
    }
  }

  async function fetchHistory(limit = 60) {
    const { data } = await client.get(`/history?limit=${limit}`)
    history.value = data
  }

  async function triggerCollect() {
    const { data } = await client.post('/collect')
    current.value = data.snapshot
    return data
  }

  // Derived helpers so templates stay clean
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