import { http } from './client'
import type {
  APIKey, AssetSource, AuditLog, CatalogItem, ContainerStats, ContainerView, CopilotResponse, DeployPayload, ModelAsset,
  EngineSnapshot, HostSnapshot, ImageView, MetricPoint, NetworkView, Overview, Service, ServiceEvent,
  User, VolumeView,
} from '@/types'

export const authApi = {
  login: (username: string, password: string) => http.post<any, { token: string; user: User }>('/auth/login', { username, password }),
  me: () => http.get<any, { user: User }>('/auth/me'),
  changePassword: (old_password: string, new_password: string) => http.post('/auth/password', { old_password, new_password }),
}

export const systemApi = {
  overview: () => http.get<any, Overview>('/system/overview'),
  host: () => http.get<any, HostSnapshot>('/system/host'),
  engine: () => http.get<any, { engine: EngineSnapshot; info: any }>('/system/engine'),
  metrics: (range: string) => http.get<any, { range: string; points: MetricPoint[] }>('/system/metrics', { params: { range } }),
  disk: () => http.get<any, any>('/system/disk'),
}

export const catalogApi = {
  list: () => http.get<any, { models: CatalogItem[]; apps: CatalogItem[]; model_categories: string[]; app_categories: string[] }>('/catalog'),
  get: (id: string) => http.get<any, CatalogItem>(`/catalog/${id}`),
}

export const serviceApi = {
  list: () => http.get<any, { services: Service[] }>('/services'),
  get: (id: string) => http.get<any, { service: Service; events: ServiceEvent[]; stats?: ContainerStats; busy: boolean }>(`/services/${id}`),
  deploy: (payload: DeployPayload) => http.post<any, Service>('/services', payload),
  action: (id: string, action: string) => http.post<any, { service: Service }>(`/services/${id}/action`, { action }),
  remove: (id: string, keepVolumes = false) => http.delete(`/services/${id}`, { params: { keep_volumes: keepVolumes ? 1 : 0 } }),
  logs: (id: string, tail = 300) => http.get<any, { logs: string; tail: number }>(`/services/${id}/logs`, { params: { tail } }),
  events: (id: string) => http.get<any, { events: ServiceEvent[] }>(`/services/${id}/events`),
  stats: (id: string) => http.get<any, ContainerStats>(`/services/${id}/stats`),
  exec: (id: string, command: string, timeout = 30) => http.post<any, { output: string; error?: string }>(`/services/${id}/exec`, { command, timeout }),
}

export const dockerApi = {
  containers: () => http.get<any, { containers: ContainerView[] }>('/docker/containers'),
  inspect: (id: string) => http.get<any, any>(`/docker/containers/${id}`),
  logs: (id: string, tail = 300) => http.get<any, { logs: string }>(`/docker/containers/${id}/logs`, { params: { tail } }),
  action: (id: string, action: string) => http.post(`/docker/containers/${id}/action`, { action }),
  images: () => http.get<any, { images: ImageView[] }>('/docker/images'),
  removeImage: (id: string, force = false) => http.delete(`/docker/images/${id}`, { params: { force: force ? 1 : 0 } }),
  networks: () => http.get<any, { networks: NetworkView[] }>('/docker/networks'),
  createNetwork: (name: string, driver = 'bridge') => http.post('/docker/networks', { name, driver }),
  removeNetwork: (id: string) => http.delete(`/docker/networks/${id}`),
  volumes: () => http.get<any, { volumes: VolumeView[] }>('/docker/volumes'),
  removeVolume: (name: string, force = false) => http.delete(`/docker/volumes/${name}`, { params: { force: force ? 1 : 0 } }),
  df: () => http.get<any, any>('/docker/df'),
}

export const gatewayApi = {
  info: () => http.get<any, any>('/gateway'),
  keys: () => http.get<any, { keys: APIKey[] }>('/gateway/keys'),
  createKey: (payload: { name: string; role?: string; token_limit?: number }) =>
    http.post<any, { key: APIKey; secret: string }>('/gateway/keys', payload),
  deleteKey: (id: string) => http.delete(`/gateway/keys/${id}`),
  test: (model: string, message: string) => http.post<any, any>('/gateway/test', { model, message }),
}

export const adminApi = {
  users: () => http.get<any, { users: User[] }>('/users'),
  createUser: (payload: { username: string; password: string; role: string }) => http.post<any, User>('/users', payload),
  updateUser: (id: number, payload: { role?: string; password?: string }) => http.patch<any, { user: User }>(`/users/${id}`, payload),
  deleteUser: (id: number) => http.delete(`/users/${id}`),
  audit: (q = '', limit = 100) => http.get<any, { logs: AuditLog[] }>('/audit', { params: { q, limit } }),
  settings: () => http.get<any, { settings: Record<string, string>; defaults: Record<string, string> }>('/settings'),
  updateSettings: (payload: Record<string, string>) => http.put<any, any>('/settings', payload),
}

export const copilotApi = {
  chat: (payload: { model?: string; messages: { role: string; content: string }[]; allow_exec: boolean; sandbox_id?: string }) =>
    http.post<any, CopilotResponse>('/copilot/chat', payload, { timeout: 180000 }),
  playground: (payload: { model: string; messages: any[]; stream?: boolean }) =>
    http.post<any, any>('/playground/chat', { stream: false, ...payload }, { timeout: 180000 }),
}

export { http }

export const assetApi = {
  sources: () => http.get<any, { sources: AssetSource[]; volume: string; volume_target: string }>('/model-assets/sources'),
  list: () => http.get<any, { assets: ModelAsset[] }>('/model-assets'),
  get: (id: string) => http.get<any, { asset: ModelAsset }>(`/model-assets/${id}`),
  create: (payload: Partial<ModelAsset> & { name: string; source: string; token?: string; url?: string }) =>
    http.post<any, ModelAsset>('/model-assets', payload),
  logs: (id: string, tail = 300) => http.get<any, { logs: string }>(`/model-assets/${id}/logs`, { params: { tail } }),
  cancel: (id: string) => http.post(`/model-assets/${id}/cancel`),
  scan: (id: string) => http.post<any, { asset: ModelAsset }>(`/model-assets/${id}/scan`),
  remove: (id: string, deleteFiles = true) => http.delete(`/model-assets/${id}`, { params: { delete_files: deleteFiles ? 1 : 0 } }),
}
