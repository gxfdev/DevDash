<template>
  <app-layout>
    <div class="page">
      <div class="page-header">
        <h2>文件管理</h2>
        <n-space>
          <n-select v-model:value="selectedNode" :options="nodeOptions" placeholder="选择节点" style="width:180px" />
          <n-button size="small" @click="refresh">刷新</n-button>
        </n-space>
      </div>

      <div class="file-layout">
        <div class="dir-tree">
          <div class="tree-title">目录</div>
          <div class="tree-scroll">
            <div v-for="d in dirs" :key="d" class="tree-item" :class="{ active: d === currentDir }" @click="cd(d)">
              {{ d }}
            </div>
          </div>
        </div>

        <div class="file-panel">
          <div class="file-toolbar">
            <n-input v-model:value="currentDir" size="small" style="flex:1" @keydown.enter="cd(currentDir)" />
            <n-button size="small" @click="cd(currentDir)">跳转</n-button>
            <n-button size="small" @click="showNewFile = true">新建文件</n-button>
            <n-button size="small" @click="showNewDir = true">新建目录</n-button>
            <n-button size="small" @click="uploadFile">上传</n-button>
            <input ref="fileInput" type="file" multiple style="display:none" @change="doUpload" />
          </div>
          <div class="file-list-container">
            <n-data-table
              :columns="fileColumns"
              :data="files"
              size="small"
              :bordered="false"
              :row-key="(row: Record<string, unknown>) => (row.path || row.name || String(Math.random()))"
              :key="'files-' + currentDir"
              :scroll-x="800"
              flex-height
            />
            <div v-if="!loading && files.length === 0" class="empty">目录为空或无法访问</div>
          </div>
        </div>
      </div>
    </div>

    <n-modal v-model:show="showPreview" preset="card" title="文件预览" style="width:80vw;max-width:1000px;max-height:80vh" :mask-closable="true">
      <div v-if="previewLoading" class="preview-loading">加载中...</div>
      <pre v-else-if="previewContent !== null" class="file-preview"><code>{{ previewContent }}</code></pre>
      <div v-else class="empty">无法预览此文件</div>
    </n-modal>

    <n-modal v-model:show="showNewFile" preset="card" title="新建文件" style="width:400px">
      <n-input v-model:value="newFileName" placeholder="filename.ext" />
      <template #footer>
        <n-button @click="showNewFile = false">取消</n-button>
        <n-button type="primary" style="margin-left:8px" @click="createFile">创建</n-button>
      </template>
    </n-modal>

    <n-modal v-model:show="showNewDir" preset="card" title="新建目录" style="width:400px">
      <n-input v-model:value="newDirName" placeholder="目录名" />
      <template #footer>
        <n-button @click="showNewDir = false">取消</n-button>
        <n-button type="primary" style="margin-left:8px" @click="createDir">创建</n-button>
      </template>
    </n-modal>
  </app-layout>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, h, watch, nextTick } from 'vue'
import { NButton, NTag, NPopconfirm, useMessage } from 'naive-ui'
import AppLayout from '@/components/AppLayout.vue'
import { useNodesStore } from '@/stores/nodes'
import client, { getErrorMessage } from '@/api/client'

const nodesStore = useNodesStore()
const message = useMessage()

const selectedNode = ref<string | null>(null)
const currentDir = ref('')
const files = ref<any[]>([])
const dirs = ref<string[]>([])
const loading = ref(false)
const showNewFile = ref(false)
const showNewDir = ref(false)
const newFileName = ref('')
const newDirName = ref('')
const fileInput = ref<HTMLInputElement>()
const isWindows = navigator.userAgent.indexOf('Windows') > -1

const showPreview = ref(false)
const previewContent = ref<string | null>(null)
const previewLoading = ref(false)

const nodeOptions = computed(() => nodesStore.nodes.map((n: { name: string; hostname?: string; ip: string; id: string }) => ({ label: n.name || n.hostname || n.ip, value: n.id })))

function getDefaultRoot() {
  return isWindows ? 'C:\\' : '/'
}

function normalizePath(p: string): string {
  if (!p) return getDefaultRoot()
  p = p.trim()
  if (isWindows) {
    if (p === '/' || p === '') return 'C:\\'
    return p.replace(/\//g, '\\')
  }
  if (!p.startsWith('/')) p = '/' + p
  return p
}

const fileColumns = [
  {
    title: '', key: 'type', width: 30,
    render: (r: any) => String(r.type) === 'dir' ? '📁' : Boolean(r.is_dir) ? '📁' : '📄',
  },
  { title: '名称', key: 'name', ellipsis: { tooltip: true } },
  { title: '大小', key: 'size', width: 90, render: (r: any) => (r.type === 'dir' || r.is_dir) ? '--' : formatSize(Number(r.size) || 0) },
  { title: '权限', key: 'mode', width: 80, render: (r: any) => String(r.mode || '--') },
  { title: '修改时间', key: 'mtime', width: 160, render: (r: any) => String(r.mtime || r.modified || '--') },
  {
    title: '操作',
    key: 'actions',
    width: 200,
    render: (r: any) =>
      h('div', { style: 'display:flex;gap:4px;flex-wrap:wrap' }, [
        (r.type === 'dir' || r.is_dir)
          ? h(NButton, { size: 'tiny', onClick: () => cd(r.path || joinPath(currentDir.value, r.name)) }, () => '进入')
          : h(NButton, { size: 'tiny', onClick: () => previewFile(r) }, () => '预览'),
        h(NButton, { size: 'tiny', onClick: () => downloadFile(r) }, () => '下载'),
        h(NPopconfirm, { onPositiveClick: () => delFile(r) }, {
          trigger: () => h(NButton, { size: 'tiny', type: 'error' }, () => '删除'),
          default: () => '确认删除？',
        }),
      ]),
  },
]

function formatSize(s: number) {
  if (!s || s < 0) return '0 B'
  if (s < 1024) return s + ' B'
  if (s < 1024 * 1024) return (s / 1024).toFixed(1) + ' KB'
  return (s / 1024 / 1024).toFixed(1) + ' MB'
}

function joinPath(base: string, name: string): string {
  if (isWindows) {
    base = base.replace(/\\/g, '/')
    if (base.endsWith('/')) base = base.slice(0, -1)
    return (base + '/' + name).replace(/\//g, '\\')
  }
  if (base.endsWith('/')) return base + name
  return base + '/' + name
}

async function fetchDir() {
  if (!selectedNode.value) return
  loading.value = true
  try {
    const path = normalizePath(currentDir.value)
    const { data } = await client.get(`/node/${selectedNode.value}/fs/list`, { params: { path } })
    files.value = Array.isArray(data) ? data : []

    if (isWindows && !dirs.value.some(d => d === 'C:\\')) {
      const drives = ['C:\\', 'D:\\', 'E:\\']
      dirs.value.unshift(...drives.filter(d => !dirs.value.includes(d)))
    }
  } catch (e: unknown) {
    message.error('读取失败: ' + (getErrorMessage(e, '未知错误')))
    files.value = []
  } finally {
    loading.value = false
  }
}

async function cd(path: string) {
  path = normalizePath(path)
  currentDir.value = path
  if (!dirs.value.includes(path)) dirs.value.push(path)
  await fetchDir()
}

function refresh() { fetchDir() }

async function previewFile(f: any) {
  showPreview.value = true
  previewContent.value = null
  previewLoading.value = true

  try {
    const fpath = f.path || joinPath(currentDir.value, f.name)
    const response = await client.get(`/node/${selectedNode.value}/fs/read`, {
      params: { path: fpath },
      responseType: 'text',
      transformResponse: [(data: string) => data],
    })

    previewContent.value = typeof response.data === 'string' ? response.data : JSON.stringify(response.data, null, 2)
  } catch (e: unknown) {
    message.error(getErrorMessage(e, '读取文件失败'))
    previewContent.value = null
  } finally {
    previewLoading.value = false
  }
}

async function createFile() {
  if (!selectedNode.value || !newFileName.value.trim()) { message.warning('请输入文件名'); return }
  try {
    const fullPath = joinPath(currentDir.value, newFileName.value.trim())
    await client.post(`/node/${selectedNode.value}/fs/mkfile`, { path: fullPath })
    message.success('创建成功')
    showNewFile.value = false
    newFileName.value = ''
    fetchDir()
  } catch (e: unknown) { message.error(getErrorMessage(e, '创建失败')) }
}

async function createDir() {
  if (!selectedNode.value || !newDirName.value.trim()) { message.warning('请输入目录名'); return }
  try {
    const fullPath = joinPath(currentDir.value, newDirName.value.trim())
    await client.post(`/node/${selectedNode.value}/fs/mkdir`, { path: fullPath })
    message.success('创建成功')
    showNewDir.value = false
    newDirName.value = ''
    fetchDir()
  } catch (e: unknown) { message.error(getErrorMessage(e, '创建失败')) }
}

async function delFile(f: any) {
  try {
    const fpath = f.path || joinPath(currentDir.value, f.name)
    await client.delete(`/node/${selectedNode.value}/fs/remove`, { data: { path: fpath } })
    message.success('已删除')
    fetchDir()
  } catch (e: unknown) { message.error(getErrorMessage(e, '删除失败')) }
}

function downloadFile(f: any) {
  const fpath = encodeURIComponent(f.path || joinPath(currentDir.value, f.name))
  window.open(`/api/v1/node/${selectedNode.value}/fs/download?path=${fpath}`, '_blank')
}

function uploadFile() { fileInput.value?.click() }

async function doUpload(e: Event) {
  const input = e.target as HTMLInputElement
  if (!input.files?.length || !selectedNode.value) return
  const form = new FormData()
  for (const f of input.files) form.append('files', f)
  form.append('path', currentDir.value)
  try {
    await client.post(`/node/${selectedNode.value}/fs/upload`, form, { headers: { 'Content-Type': 'multipart/form-data' } })
    message.success('上传成功')
    fetchDir()
  } catch (e: unknown) { message.error(getErrorMessage(e, '上传失败')) }
  input.value = ''
}

onMounted(async () => {
  await nodesStore.fetchNodes()
  if (nodesStore.nodes.length) {
    selectedNode.value = nodesStore.nodes[0]?.id
    currentDir.value = getDefaultRoot()
    dirs.value = [getDefaultRoot()]
    await nextTick()
    fetchDir()
  }
})

watch(selectedNode, async () => {
  if (selectedNode.value) {
    currentDir.value = getDefaultRoot()
    dirs.value = [getDefaultRoot()]
    await fetchDir()
  }
})
</script>

<style scoped>
.page { padding: 24px; height: 100%; display: flex; flex-direction: column; }
.page-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 16px; flex-shrink: 0; }
h2 { font-size: 20px; font-weight: 600; margin: 0; }
.file-layout { display: grid; grid-template-columns: 200px 1fr; gap: 16px; flex: 1; min-height: 0; overflow: hidden; }
.dir-tree { background: #161b22; border: 1px solid #30363d; border-radius: 8px; overflow: hidden; display: flex; flex-direction: column; min-height: 0; }
.tree-title { padding: 12px 16px; font-size: 12px; color: #8b949e; border-bottom: 1px solid #21262d; flex-shrink: 0; }
.tree-scroll { padding: 8px; overflow-y: auto; flex: 1; min-height: 0; }
.tree-item { padding: 6px 10px; border-radius: 4px; cursor: pointer; font-size: 13px; color: #e6edf3; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.tree-item:hover { background: #21262d; }
.tree-item.active { background: #388bfd33; color: #58a6ff; font-weight: 500; }
.file-panel { background: #161b22; border: 1px solid #30363d; border-radius: 8px; display: flex; flex-direction: column; overflow: hidden; min-height: 0; }
.file-toolbar { padding: 12px; display: flex; gap: 8px; align-items: center; border-bottom: 1px solid #21262d; flex-wrap: wrap; flex-shrink: 0; }
.file-list-container { flex: 1; overflow: hidden; padding: 0 12px 12px; min-height: 0; }
.empty { padding: 40px; text-align: center; color: #6e7681; }
.preview-loading { text-align: center; padding: 20px; color: #8b949e; }
.file-preview { background: #0d1117; border: 1px solid #30363d; border-radius: 6px; padding: 16px; max-height: 60vh; overflow: auto; font-family: 'Consolas', 'Courier New', monospace; font-size: 13px; line-height: 1.5; color: #e6edf3; white-space: pre-wrap; word-break: break-all; margin: 0; }
.file-preview code { font-family: inherit; }
</style>