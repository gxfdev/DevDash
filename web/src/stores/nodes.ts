import { defineStore } from 'pinia'
import { ref } from 'vue'
import client from '@/api/client'

export interface Node {
  id: string
  name: string
  os: string
  arch: string
  ip: string
  role: string
  status: string
  uptime?: number
  last_heartbeat: string
  note?: string
}

export const useNodesStore = defineStore('nodes', () => {
  const nodes = ref<Node[]>([])
  const currentNode = ref<Node | null>(null)
  const loading = ref(false)

  async function fetchNodes() {
    loading.value = true
    try {
      const { data } = await client.get('/nodes')
      nodes.value = data
    } catch {
      nodes.value = []
    } finally {
      loading.value = false
    }
  }

  async function fetchNode(id: string) {
    const { data } = await client.get(`/node/${id}`)
    currentNode.value = data
  }

  async function addNode(payload: { name: string; addr: string; token: string; note?: string }) {
    const { data } = await client.post('/node/register', payload)
    await fetchNodes()
    return data
  }

  async function deleteNode(id: string) {
    await client.delete(`/node/${id}`)
    nodes.value = nodes.value.filter((n) => n.id !== id)
  }

  return { nodes, currentNode, loading, fetchNodes, fetchNode, addNode, deleteNode }
})