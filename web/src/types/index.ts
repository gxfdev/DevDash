export interface ApiResponse<T = unknown> {
  code: number
  message: string
  data: T
  error?: string
}

export interface CpuInfo {
  cores: number
  usage_percent: number
  per_core_usage?: number[]
}

export interface MemoryInfo {
  total_gb: number
  used_gb: number
  available_gb: number
  usage_percent: number
  swap_total_gb?: number
  swap_used_gb?: number
}

export interface DiskInfo {
  total_gb: number
  used_gb: number
  free_gb: number
  usage_percent: number
}

export interface NetworkInfo {
  bytes_recv: number
  bytes_sent: number
  recv_rate_mb: number
  sent_rate_mb: number
}

export interface LoadInfo {
  load1: number
  load5: number
  load15: number
}

export interface HostInfo {
  hostname: string
  os: string
  platform: string
  platform_version: string
  uptime_seconds: number
}

export interface TcpConns {
  established: number
  time_wait: number
  close_wait: number
  listen: number
}

export interface DiskIO {
  read_mb: number
  write_mb: number
  iops?: number
  read_rate_mb?: number
  write_rate_mb?: number
}

export interface GPUDevice {
  gpu_index: number
  name: string
  usage_percent: number
  mem_used_mb: number
  mem_total_mb: number
  temperature_celsius: number
  power_draw_w?: number
  power_limit_w?: number
  fan_speed_percent?: number
}

export interface GPUMetrics {
  name: string
  usage_percent: number
  mem_used_mb: number
  mem_total_mb: number
  temperature: number
  devices: GPUDevice[]
}

export interface ProcessInfo {
  pid: number
  name: string
  cpu_percent: number
  mem_percent: number
  status?: string
  create_time?: number
  cmdline?: string
}

export interface Snapshot {
  node_id: string
  timestamp: string
  cpu: CpuInfo
  memory: MemoryInfo
  disk: DiskInfo
  network: NetworkInfo
  load: LoadInfo
  host: HostInfo
  processes?: ProcessInfo[]
  tcp_conns?: TcpConns
  disk_io?: DiskIO
  gpu?: GPUMetrics
}

export interface FileInfo {
  name: string
  path: string
  type: 'dir' | 'file'
  is_dir: boolean
  size: number
  mode: string
  mtime: string
  modified?: string
  permission?: string
}

export interface GPUMetricHistoryPoint {
  timestamp: string
  gpu_index: number
  usage_percent: number
  mem_used_mb: number
  mem_total_mb: number
  temperature_celsius: number
}

export interface CronJobLog {
  id: number
  job_id: number
  command: string
  output: string
  exit_code: number
  duration_ms: number
  timestamp: string
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
