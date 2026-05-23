import axios, { type AxiosResponse } from 'axios'

interface AxiosErrorLike {
  response?: {
    data?: {
      error?: string
    }
  }
  message?: string
}

export function getErrorMessage(e: unknown, fallback = '操作失败'): string {
  if (!e) return fallback
  if (typeof e === 'object') {
    const err = e as AxiosErrorLike
    if (err.response?.data?.error) return err.response.data.error
    if (err.message) return err.message
  }
  if (typeof e === 'string') return e
  return fallback
}

function getCsrfToken(): string | null {
  const match = document.cookie.match(/(^|;)\\s*csrf_token=([^;]*)/)
  return match ? match[2] : null
}

const client = axios.create({ baseURL: '/api/v1' })

client.interceptors.request.use((config) => {
  const token = localStorage.getItem('token')
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  
  const csrfToken = getCsrfToken()
  if (csrfToken && ['post', 'put', 'patch', 'delete'].includes(config.method?.toLowerCase() || '')) {
    config.headers['X-CSRF-Token'] = csrfToken
  }
  
  return config
})

client.interceptors.response.use(
  (res) => res,
  (err) => {
    if (err.response?.status === 401) {
      localStorage.removeItem('token')
      const path = window.location.pathname
      if (!path.includes('/login') && !path.includes('/force-change-password')) {
        window.location.href = '/login'
      }
    }
    if (err.response?.status === 403 && err.response?.data?.error?.includes('CSRF')) {
      console.error('CSRF validation failed. Please refresh the page.')
    }
    return Promise.reject(err)
  }
)

export default client

export const authClient = axios.create({ baseURL: '/api' })

authClient.interceptors.request.use((config) => {
  const token = localStorage.getItem('token')
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  const csrfToken = getCsrfToken()
  if (csrfToken && ['post', 'put', 'patch', 'delete'].includes(config.method?.toLowerCase() || '')) {
    config.headers['X-CSRF-Token'] = csrfToken
  }
  return config
})

authClient.interceptors.response.use(
  (res) => res,
  (err) => {
    if (err.response?.status === 401) {
      const path = window.location.pathname
      const errMsg = err.response?.data?.error || ''
      if (errMsg.includes('expired') || errMsg.includes('invalid')) {
        localStorage.removeItem('token')
        if (!path.includes('/login')) {
          window.location.href = '/login'
        }
      }
    }
    return Promise.reject(err)
  }
)

export interface ApiResponse<T = unknown> {
  code: number
  message: string
  data: T
  error?: string
}

export interface NodeInfo {
  id: string
  name: string
  os: string
  arch: string
  ip: string
  role: string
  status: string
  last_heartbeat: string
  created_at: string
}

export interface AlertRule {
  id: number
  metric: string
  op: string
  threshold: number
  level: string
  channels: string[]
  enabled: boolean
}

export interface FirewallRule {
  proto: string
  port: string
  src_ip: string
  note: string
}

export interface DatabaseConnection {
  id: number
  node_id: string
  name: string
  type: string
  host: string
  port: number
  user: string
  password: string
  dbname: string
}

export interface CronJob {
  id: number
  node_id: string
  name: string
  expression: string
  command: string
  enabled: boolean
}

export interface SoftwareInfo {
  id: number
  node_id: string
  node_name: string
  name: string
  version: string
  status: string
  running: boolean
}

export interface AlertInfo {
  id: number
  node_id: string
  node_name: string
  metric: string
  level: string
  message: string
  value: number
  threshold: number
  created_at: number
  status: string
}

export const dockerApi = {
  async ping() {
    return client.get('/docker/ping').then(res => res.data)
  },

  async info() {
    return client.get('/docker/info').then(res => res.data)
  },

  async usage() {
    return client.get('/docker/usage').then(res => res.data)
  },

  async listContainers(all: boolean = false) {
    return client.get(`/docker/containers?all=${all}`).then(res => res.data)
  },

  async getContainer(id: string) {
    return client.get(`/docker/containers/${id}`).then(res => res.data)
  },

  async startContainer(id: string) {
    return client.post(`/docker/containers/${id}/start`).then(res => res.data)
  },

  async stopContainer(id: string, timeout?: number) {
    const params = timeout ? `?timeout=${timeout}` : ''
    return client.post(`/docker/containers/${id}/stop${params}`).then(res => res.data)
  },

  async restartContainer(id: string, timeout?: number) {
    const params = timeout ? `?timeout=${timeout}` : ''
    return client.post(`/docker/containers/${id}/restart${params}`).then(res => res.data)
  },

  async removeContainer(id: string, force: boolean = false, volumes: boolean = false) {
    const params = `?force=${force}&volumes=${volumes}`
    return client.delete(`/docker/containers/${id}${params}`).then(res => res.data)
  },

  async getContainerLogs(id: string, tail: string = '100') {
    return client.get(`/docker/containers/${id}/logs?tail=${tail}`, { responseType: 'text' }).then(res => res.data)
  },

  async getContainerStats(id: string) {
    return client.get(`/docker/containers/${id}/stats`).then(res => res.data)
  },

  async listImages() {
    return client.get('/docker/images').then(res => res.data)
  },

  async pullImage(image: string) {
    return client.post('/docker/images/pull', { image }).then(res => res.data)
  },

  async removeImage(id: string, force: boolean = false) {
    const params = `?force=${force}`
    return client.delete(`/docker/images/${id}${params}`).then(res => res.data)
  },

  async listNetworks() {
    return client.get('/docker/networks').then(res => res.data)
  },

  async listVolumes() {
    return client.get('/docker/volumes').then(res => res.data)
  },

  async listComposeProjects() {
    return client.get('/docker/compose/projects').then(res => res.data)
  },

  async startComposeProject(path: string) {
    return client.post('/docker/compose/start', { path }).then(res => res.data)
  },

  async stopComposeProject(path: string) {
    return client.post('/docker/compose/stop', { path }).then(res => res.data)
  },

  async restartComposeService(path: string, serviceName: string) {
    return client.post('/docker/compose/restart', { path, service_name: serviceName }).then(res => res.data)
  },

  async getComposeLogs(path: string, serviceName: string, tail: string = '100') {
    return client.get(`/docker/compose/logs?path=${encodeURIComponent(path)}&service_name=${serviceName}&tail=${tail}`, { responseType: 'text' }).then(res => res.data)
  },

  async validateCompose(content: string) {
    return client.post('/docker/compose/validate', { content }).then(res => res.data)
  },

  async deployFromTemplate(templateType: string, config: Record<string, string>) {
    return client.post('/docker/compose/deploy', { template_type: templateType, config }).then(res => res.data)
  }
}
