import axios from 'axios'

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

const client = axios.create({ baseURL: '/api/v1', withCredentials: true })

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

let isRefreshing = false
let refreshSubscribers: Array<(token: string) => void> = []

function onTokenRefreshed(token: string) {
  refreshSubscribers.forEach((cb) => cb(token))
  refreshSubscribers = []
}

function addRefreshSubscriber(cb: (token: string) => void) {
  refreshSubscribers.push(cb)
}

client.interceptors.response.use(
  (res) => res,
  async (err) => {
    const originalRequest = err.config
    if (err.response?.status === 401 && !originalRequest._retry) {
      originalRequest._retry = true
      if (isRefreshing) {
        return new Promise((resolve) => {
          addRefreshSubscriber((newToken: string) => {
            originalRequest.headers.Authorization = `Bearer ${newToken}`
            resolve(client(originalRequest))
          })
        })
      }
      isRefreshing = true
      try {
        const { data } = await authClient.post('/auth/refresh')
        const newToken = data.access_token
        if (newToken) {
          localStorage.setItem('token', newToken)
          client.defaults.headers.common['Authorization'] = `Bearer ${newToken}`
          onTokenRefreshed(newToken)
          originalRequest.headers.Authorization = `Bearer ${newToken}`
          return client(originalRequest)
        }
      } catch {
        localStorage.removeItem('token')
        const path = window.location.pathname
        if (!path.includes('/login') && !path.includes('/force-change-password')) {
          window.location.href = '/login'
        }
      } finally {
        isRefreshing = false
      }
    }
    return Promise.reject(err)
  }
)

export default client

export const authClient = axios.create({ baseURL: '/api', withCredentials: true })

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
      const errMsg = err.response?.data?.error || ''
      if (errMsg.includes('expired') || errMsg.includes('invalid')) {
        localStorage.removeItem('token')
        if (!window.location.pathname.includes('/login')) {
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

export interface AlertRule {
  id: number
  metric: string
  op: string
  threshold: number
  level: string
  channels: string[]
  enabled: boolean
}

export interface CronJob {
  id: number
  name: string
  expression: string
  command: string
  enabled: boolean
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

export interface ScriptInfo {
  id: number
  name: string
  interpreter: string
  description: string
  content: string
  created_at: string
  updated_at: string
}

export interface CommandHistoryItem {
  id: number
  command: string
  timestamp: string
}
