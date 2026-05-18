<template>
  <app-layout>
    <div class="page">
      <div class="page-header">
        <h2>软件商店</h2>
        <n-select v-model:value="selectedNode" :options="nodeOptions" placeholder="选择节点" style="width:200px" clearable />
      </div>

      <n-tabs type="line" animated>
        <n-tab-pane name="installed" tab="已安装">
          <n-data-table :columns="installedColumns" :data="installedList" :bordered="false" :loading="loading" row-key="name" />
        </n-tab-pane>
        <n-tab-pane name="all" tab="全部软件">
          <div class="category-tabs">
            <n-button v-for="cat in categories" :key="cat" :type="activeCat === cat ? 'primary' : 'default'" size="small" @click="activeCat = cat">{{ cat }}</n-button>
          </div>
          <div class="soft-grid">
            <div v-for="s in filteredSoft" :key="s.name" class="soft-card">
              <div class="soft-icon">{{ s.icon }}</div>
              <div class="soft-name">{{ s.name }}</div>
              <div class="soft-ver">{{ s.version }}</div>
              <div class="soft-desc">{{ s.desc }}</div>
              <n-button type="primary" size="tiny" :loading="s.installing" :disabled="s.installed" @click="install(s)">
                {{ s.installed ? '已安装' : '安装' }}
              </n-button>
            </div>
          </div>
        </n-tab-pane>
      </n-tabs>
    </div>
  </app-layout>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, h, watch } from 'vue'
import { NButton, NTag, useMessage } from 'naive-ui'
import AppLayout from '@/components/AppLayout.vue'
import { useNodesStore } from '@/stores/nodes'
import client from '@/api/client'

const nodesStore = useNodesStore()
const message = useMessage()

const selectedNode = ref<string | null>(null)
const installedList = ref<any[]>([])
const loading = ref(false)
const activeCat = ref('Web服务器')
const installing = ref<Set<string>>(new Set())

const categories = ['Web服务器', '数据库', '缓存', '容器', '语言环境', '运维工具']
const allSoft = ref([
  { name: 'Nginx', cat: 'Web服务器', icon: '🌐', version: '1.24+', desc: '高性能HTTP服务器', installed: false, installing: false },
  { name: 'Apache', cat: 'Web服务器', icon: '🕸️', version: '2.4+', desc: 'Apache HTTP服务器', installed: false, installing: false },
  { name: 'MySQL', cat: '数据库', icon: '🗄️', version: '8.0', desc: '关系型数据库', installed: false, installing: false },
  { name: 'PostgreSQL', cat: '数据库', icon: '🐘', version: '15+', desc: '高级关系型数据库', installed: false, installing: false },
  { name: 'Redis', cat: '缓存', icon: '🔴', version: '7.0+', desc: '内存数据结构存储', installed: false, installing: false },
  { name: 'Docker', cat: '容器', icon: '🐳', version: '24+', desc: '容器化平台', installed: false, installing: false },
  { name: 'JDK', cat: '语言环境', icon: '☕', version: '17', desc: 'Java运行环境', installed: false, installing: false },
  { name: 'Node.js', cat: '语言环境', icon: '🟢', version: '20 LTS', desc: 'Node.js运行时', installed: false, installing: false },
  { name: 'Python', cat: '语言环境', icon: '🐍', version: '3.11', desc: 'Python解释器', installed: false, installing: false },
  { name: 'UFW', cat: '运维工具', icon: '🔥', version: '0.36+', desc: 'Ubuntu防火墙', installed: false, installing: false },
])

const filteredSoft = computed(() => allSoft.value.filter(s => s.cat === activeCat.value))

const nodeOptions = computed(() => nodesStore.nodes.map((n: any) => ({ label: n.name || n.hostname, value: n.id })))

const installedColumns = [
  { title: '软件', key: 'name' },
  { title: '版本', key: 'version' },
  { title: '节点', key: 'node_name' },
  {
    title: '状态',
    key: 'status',
    render: (row: any) => h(NTag, { type: row.running ? 'success' : 'default', size: 'small' }, () => row.running ? '运行中' : '已停止'),
  },
  {
    title: '操作',
    key: 'actions',
    render: (row: any) =>
      h('div', { style: 'display:flex;gap:6px' }, [
        h(NButton, { size: 'tiny', onClick: () => serviceCtrl(row, 'start') }, () => '启动'),
        h(NButton, { size: 'tiny', onClick: () => serviceCtrl(row, 'stop') }, () => '停止'),
        h(NButton, { size: 'tiny', onClick: () => serviceCtrl(row, 'restart') }, () => '重启'),
      ]),
  },
]

async function load() {
  if (!selectedNode.value) return
  loading.value = true
  try {
    const { data } = await client.get(`/node/${selectedNode.value}/software`)
    installedList.value = data || []
  } catch {} finally { loading.value = false }
}

async function install(s: any) {
  if (!selectedNode.value) { message.warning('请先选择节点'); return }
  s.installing = true
  try {
    await client.post(`/node/${selectedNode.value}/software/install`, { name: s.name })
    message.success(`${s.name} 安装中，稍后刷新查看`)
    s.installed = true
  } catch (e: any) {
    message.error(e?.response?.data?.error || '安装失败')
  } finally { s.installing = false }
}

async function serviceCtrl(row: any, action: string) {
  try {
    await client.post(`/node/${selectedNode.value}/software/service`, { name: row.name, action })
    message.success(`${action === 'start' ? '启动' : action === 'stop' ? '停止' : '重启'}成功`)
    load()
  } catch { message.error('操作失败') }
}

onMounted(async () => {
  await nodesStore.fetchNodes()
  if (nodesStore.nodes.length) { selectedNode.value = nodesStore.nodes[0]?.id; load() }
})

watch(selectedNode, () => { load() })
</script>

<style scoped>
.page { padding: 24px; }
.page-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 16px; }
h2 { font-size: 20px; font-weight: 600; margin: 0; }
.category-tabs { display: flex; gap: 8px; margin-bottom: 16px; flex-wrap: wrap; }
.soft-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(200px, 1fr)); gap: 12px; }
.soft-card { background: #161b22; border: 1px solid #30363d; border-radius: 8px; padding: 16px; display: flex; flex-direction: column; gap: 6px; }
.soft-icon { font-size: 28px; }
.soft-name { font-weight: 600; color: #e6edf3; }
.soft-ver { font-size: 12px; color: #8b949e; }
.soft-desc { font-size: 12px; color: #6e7681; flex: 1; }
</style>