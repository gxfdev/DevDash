<template>
  <div style="display: flex; gap: 12px; height: calc(100vh - 100px)">
    <div style="width: 300px; display: flex; flex-direction: column">
      <n-space style="margin-bottom: 8px">
        <n-input v-model:value="currentPath" size="small" placeholder="路径" @keyup.enter="navigateTo(currentPath)" />
        <n-button size="small" @click="navigateTo(currentPath)">前往</n-button>
        <n-button size="small" @click="navigateTo('/')">根目录</n-button>
      </n-space>
      <div style="flex: 1; overflow: auto; border: 1px solid var(--n-border-color); border-radius: 4px">
        <n-data-table :columns="fileColumns" :data="files" :bordered="false" size="tiny" :row-props="rowProps" />
      </div>
    </div>
    <div style="flex: 1; display: flex; flex-direction: column">
      <n-space style="margin-bottom: 8px" justify="space-between">
        <n-text strong>{{ editingFile || '选择文件查看/编辑' }}</n-text>
        <n-space>
          <n-button size="small" @click="showNewFileModal = true">新建文件</n-button>
          <n-button size="small" @click="showNewDirModal = true">新建目录</n-button>
          <n-button size="small" type="primary" :disabled="!editingFile" @click="saveFile">保存</n-button>
          <n-button size="small" type="error" :disabled="!selectedPath" @click="deleteSelected">删除</n-button>
        </n-space>
      </n-space>
      <div style="flex: 1; border: 1px solid var(--n-border-color); border-radius: 4px; overflow: hidden">
        <textarea v-model="fileContent" class="file-editor" :placeholder="editingFile ? '' : '点击左侧文件查看内容'" :readonly="!editingFile" />
      </div>
    </div>

    <n-modal v-model:show="showNewFileModal" title="新建文件" preset="dialog" positive-text="创建" negative-text="取消" @positive-click="createNewFile">
      <n-input v-model:value="newFilePath" placeholder="/path/to/newfile.txt" />
    </n-modal>
    <n-modal v-model:show="showNewDirModal" title="新建目录" preset="dialog" positive-text="创建" negative-text="取消" @positive-click="createNewDir">
      <n-input v-model:value="newDirPath" placeholder="/path/to/newdir" />
    </n-modal>
  </div>
</template>

<script setup lang="ts">
import { ref, h, onMounted } from 'vue'
import { NButton, NIcon, useMessage, useDialog } from 'naive-ui'
import { fileApi } from '../api/client'

interface FileInfo { name: string; path: string; isDir: boolean; size: number; modTime: string; mode: string }

const message = useMessage()
const dialog = useDialog()
const currentPath = ref('/')
const files = ref<FileInfo[]>([])
const editingFile = ref('')
const fileContent = ref('')
const selectedPath = ref('')
const showNewFileModal = ref(false)
const showNewDirModal = ref(false)
const newFilePath = ref('')
const newDirPath = ref('')

const fileColumns = [
  { title: '名称', key: 'name', render: (row: FileInfo) => h('span', { style: row.isDir ? 'color: #63e2b7; font-weight: bold' : '' }, (row.isDir ? '📁 ' : '📄 ') + row.name) },
  { title: '大小', key: 'size', width: 80, render: (row: FileInfo) => row.isDir ? '-' : formatSize(row.size) },
]

function formatSize(bytes: number): string {
  if (bytes < 1024) return bytes + ' B'
  if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' K'
  if (bytes < 1024 * 1024 * 1024) return (bytes / 1024 / 1024).toFixed(1) + ' M'
  return (bytes / 1024 / 1024 / 1024).toFixed(1) + ' G'
}

function rowProps(row: FileInfo) {
  return {
    style: 'cursor: pointer',
    onClick: () => {
      selectedPath.value = row.path
      if (row.isDir) {
        navigateTo(row.path)
      } else {
        openFile(row.path)
      }
    },
  }
}

async function navigateTo(path: string) {
  try {
    const res = await fileApi.list(path)
    files.value = res.data.files
    currentPath.value = res.data.path
    if (path !== '/') {
      files.value = [{ name: '..', path: parentDir(path), isDir: true, size: 0, modTime: '', mode: '' }, ...files.value]
    }
  } catch (err: any) { message.error(err.response?.data?.error || '无法访问') }
}

function parentDir(path: string): string {
  const parts = path.replace(/\/$/, '').split('/')
  parts.pop()
  return parts.length <= 1 ? '/' : parts.join('/')
}

async function openFile(path: string) {
  try {
    const res = await fileApi.read(path)
    editingFile.value = res.data.path
    fileContent.value = res.data.content
  } catch (err: any) { message.error(err.response?.data?.error || '无法读取') }
}

async function saveFile() {
  if (!editingFile.value) return
  try {
    await fileApi.write(editingFile.value, fileContent.value)
    message.success('已保存')
  } catch (err: any) { message.error(err.response?.data?.error || '保存失败') }
}

async function deleteSelected() {
  if (!selectedPath.value) return
  dialog.warning({
    title: '确认删除',
    content: `确定删除「${selectedPath.value}」？此操作不可恢复！`,
    positiveText: '删除',
    negativeText: '取消',
    onPositiveClick: async () => {
      try {
        await fileApi.delete(selectedPath.value)
        message.success('已删除')
        if (editingFile.value === selectedPath.value) {
          editingFile.value = ''
          fileContent.value = ''
        }
        selectedPath.value = ''
        navigateTo(currentPath.value)
      } catch (err: any) { message.error(err.response?.data?.error || '删除失败') }
    },
  })
}

async function createNewFile() {
  if (!newFilePath.value) { message.warning('请输入路径'); return false }
  try {
    await fileApi.write(newFilePath.value, '')
    message.success('已创建')
    navigateTo(currentPath.value)
    newFilePath.value = ''
  } catch (err: any) { message.error(err.response?.data?.error || '创建失败') }
  return true
}

async function createNewDir() {
  if (!newDirPath.value) { message.warning('请输入路径'); return false }
  try {
    await fileApi.mkdir(newDirPath.value)
    message.success('已创建')
    navigateTo(currentPath.value)
    newDirPath.value = ''
  } catch (err: any) { message.error(err.response?.data?.error || '创建失败') }
  return true
}

onMounted(() => navigateTo('/'))
</script>

<style scoped>
.file-editor {
  width: 100%;
  height: 100%;
  border: none;
  outline: none;
  resize: none;
  padding: 12px;
  font-family: 'Cascadia Code', 'Fira Code', monospace;
  font-size: 13px;
  line-height: 1.6;
  background: #1e1e1e;
  color: #d4d4d4;
  tab-size: 4;
}
</style>
