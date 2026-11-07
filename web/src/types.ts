export interface User {
  id: number
  username: string
  role: 'admin' | 'operator' | 'viewer'
  must_change_pwd: boolean
  created_at: string
  last_login_at?: string
}

export interface PortMap {
  host_port: number
  container_port: number
  protocol: string
  bind_host?: string
  accessible: boolean
}

export interface VolumeMount {
  source: string
  target: string
  read_only: boolean
}

export interface ResourceSpec {
  cpus: number
  memory_mb: number
  shm_mb: number
  gpu_count: number
  gpu_kind?: string
}

export interface HostAccess {
  enabled: boolean
  privileged: boolean
  host_pid: boolean
  mount_host_fs: boolean
  mount_docker_socket: boolean
  extra_mounts?: VolumeMount[]
  cap_add?: string[]
}

export interface ServiceSpec {
  name: string
  kind: string
  catalog_id: string
  image: string
  command?: string[]
  entrypoint?: string[]
  env: Record<string, string>
  ports: PortMap[]
  volumes: VolumeMount[]
  resources: ResourceSpec
  host_access: HostAccess
  restart: string
  model_name?: string
  description?: string
  auto_start: boolean
  labels?: Record<string, string>
}

export interface Service {
  id: string
  name: string
  kind: string
  catalog_id: string
  image: string
  model_name?: string
  description?: string
  spec: ServiceSpec
  status: string
  container_id?: string
  health: string
  endpoints: string[]
  error?: string
  owner: string
  created_at: string
  updated_at: string
  started_at?: string
  host_access: boolean
}

export interface ServiceEvent {
  id: number
  service_id: string
  level: string
  message: string
  created_at: string
}

export interface CatalogItem {
  id: string
  name: string
  category: string
  vendor: string
  kind: 'model' | 'app'
  icon: string
  accent: string
  summary: string
  description: string
  tags: string[]
  params?: string
  size_gb?: number
  image: string
  image_tag: string
  command?: string[]
  env?: Record<string, string>
  ports: PortMap[]
  volumes?: VolumeMount[]
  resources: ResourceSpec
  requires_gpu: boolean
  model_name?: string
  runtime?: string
  model_ref?: string
  default_host_access: boolean
  copilot: boolean
  docs_url?: string
  managed: boolean
  order: number
}

export interface MetricPoint {
  ts: number
  cpu: number
  memory: number
  disk: number
  net_rx: number
  net_tx: number
  gpu: number
  gpu_mem: number
}

export interface HostSnapshot {
  timestamp: string
  cpu: { cores: number; usage_percent: number; per_core_usage: number[] }
  memory: { total_gb: number; used_gb: number; available_gb: number; usage_percent: number; swap_total_gb: number; swap_used_gb: number }
  disk: { total_gb: number; used_gb: number; free_gb: number; usage_percent: number; read_rate_mb: number; write_rate_mb: number }
  network: { bytes_recv: number; bytes_sent: number; recv_rate_mb: number; sent_rate_mb: number }
  load: { load1: number; load5: number; load15: number }
  host: { hostname: string; os: string; platform: string; platform_version: string; kernel_version: string; uptime_seconds: number; arch: string }
  gpu?: { name: string; usage_percent: number; mem_used_mb: number; mem_total_mb: number; temperature: number; devices: any[]; driver?: string }
  containers: { total: number; running: number; stopped: number; ai_services: number }
}

export interface EngineSnapshot {
  connected: boolean
  host: string
  version: string
  api_version: string
  os: string
  arch: string
  cpus: number
  memory_gb: number
  images: number
  containers: number
  running: number
  driver: string
  error?: string
}

export interface Overview {
  edition: string
  version: string
  server_time: string
  uptime_seconds: number
  host: HostSnapshot['host']
  metrics: {
    cpu: HostSnapshot['cpu']
    memory: HostSnapshot['memory']
    disk: HostSnapshot['disk']
    network: HostSnapshot['network']
    load: HostSnapshot['load']
    gpu?: HostSnapshot['gpu']
    at: string
  }
  containers: { total: number; running: number; stopped: number; ai_services: number }
  services: { total: number; running: number; models: number; models_running: number; apps: number; host_access: number; unhealthy: number }
  engine: EngineSnapshot
  usage: { today: UsageTotals; month: UsageTotals }
  alerts: { level: string; title: string; detail: string }[]
}

export interface UsageTotals {
  requests: number
  prompt_tokens: number
  completion_tokens: number
  total_tokens: number
}

export interface ContainerView {
  id: string
  names: string[] | null
  image: string
  image_id: string
  command: string
  created: number
  state: string
  status: string
  ports: { ip?: string; private_port: number; public_port?: number; type: string }[] | null
  labels: Record<string, string>
  size_rw: number
  mounts: string[] | null
  network: string
  managed: boolean
  managed_service_id?: string
  managed_for?: string
  host_access: boolean
}

export interface ImageView {
  id: string
  tags: string[] | null
  size_mb: number
  created: number
  in_use: number
  is_ai_base: boolean
}

export interface NetworkView { id: string; name: string; driver: string; scope: string; subnet?: string; gateway?: string }
export interface VolumeView { name: string; driver: string; mountpoint: string; size_mb: number; in_use: number }
export interface APIKey {
  id: string
  name: string
  prefix: string
  role: string
  created_at: string
  last_used_at?: string
  revoked: boolean
  token_limit: number
  tokens_used: number
  requests_used: number
}
export interface AuditLog {
  id: number
  user: string
  role: string
  action: string
  target: string
  detail: string
  ip: string
  result: string
  created_at: string
}
export interface DeployPayload {
  catalog_id?: string
  name: string
  kind?: string
  image?: string
  image_tag?: string
  command?: string[]
  env?: Record<string, string>
  ports?: PortMap[]
  volumes?: VolumeMount[]
  resources?: ResourceSpec
  host_access?: HostAccess
  model_name?: string
  model_asset_id?: string
  model_runtime?: string
  auto_start?: boolean
}
export interface ContainerStats {
  cpu_percent: number
  memory_mb: number
  memory_limit_mb: number
  memory_percent: number
  net_rx_mb: number
  net_tx_mb: number
  blk_read_mb: number
  blk_write_mb: number
  pids: number
  disk_mb: number
}
export interface CopilotStep { tool: string; args: string; result: string }
export interface CopilotResponse { reply: string; model: string; steps: CopilotStep[]; error?: string }

export interface ModelAsset {
  id: string
  name: string
  source: string
  repo?: string
  revision?: string
  file?: string
  mirror?: string
  local_path: string
  host_path?: string
  volume?: string
  size_bytes: number
  file_count: number
  status: string
  error?: string
  log?: string
  container_id?: string
  runtime: string
  gguf?: string
  created_at: string
  updated_at: string
}

export interface AssetSource {
  id: string
  name: string
  description: string
  downloads: boolean
  requires_repo: boolean
  requires_url: boolean
  requires_path: boolean
  default_mirror: string
  docs_url: string
  badge: string
}
