<template>
  <div>
    <n-space justify="space-between" style="margin-bottom: 12px">
      <n-button type="primary" @click="openCreate">新建脚本</n-button>
      <n-button @click="fetchScripts">刷新</n-button>
    </n-space>

    <n-grid :cols="2" :x-gap="12" :y-gap="12">
      <n-gi>
        <n-data-table :columns="columns" :data="scripts" :bordered="true" size="small" :row-props="rowProps" />
      </n-gi>
      <n-gi>
        <n-card :title="editingId ? '编辑: ' + form.name : '新建脚本'" size="small">
          <n-form :model="form" label-placement="left" label-width="60">
            <n-form-item label="名称">
              <n-input v-model:value="form.name" placeholder="脚本名称" />
            </n-form-item>
            <n-form-item label="描述">
              <n-input v-model:value="form.description" placeholder="脚本描述" />
            </n-form-item>
            <n-form-item label="解释器">
              <n-select v-model:value="form.interpreter" :options="interpreterOptions" />
            </n-form-item>
            <n-form-item label="内容">
              <n-input v-model:value="form.content" type="textarea" :rows="12" placeholder="#!/bin/bash\necho 'Hello World'" style="font-family: monospace" />
            </n-form-item>
          </n-form>
          <n-space>
            <n-button type="primary" @click="handleSave" :loading="saving">保存</n-button>
            <n-button v-if="editingId" type="warning" @click="runScript" :loading="running">执行</n-button>
            <n-button @click="resetForm">重置</n-button>
          </n-space>
        </n-card>

        <n-card v-if="execResult" title="执行结果" size="small" style="margin-top: 12px">
          <n-descriptions bordered :column="2" size="small">
            <n-descriptions-item label="退出码">
              <n-tag :type="execResult.exitCode === 0 ? 'success' : 'error'" size="small">{{ execResult.exitCode }}</n-tag>
            </n-descriptions-item>
            <n-descriptions-item label="耗时">{{ execResult.startTime }} → {{ execResult.endTime }}</n-descriptions-item>
          </n-descriptions>
          <div v-if="execResult.output" style="margin-top: 8px">
            <n-text strong>输出：</n-text>
            <pre class="output-box">{{ execResult.output }}</pre>
          </div>
          <div v-if="execResult.error" style="margin-top: 8px">
            <n-text strong type="error">错误：</n-text>
            <pre class="output-box error">{{ execResult.error }}</pre>
          </div>
        </n-card>
      </n-gi>
    </n-grid>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, h, onMounted } from 'vue'
import { NButton, NSpace, NTag, useMessage } from 'naive-ui'
import { scriptApi } from '../api/client'

interface Script { id: number; name: string; description: string; content: string; interpreter: string; created_at: string; updated_at: string }
interface ExecResult { exitCode: number; output: string; error: string; startTime: string; endTime: string }

const message = useMessage()
const scripts = ref<Script[]>([])
const editingId = ref<number | null>(null)
const saving = ref(false)
const running = ref(false)
const execResult = ref<ExecResult | null>(null)

const form = reactive({ name: '', description: '', content: '', interpreter: '/bin/bash' })

const interpreterOptions = [
  { label: '/bin/bash', value: '/bin/bash' },
  { label: '/bin/sh', value: '/bin/sh' },
  { label: '/usr/bin/python3', value: '/usr/bin/python3' },
  { label: '/usr/bin/perl', value: '/usr/bin/perl' },
  { label: '/usr/bin/node', value: '/usr/bin/node' },
]

const columns = [
  { title: 'ID', key: 'id', width: 50 },
  { title: '名称', key: 'name', width: 120 },
  { title: '描述', key: 'description', ellipsis: { tooltip: true } },
  { title: '解释器', key: 'interpreter', width: 130 },
  {
    title: '操作', key: 'actions', width: 150,
    render: (row: Script) => h(NSpace, { size: 'small' }, () => [
      h(NButton, { size: 'small', onClick: () => editScript(row) }, () => '编辑'),
      h(NButton, { size: 'small', type: 'error', onClick: () => deleteScript(row) }, () => '删除'),
    ]),
  },
]

function rowProps(row: Script) {
  return { style: editingId.value === row.id ? 'background: rgba(24,160,88,0.1)' : '' }
}

async function fetchScripts() {
  try {
    const res = await scriptApi.list()
    scripts.value = res.data
  } catch { message.error('获取脚本列表失败') }
}

function editScript(sc: Script) {
  editingId.value = sc.id
  form.name = sc.name
  form.description = sc.description
  form.content = sc.content
  form.interpreter = sc.interpreter
  execResult.value = null
}

function openCreate() {
  resetForm()
}

async function handleSave() {
  if (!form.name || !form.content) { message.warning('请填写名称和内容'); return }
  saving.value = true
  try {
    if (editingId.value) {
      await scriptApi.update(editingId.value, { name: form.name, description: form.description, content: form.content, interpreter: form.interpreter })
      message.success('已更新')
    } else {
      await scriptApi.create({ name: form.name, description: form.description, content: form.content, interpreter: form.interpreter })
      message.success('已创建')
    }
    fetchScripts()
  } catch { message.error('保存失败') }
  finally { saving.value = false }
}

async function runScript() {
  if (!editingId.value) return
  running.value = true
  execResult.value = null
  try {
    const res = await scriptApi.run(editingId.value)
    execResult.value = res.data
  } catch { message.error('执行失败') }
  finally { running.value = false }
}

async function deleteScript(sc: Script) {
  try {
    await scriptApi.delete(sc.id)
    message.success('已删除')
    if (editingId.value === sc.id) resetForm()
    fetchScripts()
  } catch { message.error('删除失败') }
}

function resetForm() {
  editingId.value = null
  form.name = ''
  form.description = ''
  form.content = ''
  form.interpreter = '/bin/bash'
  execResult.value = null
}

onMounted(fetchScripts)
</script>

<style scoped>
.output-box {
  background: #1e1e1e;
  color: #d4d4d4;
  padding: 8px 12px;
  border-radius: 4px;
  font-family: monospace;
  font-size: 13px;
  max-height: 200px;
  overflow: auto;
  white-space: pre-wrap;
  margin: 0;
}
.output-box.error {
  color: #e88080;
}
</style>
