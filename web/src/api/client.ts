import axios from 'axios'

const api = axios.create({
  baseURL: '/api',
  timeout: 15000,
})

api.interceptors.request.use((config) => {
  const token = localStorage.getItem('token')
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

api.interceptors.response.use(
  (res) => res,
  (err) => {
    if (err.response?.status === 401) {
      localStorage.removeItem('token')
      localStorage.removeItem('user')
      window.location.href = '/login'
    }
    return Promise.reject(err)
  }
)

export default api

export const authApi = {
  login: (username: string, password: string) => api.post('/login', { username, password }),
  profile: () => api.get('/profile'),
  changePassword: (oldPassword: string, newPassword: string) => api.put('/password', { oldPassword, newPassword }),
}

export const monitorApi = {
  getStatus: () => api.get('/monitor'),
}

export const cronApi = {
  list: () => api.get('/cron'),
  create: (data: { name: string; schedule: string; command: string }) => api.post('/cron', data),
  update: (id: number, data: { name: string; schedule: string; command: string; enabled: boolean }) => api.put(`/cron/${id}`, data),
  delete: (id: number) => api.delete(`/cron/${id}`),
  sync: () => api.post('/cron/sync'),
  systemCrontab: () => api.get('/cron/system'),
}

export const fileApi = {
  list: (path: string) => api.get('/files', { params: { path } }),
  read: (path: string) => api.get('/files/content', { params: { path } }),
  write: (path: string, content: string) => api.put('/files/content', { path, content }),
  delete: (path: string) => api.delete('/files', { params: { path } }),
  mkdir: (path: string) => api.post('/files/mkdir', { path }),
  tree: (root: string, depth?: number) => api.get('/files/tree', { params: { root, depth } }),
}

export const scriptApi = {
  list: () => api.get('/scripts'),
  get: (id: number) => api.get(`/scripts/${id}`),
  create: (data: { name: string; description: string; content: string; interpreter: string }) => api.post('/scripts', data),
  update: (id: number, data: { name: string; description: string; content: string; interpreter: string }) => api.put(`/scripts/${id}`, data),
  delete: (id: number) => api.delete(`/scripts/${id}`),
  run: (id: number) => api.post(`/scripts/${id}/run`),
}

export const userApi = {
  list: () => api.get('/users'),
  create: (data: { username: string; password: string; role: string }) => api.post('/users', data),
  delete: (id: number) => api.delete(`/users/${id}`),
}

export const auditApi = {
  list: (limit?: number) => api.get('/audit-logs', { params: { limit } }),
}
