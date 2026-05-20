<template>
  <div class="docker-container">
    <div class="page-header">
      <h1>Docker 容器管理</h1>
      <div class="header-actions">
        <button class="btn btn-primary" @click="refreshContainers" :disabled="loading">
          <span v-if="!loading">🔄 刷新</span>
          <span v-else>⏳ 加载中...</span>
        </button>
        <button class="btn btn-success" @click="showDeployModal = true">
          🚀 快速部署
        </button>
      </div>
    </div>

    <!-- Docker Status Card -->
    <div class="status-card" :class="{ 'status-error': !dockerStatus.connected }">
      <div class="status-icon">{{ dockerStatus.connected ? '🟢' : '🔴' }}</div>
      <div class="status-info">
        <h3>{{ dockerStatus.connected ? 'Docker 守护进程已连接' : 'Docker 未连接' }}</h3>
        <p v-if="dockerStatus.info">{{ dockerStatus.info.ServerVersion }}</p>
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
        {{ tab.label }}
      </button>
    </div>

    <!-- Containers Tab -->
    <div v-if="activeTab === 'containers'" class="tab-content">
      <div class="toolbar">
        <input
          type="text"
          v-model="searchQuery"
          placeholder="搜索容器..."
          class="search-input"
        />
        <label class="checkbox-label">
          <input type="checkbox" v-model="showAll" />
          显示全部（含已停止）
        </label>
      </div>

      <div class="containers-grid">
        <div
          v-for="container in filteredContainers"
          :key="container.id"
          class="container-card"
          :class="getStatusClass(container.state)"
        >
          <div class="card-header">
            <h3>{{ container.name }}</h3>
            <span class="badge" :class="getStatusClass(container.state)">
              {{ container.state }}
            </span>
          </div>

          <div class="card-body">
            <div class="info-row">
              <span class="label">镜像：</span>
              <span class="value">{{ container.image }}</span>
            </div>
            <div class="info-row">
              <span class="label">ID：</span>
              <span class="value mono">{{ container.id.substring(0, 12) }}</span>
            </div>
            <div class="info-row" v-if="container.ports && container.ports.length > 0">
              <span class="label">端口：</span>
              <span class="value">{{ container.ports.join(', ') }}</span>
            </div>
            <div class="info-row" v-if="container.stats">
              <span class="label">CPU：</span>
              <span class="value">{{ formatPercent(container.stats.cpu_percent) }}%</span>
            </div>
            <div class="info-row" v-if="container.stats">
              <span class="label">内存：</span>
              <span class="value">{{ formatBytes(container.stats.memory_usage) }} / {{ formatBytes(container.stats.memory_limit) }}</span>
            </div>
          </div>

          <div class="card-actions">
            <button
              v-if="container.state !== 'running'"
              class="btn btn-sm btn-success"
              @click="startContainer(container.id)"
              title="启动"
            >
              ▶️ 启动
            </button>
            <button
              v-if="container.state === 'running'"
              class="btn btn-sm btn-warning"
              @click="stopContainer(container.id)"
              title="停止"
            >
              ⏹️ 停止
            </button>
            <button
              v-if="container.state === 'running'"
              class="btn btn-sm btn-info"
              @click="restartContainer(container.id)"
              title="重启"
            >
              🔄 重启
            </button>
            <button
              class="btn btn-sm btn-secondary"
              @click="showLogs(container.id, container.name)"
              title="日志"
            >
              📋 日志
            </button>
            <button
              class="btn btn-sm btn-danger"
              @click="removeContainer(container.id)"
              title="删除"
            >
              🗑️ 删除
            </button>
          </div>
        </div>
      </div>

      <div v-if="filteredContainers.length === 0 && !loading" class="empty-state">
        <p>暂无容器</p>
      </div>
    </div>

    <!-- Images Tab -->
    <div v-if="activeTab === 'images'" class="tab-content">
      <div class="toolbar">
        <button class="btn btn-primary" @click="showPullImageModal = true">
          + 拉取镜像
        </button>
      </div>

      <table class="data-table">
        <thead>
          <tr>
            <th>仓库</th>
            <th>标签</th>
            <th>ID</th>
            <th>大小</th>
            <th>创建时间</th>
            <th>操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="image in images" :key="image.id">
            <td>{{ image.repository }}</td>
            <td>{{ image.tag }}</td>
            <td class="mono">{{ image.id.substring(0, 12) }}</td>
            <td>{{ formatBytes(image.size) }}</td>
            <td>{{ formatDate(image.created) }}</td>
            <td>
              <button class="btn btn-sm btn-danger" @click="removeImage(image.id)">
                删除
              </button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <!-- Networks Tab -->
    <div v-if="activeTab === 'networks'" class="tab-content">
      <table class="data-table">
        <thead>
          <tr>
            <th>名称</th>
            <th>ID</th>
            <th>驱动</th>
            <th>作用域</th>
            <th>子网</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="network in networks" :key="network.id">
            <td>{{ network.name }}</td>
            <td class="mono">{{ network.id }}</td>
            <td>{{ network.driver }}</td>
            <td>{{ network.scope }}</td>
            <td>{{ network.ipv4 || '-' }}</td>
          </tr>
        </tbody>
      </table>
    </div>

    <!-- Volumes Tab -->
    <div v-if="activeTab === 'volumes'" class="tab-content">
      <table class="data-table">
        <thead>
          <tr>
            <th>名称</th>
            <th>驱动</th>
            <th>挂载点</th>
            <th>大小</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="volume in volumes" :key="volume.name">
            <td>{{ volume.name }}</td>
            <td>{{ volume.driver }}</td>
            <td class="mono">{{ volume.mountpoint }}</td>
            <td>{{ volume.usage_size || '-' }}</td>
          </tr>
        </tbody>
      </table>
    </div>

    <!-- Compose Tab -->
    <div v-if="activeTab === 'compose'" class="tab-content">
      <div class="toolbar">
        <button class="btn btn-primary" @click="loadComposeProjects">
          刷新项目
        </button>
      </div>

      <div v-for="project in composeProjects" :key="project.name" class="compose-project-card">
        <div class="project-header">
          <h3>{{ project.name }}</h3>
          <span class="badge">{{ project.status }}</span>
        </div>
        <div class="project-info">
          <p><strong>路径：</strong> {{ project.path }}</p>
            <p><strong>配置文件：</strong> {{ project.config_file }}</p>
        </div>
        <div v-if="project.services && project.services.length > 0" class="services-list">
          <h4>服务：</h4>
          <div class="service-items">
            <div
              v-for="service in project.services"
              :key="service.name"
              class="service-item"
            >
              <span class="service-name">{{ service.name }}</span>
              <span class="service-state" :class="getServiceStateClass(service.state)">
                {{ service.state }}
              </span>
              <button
                v-if="service.state === 'running'"
                class="btn btn-sm btn-warning"
                @click="restartComposeService(project.config_file, service.name)"
              >
                重启
              </button>
            </div>
          </div>
        </div>
        <div class="project-actions">
          <button
            v-if="project.status.includes('stopped')"
            class="btn btn-sm btn-success"
            @click="startComposeProject(project.config_file)"
          >
            启动
          </button>
          <button
            v-if="project.status.includes('running')"
            class="btn btn-sm btn-warning"
            @click="stopComposeProject(project.config_file)"
          >
            停止
          </button>
          <button
            class="btn btn-sm btn-info"
            @click="showComposeLogs(project.config_file)"
          >
            日志
          </button>
        </div>
      </div>
    </div>

    <!-- Logs Modal -->
    <div v-if="showLogsModal" class="modal-overlay" @click.self="closeLogsModal">
      <div class="modal-content logs-modal">
        <div class="modal-header">
          <h3>📋 容器日志：{{ logsTitle }}</h3>
          <button class="modal-close" @click="closeLogsModal">&times;</button>
        </div>
        <div class="modal-body">
          <pre class="logs-output"><code>{{ logsContent }}</code></pre>
        </div>
      </div>
    </div>

    <!-- Pull Image Modal -->
    <div v-if="showPullImageModal" class="modal-overlay" @click.self="showPullImageModal = false">
      <div class="modal-content">
        <div class="modal-header">
          <h3>📦 拉取镜像</h3>
          <button class="modal-close" @click="showPullImageModal = false">&times;</button>
        </div>
        <div class="modal-body">
          <div class="form-group">
            <label>镜像名称：</label>
            <input
              type="text"
              v-model="pullImageName"
              placeholder="例如：nginx:latest, mysql:8.0"
              class="form-control"
            />
          </div>
        </div>
        <div class="modal-footer">
          <button class="btn btn-secondary" @click="showPullImageModal = false">取消</button>
          <button class="btn btn-primary" @click="pullImage" :disabled="!pullImageName || pulling">
            {{ pulling ? '拉取中...' : '拉取镜像' }}
          </button>
        </div>
      </div>
    </div>

    <!-- Quick Deploy Modal -->
    <div v-if="showDeployModal" class="modal-overlay" @click.self="showDeployModal = false">
      <div class="modal-content deploy-modal">
        <div class="modal-header">
          <h3>🚀 模板快速部署</h3>
          <button class="modal-close" @click="showDeployModal = false">&times;</button>
        </div>
        <div class="modal-body">
          <div class="template-grid">
            <div
              v-for="template in deployTemplates"
              :key="template.id"
              class="template-card"
              :class="{ selected: selectedTemplate === template.id }"
              @click="selectedTemplate = template.id"
            >
              <div class="template-icon">{{ template.icon }}</div>
              <h4>{{ template.name }}</h4>
              <p>{{ template.description }}</p>
            </div>
          </div>

          <div v-if="selectedTemplate" class="deploy-config">
            <h4>{{ getSelectedTemplateName() }} 配置</h4>
            <div class="form-group" v-for="config in getTemplateConfig()" :key="config.key">
              <label>{{ config.label }}:</label>
              <input
                :type="config.type || 'text'"
                v-model="deployConfig[config.key]"
                :placeholder="config.placeholder"
                class="form-control"
              />
            </div>
          </div>
        </div>
        <div class="modal-footer">
          <button class="btn btn-secondary" @click="showDeployModal = false">取消</button>
          <button class="btn btn-primary" @click="deployFromTemplate" :disabled="!selectedTemplate || deploying">
            {{ deploying ? '部署中...' : '立即部署' }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { dockerApi } from '../api/client'

interface Container {
  id: string
  name: string
  image: string
  state: string
  status: string
  ports: string[]
  stats?: {
    cpu_percent: number
    memory_usage: number
    memory_limit: number
  }
}

interface Image {
  id: string
  repository: string
  tag: string
  size: number
  created: Date
}

interface Network {
  id: string
  name: string
  driver: string
  scope: string
  ipv4: string
}

interface Volume {
  name: string
  driver: string
  mountpoint: string
  usage_size: string
}

interface ComposeProject {
  name: string
  path: string
  status: string
  services: Array<{
    name: string
    state: string
    health: string
  }>
  config_file: string
}

const loading = ref(false)
const activeTab = ref('containers')
const searchQuery = ref('')
const showAll = ref(false)
const containers = ref<Container[]>([])
const images = ref<Image[]>([])
const networks = ref<Network[]>([])
const volumes = ref<Volume[]>([])
const composeProjects = ref<ComposeProject[]>([])

const showLogsModal = ref(false)
const logsContent = ref('')
const logsTitle = ref('')
const currentLogContainerId = ref('')

const showPullImageModal = ref(false)
const pullImageName = ref('')
const pulling = ref(false)

const showDeployModal = ref(false)
const selectedTemplate = ref('')
const deployConfig = ref<Record<string, string>>({})
const deploying = ref(false)

const dockerStatus = ref({
  connected: false,
  info: null as any
})

const tabs = [
  { id: 'containers', label: '📦 容器' },
  { id: 'images', label: '🖼️ 镜像' },
  { id: 'networks', label: '🌐 网络' },
  { id: 'volumes', label: '💾 卷' },
  { id: 'compose', label: '🎭 编排' }
]

const deployTemplates = [
  {
    id: 'nginx',
    icon: '🌐',
    name: 'Nginx',
    description: '高性能 Web 服务器和反向代理'
  },
  {
    id: 'mysql',
    icon: '🐬',
    name: 'MySQL',
    description: '流行的开源关系型数据库'
  },
  {
    id: 'redis',
    icon: '🔴',
    name: 'Redis',
    description: '内存数据结构存储，用于缓存'
  },
  {
    id: 'wordpress',
    icon: '✍️',
    name: 'WordPress',
    description: '流行的内容管理系统，基于 MySQL'
  }
]

let refreshInterval: number | null = null

onMounted(async () => {
  await checkDockerStatus()
  await refreshContainers()
  startAutoRefresh()
})

onUnmounted(() => {
  stopAutoRefresh()
})

function startAutoRefresh() {
  refreshInterval = window.setInterval(() => {
    if (activeTab.value === 'containers') {
      refreshContainers()
    }
  }, 10000)
}

function stopAutoRefresh() {
  if (refreshInterval) {
    clearInterval(refreshInterval)
    refreshInterval = null
  }
}

async function checkDockerStatus() {
  try {
    const response = await dockerApi.ping()
    dockerStatus.value.connected = response.success

    if (dockerStatus.value.connected) {
      const infoResponse = await dockerApi.info()
      if (infoResponse.success) {
        dockerStatus.value.info = infoResponse.data
      }
    }
  } catch (error) {
    console.error('Failed to check Docker status:', error)
    dockerStatus.value.connected = false
  }
}

async function refreshContainers() {
  loading.value = true
  try {
    const response = await dockerApi.listContainers(showAll.value)
    if (response.success) {
      containers.value = response.data

      // Fetch stats for running containers
      for (const container of containers.value) {
        if (container.state === 'running') {
          try {
            const statsResponse = await dockerApi.getContainerStats(container.id)
            if (statsResponse.success) {
              container.stats = statsResponse.data
            }
          } catch (error) {
            // Ignore stats errors
          }
        }
      }
    }

    // Load other data based on active tab
    if (activeTab.value === 'images') {
      const imagesResponse = await dockerApi.listImages()
      if (imagesResponse.success) {
        images.value = imagesResponse.data
      }
    } else if (activeTab.value === 'networks') {
      const networksResponse = await dockerApi.listNetworks()
      if (networksResponse.success) {
        networks.value = networksResponse.data
      }
    } else if (activeTab.value === 'volumes') {
      const volumesResponse = await dockerApi.listVolumes()
      if (volumesResponse.success) {
        volumes.value = volumesResponse.data
      }
    }
  } catch (error) {
    console.error('Failed to load containers:', error)
  } finally {
    loading.value = false
  }
}

async function startContainer(id: string) {
  try {
    const response = await dockerApi.startContainer(id)
    if (response.success) {
      await refreshContainers()
    }
  } catch (error) {
    alert('启动容器失败: ' + error)
  }
}

async function stopContainer(id: string) {
  if (!confirm('确定要停止该容器吗？')) return

  try {
    const response = await dockerApi.stopContainer(id)
    if (response.success) {
      await refreshContainers()
    }
  } catch (error) {
    alert('停止容器失败: ' + error)
  }
}

async function restartContainer(id: string) {
  try {
    const response = await dockerApi.restartContainer(id)
    if (response.success) {
      await refreshContainers()
    }
  } catch (error) {
    alert('重启容器失败: ' + error)
  }
}

async function removeContainer(id: string) {
  if (!confirm('确定要删除该容器吗？此操作不可撤销。')) return

  try {
    const response = await dockerApi.removeContainer(id, true, false)
    if (response.success) {
      await refreshContainers()
    }
  } catch (error) {
    alert('删除容器失败: ' + error)
  }
}

async function showLogs(id: string, name: string) {
  currentLogContainerId.value = id
  logsTitle.value = name
  logsContent.value = '加载日志中...'
  showLogsModal.value = true

  try {
    const response = await fetch(`/api/v1/docker/containers/${id}/logs?tail=200`, {
      headers: {
        'Authorization': `Bearer ${localStorage.getItem('token')}`
      }
    })

    if (!response.ok) throw new Error('Failed to fetch logs')

    const reader = response.body?.getReader()
    if (reader) {
      const decoder = new TextDecoder()
      while (true) {
        const { done, value } = await reader.read()
        if (done) break
        logsContent.value += decoder.decode(value, { stream: true })
      }
    }
  } catch (error) {
    logsContent.value = '加载日志出错: ' + error
  }
}

function closeLogsModal() {
  showLogsModal.value = false
  logsContent.value = ''
  currentLogContainerId.value = ''
}

async function pullImage() {
  if (!pullImageName.value) return

  pulling.value = true
  try {
    const response = await dockerApi.pullImage(pullImageName.value)
    if (response.success) {
      showPullImageModal.value = false
      pullImageName.value = ''
      await refreshContainers()
      // Switch to images tab
      activeTab.value = 'images'
    }
  } catch (error) {
    alert('拉取镜像失败: ' + error)
  } finally {
    pulling.value = false
  }
}

async function removeImage(id: string) {
  if (!confirm('确定要删除该镜像吗？')) return

  try {
    const response = await dockerApi.removeImage(id, true)
    if (response.success) {
      await refreshContainers()
    }
  } catch (error) {
    alert('删除镜像失败: ' + error)
  }
}

async function loadComposeProjects() {
  try {
    const response = await dockerApi.listComposeProjects()
    if (response.success) {
      composeProjects.value = response.data
    }
  } catch (error) {
    console.error('Failed to load compose projects:', error)
  }
}

async function startComposeProject(path: string) {
  try {
    const response = await dockerApi.startComposeProject(path)
    if (response.success) {
      await loadComposeProjects()
    }
  } catch (error) {
    alert('启动编排项目失败: ' + error)
  }
}

async function stopComposeProject(path: string) {
  if (!confirm('确定要停止该编排项目吗？')) return

  try {
    const response = await dockerApi.stopComposeProject(path)
    if (response.success) {
      await loadComposeProjects()
    }
  } catch (error) {
    alert('停止编排项目失败: ' + error)
  }
}

async function restartComposeService(path: string, serviceName: string) {
  try {
    const response = await dockerApi.restartComposeService(path, serviceName)
    if (response.success) {
      await loadComposeProjects()
    }
  } catch (error) {
    alert('重启服务失败: ' + error)
  }
}

function showComposeLogs(path: string) {
  // Similar to container logs but for compose
  alert('编排日志功能即将上线！')
}

async function deployFromTemplate() {
  if (!selectedTemplate.value) return

  deploying.value = true
  try {
    const response = await dockerApi.deployFromTemplate(selectedTemplate.value, deployConfig.value)
    if (response.success) {
      showDeployModal.value = false
      selectedTemplate.value = ''
      deployConfig.value = {}
      alert('部署成功！请在编排标签页中管理。')
      activeTab.value = 'compose'
      await loadComposeProjects()
    }
  } catch (error) {
    alert('部署失败: ' + error)
  } finally {
    deploying.value = false
  }
}

function getSelectedTemplateName(): string {
  const template = deployTemplates.find(t => t.id === selectedTemplate.value)
  return template?.name || ''
}

function getTemplateConfig(): Array<{ key: string; label: string; placeholder: string; type?: string }> {
  switch (selectedTemplate.value) {
    case 'nginx':
      return [
        { key: 'port', label: '端口', placeholder: '80', type: 'number' }
      ]
    case 'mysql':
      return [
        { key: 'root_password', label: 'Root 密码', placeholder: 'root123' },
        { key: 'database', label: '数据库名', placeholder: 'appdb' },
        { key: 'user', label: '用户名', placeholder: 'appuser' },
        { key: 'password', label: '密码', placeholder: 'apppass' },
        { key: 'port', label: '端口', placeholder: '3306', type: 'number' }
      ]
    case 'redis':
      return [
        { key: 'password', label: '密码', placeholder: 'redis123' },
        { key: 'port', label: '端口', placeholder: '6379', type: 'number' }
      ]
    case 'wordpress':
      return [
        { key: 'db_root_password', label: '数据库 Root 密码', placeholder: 'root123' },
        { key: 'db_password', label: '数据库密码', placeholder: 'wp123' },
        { key: 'port', label: '端口', placeholder: '8080', type: 'number' }
      ]
    default:
      return []
  }
}

// Computed properties
const filteredContainers = computed(() => {
  let result = containers.value

  if (searchQuery.value) {
    const query = searchQuery.value.toLowerCase()
    result = result.filter(c =>
      c.name.toLowerCase().includes(query) ||
      c.image.toLowerCase().includes(query) ||
      c.id.toLowerCase().includes(query)
    )
  }

  return result
})

// Helper functions
function getStatusClass(state: string): string {
  switch (state.toLowerCase()) {
    case 'running':
      return 'status-running'
    case 'exited':
      return 'status-stopped'
    case 'created':
      return 'status-created'
    default:
      return 'status-other'
  }
}

function getServiceStateClass(state: string): string {
  return getStatusClass(state)
}

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B'

  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))

  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
}

function formatPercent(value: number): string {
  return value.toFixed(1)
}

function formatDate(date: Date): string {
  return new Date(date).toLocaleDateString()
}
</script>

<style scoped>
.docker-container {
  padding: 20px;
  max-width: 1400px;
  margin: 0 auto;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;
}

.page-header h1 {
  margin: 0;
  font-size: 28px;
  color: #1a1a1a;
}

.header-actions {
  display: flex;
  gap: 10px;
}

.status-card {
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  border-radius: 12px;
  padding: 20px;
  margin-bottom: 20px;
  display: flex;
  align-items: center;
  gap: 15px;
  color: white;
}

.status-card.status-error {
  background: linear-gradient(135deg, #ff6b6b 0%, #ee5a52 100%);
}

.status-icon {
  font-size: 36px;
}

.status-info h3 {
  margin: 0 0 5px 0;
  font-size: 18px;
}

.status-info p {
  margin: 0;
  opacity: 0.9;
  font-size: 14px;
}

.tabs {
  display: flex;
  gap: 5px;
  margin-bottom: 20px;
  border-bottom: 2px solid #e5e7eb;
  padding-bottom: 0;
}

.tab {
  padding: 12px 24px;
  border: none;
  background: transparent;
  cursor: pointer;
  font-size: 15px;
  font-weight: 500;
  color: #6b7280;
  border-bottom: 3px solid transparent;
  transition: all 0.3s ease;
  position: relative;
  bottom: -2px;
}

.tab:hover {
  color: #374151;
  background: rgba(99, 102, 241, 0.05);
}

.tab.active {
  color: #6366f1;
  border-bottom-color: #6366f1;
  background: rgba(99, 102, 241, 0.08);
}

.toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 15px;
  gap: 10px;
  flex-wrap: wrap;
}

.search-input {
  padding: 10px 16px;
  border: 1px solid #d1d5db;
  border-radius: 8px;
  font-size: 14px;
  min-width: 300px;
  outline: none;
  transition: all 0.3s ease;
}

.search-input:focus {
  border-color: #6366f1;
  box-shadow: 0 0 0 3px rgba(99, 102, 241, 0.1);
}

.checkbox-label {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 14px;
  color: #4b5563;
  cursor: pointer;
}

.containers-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(380px, 1fr));
  gap: 16px;
}

.container-card {
  background: white;
  border-radius: 12px;
  padding: 20px;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.1);
  border-left: 4px solid #d1d5db;
  transition: all 0.3s ease;
}

.container-card:hover {
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
  transform: translateY(-2px);
}

.container-card.status-running {
  border-left-color: #10b981;
}

.container-card.status-stopped {
  border-left-color: #ef4444;
}

.container-card.status-created {
  border-left-color: #f59e0b;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 15px;
}

.card-header h3 {
  margin: 0;
  font-size: 18px;
  color: #111827;
  word-break: break-all;
}

.badge {
  padding: 4px 12px;
  border-radius: 20px;
  font-size: 12px;
  font-weight: 600;
  text-transform: uppercase;
}

.badge.status-running {
  background: #d1fae5;
  color: #065f46;
}

.badge.status-stopped {
  background: #fee2e2;
  color: #991b1b;
}

.badge.status-created {
  background: #fef3c7;
  color: #92400e;
}

.badge.status-other {
  background: #e5e7eb;
  color: #374151;
}

.card-body {
  margin-bottom: 15px;
}

.info-row {
  display: flex;
  margin-bottom: 8px;
  font-size: 14px;
}

.info-row .label {
  width: 70px;
  color: #6b7280;
  font-weight: 500;
  flex-shrink: 0;
}

.info-row .value {
  color: #111827;
  word-break: break-all;
}

.mono {
  font-family: 'Courier New', monospace;
  font-size: 13px;
}

.card-actions {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}

.btn {
  padding: 8px 16px;
  border: none;
  border-radius: 6px;
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.2s ease;
  display: inline-flex;
  align-items: center;
  gap: 4px;
}

.btn:hover:not(:disabled) {
  transform: translateY(-1px);
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.15);
}

.btn:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.btn-sm {
  padding: 6px 12px;
  font-size: 12px;
}

.btn-primary {
  background: #6366f1;
  color: white;
}

.btn-primary:hover:not(:disabled) {
  background: #5558e3;
}

.btn-success {
  background: #10b981;
  color: white;
}

.btn-success:hover:not(:disabled) {
  background: #059669;
}

.btn-warning {
  background: #f59e0b;
  color: white;
}

.btn-warning:hover:not(:disabled) {
  background: #d97706;
}

.btn-danger {
  background: #ef4444;
  color: white;
}

.btn-danger:hover:not(:disabled) {
  background: #dc2626;
}

.btn-info {
  background: #3b82f6;
  color: white;
}

.btn-info:hover:not(:disabled) {
  background: #2563eb;
}

.btn-secondary {
  background: #6b7280;
  color: white;
}

.btn-secondary:hover:not(:disabled) {
  background: #4b5563;
}

.empty-state {
  text-align: center;
  padding: 60px 20px;
  color: #9ca3af;
  font-size: 16px;
}

.data-table {
  width: 100%;
  border-collapse: collapse;
  background: white;
  border-radius: 8px;
  overflow: hidden;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.1);
}

.data-table thead {
  background: #f9fafb;
}

.data-table th,
.data-table td {
  padding: 12px 16px;
  text-align: left;
  border-bottom: 1px solid #e5e7eb;
  font-size: 14px;
}

.data-table th {
  font-weight: 600;
  color: #374151;
}

.data-table tbody tr:hover {
  background: #f9fafb;
}

.compose-project-card {
  background: white;
  border-radius: 12px;
  padding: 20px;
  margin-bottom: 16px;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.1);
}

.project-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 12px;
}

.project-header h3 {
  margin: 0;
  font-size: 20px;
  color: #111827;
}

.project-info p {
  margin: 4px 0;
  font-size: 14px;
  color: #6b7280;
}

.services-list {
  margin: 15px 0;
}

.services-list h4 {
  margin: 0 0 10px 0;
  font-size: 16px;
  color: #374151;
}

.service-items {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.service-item {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 8px 12px;
  background: #f9fafb;
  border-radius: 6px;
}

.service-name {
  font-weight: 600;
  color: #111827;
  min-width: 150px;
}

.service-state {
  padding: 3px 10px;
  border-radius: 12px;
  font-size: 11px;
  font-weight: 600;
  text-transform: uppercase;
}

.project-actions {
  display: flex;
  gap: 8px;
  margin-top: 15px;
  padding-top: 15px;
  border-top: 1px solid #e5e7eb;
}

/* Modal Styles */
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
  border-radius: 12px;
  max-width: 800px;
  width: 100%;
  max-height: 90vh;
  overflow-y: auto;
  box-shadow: 0 20px 25px -5px rgba(0, 0, 0, 0.1), 0 10px 10px -5px rgba(0, 0, 0, 0.04);
}

.logs-modal {
  max-width: 900px;
}

.modal-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 20px 24px;
  border-bottom: 1px solid #e5e7eb;
}

.modal-header h3 {
  margin: 0;
  font-size: 20px;
  color: #111827;
}

.modal-close {
  background: none;
  border: none;
  font-size: 28px;
  cursor: pointer;
  color: #9ca3af;
  line-height: 1;
  padding: 0;
  width: 32px;
  height: 32px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 6px;
  transition: all 0.2s ease;
}

.modal-close:hover {
  background: #f3f4f6;
  color: #374151;
}

.modal-body {
  padding: 24px;
}

.modal-footer {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
  padding: 16px 24px;
  border-top: 1px solid #e5e7eb;
  background: #f9fafb;
  border-radius: 0 0 12px 12px;
}

.logs-output {
  background: #1e293b;
  color: #e2e8f0;
  padding: 20px;
  border-radius: 8px;
  font-family: 'Courier New', monospace;
  font-size: 13px;
  line-height: 1.6;
  max-height: 500px;
  overflow-y: auto;
  white-space: pre-wrap;
  word-break: break-all;
  margin: 0;
}

.form-group {
  margin-bottom: 16px;
}

.form-group label {
  display: block;
  margin-bottom: 6px;
  font-weight: 500;
  color: #374151;
  font-size: 14px;
}

.form-control {
  width: 100%;
  padding: 10px 14px;
  border: 1px solid #d1d5db;
  border-radius: 8px;
  font-size: 14px;
  outline: none;
  transition: all 0.3s ease;
  box-sizing: border-box;
}

.form-control:focus {
  border-color: #6366f1;
  box-shadow: 0 0 0 3px rgba(99, 102, 241, 0.1);
}

.template-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(180px, 1fr));
  gap: 16px;
  margin-bottom: 24px;
}

.template-card {
  padding: 20px;
  border: 2px solid #e5e7eb;
  border-radius: 12px;
  cursor: pointer;
  text-align: center;
  transition: all 0.3s ease;
}

.template-card:hover {
  border-color: #6366f1;
  background: rgba(99, 102, 241, 0.03);
  transform: translateY(-2px);
}

.template-card.selected {
  border-color: #6366f1;
  background: rgba(99, 102, 241, 0.08);
  box-shadow: 0 0 0 3px rgba(99, 102, 241, 0.1);
}

.template-icon {
  font-size: 40px;
  margin-bottom: 10px;
}

.template-card h4 {
  margin: 0 0 8px 0;
  font-size: 16px;
  color: #111827;
}

.template-card p {
  margin: 0;
  font-size: 13px;
  color: #6b7280;
  line-height: 1.4;
}

.deploy-config {
  background: #f9fafb;
  padding: 20px;
  border-radius: 8px;
  border: 1px solid #e5e7eb;
}

.deploy-config h4 {
  margin: 0 0 16px 0;
  font-size: 16px;
  color: #111827;
}

@media (max-width: 768px) {
  .containers-grid {
    grid-template-columns: 1fr;
  }

  .page-header {
    flex-direction: column;
    align-items: flex-start;
    gap: 12px;
  }

  .toolbar {
    flex-direction: column;
    align-items: stretch;
  }

  .search-input {
    min-width: auto;
    width: 100%;
  }

  .tabs {
    overflow-x: auto;
  }

  .tab {
    padding: 10px 16px;
    font-size: 14px;
    white-space: nowrap;
  }
}
</style>