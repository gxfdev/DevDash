<template>
  <app-layout>
    <div class="page">
      <div class="page-header">
        <h2>计划任务</h2>
        <n-space>
          <n-select v-model:value="selectedNode" :options="nodeOptions" placeholder="选择节点" style="width:180px" />
          <n-button type="primary" size="small" @click="showAdd = true">新建任务</n-button>
        </n-space>
      </div>

      <n-data-table :columns="columns" :data="jobs" :bordered="false" :loading="loading" row-key="id" />

      <!-- 新建任务弹窗 -->
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
          <n-form-item label="执行方式">
            <n-radio-group v-model:value="form.type">
              <n-radio value="shell">Shell 命令</n-radio>
              <n-radio value="http">HTTP 请求</n-radio>
              <n-radio value="script">脚本文件</n-radio>
            </n-radio-group>
          </n-form-item>
          <n-form-item label="执行内容">
            <n-input v-model:value="form.command" type="textarea" placeholder="tar -czf /backup/db.tar.gz /data" :rows="3" />
          </n-form-item>
        </n-form>
        <template #footer>
          <n-button @click="showAdd = false">取消</n-button>
          <n-button type="primary" style="margin-left:8px" :loading="saving" @click="addJob">创建</n-button>
        </template>
      </n-modal>

      <!-- 执行历史 -->
      <n-modal v-model:show="showLog" preset="card" :title="`历史: ${logJobName}`" style="width:700px">
        <n-data-table :columns="logColumns" :data="logs" size="small" :bordered="false" />
      </n-modal>
    </div>
  </app-layout>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, watch, h } from 'vue'
import { NButton, NTag, NSwitch, NPopconfirm, useMessage } from 'naive-ui'
import AppLayout from '@/components/AppLayout.vue'
import { useNodesStore } from '@/stores/nodes'
import client from '@/api/client'

const nodesStore = useNodesStore()
const message = useMessage()

const selectedNode = ref<string | null>(null)
const jobs = ref<any[]>([])
const logs = ref<any[]>([])
const loading = ref(false)
const showAdd = ref(false)
const showLog = ref(false)
const logJobName = ref('')
const saving = ref(false)
const form = ref({ name: '', cron: '', type: 'shell', command: '' })

const nodeOptions = computed(() =>
  nodesStore.nodes.map((n: any) => ({ label: n.name || n.hostname || n.ip, value: n.id }))
)

const columns = [
  { title: '名称', key: 'name' },
  { title: 'Cron', key: 'expression' },
  { title: '类型', key: 'type' },
  {
    title: '状态', key: 'enabled', width: 80,
    render: (r: any) => h(NSwitch, {
      value: r.enabled === true || r.enabled === 1,
      size: 'small',
      onUpdateValue: (v: boolean) => toggleJob(r, v),
    }),
  },
  {
    title: '最近执行', key: 'last_run',
    render: (r: any) => r.last_run ? new Date(r.last_run * 1000).toLocaleString() : '从未',
  },
  {
    title: '操作', key: 'actions', width: 200,
    render: (r: any) =>
      h('div', { style: 'display:flex;gap:6px' }, [
        h(NButton, { size: 'tiny', onClick: () => showHistory(r) }, () => '历史'),
        h(NButton, { size: 'tiny', onClick: () => runNow(r) }, () => '立即执行'),
        h(NPopconfirm, { onPositiveClick: () => delJob(r) }, {
          trigger: () => h(NButton, { size: 'tiny', type: 'error' }, () => '删除'),
          default: () => '确认删除？',
        }),
      ]),
  },
]

const logColumns = [
  { title: '执行时间', key: 'start_time', render: (r: any) => new Date(r.start_time * 1000).toLocaleString() },
  { title: '耗时', key: 'duration', render: (r: any) => r.duration ? r.duration + 'ms' : '--' },
  {
    title: '状态', key: 'exit_code',
    render: (r: any) => h(NTag, { type: r.exit_code === 0 ? 'success' : 'error', size: 'small' }, () => r.exit_code === 0 ? '成功' : '失败'),
  },
]

async function fetchJobs() {
  if (!selectedNode.value) {
    jobs.value = []
    return
  }
  loading.value = true
  try {
    const { data } = await client.get(`/node/${selectedNode.value}/cronjobs`)
    jobs.value = Array.isArray(data) ? data : []
  } catch (e: any) {
    message.error('获取任务列表失败: ' + (e?.response?.data?.error || e?.message || ''))
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
  if (!selectedNode.value) {
    message.warning('请先选择节点')
    return
  }
  saving.value = true
  try {
    const payload = {
      name: form.value.name,
      expression: form.value.cron,
      command: form.value.command,
      type: form.value.type,
      enabled: true,
    }
    await client.post(`/node/${selectedNode.value}/cronjobs`, payload)
    message.success('创建成功')
    showAdd.value = false
    Object.assign(form.value, { name: '', cron: '', type: 'shell', command: '' })
    fetchJobs()
  } catch (e: any) {
    message.error(e?.response?.data?.error || '创建失败')
  } finally {
    saving.value = false
  }
}

async function toggleJob(r: any, enabled: boolean) {
  try {
    await client.patch(`/node/${selectedNode.value}/cronjobs/${r.id}`, { ...r, enabled })
    r.enabled = enabled
  } catch {
    message.error('更新失败')
  }
}

async function delJob(r: any) {
  try {
    await client.delete(`/node/${selectedNode.value}/cronjobs/${r.id}`)
    message.success('已删除')
    fetchJobs()
  } catch {
    message.error('删除失败')
  }
}

async function runNow(r: any) {
  try {
    await client.post(`/node/${selectedNode.value}/cronjobs/${r.id}/run`)
    message.success('已触发执行')
  } catch {
    message.error('触发失败')
  }
}

async function showHistory(r: any) {
  logJobName.value = r.name
  showLog.value = true
  logs.value = []
}

onMounted(async () => {
  await nodesStore.fetchNodes()
  if (nodesStore.nodes.length > 0 && !selectedNode.value) {
    selectedNode.value = nodesStore.nodes[0]?.id
  }
})

watch(selectedNode, () => {
  if (selectedNode.value) {
    fetchJobs()
  }
})
</script>

<style scoped>
.page { padding: 24px; }
.page-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 16px; }
h2 { font-size: 20px; font-weight: 600; margin: 0; }
</style>