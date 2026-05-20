<template>
  <app-layout>
    <div class="page">
      <div class="toolbar">
        <div class="toolbar-left">
          <n-input v-model:value="search" placeholder="搜索名称 / IP / 系统..." clearable style="width: 240px" />
          <n-select
            v-model:value="statusFilter"
            :options="statusOptions"
            placeholder="状态筛选"
            clearable
            style="width: 140px; margin-left: 8px"
          />
        </div>
        <div class="toolbar-right">
          <n-button @click="refresh">
            <template #icon><refresh-icon /></template>
            刷新
          </n-button>
          <n-button type="primary" @click="showAddModal = true">
            <template #icon><add-icon /></template>
            添加节点
          </n-button>
        </div>
      </div>

      <div class="table-wrap">
        <n-data-table
          :columns="columns"
          :data="filteredNodes"
          :bordered="false"
          :row-key="(row: any) => row.id"
          :loading="nodesStore.loading"
        />
      </div>

      <div v-if="filteredNodes.length === 0 && !nodesStore.loading" class="empty">
        <div class="empty-icon">📡</div>
        <p>暂无节点</p>
        <n-button size="small" @click="showAddModal = true">添加第一个节点</n-button>
      </div>

      <!-- 添加节点弹窗 -->
      <n-modal v-model:show="showAddModal" preset="card" title="添加节点" style="width: 480px">
        <n-form :model="addForm" label-placement="top">
          <n-form-item label="节点名称">
            <n-input v-model:value="addForm.name" placeholder="例如：生产服务器-1" />
          </n-form-item>
          <n-form-item label="节点地址">
            <n-input v-model:value="addForm.addr" placeholder="http://192.168.1.100:9090" />
          </n-form-item>
          <n-form-item label="连接 Token">
            <n-input v-model:value="addForm.token" placeholder="节点注册时生成的 Token" />
          </n-form-item>
          <n-form-item label="备注">
            <n-input v-model:value="addForm.note" placeholder="可选备注信息" />
          </n-form-item>
        </n-form>
        <template #footer>
          <div style="display:flex;justify-content:flex-end;gap:8px">
            <n-button @click="showAddModal = false">取消</n-button>
            <n-button type="primary" :loading="addLoading" @click="handleAdd">确认添加</n-button>
          </div>
        </template>
      </n-modal>
    </div>
  </app-layout>
</template>

<script setup lang="ts">
import { getErrorMessage } from '@/api/client'
import { ref, computed, reactive, onMounted, h } from 'vue'
import { useRouter } from 'vue-router'
import { NButton, NTag, NPopconfirm, useMessage } from 'naive-ui'
import AppLayout from '@/components/AppLayout.vue'
import { useNodesStore } from '@/stores/nodes'
import { Refresh as RefreshIcon, Add as AddIcon } from '@vicons/ionicons5'

const router = useRouter()
const nodesStore = useNodesStore()
const message = useMessage()

const search = ref('')
const statusFilter = ref(null)
const showAddModal = ref(false)
const addLoading = ref(false)
const addForm = reactive({ name: '', addr: '', token: '', note: '' })

const statusOptions = [
  { label: '在线', value: 'online' },
  { label: '离线', value: 'offline' },
]

const filteredNodes = computed(() => {
  return nodesStore.nodes.filter((n: any) => {
    const matchSearch =
      !search.value ||
      n.name?.includes(search.value) ||
      n.ip?.includes(search.value) ||
      n.os?.includes(search.value)
    const matchStatus = !statusFilter.value || n.status === statusFilter.value
    return matchSearch && matchStatus
  })
})

const columns = [
  { title: '名称', key: 'name', render: (row: any) => row.name || row.hostname || '未知' },
  { title: 'IP', key: 'ip', render: (row: any) => row.ip || '--' },
  { title: '系统', key: 'os', render: (row: any) => row.os || '--' },
  { title: '架构', key: 'arch', render: (row: any) => row.arch || '--' },
  {
    title: '在线时长',
    key: 'uptime',
    render: (row: any) => {
      if (!row.uptime) return '--'
      const s = row.uptime
      const d = Math.floor(s / 86400)
      const h2 = Math.floor((s % 86400) / 3600)
      return d > 0 ? `${d}天 ${h2}小时` : `${h2}小时`
    },
  },
  {
    title: '状态',
    key: 'status',
    width: 90,
    render: (row: any) =>
      h(NTag, { type: row.status === 'online' ? 'success' : 'error', size: 'small' }, () => row.status === 'online' ? '在线' : '离线'),
  },
  {
    title: '操作',
    key: 'actions',
    width: 160,
    render: (row: any) =>
      h('div', { style: 'display:flex;gap:8px' }, [
        h(NButton, { size: 'small', onClick: () => router.push(`/hosts/${row.id}`) }, () => '详情'),
        h(NPopconfirm, {
          onPositiveClick: () => handleDelete(row.id),
        }, {
          trigger: () => h(NButton, { size: 'small', type: 'error' }, () => '删除'),
          default: () => '确认删除该节点？',
        }),
      ]),
  },
]

function refresh() {
  nodesStore.fetchNodes()
}

async function handleAdd() {
  if (!addForm.name || !addForm.addr || !addForm.token) {
    message.warning('请填写名称、地址和 Token')
    return
  }
  addLoading.value = true
  try {
    await nodesStore.addNode(addForm)
    message.success('节点添加成功')
    showAddModal.value = false
    Object.assign(addForm, { name: '', addr: '', token: '', note: '' })
  } catch (e: unknown) {
    message.error(getErrorMessage(e, '添加失败'))
  } finally {
    addLoading.value = false
  }
}

async function handleDelete(id: string) {
  try {
    await nodesStore.deleteNode(id)
    message.success('已删除')
  } catch {
    message.error('删除失败')
  }
}

onMounted(() => nodesStore.fetchNodes())
</script>

<style scoped>
.page { padding: 24px; }
.toolbar { display: flex; justify-content: space-between; align-items: center; margin-bottom: 16px; }
.toolbar-left { display: flex; align-items: center; }
.toolbar-right { display: flex; gap: 8px; }
.table-wrap { background: #161b22; border: 1px solid #30363d; border-radius: 8px; overflow: hidden; }
.empty { text-align: center; padding: 60px 0; color: #8b949e; }
.empty-icon { font-size: 48px; margin-bottom: 12px; }
.empty p { margin: 0 0 16px; }
</style>