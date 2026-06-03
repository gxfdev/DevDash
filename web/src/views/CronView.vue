<template>
  <app-layout>
    <div class="page">
      <div class="page-header">
        <h2>计划任务</h2>
        <n-space>
          <n-button type="primary" size="small" @click="showAdd = true">新建任务</n-button>
        </n-space>
      </div>

      <n-data-table :columns="columns" :data="jobs" :bordered="false" :loading="loading" row-key="id" />

      <n-modal v-model:show="showAdd" preset="card" title="新建计划任务" style="width:500px">
        <n-form :model="form" label-placement="top">
          <n-form-item label="任务名称">
            <n-input v-model:value="form.name" placeholder="数据库每日备份" />
          </n-form-item>
          <n-form-item label="Cron 表达式">
            <n-input v-model:value="form.cron" placeholder="0 2 * * *" />
            <template #label-extra>
              <span style="font-size:11px;color:#8b949e">分 时 日 月 周</span>
            </template>
          </n-form-item>
          <n-form-item label="执行命令">
            <n-input v-model:value="form.command" type="textarea" placeholder="tar -czf /backup/db.tar.gz /data" :rows="3" />
          </n-form-item>
        </n-form>
        <template #footer>
          <n-button @click="showAdd = false">取消</n-button>
          <n-button type="primary" style="margin-left:8px" :loading="saving" @click="addJob">创建</n-button>
        </template>
      </n-modal>

      <n-modal v-model:show="showLog" preset="card" :title="`执行日志: ${logJobName}`" style="width:700px">
        <div style="margin-bottom:12px;display:flex;gap:8px;align-items:center">
          <n-input v-model:value="logSearch" size="small" placeholder="搜索关键词" style="width:200px" clearable />
        </div>
        <n-data-table :columns="logColumns" :data="filteredLogs" size="small" :bordered="false" />
      </n-modal>
    </div>
  </app-layout>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, h } from 'vue'
import { NButton, NTag, NSwitch, NPopconfirm, useMessage } from 'naive-ui'
import AppLayout from '@/components/AppLayout.vue'
import { cronAPI } from '@/api'
import { getErrorMessage } from '@/api/client'

const message = useMessage()

const jobs = ref<any[]>([])
const logs = ref<any[]>([])
const loading = ref(false)
const showAdd = ref(false)
const showLog = ref(false)
const logJobName = ref('')
const logSearch = ref('')
const saving = ref(false)
const form = ref({ name: '', cron: '', command: '' })

const filteredLogs = computed(() => {
  if (!logSearch.value) return logs.value
  const kw = logSearch.value.toLowerCase()
  return logs.value.filter((l: any) =>
    (l.command || '').toLowerCase().includes(kw) ||
    (l.output || '').toLowerCase().includes(kw)
  )
})

const columns = [
  { title: '名称', key: 'name' },
  { title: 'Cron', key: 'expression' },
  { title: '命令', key: 'command', ellipsis: { tooltip: true } },
  {
    title: '状态', key: 'enabled', width: 80,
    render: (r: any) => h(NSwitch, {
      value: r.enabled === true || r.enabled === 1,
      size: 'small',
      onUpdateValue: (v: boolean) => toggleJob(r, v),
    }),
  },
  {
    title: '操作', key: 'actions', width: 200,
    render: (r: any) =>
      h('div', { style: 'display:flex;gap:6px' }, [
        h(NButton, { size: 'tiny', onClick: () => showHistory(r) }, () => '日志'),
        h(NButton, { size: 'tiny', onClick: () => runNow(r) }, () => '立即执行'),
        h(NPopconfirm, { onPositiveClick: () => delJob(r) }, {
          trigger: () => h(NButton, { size: 'tiny', type: 'error' }, () => '删除'),
          default: () => '确认删除？',
        }),
      ]),
  },
]

const logColumns = [
  { title: '执行时间', key: 'timestamp', width: 170, render: (r: any) => r.timestamp ? new Date(r.timestamp).toLocaleString() : '--' },
  { title: '耗时', key: 'duration_ms', width: 80, render: (r: any) => r.duration_ms ? r.duration_ms + 'ms' : '--' },
  {
    title: '状态', key: 'exit_code', width: 70,
    render: (r: any) => h(NTag, { type: r.exit_code === 0 ? 'success' : 'error', size: 'small' }, () => r.exit_code === 0 ? '成功' : '失败'),
  },
  { title: '命令', key: 'command', ellipsis: { tooltip: true } },
  { title: '输出', key: 'output', ellipsis: { tooltip: true } },
]

async function fetchJobs() {
  loading.value = true
  try {
    const { data } = await cronAPI.list()
    jobs.value = Array.isArray(data) ? data : []
  } catch (e: unknown) {
    message.error('获取任务列表失败: ' + getErrorMessage(e, ''))
    jobs.value = []
  } finally {
    loading.value = false
  }
}

async function addJob() {
  if (!form.value.name || !form.value.cron || !form.value.command) {
    message.warning('请填写完整')
    return
  }
  saving.value = true
  try {
    await cronAPI.create({
      name: form.value.name,
      expression: form.value.cron,
      command: form.value.command,
      enabled: true,
    })
    message.success('创建成功')
    showAdd.value = false
    Object.assign(form.value, { name: '', cron: '', command: '' })
    fetchJobs()
  } catch (e: unknown) {
    message.error(getErrorMessage(e, '创建失败'))
  } finally {
    saving.value = false
  }
}

async function toggleJob(r: any, enabled: boolean) {
  try {
    await cronAPI.update(String(r.id), { enabled })
    r.enabled = enabled
  } catch {
    message.error('更新失败')
  }
}

async function delJob(r: any) {
  try {
    await cronAPI.delete(String(r.id))
    message.success('已删除')
    fetchJobs()
  } catch {
    message.error('删除失败')
  }
}

async function runNow(r: any) {
  try {
    await cronAPI.run(String(r.id))
    message.success('已触发执行')
  } catch {
    message.error('触发失败')
  }
}

async function showHistory(r: any) {
  logJobName.value = String(r.name || '')
  logSearch.value = ''
  showLog.value = true
  logs.value = []
  try {
    const { data } = await cronAPI.logs(String(r.id))
    logs.value = Array.isArray(data) ? data : []
  } catch (e: unknown) {
    message.error(getErrorMessage(e, '获取日志失败'))
  }
}

onMounted(() => {
  fetchJobs()
})
</script>

<style scoped>
.page { padding: 24px; }
.page-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 16px; }
h2 { font-size: 20px; font-weight: 600; margin: 0; }
</style>
