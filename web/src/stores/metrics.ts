import { defineStore } from 'pinia'
import { ref } from 'vue'
import client from '@/api/client'
import { wsUrl } from '@/api'
import type { GPUMetricHistoryPoint } from '@/types'

export interface MetricSnapshot {
  node_id: string
  timestamp: string
  cpu: number
  mem_used: number
  mem_total: number
  disk_used: number
  disk_total: number
  net_in: number
  net_out: number
  load_1: number
  load_5: number
  load_15: number
  procs: number
}

const MAX_RECONNECT_ATTEMPTS = 10
const BASE_RECONNECT_DELAY = 1000
const MAX_RECONNECT_DELAY = 30000

export const useMetricsStore = defineStore('metrics', () => {
  const snapshot = ref<MetricSnapshot | null>(null)
  const history = ref<MetricSnapshot[]>([])
  const gpuHistory = ref<GPUMetricHistoryPoint[]>([])
  const ws = ref<WebSocket | null>(null)
  const wsTimer = ref<ReturnType<typeof setTimeout> | null>(null)
  const reconnectAttempts = ref(0)
  const isIntentionalClose = ref(false)

  async function fetchSnapshot(nodeId?: string) {
    try {
      const url = '/snapshot'
      const { data } = await client.get<MetricSnapshot>(url)
      snapshot.value = data
    } catch (err) {
      console.error('[metrics] fetchSnapshot failed:', err)
    }
  }

  async function fetchHistory(nodeId: string, duration = '1h') {
    try {
      const { data } = await client.get<MetricSnapshot[]>(`/node/${nodeId}/history?duration=${duration}`)
      history.value = data
    } catch (err) {
      console.error('[metrics] fetchHistory failed:', err)
    }
  }

  async function fetchGPUHistory(nodeId: string, hours = 1) {
    try {
      const { data } = await client.get<GPUMetricHistoryPoint[]>(`/node/${nodeId}/gpu/history?hours=${hours}`)
      gpuHistory.value = data
    } catch (err) {
      console.error('[metrics] fetchGPUHistory failed:', err)
      gpuHistory.value = []
    }
  }

  function getReconnectDelay(): number {
    const delay = Math.min(
      BASE_RECONNECT_DELAY * Math.pow(2, reconnectAttempts.value),
      MAX_RECONNECT_DELAY
    )
    return delay + Math.random() * 1000
  }

  function connectWS(_nodeId?: string) {
    disconnectWS()
    isIntentionalClose.value = false

    const conn = new WebSocket(wsUrl())

    conn.onopen = () => {
      reconnectAttempts.value = 0
    }

    conn.onmessage = (e) => {
      try {
        const msg: MetricSnapshot = JSON.parse(e.data)
        if (msg.cpu || msg.node_id) {
          snapshot.value = msg
        }
      } catch {
        // ignore malformed messages
      }
    }

    conn.onerror = () => {
      conn.close()
    }

    conn.onclose = () => {
      if (isIntentionalClose.value) return
      if (reconnectAttempts.value >= MAX_RECONNECT_ATTEMPTS) {
        console.error('[metrics] Max reconnect attempts reached')
        return
      }
      reconnectAttempts.value++
      const delay = getReconnectDelay()
      wsTimer.value = setTimeout(() => connectWS(), delay)
    }

    ws.value = conn
  }

  function disconnectWS() {
    isIntentionalClose.value = true
    if (wsTimer.value) clearTimeout(wsTimer.value)
    wsTimer.value = null
    ws.value?.close()
    ws.value = null
    reconnectAttempts.value = 0
  }

  return { snapshot, history, gpuHistory, fetchSnapshot, fetchHistory, fetchGPUHistory, connectWS, disconnectWS }
})
