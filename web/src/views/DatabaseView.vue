<template>
  <app-layout>
    <div class="page">
      <div class="page-header">
        <h2>数据库管理</h2>
        <n-select v-model:value="selectedNode" :options="nodeOptions" placeholder="选择节点" style="width:180px" />
      </div>

      <n-tabs type="line" animated>
        <n-tab-pane name="connections" tab="连接管理">
          <div class="db-list">
            <div v-for="db in dbConnections" :key="db.id" class="db-card">
              <div class="db-info">
                <div class="db-name">{{ db.name || db.dbname }}</div>
                <div class="db-meta">{{ db.type }} · {{ db.host }}:{{ db.port }} · {{ db.user || db.username }}</div>
              </div>
              <div class="db-actions">
                <n-button size="tiny" type="primary" @click="selectDb(db)">管理</n-button>
                <n-popconfirm @positiveClick="deleteDb(db)">
                  <template #trigger><n-button size="tiny" type="error">删除</n-button></template>
                  确认删除此连接？
                </n-popconfirm>
              </div>
            </div>
          </div>
          <n-button type="primary" size="small" style="margin-top:12px" @click="showAddDb = true">+ 添加连接</n-button>
        </n-tab-pane>

        <n-tab-pane v-if="currentDb" name="browser" tab="数据浏览">
          <div class="db-toolbar">
            <n-button size="tiny" @click="currentDb = null; tables = []; selectedTable = null; rows = []">← 返回列表</n-button>
            <span style="margin-left:12px;color:#8b949e;font-weight:600">{{ currentDb.name || currentDb.dbname }}</span>
            <n-tag size="small" :type="currentDb.type === 'mysql' ? 'info' : 'success'">{{ currentDb.type }}</n-tag>
          </div>
          <div class="table-list">
            <div v-for="t in tables" :key="t" class="table-item" :class="{ active: selectedTable === t }" @click="selectTable(t)">{{ t }}</div>
            <div v-if="tables.length === 0 && !tableLoading" class="empty-hint">暂无表，请先选择一个数据库连接</div>
          </div>
          <div v-if="selectedTable" class="table-data">
            <div class="table-toolbar">
              <span>{{ selectedTable }} ({{ rows.length }} 行)</span>
              <n-button size="tiny" @click="showSql = true">SQL 查询</n-button>
            </div>
            <n-data-table :columns="dataColumns" :data="rows" size="small" :bordered="false" :max-height="400" :loading="tableLoading" />
          </div>
        </n-tab-pane>
      </n-tabs>

      <n-modal v-model:show="showSql" preset="card" title="SQL 查询" style="width:700px">
        <n-input v-model:value="sqlQuery" type="textarea" placeholder="SELECT * FROM users LIMIT 100" :rows="5" style="font-family:monospace" />
        <div style="margin-top:8px;display:flex;justify-content:flex-end">
          <n-button type="primary" :loading="sqlRunning" @click="runSql">执行</n-button>
        </div>
        <n-data-table v-if="sqlResult.length" :columns="sqlResultCols" :data="sqlResult" size="small" :bordered="false" style="margin-top:12px" />
        <n-alert v-if="sqlError" type="error" style="margin-top:8px">{{ sqlError }}</n-alert>
      </n-modal>

      <n-modal v-model:show="showAddDb" preset="card" title="添加数据库连接" style="width:440px">
        <n-form :model="dbForm" label-placement="top">
          <n-form-item label="名称 *"><n-input v-model:value="dbForm.name" placeholder="例如：生产MySQL" /></n-form-item>
          <n-form-item label="类型 *">
            <n-select v-model:value="dbForm.type" :options="[{label:'MySQL',value:'mysql'},{label:'PostgreSQL',value:'postgres'}]" />
          </n-form-item>
          <n-form-item label="主机 *"><n-input v-model:value="dbForm.host" placeholder="localhost 或 192.168.1.100" /></n-form-item>
          <n-form-item label="端口"><n-input-number v-model:value="dbForm.portNum" :min="1" :max="65535" style="width:100%" /></n-form-item>
          <n-form-item label="用户名 *"><n-input v-model:value="dbForm.username" placeholder="root" /></n-form-item>
          <n-form-item label="密码"><n-input v-model:value="dbForm.password" type="password" show-password-on="click" /></n-form-item>
          <n-form-item label="数据库名"><n-input v-model:value="dbForm.dbname" placeholder="留空则使用名称作为数据库名" /></n-form-item>
        </n-form>
        <div style="margin-top:8px;display:flex;align-items:center;gap:8px">
          <n-button size="small" :loading="dbTesting" @click="testDbConn">测试连接</n-button>
          <span v-if="dbTestResult" :style="{color: dbTestOk ? '#3fb950' : '#f85149', fontSize:'12px'}">{{ dbTestResult }}</span>
        </div>
        <template #footer>
          <n-button @click="showAddDb=false">取消</n-button>
          <n-button type="primary" style="margin-left:8px" :loading="dbSaving" @click="addDb">保存</n-button>
        </template>
      </n-modal>
    </div>
  </app-layout>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { NButton, NTag, useMessage } from 'naive-ui'
import AppLayout from '@/components/AppLayout.vue'
import { useNodesStore } from '@/stores/nodes'
import client, { getErrorMessage } from '@/api/client'

const nodesStore = useNodesStore()
const message = useMessage()

const selectedNode = ref<string | null>(null)
const dbConnections = ref<any[]>([])
const currentDb = ref<any>(null)
const tables = ref<string[]>([])
const selectedTable = ref<string | null>(null)
const rows = ref<any[]>([])
const tableLoading = ref(false)
const showSql = ref(false)
const showAddDb = ref(false)
const sqlQuery = ref('SELECT 1')
const sqlResult = ref<any[]>([])
const sqlResultCols = ref<any[]>([])
const sqlError = ref('')
const sqlRunning = ref(false)
const dbSaving = ref(false)
const dbTesting = ref(false)
const dbTestResult = ref('')
const dbTestOk = ref(false)
const dbForm = ref({ name: '', type: 'mysql', host: '', portNum: 3306, username: '', password: '', dbname: '' })

const nodeOptions = computed(() => nodesStore.nodes.map((n: { name: string; hostname?: string; ip: string; id: string }) => ({ label: n.name || n.hostname || n.ip, value: n.id })))

const dataColumns = computed(() => {
  if (!rows.value.length) return []
  return Object.keys(rows.value[0]).map(k => ({ title: k, key: k, ellipsis: true, width: Math.max(100, k.length * 12 + 20) }))
})

async function fetchDbs() {
  if (!selectedNode.value) return
  try {
    const { data } = await client.get(`/node/${selectedNode.value}/databases`)
    dbConnections.value = Array.isArray(data) ? data : []
  } catch (e: unknown) {
    console.error('[db] fetchDbs error:', e)
    dbConnections.value = []
  }
}

async function selectDb(db: Record<string, unknown>) {
  currentDb.value = db
  selectedTable.value = null
  rows.value = []
  tableLoading.value = true
  try {
    const { data } = await client.get(`/node/${selectedNode.value}/databases/${db.id}/tables`)
    tables.value = Array.isArray(data) ? data : []
    if (tables.value.length === 0) {
      message.info('该数据库暂无表或连接信息有误')
    }
  } catch (e: unknown) {
    console.error('[db] fetchTables error:', e)
    const errMsg = getErrorMessage(e, '未知错误')
    message.error(`获取表列表失败: ${errMsg}`)
    tables.value = []
  } finally {
    tableLoading.value = false
  }
}

async function selectTable(t: string) {
  selectedTable.value = t
  rows.value = []
  tableLoading.value = true
  try {
    const { data } = await client.post(`/node/${selectedNode.value}/databases/${currentDb.value.id}/query`, {
      sql: `SELECT * FROM \`${t}\` LIMIT 100`,
    })
    rows.value = data?.rows || (Array.isArray(data) ? data : [])
  } catch (e: unknown) {
    console.error('[db] query error:', e)
    message.error('查询失败: ' + (getErrorMessage(e)))
  } finally {
    tableLoading.value = false
  }
}

async function runSql() {
  if (!sqlQuery.value.trim()) return
  sqlRunning.value = true
  sqlError.value = ''
  sqlResult.value = []
  sqlResultCols.value = []
  try {
    const { data } = await client.post(`/node/${selectedNode.value}/databases/${currentDb.value.id}/query`, { sql: sqlQuery.value })
    const resultRows = data?.rows || (Array.isArray(data) ? data : [])
    sqlResult.value = resultRows
    if (resultRows.length) {
      sqlResultCols.value = Object.keys(resultRows[0]).map(k => ({ title: k, key: k }))
    }
    message.success(`查询成功，返回 ${resultRows.length} 行`)
  } catch (e: unknown) {
    sqlError.value = getErrorMessage(e, '查询失败')
  } finally {
    sqlRunning.value = false
  }
}

async function addDb() {
  if (!selectedNode.value) { message.warning('请先选择节点'); return }
  if (!dbForm.value.name.trim()) { message.warning('请填写名称'); return }
  if (!dbForm.value.host.trim()) { message.warning('请填写主机地址'); return }

  dbSaving.value = true
  try {
    const payload: Record<string, any> = {
      name: dbForm.value.name,
      type: dbForm.value.type,
      host: dbForm.value.host,
      port: dbForm.value.portNum || 3306,
      username: dbForm.value.username,
      password: dbForm.value.password,
      dbname: dbForm.value.dbname || dbForm.value.name,
    }
    await client.post(`/node/${selectedNode.value}/databases`, payload)
    message.success('添加成功')
    showAddDb.value = false
    Object.assign(dbForm.value, { name: '', type: 'mysql', host: '', portNum: 3306, username: '', password: '', dbname: '' })
    dbTestResult.value = ''
    await fetchDbs()
  } catch (e: unknown) {
    message.error(getErrorMessage(e, '添加失败'))
  } finally {
    dbSaving.value = false
  }
}

async function testDbConn() {
  if (!dbForm.value.host.trim()) { message.warning('请填写主机地址'); return }
  dbTesting.value = true
  dbTestResult.value = ''
  try {
    const { data } = await client.post(`/node/${selectedNode.value}/databases/test`, {
      type: dbForm.value.type,
      host: dbForm.value.host,
      port: dbForm.value.portNum || (dbForm.value.type === 'mysql' ? 3306 : 5432),
      username: dbForm.value.username,
      password: dbForm.value.password,
      dbname: dbForm.value.dbname || dbForm.value.name,
    })
    if (data.ok) {
      dbTestOk.value = true
      dbTestResult.value = `连接成功! 版本: ${data.version || '--'}`
    } else {
      dbTestOk.value = false
      dbTestResult.value = `连接失败: ${data.error || '未知错误'}`
    }
  } catch (e: unknown) {
    dbTestOk.value = false
    dbTestResult.value = '测试请求失败: ' + (getErrorMessage(e, '网络错误'))
  } finally {
    dbTesting.value = false
  }
}

async function deleteDb(db: Record<string, unknown>) {
  try {
    await client.delete(`/node/${selectedNode.value}/databases/${db.id}`)
    message.success('已删除')
    if (currentDb.value?.id === db.id) {
      currentDb.value = null
      tables.value = []
      selectedTable.value = null
      rows.value = []
    }
    await fetchDbs()
  } catch (e: unknown) {
    message.error(getErrorMessage(e, '删除失败'))
  }
}

onMounted(async () => {
  await nodesStore.fetchNodes()
  if (nodesStore.nodes.length) {
    selectedNode.value = nodesStore.nodes[0]?.id
    fetchDbs()
  }
})

watch(selectedNode, () => {
  currentDb.value = null
  tables.value = []
  selectedTable.value = null
  rows.value = []
  fetchDbs()
})
</script>

<style scoped>
.page { padding: 24px; }
.page-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 16px; }
h2 { font-size: 20px; font-weight: 600; margin: 0; }
.db-list { display: flex; flex-direction: column; gap: 8px; min-height: 80px; }
.db-card { background: #161b22; border: 1px solid #30363d; border-radius: 8px; padding: 16px; display: flex; justify-content: space-between; align-items: center; transition: border-color 0.2s; }
.db-card:hover { border-color: #388bfd; }
.db-name { font-weight: 600; color: #e6edf3; font-size: 14px; }
.db-meta { font-size: 12px; color: #8b949e; margin-top: 4px; }
.db-actions { display: flex; align-items: center; gap: 8px; flex-shrink: 0; }
.table-list { display: flex; flex-wrap: wrap; gap: 8px; margin: 12px 0; min-height: 40px; }
.table-item { background: #161b22; border: 1px solid #30363d; border-radius: 4px; padding: 6px 14px; cursor: pointer; font-size: 13px; transition: all 0.15s; }
.table-item:hover { border-color: #388bfd; color: #58a6ff; transform: translateY(-1px); }
.table-item.active { border-color: #388bfd; background: #388bfd15; color: #58a6ff; font-weight: 600; }
.empty-hint { color: #6e7681; font-size: 13px; padding: 20px 0; text-align: center; }
.table-data { margin-top: 12px; }
.table-toolbar { display: flex; justify-content: space-between; align-items: center; margin-bottom: 8px; font-size: 13px; color: #8b949e; }
.db-toolbar { display: flex; align-items: center; gap: 8px; margin-bottom: 12px; padding-bottom: 8px; border-bottom: 1px solid #21262d; }
</style>