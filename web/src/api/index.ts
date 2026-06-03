import client, { authClient } from './client'
import type { AlertRule, CronJob } from './client'

export const authAPI = {
  login: (username: string, password: string) =>
    authClient.post('/auth/login', { username, password }),
  changePassword: (old: string, newPw: string) =>
    authClient.put('/auth/password', { old, new: newPw }),
  changeUsername: (newUsername: string) =>
    authClient.put('/auth/username', { new_username: newUsername }),
  logout: () => authClient.post('/auth/logout'),
  me: () => authClient.get('/auth/me'),
}

export const cronAPI = {
  list: () => client.get('/cronjobs'),
  create: (job: Omit<CronJob, 'id'>) => client.post('/cronjobs', job),
  update: (id: string, patch: Partial<CronJob>) => client.patch(`/cronjobs/${id}`, patch),
  delete: (id: string) => client.delete(`/cronjobs/${id}`),
  run: (id: string) => client.post(`/cronjobs/${id}/run`),
  logs: (id: string, params?: Record<string, string>) => client.get(`/cronjobs/${id}/logs`, { params }),
}

export const fileAPI = {
  list: (path: string) =>
    client.get('/fs/list', { params: { path } }),
  read: (path: string) =>
    client.get('/fs/read', { params: { path } }),
  mkdir: (path: string) =>
    client.post('/fs/mkdir', { path }),
  mkfile: (path: string) =>
    client.post('/fs/mkfile', { path }),
  remove: (path: string) =>
    client.delete('/fs/remove', { data: { path } }),
  upload: (path: string, formData: FormData) =>
    client.post(`/fs/upload?path=${encodeURIComponent(path)}`, formData, {
      headers: { 'Content-Type': 'multipart/form-data' },
    }),
  downloadUrl: (path: string) =>
    `/api/v1/fs/download?path=${encodeURIComponent(path)}`,
  chmod: (path: string, mode: string) =>
    client.post('/fs/chmod', { path, mode }),
}

export const alertAPI = {
  active: (params?: Record<string, string>) => client.get('/alerts/active', { params }),
  history: (params?: Record<string, string>) => client.get('/alerts/history', { params }),
  rules: () => client.get('/alert-rules'),
  createRule: (rule: Omit<AlertRule, 'id'>) => client.post('/alert-rules', rule),
  updateRule: (id: string, rule: Partial<AlertRule>) => client.put(`/alert-rules/${id}`, rule),
  deleteRule: (id: string) => client.delete(`/alert-rules/${id}`),
  silence: (id: string) => client.post(`/alerts/${id}/silence`),
}

export const scriptAPI = {
  list: () => client.get('/scripts'),
  create: (data: { name: string; interpreter: string; description: string; content: string }) =>
    client.post('/scripts', data),
  update: (id: string, data: { name?: string; interpreter?: string; description?: string; content?: string }) =>
    client.put(`/scripts/${id}`, data),
  delete: (id: string) => client.delete(`/scripts/${id}`),
  execute: (id: string) => client.post(`/scripts/${id}/run`),
  get: (id: string) => client.get(`/scripts/${id}`),
}

export const commandHistoryAPI = {
  list: (limit?: number) => client.get('/terminal/history', { params: { limit: limit || 100 } }),
}

export const metricsAPI = {
  current: () => client.get('/metrics'),
  history: (duration: string) => client.get(`/metrics/history?duration=${duration}`),
  stream: () => {
    const proto = location.protocol === 'https:' ? 'wss:' : 'ws:'
    const token = localStorage.getItem('token') || ''
    return `${proto}//${location.host}/api/v1/monitor/stream?token=${encodeURIComponent(token)}`
  },
}

export const settingsAPI = {
  alertSettings: () => client.get('/alert-rules'),
  saveAlertSettings: (data: Record<string, unknown>) => client.put('/alert-rules/1', data),
}

export function wsUrl() {
  const proto = location.protocol === 'https:' ? 'wss:' : 'ws:'
  const token = localStorage.getItem('token') || ''
  return `${proto}//${location.host}/api/v1/ws?token=${encodeURIComponent(token)}`
}
