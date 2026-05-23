<template>
  <app-layout>
  <div class="container-monitor">
    <div class="page-header">
      <h1>📊 容器监控</h1>
      <div class="header-actions">
        <button class="btn btn-primary" @click="refreshAll" :disabled="loading">
          {{ loading ? '⏳ 加载中...' : '🔄 刷新' }}
        </button>
        <label class="toggle-switch">
          <input type="checkbox" v-model="realtimeEnabled" @change="toggleRealtime" />
          <span class="slider"></span>
          <span class="label">实时</span>
        </label>
      </div>
    </div>

    <!-- Overview Cards -->
    <div class="overview-grid">
      <div class="overview-card docker-card">
        <h3>🐳 Docker</h3>
        <div class="metric">
          <span class="value">{{ overview.docker?.total_containers || 0 }}</span>
          <span class="label">容器数</span>
        </div>
        <div class="metric">
          <span class="value">{{ overview.docker?.running || 0 }}</span>
          <span class="label">运行中</span>
        </div>
        <div class="metric">
          <span class="value">{{ formatPercent(overview.docker?.avg_cpu_percent) }}%</span>
          <span class="label">平均 CPU</span>
        </div>
      </div>

      <div class="overview-card k8s-card">
        <h3>☸️ Kubernetes</h3>
        <div class="metric">
          <span class="value">{{ overview.kubernetes?.cluster_count || 0 }}</span>
          <span class="label">集群数</span>
        </div>
        <div class="metric">
          <span class="value">{{ overview.kubernetes?.total_nodes || 0 }}</span>
          <span class="label">节点数</span>
        </div>
        <div class="metric">
          <span class="value">{{ overview.kubernetes?.total_pods || 0 }}</span>
          <span class="label">Pod 数</span>
        </div>
      </div>

      <div class="overview-card websocket-card">
        <h3>📡 Connection</h3>
        <div class="metric">
          <span class="value" :class="{ connected: wsConnected }">{{ wsConnected ? '🟢' : '🔴' }}</span>
          <span class="label">WebSocket</span>
        </div>
        <div class="metric">
          <span class="value">{{ realtimeEnabled ? '开启' : '关闭' }}</span>
          <span class="label">数据流</span>
        </div>
      </div>
    </div>

    <!-- Tabs -->
    <div class="tabs">
      <button
        v-for="tab in tabs"
        :key="tab.id"
        :class="['tab', { active: activeTab === tab.id }]"
        @click="activeTab = tab.id"
      >
        {{ tab.icon }} {{ tab.label }}
      </button>
    </div>

    <!-- Docker Containers Tab -->
    <div v-if="activeTab === 'docker'" class="tab-content">
      <div class="toolbar">
        <input
          type="text"
          v-model="searchQuery"
          placeholder="搜索容器..."
          class="search-input"
        />
        <select v-model="sortBy" class="sort-select">
          <option value="name">名称</option>
          <option value="cpu">CPU 使用率</option>
          <option value="memory">内存使用</option>
        </select>
      </div>

      <div class="containers-table-wrapper">
        <table class="data-table containers-table">
          <thead>
            <tr>
              <th>容器名称</th>
              <th>状态</th>
              <th>CPU %</th>
              <th>内存</th>
              <th>网络 收/发</th>
              <th>PID 数</th>
              <th>最后更新</th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="container in sortedContainers"
              :key="container.container_id"
              class="container-row"
            >
              <td class="name-cell">
                <strong>{{ container.container_name }}</strong>
                <small>{{ container.image }}</small>
              </td>
              <td>
                <span class="status-badge running">🟢 运行中</span>
              </td>
              <td>
                <div class="progress-bar">
                  <div
                    class="progress-fill cpu"
                    :style="{ width: container.cpu?.usage_percent + '%' }"
                  ></div>
                </div>
                <span class="metric-value">{{ formatPercent(container.cpu?.usage_percent) }}%</span>
              </td>
              <td>
                <div class="memory-info">
                  <span>{{ formatBytes(container.memory?.usage) }}</span>
                  <span class="limit">/ {{ formatBytes(container.memory?.limit) }}</span>
                </div>
                <div class="progress-bar small">
                  <div
                    class="progress-fill memory"
                    :style="{ width: container.memory?.usage_percent + '%' }"
                  ></div>
                </div>
              </td>
              <td>
                <div class="network-info">
                  <span>⬇️ {{ formatBytes(container.network?.bytes_recv) }}</span>
                  <span>⬆️ {{ formatBytes(container.network?.bytes_sent) }}</span>
                </div>
              </td>
              <td>{{ container.pids }}</td>
              <td><time-ago :datetime="container.timestamp" /></td>
            </tr>
          </tbody>
        </table>
      </div>

      <div v-if="containers.length === 0 && !loading" class="empty-state">
        <p>暂无容器</p>
      </div>
    </div>

    <!-- Kubernetes Clusters Tab -->
    <div v-if="activeTab === 'k8s'" class="tab-content">
      <div class="k8s-toolbar">
        <button class="btn btn-success" @click="showAddClusterModal = true">
            ➕ 添加集群
          </button>
      </div>

      <div class="clusters-grid">
        <div
          v-for="cluster in k8sClusters"
          :key="cluster.id"
          class="cluster-card"
        >
          <div class="card-header">
            <h3>{{ cluster.name }}</h3>
            <span class="badge" :class="cluster.status === 'connected' ? 'success' : 'error'">
              {{ cluster.status }}
            </span>
          </div>

          <div class="cluster-info">
            <div class="info-item">
              <span class="label">版本：</span>
              <span class="value">{{ cluster.version }}</span>
            </div>
            <div class="info-item">
              <span class="label">节点数：</span>
              <span class="value">{{ cluster.node_count }}</span>
            </div>
            <div class="info-item">
              <span class="label">命名空间：</span>
              <span class="value">{{ cluster.namespace_count }}</span>
            </div>
            <div class="info-item">
              <span class="label">Pod 数：</span>
              <span class="value">{{ cluster.pod_count }}</span>
            </div>
            <div class="info-item">
              <span class="label">API 端点：</span>
              <span class="value small">{{ cluster.api_endpoint }}</span>
            </div>
          </div>

          <div class="card-actions">
            <button class="btn btn-sm btn-primary" @click="viewClusterDetails(cluster)">
              📋 详情
            </button>
            <button class="btn btn-sm btn-danger" @click="removeCluster(cluster.id)">
              🗑️ 删除
            </button>
          </div>
        </div>
      </div>

      <div v-if="k8sClusters.length === 0 && !loading" class="empty-state">
        <p>暂未配置 Kubernetes 集群</p>
        <button class="btn btn-primary" @click="showAddClusterModal = true">
          添加第一个集群
        </button>
      </div>
    </div>

    <!-- Multi-Host Tab -->
    <div v-if="activeTab === 'hosts'" class="tab-content">
      <div class="hosts-toolbar">
        <button class="btn btn-success" @click="showAddHostModal = true">
            ➕ 添加主机
          </button>
          <button class="btn btn-primary" @click="collectAllHosts" :disabled="loading">
            🔄 采集全部
          </button>
      </div>

      <div class="hosts-overview-grid">
        <div class="overview-card host-overview-card">
          <h3>🖥️ Hosts</h3>
          <div class="metric">
            <span class="value">{{ hostOverview?.total_hosts || 0 }}</span>
            <span class="label">总数</span>
          </div>
          <div class="metric">
            <span class="value" style="color: #059669">{{ hostOverview?.online_hosts || 0 }}</span>
            <span class="label">在线</span>
          </div>
          <div class="metric">
            <span class="value" style="color: #dc2626">{{ hostOverview?.offline_hosts || 0 }}</span>
            <span class="label">离线</span>
          </div>
        </div>

        <div class="overview-card host-overview-card">
          <h3>📦 Containers</h3>
          <div class="metric">
            <span class="value">{{ hostOverview?.total_containers || 0 }}</span>
            <span class="label">Total</span>
          </div>
        </div>

        <div class="overview-card host-overview-card">
          <h3>📊 Resources</h3>
          <div class="metric">
            <span class="value">{{ formatPercent(hostOverview?.avg_cpu_percent) }}%</span>
            <span class="label">平均 CPU</span>
          </div>
          <div class="metric">
            <span class="value">{{ hostOverview?.total_memory_used_gb?.toFixed(1) || '0.0' }} / {{ hostOverview?.total_memory_total_gb?.toFixed(1) || '0.0' }} GB</span>
            <span class="label">内存</span>
          </div>
        </div>
      </div>

      <div class="hosts-grid">
        <div
          v-for="host in hostOverview?.hosts || []"
          :key="host.id"
          :class="['host-card', { 'host-selected': selectedHost?.id === host.id }]"
          @click="viewHostDetails(host)"
        >
          <div class="host-card-header">
            <h4>{{ host.name }}</h4>
            <span class="badge" :class="host.status === 'online' ? 'success' : 'error'">
              {{ host.status }}
            </span>
          </div>
          <div class="host-card-info">
            <div class="info-row">
              <span class="label">端点：</span>
              <span class="value small">{{ host.endpoint }}</span>
            </div>
            <div class="info-row">
              <span class="label">系统/架构：</span>
              <span class="value">{{ host.os }}/{{ host.arch }}</span>
            </div>
            <div class="info-row">
              <span class="label">最后心跳：</span>
              <span class="value">{{ new Date(host.last_heartbeat).toLocaleString() }}</span>
            </div>
          </div>
          <div class="host-card-actions">
            <button class="btn btn-sm btn-primary" @click.stop="collectFromHost(host.id)">🔄 Collect</button>
            <button class="btn btn-sm btn-danger" @click.stop="removeHost(host.id)">🗑️ 删除</button>
          </div>
        </div>
      </div>

      <div v-if="(!hostOverview?.hosts || hostOverview.hosts.length === 0) && !loading" class="empty-state">
        <p>暂未配置远程主机</p>
        <button class="btn btn-primary" @click="showAddHostModal = true">
          添加第一台主机
        </button>
      </div>

      <!-- Host Detail Panel -->
      <div v-if="selectedHost && hostMetrics" class="host-detail-panel">
        <div class="detail-header">
          <h3>📊 {{ selectedHost.name }} - 资源详情</h3>
          <button class="btn btn-sm btn-secondary" @click="selectedHost = null">✕ 关闭</button>
        </div>

        <div class="detail-metrics-grid">
          <div class="metric-card">
            <h4>CPU</h4>
            <div class="big-number">{{ formatPercent(hostMetrics.snapshot?.cpu?.usage_percent) }}%</div>
            <div class="sub-text">{{ hostMetrics.snapshot?.cpu?.cores || 0 }} 核</div>
          </div>
          <div class="metric-card">
            <h4>Memory</h4>
            <div class="big-number">{{ hostMetrics.snapshot?.memory?.used_gb?.toFixed(1) || '0' }} GB</div>
            <div class="sub-text">{{ formatPercent(hostMetrics.snapshot?.memory?.usage_percent) }}% / {{ hostMetrics.snapshot?.memory?.total_gb?.toFixed(1) || '0' }} GB</div>
          </div>
          <div class="metric-card">
            <h4>Disk</h4>
            <div class="big-number">{{ hostMetrics.snapshot?.disk?.used_gb?.toFixed(1) || '0' }} GB</div>
            <div class="sub-text">{{ formatPercent(hostMetrics.snapshot?.disk?.usage_percent) }}% / {{ hostMetrics.snapshot?.disk?.total_gb?.toFixed(1) || '0' }} GB</div>
          </div>
          <div class="metric-card">
            <h4>Load</h4>
            <div class="big-number">{{ hostMetrics.snapshot?.load?.load1?.toFixed(2) || '0' }}</div>
            <div class="sub-text">5分钟: {{ hostMetrics.snapshot?.load?.load5?.toFixed(2) || '0' }} / 15分钟: {{ hostMetrics.snapshot?.load?.load15?.toFixed(2) || '0' }}</div>
          </div>
        </div>

        <div class="host-containers-section">
          <h4>🐳 Containers ({{ hostContainers.length }})</h4>
          <table class="data-table" v-if="hostContainers.length > 0">
            <thead>
              <tr>
                <th>名称</th>
                <th>状态</th>
                <th>CPU %</th>
                <th>内存</th>
                <th>网络 收/发</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="c in hostContainers" :key="c.id">
                <td>
                  <strong>{{ c.name }}</strong>
                  <small>{{ c.image }}</small>
                </td>
                <td>
                  <span class="status-badge" :class="c.state === 'running' ? 'running' : 'stopped'">
                    ● {{ c.state === 'running' ? '运行中' : '已停止' }}
                  </span>
                </td>
                <td>{{ formatPercent(c.cpu_percent) }}%</td>
                <td>{{ formatBytes(c.memory_usage) }} / {{ formatBytes(c.memory_limit) }}</td>
                <td>⬇️ {{ formatBytes(c.network_rx) }} / ⬆️ {{ formatBytes(c.network_tx) }}</td>
              </tr>
            </tbody>
          </table>
          <div v-else class="empty-state small">
            <p>该主机暂无容器</p>
          </div>
        </div>
      </div>
    </div>

    <!-- Add Host Modal -->
    <div v-if="showAddHostModal" class="modal-overlay" @click.self="showAddHostModal = false">
      <div class="modal-content">
        <h2>添加远程主机</h2>
        <form @submit.prevent="addHost">
          <div class="form-group">
            <label>主机名称 *</label>
            <input type="text" v-model="newHost.name" required placeholder="例如：prod-server-01" />
          </div>
          <div class="form-group">
            <label>Agent 端点 *</label>
            <input type="url" v-model="newHost.endpoint" required placeholder="http://192.168.1.100:9090" />
          </div>
          <div class="form-group">
            <label>认证 Token（可选）</label>
            <input type="password" v-model="newHost.token" placeholder="Bearer 认证令牌" />
          </div>
          <div class="form-group">
            <label>OS</label>
            <select v-model="newHost.os">
              <option value="">自动检测</option>
              <option value="linux">Linux</option>
              <option value="windows">Windows</option>
              <option value="darwin">macOS</option>
            </select>
          </div>
          <div class="form-group">
            <label>Architecture</label>
            <select v-model="newHost.arch">
              <option value="">自动检测</option>
              <option value="x86_64">x86_64 (amd64)</option>
              <option value="aarch64">aarch64 (arm64)</option>
            </select>
          </div>
          <div class="modal-actions">
            <button type="button" class="btn btn-secondary" @click="showAddHostModal = false">取消</button>
            <button type="submit" class="btn btn-primary" :disabled="addingHost">
              {{ addingHost ? '⏳ 添加中...' : '➕ 添加主机' }}
            </button>
          </div>
        </form>
      </div>
    </div>

    <!-- Top Resources Tab -->
    <div v-if="activeTab === 'top'" class="tab-content">
      <div class="top-resources-section">
        <div class="resource-panel">
          <h3>🔥 CPU 占用 Top10 (24h)</h3>
          <div class="top-list">
            <div
              v-for="(item, index) in topCPUContainers"
              :key="item.container_id"
              class="top-item"
            >
              <span class="rank">#{{ index + 1 }}</span>
              <span class="name">{{ item.container_name }}</span>
              <div class="bar-container">
                <div
                  class="bar cpu-bar"
                  :style="{ width: (item.avg_cpu / maxCPUPercent * 100) + '%' }"
                ></div>
              </div>
              <span class="value">{{ formatPercent(item.avg_cpu) }}%</span>
            </div>
          </div>
        </div>

        <div class="resource-panel">
          <h3>💾 内存占用 Top10 (24h)</h3>
          <div class="top-list">
            <div
              v-for="(item, index) in topMemoryContainers"
              :key="item.container_id"
              class="top-item"
            >
              <span class="rank">#{{ index + 1 }}</span>
              <span class="name">{{ item.container_name }}</span>
              <div class="bar-container">
                <div
                  class="bar memory-bar"
                  :style="{ width: (item.avg_memory / maxMemoryUsage * 100) + '%' }"
                ></div>
              </div>
              <span class="value">{{ formatBytes(item.avg_memory) }}</span>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- Add Cluster Modal -->
    <div v-if="showAddClusterModal" class="modal-overlay" @click.self="showAddClusterModal = false">
      <div class="modal-content">
        <h2>添加 Kubernetes 集群</h2>
        
        <form @submit.prevent="addCluster">
          <div class="form-group">
            <label>集群名称 *</label>
            <input type="text" v-model="newCluster.name" required placeholder="例如：production-cluster" />
          </div>

          <div class="form-group">
            <label>连接方式</label>
            <select v-model="newCluster.method">
              <option value="kubeconfig">Kubeconfig 文件</option>
              <option value="manual">手动配置</option>
            </select>
          </div>

          <div v-if="newCluster.method === 'kubeconfig'" class="form-group">
            <label>Kubeconfig 内容 *</label>
            <textarea
              v-model="newCluster.kubeconfig"
              required
              rows="8"
              placeholder="粘贴 kubeconfig 文件内容..."
            ></textarea>
          </div>

          <div v-if="newCluster.method === 'manual'">
            <div class="form-group">
              <label>API 端点 *</label>
            <input type="url" v-model="newCluster.apiEndpoint" required placeholder="https://your-k8s-api:6443" />
            </div>
            <div class="form-group">
              <label>CA Certificate</label>
              <textarea
                v-model="newCluster.caCert"
                rows="4"
                placeholder="Paste CA certificate content..."
              ></textarea>
            </div>
            <div class="form-group">
              <label>认证 Token *</label>
              <input type="password" v-model="newCluster.token" required placeholder="Bearer 认证令牌..." />
            </div>
          </div>

          <div class="modal-actions">
            <button type="button" class="btn btn-secondary" @click="showAddClusterModal = false">取消</button>
            <button type="submit" class="btn btn-primary" :disabled="addingCluster">
              {{ addingCluster ? '⏳ 添加中...' : '➕ 添加集群' }}
            </button>
          </div>
        </form>
      </div>
    </div>

    <!-- Container Detail Modal -->
    <div v-if="selectedContainer" class="modal-overlay" @click.self="selectedContainer = null">
      <div class="modal-content large">
        <h2>{{ selectedContainer.container_name }} - Metrics Detail</h2>
        
        <div class="detail-tabs">
          <button
            v-for="tab in detailTabs"
            :key="tab"
            :class="['detail-tab', { active: activeDetailTab === tab }]"
            @click="activeDetailTab = tab"
          >
            {{ tab }}
          </button>
        </div>

        <div v-if="activeDetailTab === 'Real-time'" class="detail-content">
          <div class="metrics-grid">
            <div class="metric-card">
              <h4>CPU Usage</h4>
              <div class="big-number">{{ formatPercent(selectedContainer.cpu?.usage_percent) }}%</div>
              <div class="mini-chart" ref="cpuChart"></div>
            </div>
            <div class="metric-card">
              <h4>Memory Usage</h4>
              <div class="big-number">{{ formatBytes(selectedContainer.memory?.usage) }}</div>
              <div class="sub-text">{{ formatPercent(selectedContainer.memory?.usage_percent) }} of {{ formatBytes(selectedContainer.memory?.limit) }}</div>
            </div>
            <div class="metric-card">
              <h4>Network I/O</h4>
              <div class="network-stats">
                <div>⬇️ In: {{ formatBytes(selectedContainer.network?.bytes_recv) }}</div>
                <div>⬆️ Out: {{ formatBytes(selectedContainer.network?.bytes_sent) }}</div>
              </div>
            </div>
            <div class="metric-card">
              <h4>Disk I/O</h4>
              <div class="disk-stats">
                <div>📖 Read: {{ formatBytes(selectedContainer.disk_io?.read_bytes) }}</div>
                <div>✏️ Write: {{ formatBytes(selectedContainer.disk_io?.write_bytes) }}</div>
              </div>
            </div>
          </div>
        </div>

        <div v-if="activeDetailTab === 'History'" class="detail-content">
          <div class="history-controls">
            <select v-model="historyDuration">
              <option value="1h">Last Hour</option>
              <option value="6h">Last 6 Hours</option>
              <option value="24h">Last 24 Hours</option>
              <option value="7d">Last 7 Days</option>
            </select>
            <select v-model="historyInterval">
              <option value="1m">1 Minute</option>
              <option value="5m">5 Minutes</option>
              <option value="15m">15 Minutes</option>
              <option value="1h">1 Hour</option>
            </select>
            <button class="btn btn-sm btn-primary" @click="loadHistory">Load History</button>
          </div>
          <div class="chart-placeholder">
            <p>📈 Historical metrics chart will be displayed here</p>
          </div>
        </div>

        <button class="close-btn" @click="selectedContainer = null">✕</button>
      </div>
    </div>
  </div>
  </app-layout>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import AppLayout from '@/components/AppLayout.vue'
import apiClient from '../api/client'

interface ContainerMetrics {
  container_id: string
  container_name: string
  image: string
  timestamp: string
  cpu?: {
    usage_percent: number
    usageNano: number
  }
  memory?: {
    usage: number
    limit: number
    usage_percent: number
  }
  network?: {
    bytes_recv: number
    bytes_sent: number
  }
  disk_io?: {
    read_bytes: number
    write_bytes: number
  }
  pids: number
}

interface K8sCluster {
  id: string
  name: string
  status: string
  version: string
  node_count: number
  namespace_count: number
  pod_count: number
  api_endpoint: string
}

interface RemoteHost {
  id: string
  name: string
  endpoint: string
  status: string
  last_heartbeat: string
  os: string
  arch: string
}

interface HostOverview {
  total_hosts: number
  online_hosts: number
  offline_hosts: number
  total_containers: number
  avg_cpu_percent: number
  total_memory_used_gb: number
  total_memory_total_gb: number
  hosts: RemoteHost[]
}

const tabs = [
  { id: 'docker', label: 'Docker Containers', icon: '🐳' },
  { id: 'k8s', label: 'Kubernetes', icon: '☸️' },
  { id: 'hosts', label: 'Multi-Host', icon: '🖥️' },
  { id: 'top', label: 'Top Resources', icon: '📊' }
]

const detailTabs = ['Real-time', 'History']

const loading = ref(false)
const activeTab = ref('docker')
const searchQuery = ref('')
const sortBy = ref('name')
const realtimeEnabled = ref(false)
const wsConnected = ref(false)
const showAddClusterModal = ref(false)
const selectedContainer = ref<ContainerMetrics | null>(null)
const activeDetailTab = ref('Real-time')
const historyDuration = ref('24h')
const historyInterval = ref('5m')
const addingCluster = ref(false)

const containers = ref<ContainerMetrics[]>([])
const k8sClusters = ref<K8sCluster[]>([])
const overview = ref<any>({})
const topCPUContainers = ref<any[]>([])
const topMemoryContainers = ref<any[]>([])
const hostOverview = ref<HostOverview | null>(null)
const showAddHostModal = ref(false)
const selectedHost = ref<RemoteHost | null>(null)
const hostContainers = ref<any[]>([])
const hostMetrics = ref<any>(null)
const addingHost = ref(false)

const newHost = ref({
  name: '',
  endpoint: '',
  token: '',
  os: '',
  arch: ''
})

let ws: WebSocket | null = null
let refreshInterval: ReturnType<typeof setInterval> | null = null

const sortedContainers = computed(() => {
  let filtered = containers.value.filter(c =>
    c.container_name.toLowerCase().includes(searchQuery.value.toLowerCase()) ||
    c.image.toLowerCase().includes(searchQuery.value.toLowerCase())
  )

  if (sortBy.value === 'cpu') {
    filtered.sort((a, b) => (b.cpu?.usage_percent || 0) - (a.cpu?.usage_percent || 0))
  } else if (sortBy.value === 'memory') {
    filtered.sort((a, b) => (b.memory?.usage || 0) - (a.memory?.usage || 0))
  } else {
    filtered.sort((a, b) => a.container_name.localeCompare(b.container_name))
  }

  return filtered
})

const maxCPUPercent = computed(() => Math.max(...topCPUContainers.value.map(c => c.avg_cpu || 0), 100))
const maxMemoryUsage = computed(() => Math.max(...topMemoryContainers.value.map(c => c.avg_memory || 0), 1))

onMounted(async () => {
  await refreshAll()
  startAutoRefresh()
})

onUnmounted(() => {
  stopWebSocket()
  if (refreshInterval) clearInterval(refreshInterval)
})

async function refreshAll() {
  loading.value = true
  try {
    await Promise.all([
      fetchOverview(),
      fetchDockerMetrics(),
      fetchK8sClusters(),
      fetchTopResources(),
      fetchHostOverview()
    ])
  } catch (error) {
    console.error('Failed to refresh data:', error)
  } finally {
    loading.value = false
  }
}

async function fetchOverview() {
  try {
    const response = await apiClient.get('/monitor/overview')
    overview.value = response.data.data || {}
  } catch (error: any) {
    console.warn('Monitor overview unavailable:', error?.response?.status || error?.message)
    overview.value = {}
  }
}

async function fetchDockerMetrics() {
  try {
    const response = await apiClient.get('/monitor/docker/containers/realtime')
    const data = response.data.data
    containers.value = data ? Object.values(data) as ContainerMetrics[] : []
  } catch (error: any) {
    if (error?.response?.status !== 503) {
      console.warn('Docker metrics unavailable:', error?.response?.status || error?.message)
    }
    containers.value = []
  }
}

async function fetchK8sClusters() {
  try {
    const response = await apiClient.get('/monitor/kubernetes/clusters')
    k8sClusters.value = response.data.data || []
  } catch (error: any) {
    console.warn('K8s clusters unavailable:', error?.response?.status || error?.message)
    k8sClusters.value = []
  }
}

async function fetchTopResources() {
  try {
    const [cpuResponse, memResponse] = await Promise.all([
      apiClient.get('/monitor/docker/top/cpu?limit=10'),
      apiClient.get('/monitor/docker/top/memory?limit=10')
    ])
    topCPUContainers.value = cpuResponse.data.data
    topMemoryContainers.value = memResponse.data.data
  } catch (error) {
    console.error('Failed to fetch top resources:', error)
  }
}

function toggleRealtime() {
  if (realtimeEnabled.value) {
    connectWebSocket()
  } else {
    stopWebSocket()
  }
}

function connectWebSocket() {
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  const wsUrl = `${protocol}//${window.location.host}/ws/monitor/docker`

  ws = new WebSocket(wsUrl)

  ws.onopen = () => {
    wsConnected.value = true
    console.log('WebSocket connected')
    
    ws?.send(JSON.stringify({ action: 'ping' }))
  }

  ws.onmessage = (event) => {
    try {
      const data = JSON.parse(event.data)
      
      if (data.type === 'pong') {
        return
      }

      if (data.type === 'metrics_update') {
        const metrics = data.payload as ContainerMetrics
        const index = containers.value.findIndex(
          c => c.container_id === metrics.container_id
        )
        
        if (index !== -1) {
          containers.value[index] = metrics
        } else {
          containers.value.push(metrics)
        }

        if (selectedContainer.value?.container_id === metrics.container_id) {
          selectedContainer.value = metrics
        }
      }
    } catch (error) {
      console.error('Failed to parse WebSocket message:', error)
    }
  }

  ws.onclose = () => {
    wsConnected.value = false
    console.log('WebSocket disconnected')
    
    if (realtimeEnabled.value) {
      setTimeout(connectWebSocket, 5000)
    }
  }

  ws.onerror = (error) => {
    console.error('WebSocket error:', error)
    wsConnected.value = false
  }
}

function stopWebSocket() {
  if (ws) {
    ws.close()
    ws = null
  }
  wsConnected.value = false
}

function startAutoRefresh() {
  refreshInterval = setInterval(() => {
    if (!realtimeEnabled.value) {
      fetchDockerMetrics()
    }
  }, 10000)
}

function formatPercent(value?: number): string {
  if (!value) return '0.0'
  return value.toFixed(1)
}

function formatBytes(bytes?: number): string {
  if (!bytes) return '0 B'
  
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let unitIndex = 0
  let size = bytes

  while (size >= 1024 && unitIndex < units.length - 1) {
    size /= 1024
    unitIndex++
  }

  return `${size.toFixed(2)} ${units[unitIndex]}`
}

function viewContainerDetails(container: ContainerMetrics) {
  selectedContainer.value = container
}

function viewClusterDetails(cluster: K8sCluster) {
  console.log('View cluster details:', cluster)
}

async function removeCluster(clusterId: string) {
  if (!confirm('确定要删除该集群吗？')) return
  
  try {
    await apiClient.delete(`/monitor/kubernetes/clusters/${clusterId}`)
    k8sClusters.value = k8sClusters.value.filter(c => c.id !== clusterId)
  } catch (error) {
    console.error('Failed to remove cluster:', error)
    alert('删除集群失败')
  }
}

const newCluster = ref({
  name: '',
  method: 'kubeconfig',
  kubeconfig: '',
  apiEndpoint: '',
  caCert: '',
  token: ''
})

async function addCluster() {
  addingCluster.value = true
  
  try {
    const payload: Record<string, unknown> = {
      name: newCluster.value.name
    }

    if (newCluster.value.method === 'kubeconfig') {
      payload.kubeconfig = newCluster.value.kubeconfig
    } else {
      payload.api_endpoint = newCluster.value.apiEndpoint
      payload.ca_cert = newCluster.value.caCert
      payload.token = newCluster.value.token
    }

    await apiClient.post('/monitor/kubernetes/clusters', payload)
    
    showAddClusterModal.value = false
    newCluster.value = {
      name: '',
      method: 'kubeconfig',
      kubeconfig: '',
      apiEndpoint: '',
      caCert: '',
      token: ''
    }
    
    await fetchK8sClusters()
  } catch (error) {
    console.error('Failed to add cluster:', error)
    alert('添加集群失败: ' + error)
  } finally {
    addingCluster.value = false
  }
}

async function loadHistory() {
  if (!selectedContainer.value) return
  
  try {
    const response = await apiClient.get(
      `/monitor/docker/containers/${selectedContainer.value.container_id}/history?duration=${historyDuration.value}&interval=${historyInterval.value}`
    )
    console.log('History data:', response.data)
  } catch (error) {
    console.error('Failed to load history:', error)
  }
}

async function fetchHostOverview() {
  try {
    const response = await apiClient.get('/hosts/overview')
    hostOverview.value = response.data.data || null
  } catch (error: any) {
    console.warn('Host overview unavailable:', error?.response?.status || error?.message)
    hostOverview.value = null
  }
}

async function addHost() {
  addingHost.value = true
  try {
    await apiClient.post('/hosts', newHost.value)
    showAddHostModal.value = false
    newHost.value = { name: '', endpoint: '', token: '', os: '', arch: '' }
    await fetchHostOverview()
  } catch (error) {
    console.error('Failed to add host:', error)
    alert('添加主机失败: ' + error)
  } finally {
    addingHost.value = false
  }
}

async function removeHost(hostId: string) {
  if (!confirm('确定要删除该主机吗？')) return
  try {
    await apiClient.delete(`/hosts/${hostId}`)
    await fetchHostOverview()
    if (selectedHost.value?.id === hostId) {
      selectedHost.value = null
    }
  } catch (error) {
    console.error('Failed to remove host:', error)
    alert('删除主机失败: ' + error)
  }
}

async function collectFromHost(hostId: string) {
  try {
    await apiClient.post(`/hosts/${hostId}/collect`)
    await fetchHostOverview()
    if (selectedHost.value?.id === hostId) {
      await viewHostDetails(selectedHost.value)
    }
  } catch (error) {
    console.error('Failed to collect from host:', error)
    alert('Failed to collect metrics from host.')
  }
}

async function viewHostDetails(host: RemoteHost) {
  selectedHost.value = host
  try {
    const [metricsResp, containersResp] = await Promise.all([
      apiClient.get(`/hosts/${host.id}/metrics`),
      apiClient.get(`/hosts/${host.id}/containers`)
    ])
    hostMetrics.value = metricsResp.data.data
    hostContainers.value = containersResp.data.data || []
  } catch (error) {
    console.error('Failed to fetch host details:', error)
    hostMetrics.value = null
    hostContainers.value = []
  }
}

async function collectAllHosts() {
  try {
    await apiClient.post('/hosts/collect-all')
    await fetchHostOverview()
  } catch (error) {
    console.error('Failed to collect from all hosts:', error)
  }
}
</script>

<style scoped>
.container-monitor {
  padding: 20px;
  max-width: 1400px;
  margin: 0 auto;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 30px;
}

.page-header h1 {
  margin: 0;
  font-size: 2rem;
  color: var(--text-primary);
}

.header-actions {
  display: flex;
  gap: 12px;
  align-items: center;
}

.btn {
  padding: 10px 20px;
  border: none;
  border-radius: 6px;
  cursor: pointer;
  font-weight: 600;
  transition: all 0.2s;
  font-size: 14px;
}

.btn:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.btn-primary {
  background: #2563eb;
  color: white;
}

.btn-primary:hover:not(:disabled) {
  background: #1d4ed8;
}

.btn-success {
  background: #059669;
  color: white;
}

.btn-danger {
  background: #dc2626;
  color: white;
}

.btn-secondary {
  background: #6b7280;
  color: white;
}

.btn-sm {
  padding: 6px 12px;
  font-size: 13px;
}

.overview-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
  gap: 20px;
  margin-bottom: 30px;
}

.overview-card {
  background: white;
  border-radius: 12px;
  padding: 24px;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.08);
  border-left: 4px solid #2563eb;
}

.docker-card {
  border-left-color: #2563eb;
}

.k8s-card {
  border-left-color: #7c3aed;
}

.websocket-card {
  border-left-color: #059669;
}

.overview-card h3 {
  margin: 0 0 16px 0;
  font-size: 1.25rem;
  display: flex;
  align-items: center;
  gap: 8px;
}

.metric {
  margin: 12px 0;
  display: flex;
  justify-content: space-between;
  align-items: baseline;
}

.metric .value {
  font-size: 1.75rem;
  font-weight: 700;
  color: #1f2937;
}

.metric .label {
  color: #6b7280;
  font-size: 0.9rem;
}

.tabs {
  display: flex;
  gap: 8px;
  margin-bottom: 24px;
  border-bottom: 2px solid #e5e7eb;
  padding-bottom: 0;
}

.tab {
  padding: 12px 24px;
  background: none;
  border: none;
  border-bottom: 3px solid transparent;
  cursor: pointer;
  font-size: 15px;
  font-weight: 600;
  color: #6b7280;
  transition: all 0.2s;
  margin-bottom: -2px;
}

.tab:hover {
  color: #374151;
}

.tab.active {
  color: #2563eb;
  border-bottom-color: #2563eb;
}

.toolbar {
  display: flex;
  gap: 12px;
  margin-bottom: 20px;
  align-items: center;
}

.search-input {
  flex: 1;
  max-width: 400px;
  padding: 10px 16px;
  border: 2px solid #e5e7eb;
  border-radius: 8px;
  font-size: 14px;
  transition: border-color 0.2s;
}

.search-input:focus {
  outline: none;
  border-color: #2563eb;
}

.sort-select {
  padding: 10px 16px;
  border: 2px solid #e5e7eb;
  border-radius: 8px;
  font-size: 14px;
  background: white;
  cursor: pointer;
}

.containers-table-wrapper {
  overflow-x: auto;
  background: white;
  border-radius: 12px;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.08);
}

.containers-table {
  width: 100%;
  border-collapse: collapse;
}

.containers-table th,
.containers-table td {
  padding: 14px 16px;
  text-align: left;
  border-bottom: 1px solid #e5e7eb;
}

.containers-table th {
  background: #f9fafb;
  font-weight: 700;
  color: #374151;
  font-size: 13px;
  text-transform: uppercase;
  letter-spacing: 0.5px;
}

.container-row:hover {
  background: #f9fafb;
}

.name-cell strong {
  display: block;
  color: #1f2937;
  font-size: 15px;
}

.name-cell small {
  color: #6b7280;
  font-size: 12px;
}

.status-badge {
  padding: 4px 10px;
  border-radius: 12px;
  font-size: 12px;
  font-weight: 600;
}

.status-badge.running {
  background: #dcfce7;
  color: #166534;
}

.progress-bar {
  height: 8px;
  background: #e5e7eb;
  border-radius: 4px;
  overflow: hidden;
  margin: 6px 0;
}

.progress-bar.small {
  height: 4px;
  margin: 4px 0;
}

.progress-fill {
  height: 100%;
  border-radius: 4px;
  transition: width 0.3s ease;
}

.progress-fill.cpu {
  background: linear-gradient(90deg, #3b82f6, #60a5fa);
}

.progress-fill.memory {
  background: linear-gradient(90deg, #8b5cf6, #a78bfa);
}

.metric-value {
  font-size: 13px;
  font-weight: 600;
  color: #374151;
}

.memory-info {
  display: flex;
  flex-direction: column;
  font-size: 13px;
  margin: 4px 0;
}

.memory-info .limit {
  color: #6b7280;
  font-size: 11px;
}

.network-info {
  display: flex;
  flex-direction: column;
  font-size: 12px;
  gap: 2px;
}

.empty-state {
  text-align: center;
  padding: 60px 20px;
  color: #6b7280;
  background: white;
  border-radius: 12px;
}

.clusters-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(350px, 1fr));
  gap: 20px;
}

.cluster-card {
  background: white;
  border-radius: 12px;
  padding: 24px;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.08);
  border-top: 4px solid #7c3aed;
}

.cluster-card .card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;
}

.cluster-card h3 {
  margin: 0;
  font-size: 1.35rem;
  color: #1f2937;
}

.badge {
  padding: 4px 12px;
  border-radius: 12px;
  font-size: 12px;
  font-weight: 600;
}

.badge.success {
  background: #dcfce7;
  color: #166534;
}

.badge.error {
  background: #fee2e2;
  color: #991b1b;
}

.cluster-info {
  margin-bottom: 20px;
}

.info-item {
  display: flex;
  justify-content: space-between;
  padding: 8px 0;
  border-bottom: 1px solid #f3f4f6;
}

.info-item:last-child {
  border-bottom: none;
}

.info-item .label {
  color: #6b7280;
  font-size: 14px;
}

.info-item .value {
  font-weight: 600;
  color: #1f2937;
  font-size: 14px;
}

.info-item .value.small {
  font-size: 12px;
  word-break: break-all;
  max-width: 200px;
  text-align: right;
}

.card-actions {
  display: flex;
  gap: 8px;
  justify-content: flex-end;
}

.top-resources-section {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(450px, 1fr));
  gap: 24px;
}

.resource-panel {
  background: white;
  border-radius: 12px;
  padding: 24px;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.08);
}

.resource-panel h3 {
  margin: 0 0 20px 0;
  color: #1f2937;
  font-size: 1.2rem;
}

.top-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.top-item {
  display: grid;
  grid-template-columns: 30px 1fr auto 80px;
  gap: 12px;
  align-items: center;
  padding: 10px;
  background: #f9fafb;
  border-radius: 8px;
}

.rank {
  font-weight: 700;
  color: #6b7280;
  font-size: 14px;
}

.name {
  font-weight: 600;
  color: #1f2937;
  font-size: 14px;
}

.bar-container {
  height: 8px;
  background: #e5e7eb;
  border-radius: 4px;
  overflow: hidden;
}

.bar {
  height: 100%;
  border-radius: 4px;
  transition: width 0.3s ease;
}

.cpu-bar {
  background: linear-gradient(90deg, #ef4444, #f97316);
}

.memory-bar {
  background: linear-gradient(90deg, #8b5cf6, #ec4899);
}

.top-item .value {
  font-weight: 700;
  color: #1f2937;
  font-size: 14px;
  text-align: right;
}

.toggle-switch {
  display: flex;
  align-items: center;
  gap: 8px;
  cursor: pointer;
}

.toggle-switch input {
  display: none;
}

.slider {
  position: relative;
  width: 48px;
  height: 26px;
  background: #d1d5db;
  border-radius: 13px;
  transition: background 0.3s;
}

.slider::before {
  content: '';
  position: absolute;
  width: 22px;
  height: 22px;
  background: white;
  border-radius: 50%;
  top: 2px;
  left: 2px;
  transition: transform 0.3s;
  box-shadow: 0 2px 4px rgba(0, 0, 0, 0.2);
}

.toggle-switch input:checked + .slider {
  background: #2563eb;
}

.toggle-switch input:checked + .slider::before {
  transform: translateX(22px);
}

.toggle-switch .label {
  font-size: 14px;
  font-weight: 600;
  color: #374151;
}

.modal-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: rgba(0, 0, 0, 0.5);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
  padding: 20px;
}

.modal-content {
  background: white;
  border-radius: 16px;
  padding: 32px;
  max-width: 600px;
  width: 100%;
  max-height: 90vh;
  overflow-y: auto;
  position: relative;
}

.modal-content.large {
  max-width: 1000px;
}

.modal-content h2 {
  margin: 0 0 24px 0;
  color: #1f2937;
}

.form-group {
  margin-bottom: 20px;
}

.form-group label {
  display: block;
  margin-bottom: 8px;
  font-weight: 600;
  color: #374151;
  font-size: 14px;
}

.form-group input,
.form-group select,
.form-group textarea {
  width: 100%;
  padding: 10px 14px;
  border: 2px solid #e5e7eb;
  border-radius: 8px;
  font-size: 14px;
  font-family: inherit;
  transition: border-color 0.2s;
  box-sizing: border-box;
}

.form-group input:focus,
.form-group select:focus,
.form-group textarea:focus {
  outline: none;
  border-color: #2563eb;
}

.modal-actions {
  display: flex;
  gap: 12px;
  justify-content: flex-end;
  margin-top: 24px;
}

.detail-tabs {
  display: flex;
  gap: 8px;
  margin-bottom: 20px;
  border-bottom: 2px solid #e5e7eb;
}

.detail-tab {
  padding: 10px 20px;
  background: none;
  border: none;
  border-bottom: 3px solid transparent;
  cursor: pointer;
  font-weight: 600;
  color: #6b7280;
  margin-bottom: -2px;
  transition: all 0.2s;
}

.detail-tab.active {
  color: #2563eb;
  border-bottom-color: #2563eb;
}

.metrics-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: 16px;
}

.metric-card {
  background: #f9fafb;
  border-radius: 12px;
  padding: 20px;
  border: 1px solid #e5e7eb;
}

.metric-card h4 {
  margin: 0 0 12px 0;
  color: #6b7280;
  font-size: 14px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.5px;
}

.big-number {
  font-size: 2rem;
  font-weight: 700;
  color: #1f2937;
  margin-bottom: 8px;
}

.sub-text {
  font-size: 13px;
  color: #6b7280;
}

.network-stats,
.disk-stats {
  display: flex;
  flex-direction: column;
  gap: 6px;
  font-size: 14px;
  color: #374151;
}

.history-controls {
  display: flex;
  gap: 12px;
  margin-bottom: 20px;
  align-items: center;
}

.history-controls select {
  padding: 8px 12px;
  border: 2px solid #e5e7eb;
  border-radius: 6px;
  font-size: 14px;
}

.chart-placeholder {
  background: #f9fafb;
  border: 2px dashed #d1d5db;
  border-radius: 12px;
  padding: 60px;
  text-align: center;
  color: #6b7280;
}

.close-btn {
  position: absolute;
  top: 16px;
  right: 16px;
  background: none;
  border: none;
  font-size: 24px;
  cursor: pointer;
  color: #6b7280;
  width: 36px;
  height: 36px;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: all 0.2s;
}

.close-btn:hover {
  background: #f3f4f6;
  color: #1f2937;
}

.connected {
  color: #059669 !important;
}

.k8s-toolbar {
  margin-bottom: 20px;
}

.hosts-toolbar {
  display: flex;
  gap: 12px;
  margin-bottom: 20px;
}

.hosts-overview-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(240px, 1fr));
  gap: 16px;
  margin-bottom: 24px;
}

.host-overview-card {
  border-left-color: #f59e0b;
}

.hosts-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(320px, 1fr));
  gap: 16px;
  margin-bottom: 24px;
}

.host-card {
  background: white;
  border-radius: 12px;
  padding: 20px;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.08);
  cursor: pointer;
  transition: all 0.2s;
  border: 2px solid transparent;
}

.host-card:hover {
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.12);
  transform: translateY(-2px);
}

.host-card.host-selected {
  border-color: #2563eb;
  background: #eff6ff;
}

.host-card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 12px;
}

.host-card-header h4 {
  margin: 0;
  font-size: 1.1rem;
}

.host-card-info {
  margin-bottom: 12px;
}

.host-card-info .info-row {
  display: flex;
  justify-content: space-between;
  padding: 4px 0;
  font-size: 0.9rem;
}

.host-card-info .info-row .label {
  color: #6b7280;
}

.host-card-info .info-row .value {
  color: #1f2937;
  font-weight: 500;
}

.host-card-info .info-row .value.small {
  font-size: 0.8rem;
  max-width: 200px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.host-card-actions {
  display: flex;
  gap: 8px;
  justify-content: flex-end;
}

.host-detail-panel {
  background: white;
  border-radius: 12px;
  padding: 24px;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.08);
  margin-top: 24px;
}

.detail-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;
}

.detail-header h3 {
  margin: 0;
}

.detail-metrics-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
  gap: 16px;
  margin-bottom: 24px;
}

.detail-metrics-grid .metric-card {
  background: #f9fafb;
  border-radius: 8px;
  padding: 16px;
  text-align: center;
}

.detail-metrics-grid .metric-card h4 {
  margin: 0 0 8px 0;
  color: #6b7280;
  font-size: 0.9rem;
}

.detail-metrics-grid .big-number {
  font-size: 1.75rem;
  font-weight: 700;
  color: #1f2937;
}

.detail-metrics-grid .sub-text {
  font-size: 0.85rem;
  color: #9ca3af;
  margin-top: 4px;
}

.host-containers-section {
  border-top: 1px solid #e5e7eb;
  padding-top: 20px;
}

.host-containers-section h4 {
  margin: 0 0 16px 0;
}

.badge {
  padding: 4px 10px;
  border-radius: 12px;
  font-size: 0.75rem;
  font-weight: 600;
  text-transform: uppercase;
}

.badge.success {
  background: #d1fae5;
  color: #059669;
}

.badge.error {
  background: #fee2e2;
  color: #dc2626;
}

.status-badge.running {
  color: #059669;
}

.status-badge.stopped {
  color: #dc2626;
}

.empty-state.small {
  padding: 20px;
  font-size: 0.9rem;
}

[data-theme="dark"] .container-monitor {
  color: var(--text-primary);
}
[data-theme="dark"] .container-monitor h1,
[data-theme="dark"] .container-monitor h2,
[data-theme="dark"] .container-monitor h3,
[data-theme="dark"] .container-monitor h4 {
  color: var(--text-primary);
}
[data-theme="dark"] .container-monitor .overview-card,
[data-theme="dark"] .container-monitor .container-card,
[data-theme="dark"] .container-monitor .detail-panel,
[data-theme="dark"] .container-monitor .modal-content,
[data-theme="dark"] .container-monitor .table-container,
[data-theme="dark"] .container-monitor .host-card,
[data-theme="dark"] .container-monitor .compose-card {
  background: var(--bg-card);
  border-color: var(--border-color);
  color: var(--text-primary);
}
[data-theme="dark"] .container-monitor .tab,
[data-theme="dark"] .container-monitor .info-label,
[data-theme="dark"] .container-monitor .service-name,
[data-theme="dark"] .container-monitor .empty-state,
[data-theme="dark"] .container-monitor .metric-label {
  color: var(--text-secondary);
}
[data-theme="dark"] .container-monitor input,
[data-theme="dark"] .container-monitor select,
[data-theme="dark"] .container-monitor textarea {
  background: var(--bg-input);
  color: var(--text-primary);
  border-color: var(--border-color);
}
[data-theme="dark"] .container-monitor .search-input {
  background: var(--bg-input);
  color: var(--text-primary);
  border-color: var(--border-color);
}
</style>
