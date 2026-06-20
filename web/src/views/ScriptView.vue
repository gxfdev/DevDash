<template>
  <app-layout>
    <div class="page">
      <div class="page-header">
        <h2>脚本管理</h2>
        <n-space>
          <n-button type="primary" size="small" @click="openCreate">新建脚本</n-button>
        </n-space>
      </div>

      <n-data-table :columns="columns" :data="scripts" :bordered="false" :loading="loading" :row-key="(r:any) => r.id || String(Math.random())" />

      <n-modal v-model:show="showEditor" preset="card" :title="editingId ? '编辑脚本' : '新建脚本'" style="width:700px;max-height:85vh">
        <n-form :model="form" label-placement="top">
          <n-form-item label="脚本名称">
            <n-input v-model:value="form.name" placeholder="backup.sh" />
          </n-form-item>
          <n-form-item label="解释器">
            <n-select v-model:value="form.interpreter" :options="interpreterOptions" />
          </n-form-item>
          <n-form-item label="描述">
            <n-input v-model:value="form.description" placeholder="脚本用途说明" />
          </n-form-item>
          <n-form-item label="脚本内容">
            <n-input
              v-model:value="form.content"
              type="textarea"
              placeholder="#!/bin/bash&#10;echo 'Hello World'"
              :rows="15"
              style="font-family: 'Consolas', 'Courier New', monospace; font-size: 13px"
            />
          </n-form-item>
        </n-form>
        <template #footer>
          <n-button @click="showEditor = false">取消</n-button>
          <n-button type="primary" style="margin-left:8px" :loading="saving" @click="saveScript">保存</n-button>
        </template>
      </n-modal>

      <n-modal v-model:show="showResult" preset="card" title="执行结果" style="width:700px;max-height:80vh">
        <div v-if="execLoading" class="exec-loading">执行中...</div>
        <template v-else>
          <div class="result-meta">
            <n-tag :type="execResult.exit_code === 0 ? 'success' : 'error'" size="small">
              退出码: {{ execResult.exit_code }}
            </n-tag>
            <span class="result-duration">耗时: {{ execResult.duration_ms }}ms</span>
          </div>
          <pre class="result-output"><code>{{ execResult.output || '(无输出)' }}</code></pre>
        </template>
      </n-modal>
    </div>
  </app-layout>
</template>

<script setup lang="ts">
import { ref, onMounted, computed, h } from 'vue'
import { NButton, NTag, NPopconfirm, useMessage } from 'naive-ui'
import AppLayout from '@/components/AppLayout.vue'
import { scriptAPI, authAPI } from '@/api'
import { getErrorMessage } from '@/api/client'

const message = useMessage()

const scripts = ref<any[]>([])
const loading = ref(false)
const showEditor = ref(false)
const showResult = ref(false)
const saving = ref(false)
const execLoading = ref(false)
const editingId = ref<number | null>(null)
// 默认Linux，只有后端明确返回windows才用Windows
const isWindows = ref(false)
const form = ref({ name: '', interpreter: '/bin/bash', description: '', content: '' })
const execResult = ref<{ exit_code: number; output: string; duration_ms: number }>({ exit_code: 0, output: '', duration_ms: 0 })

const interpreterOptions = computed(() => isWindows.value ? [
  { label: 'PowerShell (powershell)', value: 'powershell' },
  { label: 'PowerShell (pwsh)', value: 'pwsh' },
  { label: 'CMD (cmd)', value: 'cmd' },
  { label: 'Python (python)', value: 'python' },
  { label: 'Node.js (node)', value: 'node' },
] : [
  { label: 'Bash (/bin/bash)', value: '/bin/bash' },
  { label: 'Sh (/bin/sh)', value: '/bin/sh' },
  { label: 'Python3 (/usr/bin/python3)', value: '/usr/bin/python3' },
  { label: 'Python (/usr/bin/python)', value: '/usr/bin/python' },
  { label: 'Node.js (/usr/bin/node)', value: '/usr/bin/node' },
  { label: 'Perl (/usr/bin/perl)', value: '/usr/bin/perl' },
])

const columns = [
  { title: '名称', key: 'name', width: 160 },
  { title: '解释器', key: 'interpreter', width: 160 },
  { title: '描述', key: 'description', ellipsis: { tooltip: true } },
  {
    title: '更新时间', key: 'updated_at', width: 170,
    render: (r: any) => r.updated_at ? new Date(r.updated_at).toLocaleString('zh-CN', { hour12: false }) : '--',
  },
  {
    title: '操作', key: 'actions', width: 220,
    render: (r: any) =>
      h('div', { style: 'display:flex;gap:6px' }, [
        h(NButton, { size: 'tiny', onClick: () => openEdit(r) }, () => '编辑'),
        h(NButton, { size: 'tiny', type: 'primary', onClick: () => executeScript(r), loading: execLoading.value }, () => '执行'),
        h(NPopconfirm, { onPositiveClick: () => delScript(r) }, {
          trigger: () => h(NButton, { size: 'tiny', type: 'error' }, () => '删除'),
          default: () => '确认删除？',
        }),
      ]),
  },
]

async function fetchScripts() {
  loading.value = true
  try {
    const { data } = await scriptAPI.list()
    scripts.value = Array.isArray(data) ? data : []
  } catch (e: unknown) {
    message.error('获取脚本列表失败: ' + getErrorMessage(e, ''))
    scripts.value = []
  } finally {
    loading.value = false
  }
}

function openCreate() {
  editingId.value = null
  form.value = { name: '', interpreter: isWindows.value ? 'powershell' : '/bin/bash', description: '', content: '' }
  showEditor.value = true
}

async function openEdit(r: any) {
  try {
    const { data } = await scriptAPI.get(String(r.id))
    form.value = {
      name: data.name || '',
      interpreter: data.interpreter || (isWindows.value ? 'powershell' : '/bin/bash'),
      description: data.description || '',
      content: data.content || '',
    }
    editingId.value = r.id
    showEditor.value = true
  } catch (e: unknown) {
    message.error(getErrorMessage(e, '加载脚本失败'))
  }
}

async function saveScript() {
  if (!form.value.name || !form.value.content) {
    message.warning('请填写脚本名称和内容')
    return
  }
  saving.value = true
  try {
    if (editingId.value) {
      await scriptAPI.update(String(editingId.value), form.value)
      message.success('保存成功')
    } else {
      await scriptAPI.create(form.value)
      message.success('创建成功')
    }
    showEditor.value = false
    fetchScripts()
  } catch (e: unknown) {
    message.error(getErrorMessage(e, '保存失败'))
  } finally {
    saving.value = false
  }
}

async function delScript(r: any) {
  try {
    await scriptAPI.delete(String(r.id))
    message.success('已删除')
    fetchScripts()
  } catch {
    message.error('删除失败')
  }
}

async function executeScript(r: any) {
  execLoading.value = true
  execResult.value = { exit_code: 0, output: '', duration_ms: 0 }
  showResult.value = true
  try {
    const { data } = await scriptAPI.execute(String(r.id))
    execResult.value = {
      exit_code: data.exit_code ?? 0,
      output: data.output ?? '',
      duration_ms: data.duration_ms ?? 0,
    }
  } catch (e: unknown) {
    execResult.value = {
      exit_code: -1,
      output: getErrorMessage(e, '执行失败'),
      duration_ms: 0,
    }
  } finally {
    execLoading.value = false
  }
}

onMounted(async () => {
  try {
    const { data: me } = await authAPI.me()
    if (me.server_os === 'windows') {
      isWindows.value = true
    }
  } catch { /* 默认Linux */ }
  fetchScripts()
})
</script>

<style scoped>
.page { padding: 24px; }
.page-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 16px; }
h2 { font-size: 20px; font-weight: 600; margin: 0; }
.exec-loading { text-align: center; padding: 40px; color: #8b949e; font-size: 14px; }
.result-meta { display: flex; align-items: center; gap: 12px; margin-bottom: 12px; }
.result-duration { font-size: 13px; color: #8b949e; }
.result-output { background: #0d1117; border: 1px solid #30363d; border-radius: 6px; padding: 16px; max-height: 50vh; overflow: auto; font-family: 'Consolas', 'Courier New', monospace; font-size: 13px; line-height: 1.5; color: #e6edf3; white-space: pre-wrap; word-break: break-all; margin: 0; }
.result-output code { font-family: inherit; }
</style>
