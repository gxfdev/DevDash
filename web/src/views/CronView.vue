<template>
  <div>
    <n-space justify="space-between" style="margin-bottom: 12px">
      <n-space>
        <n-button type="primary" @click="showModal = true">新建任务</n-button>
        <n-button @click="syncCron" :loading="syncing">同步到系统 Crontab</n-button>
      </n-space>
      <n-button @click="fetchJobs">刷新</n-button>
    </n-space>

    <n-data-table :columns="columns" :data="jobs" :bordered="true" size="small" />

    <n-modal v-model:show="showModal" :title="editingId ? '编辑任务' : '新建任务'" preset="dialog" style="width: 500px" positive-text="确认" negative-text="取消" @positive-click="handleSubmit">
      <n-form :model="form" label-placement="left" label-width="80">
        <n-form-item label="名称">
          <n-input v-model:value="form.name" placeholder="任务名称" />
        </n-form-item>
        <n-form-item label="调度">
          <n-input v-model:value="form.schedule" placeholder="*/5 * * * *" />
        </n-form-item>
        <n-form-item label="命令">
          <n-input v-model:value="form.command" type="textarea" :rows="3" placeholder="要执行的命令" />
        </n-form-item>
        <n-form-item v-if="editingId" label="启用">
          <n-switch v-model:value="form.enabled" />
        </n-form-item>
      </n-form>
    </n-modal>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, h, onMounted } from 'vue'
import { NButton, NSpace, NSwitch, NTag, useMessage, useDialog } from 'naive-ui'
import { cronApi } from '../api/client'

interface CronJob { id: number; name: string; schedule: string; command: string; enabled: boolean; created_at: string; updated_at: string }

const message = useMessage()
const dialog = useDialog()
const jobs = ref<CronJob[]>([])
const showModal = ref(false)
const syncing = ref(false)
const editingId = ref<number | null>(null)

const form = reactive({ name: '', schedule: '', command: '', enabled: true })

const columns = [
  { title: 'ID', key: 'id', width: 60 },
  { title: '名称', key: 'name', width: 150 },
  { title: '调度', key: 'schedule', width: 150 },
  { title: '命令', key: 'command', ellipsis: { tooltip: true } },
  {
    title: '状态', key: 'enabled', width: 80,
    render: (row: CronJob) => h(NTag, { type: row.enabled ? 'success' : 'default', size: 'small' }, () => row.enabled ? '启用' : '禁用'),
  },
  {
    title: '操作', key: 'actions', width: 200,
    render: (row: CronJob) => h(NSpace, { size: 'small' }, () => [
      h(NButton, { size: 'small', onClick: () => editJob(row) }, () => '编辑'),
      h(NButton, { size: 'small', type: row.enabled ? 'warning' : 'success', onClick: () => toggleJob(row) }, () => row.enabled ? '禁用' : '启用'),
      h(NButton, { size: 'small', type: 'error', onClick: () => deleteJob(row) }, () => '删除'),
    ]),
  },
]

async function fetchJobs() {
  try {
    const res = await cronApi.list()
    jobs.value = res.data
  } catch { message.error('获取任务列表失败') }
}

function editJob(job: CronJob) {
  editingId.value = job.id
  form.name = job.name
  form.schedule = job.schedule
  form.command = job.command
  form.enabled = job.enabled
  showModal.value = true
}

async function toggleJob(job: CronJob) {
  try {
    await cronApi.update(job.id, { name: job.name, schedule: job.schedule, command: job.command, enabled: !job.enabled })
    message.success(job.enabled ? '已禁用' : '已启用')
    fetchJobs()
  } catch { message.error('操作失败') }
}

function deleteJob(job: CronJob) {
  dialog.warning({
    title: '确认删除',
    content: `确定删除任务「${job.name}」？`,
    positiveText: '删除',
    negativeText: '取消',
    onPositiveClick: async () => {
      try {
        await cronApi.delete(job.id)
        message.success('已删除')
        fetchJobs()
      } catch { message.error('删除失败') }
    },
  })
}

async function handleSubmit() {
  if (!form.name || !form.schedule || !form.command) {
    message.warning('请填写完整信息')
    return false
  }
  try {
    if (editingId.value) {
      await cronApi.update(editingId.value, { name: form.name, schedule: form.schedule, command: form.command, enabled: form.enabled })
      message.success('已更新')
    } else {
      await cronApi.create({ name: form.name, schedule: form.schedule, command: form.command })
      message.success('已创建')
    }
    showModal.value = false
    resetForm()
    fetchJobs()
  } catch { message.error('操作失败') }
  return true
}

async function syncCron() {
  syncing.value = true
  try {
    await cronApi.sync()
    message.success('已同步到系统 Crontab')
  } catch (err: any) { message.error(err.response?.data?.error || '同步失败') }
  finally { syncing.value = false }
}

function resetForm() {
  editingId.value = null
  form.name = ''
  form.schedule = ''
  form.command = ''
  form.enabled = true
}

onMounted(fetchJobs)
</script>
