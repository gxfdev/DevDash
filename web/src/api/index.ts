import client, { authClient } from './client'

// ── 认证（/api/auth/* 不在 v1 组，走独立 baseURL）──
export const authAPI = {
  login: (username: string, password: string) =>
    authClient.post('/auth/login', { username, password }),
  changePassword: (old: string, newPw: string) =>
    authClient.put('/auth/password', { old, new: newPw }),
  logout: () => authClient.post('/auth/logout'),
  me: () => authClient.get('/auth/me'),
}

// ── 节点 ────────────────────────────────────────
export const nodesAPI = {
  list: () => client.get('/nodes'),
  get: (id: string) => client.get(`/node/${id}`),
  register: (data: any) => client.post('/node/register', data),
  delete: (id: string) => client.delete(`/node/${id}`),
  metrics: (id: string) => client.get(`/node/${id}/metrics`),
  history: (id: string, duration: string) => client.get(`/node/${id}/history?duration=${duration}`),
  procs: (id: string) => client.get(`/node/${id}/procs`),
  containers: (id: string) => client.get(`/node/${id}/containers`),
}

// ── 软件商店 ─────────────────────────────────────
export const softwareAPI = {
  list: (nodeId: string) => client.get(`/node/${nodeId}/software`),
  install: (nodeId: string, name: string) =>
    client.post(`/node/${nodeId}/software/install`, { name }),
  uninstall: (nodeId: string, name: string) =>
    client.post(`/node/${nodeId}/software/uninstall`, { name }),
  service: (nodeId: string, name: string, action: string) =>
    client.post(`/node/${nodeId}/software/service`, { name, action }),
}

// ── 防火墙 ───────────────────────────────────────
export const firewallAPI = {
  list: (nodeId: string) => client.get(`/node/${nodeId}/firewall/rules`),
  add: (nodeId: string, rule: any) => client.post(`/node/${nodeId}/firewall/rules`, rule),
  update: (nodeId: string, id: string, patch: any) => client.patch(`/node/${nodeId}/firewall/rules/${id}`, patch),
  delete: (nodeId: string, id: string) => client.delete(`/node/${nodeId}/firewall/rules/${id}`),
}

// ── 计划任务 ─────────────────────────────────────
export const cronAPI = {
  list: (nodeId: string) => client.get(`/node/${nodeId}/cronjobs`),
  create: (nodeId: string, job: any) => client.post(`/node/${nodeId}/cronjobs`, job),
  update: (nodeId: string, id: string, patch: any) => client.patch(`/node/${nodeId}/cronjobs/${id}`, patch),
  delete: (nodeId: string, id: string) => client.delete(`/node/${nodeId}/cronjobs/${id}`),
  run: (nodeId: string, id: string) => client.post(`/node/${nodeId}/cronjobs/${id}/run`),
  logs: (nodeId: string, id: string) => client.get(`/node/${nodeId}/cronjobs/${id}/logs`),
}

// ── 数据库 ───────────────────────────────────────
export const dbAPI = {
  list: (nodeId: string) => client.get(`/node/${nodeId}/databases`),
  add: (nodeId: string, cfg: any) => client.post(`/node/${nodeId}/databases`, cfg),
  tables: (nodeId: string, dbId: string) => client.get(`/node/${nodeId}/databases/${dbId}/tables`),
  query: (nodeId: string, dbId: string, sql: string) =>
    client.post(`/node/${nodeId}/databases/${dbId}/query`, { sql }),
}

// ── 文件管理 ─────────────────────────────────────
export const fileAPI = {
  list: (nodeId: string, path: string) =>
    client.get(`/node/${nodeId}/fs/list`, { params: { path } }),
  mkdir: (nodeId: string, path: string) =>
    client.post(`/node/${nodeId}/fs/mkdir`, { path }),
  mkfile: (nodeId: string, path: string) =>
    client.post(`/node/${nodeId}/fs/mkfile`, { path }),
  remove: (nodeId: string, path: string) =>
    client.delete(`/node/${nodeId}/fs/remove`, { data: { path } }),
  upload: (nodeId: string, path: string, formData: FormData) =>
    client.post(`/node/${nodeId}/fs/upload?path=${encodeURIComponent(path)}`, formData, {
      headers: { 'Content-Type': 'multipart/form-data' },
    }),
  downloadUrl: (nodeId: string, path: string) =>
    `/api/v1/node/${nodeId}/fs/download?path=${encodeURIComponent(path)}`,
}

// ── 告警 ─────────────────────────────────────────
export const alertAPI = {
  active: (params?: any) => client.get('/alerts/active', { params }),
  history: (params?: any) => client.get('/alerts/history', { params }),
  rules: () => client.get('/alert-rules'),
  createRule: (rule: any) => client.post('/alert-rules', rule),
  updateRule: (id: string, rule: any) => client.put(`/alert-rules/${id}`, rule),
  deleteRule: (id: string) => client.delete(`/alert-rules/${id}`),
  silence: (id: string) => client.post(`/alerts/${id}/silence`),
  testFeishu: (url: string) => client.post('/alert/test-feishu', { url }),
}

// ── 设置 ──────────────────────────────────────────
export const settingsAPI = {
  get: () => client.get('/settings'),
  update: (data: any) => client.put('/settings', data),
  alertSettings: () => client.get('/alert-settings'),
  saveAlertSettings: (data: any) => client.put('/alert-settings', data),
}

// ── WebSocket URL（需带 token）───────────────────────────
export function wsUrl() {
  const proto = location.protocol === 'https:' ? 'wss:' : 'ws:'
  const token = localStorage.getItem('token') || ''
  return `${proto}//${location.host}/api/v1/ws?token=${encodeURIComponent(token)}`
}