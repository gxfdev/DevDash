import { defineStore } from 'pinia'
import { ref } from 'vue'
import client from '@/api/client'
import { wsUrl } from '@/api'

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

export const useMetricsStore = defineStore('metrics', () => {
  const snapshot = ref<MetricSnapshot | null>(null)
  const history = ref<MetricSnapshot[]>([])
  const ws = ref<WebSocket | null>(null)
  const wsTimer = ref<ReturnType<typeof setTimeout> | null>(null)

  async function fetchSnapshot(nodeId?: string) {
    const url = '/snapshot'
    const { data } = await client.get(url)
    snapshot.value = data
  }

  async function fetchHistory(nodeId: string, duration = '1h') {
    const { data } = await client.get(`/node/${nodeId}/history?duration=${duration}`)
    history.value = data
  }

  function connectWS(_nodeId?: string) {
    disconnectWS()
    const conn = new WebSocket(wsUrl())

    conn.onopen = () => {
      // Connected, metrics will stream automatically
    }

    conn.onmessage = (e) => {
      try {
        const msg = JSON.parse(e.data)
        if (msg.cpu || msg.node_id) {
          snapshot.value = msg
        }
      } catch {}
    }

    conn.onerror = () => {
      conn.close()
    }

    conn.onclose = () => {
      // auto reconnect after 3s
      wsTimer.value = setTimeout(() => connectWS(), 3000)
    }

    ws.value = conn
  }

  function disconnectWS() {
    if (wsTimer.value) clearTimeout(wsTimer.value)
    ws.value?.close()
    ws.value = null
  }

  return { snapshot, history, fetchSnapshot, fetchHistory, connectWS, disconnectWS }
})