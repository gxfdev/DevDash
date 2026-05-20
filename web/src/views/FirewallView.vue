<template>
  <app-layout>
    <div class="page">
      <div class="page-header">
        <h2>防火墙</h2>
        <n-space>
          <n-select v-model:value="selectedNode" :options="nodeOptions" placeholder="选择节点" style="width:180px" />
          <n-button type="primary" size="small" @click="showAdd = true">添加规则</n-button>
        </n-space>
      </div>

      <div class="presets">
        <div class="presets-title">快速添加</div>
        <div class="preset-btns">
          <n-button v-for="p in presets" :key="p.port" size="tiny" @click="quickAdd(p)">{{ p.name }} ({{ p.port }})</n-button>
        </div>
      </div>

      <n-data-table :columns="columns" :data="rules" :bordered="false" :loading="loading" row-key="id" />

      <!-- 添加规则弹窗 -->
      <n-modal v-model:show="showAdd" preset="card" title="添加规则" style="width:440px">
        <n-form :model="form" label-placement="top">
          <n-form-item label="协议">
            <n-radio-group v-model:value="form.proto">
              <n-radio value="tcp">TCP</n-radio>
              <n-radio value="udp">UDP</n-radio>
              <n-radio value="both">TCP+UDP</n-radio>
            </n-radio-group>
          </n-form-item>
          <n-form-item label="端口">
            <n-input v-model:value="form.port" placeholder="8080 或 8000-9000" />
          </n-form-item>
          <n-form-item label="来源 IP（留空允许所有）">
            <n-input v-model:value="form.src_ip" placeholder="0.0.0.0/0" />
          </n-form-item>
          <n-form-item label="备注">
            <n-input v-model:value="form.note" placeholder="可选备注" />
          </n-form-item>
        </n-form>
        <template #footer>
          <n-button @click="showAdd = false">取消</n-button>
          <n-button type="primary" style="margin-left:8px" :loading="saving" @click="addRule">添加</n-button>
        </template>
      </n-modal>
    </div>
  </app-layout>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, h, watch } from 'vue'
import { NButton, NTag, NSwitch, NPopconfirm, useMessage } from 'naive-ui'
import AppLayout from '@/components/AppLayout.vue'
import { useNodesStore } from '@/stores/nodes'
import client, { getErrorMessage } from '@/api/client'

const nodesStore = useNodesStore()
const message = useMessage()

const selectedNode = ref<string | null>(null)
const rules = ref<any[]>([])
const loading = ref(false)
const showAdd = ref(false)
const saving = ref(false)
const form = ref({ proto: 'tcp', port: '', src_ip: '', note: '' })

const presets = [
  { name: 'SSH', port: '22', proto: 'tcp' },
  { name: 'HTTP', port: '80', proto: 'tcp' },
  { name: 'HTTPS', port: '443', proto: 'tcp' },
  { name: 'MySQL', port: '3306', proto: 'tcp' },
  { name: 'Redis', port: '6379', proto: 'tcp' },
  { name: 'PostgreSQL', port: '5432', proto: 'tcp' },
]

const nodeOptions = computed(() => nodesStore.nodes.map((n: { name: string; hostname?: string; ip: string; id: string }) => ({ label: n.name || n.hostname, value: n.id })))

const columns = [
  { title: '协议', key: 'proto', width: 90 },
  { title: '端口', key: 'port', width: 120 },
  { title: '来源', key: 'src_ip', render: (r: any) => r.src_ip || '0.0.0.0/0' },
  { title: '备注', key: 'note', render: (r: any) => r.note || '--' },
  {
    title: '状态', key: 'enabled', width: 90,
    render: (r: any) => h(NSwitch, {
      value: r.enabled === true || r.enabled === 1,
      size: 'small',
      onUpdateValue: (v: boolean) => toggleRule(r, v),
    }),
  },
  {
    title: '操作', key: 'actions', width: 120,
    render: (r: any) => h(NPopconfirm, { onPositiveClick: () => delRule(r) }, {
      trigger: () => h(NButton, { size: 'small', type: 'error' }, () => '删除'),
      default: () => '确认删除？',
    }),
  },
]

async function fetchRules() {
  if (!selectedNode.value) return
  loading.value = true
  try {
    const { data } = await client.get(`/node/${selectedNode.value}/firewall/rules`)
    rules.value = data || []
  } catch {} finally { loading.value = false }
}

async function addRule() {
  if (!form.value.port) { message.warning('请填写端口'); return }
  saving.value = true
  try {
    await client.post(`/node/${selectedNode.value}/firewall/rules`, form.value)
    message.success('添加成功')
    showAdd.value = false
    Object.assign(form.value, { proto: 'tcp', port: '', src_ip: '', note: '' })
    fetchRules()
  } catch (e: unknown) { message.error(getErrorMessage(e, '添加失败')) }
  finally { saving.value = false }
}

async function quickAdd(p: any) {
  form.value = { proto: p.proto || p.protocol, port: p.port, src_ip: '', note: p.name || p.note || '' }
  showAdd.value = true
}

async function toggleRule(r: any, enabled: boolean) {
  try {
    await client.patch(`/node/${selectedNode.value}/firewall/rules/${r.id}`, { enabled })
    r.enabled = enabled
  } catch { message.error('更新失败') }
}

async function delRule(r: any) {
  try {
    await client.delete(`/node/${selectedNode.value}/firewall/rules/${r.id}`)
    message.success('已删除')
    fetchRules()
  } catch { message.error('删除失败') }
}

onMounted(async () => {
  await nodesStore.fetchNodes()
  if (nodesStore.nodes.length) {
    selectedNode.value = nodesStore.nodes[0]?.id
    fetchRules()
  }
})

watch(selectedNode, () => { fetchRules() })
</script>

<style scoped>
.page { padding: 24px; }
.page-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 16px; }
h2 { font-size: 20px; font-weight: 600; margin: 0; }
.presets { margin-bottom: 16px; background: #161b22; border: 1px solid #30363d; border-radius: 8px; padding: 12px 16px; }
.presets-title { font-size: 12px; color: #8b949e; margin-bottom: 8px; }
.preset-btns { display: flex; gap: 8px; flex-wrap: wrap; }
</style>