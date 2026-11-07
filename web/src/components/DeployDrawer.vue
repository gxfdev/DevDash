<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { useMessage } from 'naive-ui'
import { RocketOutline, ShieldCheckmarkOutline, WarningOutline, AddOutline, TrashOutline } from '@vicons/ionicons5'
import { serviceApi } from '@/api'
import type { CatalogItem, DeployPayload, Service } from '@/types'

const props = defineProps<{ show: boolean; item: CatalogItem | null }>()
const emit = defineEmits<{ (e: 'update:show', v: boolean): void; (e: 'deployed', svc: Service): void }>()

const message = useMessage()
const submitting = ref(false)

const form = reactive({
  name: '',
  image: '',
  image_tag: '',
  model_name: '',
  cpus: 2,
  memory_mb: 4096,
  shm_mb: 64,
  gpu_count: 0,
  host_enabled: false,
  privileged: true,
  host_pid: true,
  host_fs: true,
  docker_sock: true,
  auto_start: true,
  envRows: [{ k: '', v: '' }],
  ports: [] as { container_port: number; host_port: number; protocol: string; accessible: boolean }[],
  volumes: [] as { source: string; target: string; read_only: boolean }[],
})

const isModel = computed(() => props.item?.kind === 'model')
const requiresGpu = computed(() => !!props.item?.requires_gpu)

watch(() => props.item, (item) => {
  if (!item) return
  form.name = item.id
  form.image = item.image
  form.image_tag = item.image_tag
  form.model_name = item.model_name || ''
  form.cpus = item.resources?.cpus || 2
  form.memory_mb = item.resources?.memory_mb || 4096
  form.shm_mb = item.resources?.shm_mb || 64
  form.gpu_count = item.resources?.gpu_count || 0
  form.host_enabled = item.default_host_access
  form.privileged = true
  form.host_pid = true
  form.host_fs = true
  form.docker_sock = true
  form.auto_start = true
  form.ports = (item.ports || []).map((p) => ({
    container_port: p.container_port, host_port: p.host_port || 0,
    protocol: p.protocol || 'tcp', accessible: p.accessible !== false,
  }))
  form.volumes = (item.volumes || []).map((v) => ({ ...v }))
  form.envRows = Object.entries(item.env || {}).map(([k, v]) => ({ k, v })) || []
  if (!form.envRows.length) form.envRows = [{ k: '', v: '' }]
}, { immediate: true })

function buildPayload(): DeployPayload | null {
  if (!props.item) return null
  if (!form.name.trim()) {
    message.warning('请填写服务名称')
    return null
  }
  if (!/^[a-z0-9][a-z0-9-]{1,40}$/.test(form.name.trim())) {
    message.warning('服务名称只能包含小写字母、数字和连字符')
    return null
  }
  const env: Record<string, string> = {}
  for (const row of form.envRows) {
    if (row.k.trim()) env[row.k.trim()] = row.v
  }
  return {
    catalog_id: props.item.id,
    name: form.name.trim(),
    kind: props.item.kind,
    image: form.image,
    image_tag: form.image_tag,
    model_name: isModel.value ? form.model_name.trim() : undefined,
    resources: {
      cpus: form.cpus,
      memory_mb: form.memory_mb,
      shm_mb: form.shm_mb,
      gpu_count: form.gpu_count,
      gpu_kind: '',
    },
    env,
    ports: form.ports.map((p) => ({
      host_port: p.host_port, container_port: p.container_port,
      protocol: p.protocol, accessible: p.accessible,
    })),
    volumes: form.volumes,
    host_access: {
      enabled: form.host_enabled,
      privileged: form.host_enabled && form.privileged,
      host_pid: form.host_enabled && form.host_pid,
      mount_host_fs: form.host_enabled && form.host_fs,
      mount_docker_socket: form.host_enabled && form.docker_sock,
    },
    auto_start: form.auto_start,
  }
}

async function submit() {
  const payload = buildPayload()
  if (!payload) return
  submitting.value = true
  try {
    const svc = await serviceApi.deploy(payload)
    message.success(`已提交部署：${svc.name}`)
    emit('update:show', false)
    emit('deployed', svc)
  } catch (e: any) {
    message.error(e.message)
  } finally {
    submitting.value = false
  }
}

function addEnv() { form.envRows.push({ k: '', v: '' }) }
function removeEnv(i: number) { form.envRows.splice(i, 1) }
</script>

<template>
  <n-drawer :show="show" :width="620" placement="right" @update:show="emit('update:show', $event)">
    <div v-if="item" class="deploy">
      <header class="d-head">
        <div class="d-icon" :style="{ background: 'rgba(255,255,255,0.04)', borderColor: `${item.accent}55` }">
          <n-icon :component="isModel ? RocketOutline : WarningOutline" :size="22" :style="{ color: item.accent }" />
        </div>
        <div style="flex: 1; min-width: 0">
          <div class="d-title">部署 {{ item.name }}</div>
          <div class="d-sub">{{ item.image }}:{{ item.image_tag }} · {{ item.vendor }} · {{ item.params || item.category }}</div>
        </div>
        <n-tag size="small" :bordered="false" round>{{ item.kind === 'model' ? '模型' : '应用' }}</n-tag>
      </header>

      <div class="d-body">
        <div class="d-section">
          <div class="d-label">基础信息</div>
          <n-form label-placement="left" label-width="92" size="small">
            <n-form-item label="服务名称">
              <n-input v-model:value="form.name" placeholder="例如 qwen-7b" />
            </n-form-item>
            <n-form-item v-if="isModel" label="模型别名">
              <n-input v-model:value="form.model_name" placeholder="网关中暴露的 model 名称" />
            </n-form-item>
            <n-form-item label="镜像">
              <n-input v-model:value="form.image" />
            </n-form-item>
            <n-form-item label="镜像标签">
              <n-input v-model:value="form.image_tag" />
            </n-form-item>
          </n-form>
          <div class="d-note">部署引擎将自动拉取镜像、创建容器并完成健康检查，全过程可在「模型服务」中查看日志与事件。</div>
        </div>

        <div class="d-section">
          <div class="d-label">资源配置</div>
          <div class="d-grid3">
            <div class="d-field">
              <span>CPU 核数</span>
              <n-input-number v-model:value="form.cpus" :min="0.5" :step="0.5" size="small" />
            </div>
            <div class="d-field">
              <span>内存 (MB)</span>
              <n-input-number v-model:value="form.memory_mb" :min="128" :step="512" size="small" />
            </div>
            <div class="d-field">
              <span>共享内存 (MB)</span>
              <n-input-number v-model:value="form.shm_mb" :min="16" :step="64" size="small" />
            </div>
          </div>
          <div class="d-grid3" style="margin-top: 10px">
            <div class="d-field">
              <span>GPU 数量 {{ requiresGpu ? '（该模型建议 GPU）' : '' }}</span>
              <n-input-number v-model:value="form.gpu_count" :min="0" :max="8" size="small" />
            </div>
            <div class="d-field">
              <span>自动启动</span>
              <n-switch v-model:value="form.auto_start" size="small" />
            </div>
          </div>
        </div>

        <div class="d-section">
          <div class="d-label">网络端口</div>
          <div v-for="(p, i) in form.ports" :key="i" class="port-row">
            <n-input-number v-model:value="p.host_port" size="small" :min="0" :max="65535" placeholder="宿主机 (0=自动)" style="width: 150px">
              <template #prefix>宿主机</template>
            </n-input-number>
            <span class="arrow">→</span>
            <n-input-number v-model:value="p.container_port" size="small" :min="1" :max="65535" style="width: 140px">
              <template #prefix>容器</template>
            </n-input-number>
            <n-checkbox v-model:checked="p.accessible">对外访问</n-checkbox>
            <n-button quaternary circle size="tiny" @click="form.ports.splice(i, 1)"><n-icon :component="TrashOutline" /></n-button>
          </div>
          <n-button dashed size="tiny" style="margin-top: 8px" @click="form.ports.push({ container_port: 8000, host_port: 0, protocol: 'tcp', accessible: true })">
            <template #icon><n-icon :component="AddOutline" /></template>添加端口
          </n-button>
          <div class="d-note">宿主机端口填 0 表示自动分配空闲端口；部署完成后可在服务详情中复制访问地址。</div>
        </div>

        <div class="d-section">
          <div class="d-label">
            宿主机权限（运维模式）
            <n-switch v-model:value="form.host_enabled" size="small" />
          </div>
          <div v-if="form.host_enabled" class="host-box">
            <div class="host-warn">
              <n-icon :component="WarningOutline" :size="15" />
              开启后容器将获得宿主机级能力，请仅授予可信的 AI 应用。
            </div>
            <div class="host-opts">
              <n-checkbox v-model:checked="form.privileged">特权容器（Privileged）</n-checkbox>
              <n-checkbox v-model:checked="form.host_pid">共享宿主机 PID</n-checkbox>
              <n-checkbox v-model:checked="form.host_fs">挂载宿主机根目录 / → /host</n-checkbox>
              <n-checkbox v-model:checked="form.docker_sock">挂载 Docker Socket</n-checkbox>
            </div>
            <div class="d-note">
              容器仍然隔离运行，但可读写宿主机文件、管理 Docker 与执行系统命令；
              AI 运维助手可将该容器作为命令沙箱。
            </div>
          </div>
          <div v-else class="d-note">
            未开启宿主机权限：容器运行在独立命名空间，仅能访问自身文件系统与映射端口。
          </div>
        </div>

        <div class="d-section">
          <div class="d-label">环境变量</div>
          <div v-for="(row, i) in form.envRows" :key="i" class="env-row">
            <n-input v-model:value="row.k" size="small" placeholder="KEY" />
            <n-input v-model:value="row.v" size="small" placeholder="value" />
            <n-button quaternary circle size="tiny" @click="removeEnv(i)"><n-icon :component="TrashOutline" /></n-button>
          </div>
          <n-button dashed size="tiny" style="margin-top: 8px" @click="addEnv">
            <template #icon><n-icon :component="AddOutline" /></template>添加变量
          </n-button>
        </div>

        <div class="d-section">
          <div class="d-label">数据卷</div>
          <div v-for="(v, i) in form.volumes" :key="i" class="env-row">
            <n-input v-model:value="v.source" size="small" placeholder="aidash-data" />
            <n-input v-model:value="v.target" size="small" placeholder="/data" />
            <n-button quaternary circle size="tiny" @click="form.volumes.splice(i, 1)"><n-icon :component="TrashOutline" /></n-button>
          </div>
          <n-button dashed size="tiny" style="margin-top: 8px" @click="form.volumes.push({ source: '', target: '', read_only: false })">
            <template #icon><n-icon :component="AddOutline" /></template>添加数据卷
          </n-button>
        </div>
      </div>

      <footer class="d-foot">
        <div class="d-summary">
          <n-icon :component="ShieldCheckmarkOutline" :size="14" />
          {{ form.host_enabled ? '已启用宿主机权限' : '隔离模式' }} · {{ form.cpus }}C / {{ (form.memory_mb / 1024).toFixed(1) }}G
          <template v-if="form.gpu_count"> · {{ form.gpu_count }} GPU</template>
        </div>
        <div class="row gap-12">
          <n-button quaternary @click="emit('update:show', false)">取消</n-button>
          <n-button type="primary" :loading="submitting" @click="submit">
            <template #icon><n-icon :component="RocketOutline" /></template>
            开始部署
          </n-button>
        </div>
      </footer>
    </div>
  </n-drawer>
</template>

<style scoped>
.deploy { display: flex; flex-direction: column; height: 100%; }
.d-head { display: flex; align-items: center; gap: 14px; padding: 20px 24px 16px; border-bottom: 1px solid var(--line); }
.d-icon { width: 48px; height: 48px; border-radius: 14px; border: 1px solid; display: grid; place-items: center; flex: none; }
.d-title { font-size: 17px; font-weight: 700; }
.d-sub { font-size: 11.5px; color: var(--muted); margin-top: 3px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.d-body { flex: 1; overflow-y: auto; padding: 18px 24px 24px; }
.d-section { padding: 14px 0 18px; border-bottom: 1px solid rgba(255,255,255,0.055); }
.d-section:last-child { border-bottom: none; }
.d-label { display: flex; align-items: center; justify-content: space-between; gap: 10px; font-size: 12px; font-weight: 650; color: #b9c5dd; margin-bottom: 12px; letter-spacing: 0.2px; }
.d-note { font-size: 11px; color: #5d6a89; line-height: 1.7; margin-top: 8px; }
.d-grid3 { display: grid; grid-template-columns: repeat(3, 1fr); gap: 10px; }
.d-field { display: flex; flex-direction: column; gap: 5px; font-size: 11px; color: var(--muted); }
.port-row, .env-row { display: flex; align-items: center; gap: 8px; margin-bottom: 8px; }
.arrow { color: #4b5a7a; }
.host-box { border: 1px solid rgba(251,191,36,0.25); background: rgba(251,191,36,0.06); border-radius: 12px; padding: 12px 14px; }
.host-warn { display: flex; align-items: center; gap: 7px; font-size: 11.5px; color: #fcd34d; margin-bottom: 10px; }
.host-opts { display: grid; grid-template-columns: 1fr 1fr; gap: 8px; }
.d-foot { flex: none; display: flex; align-items: center; justify-content: space-between; gap: 12px; padding: 14px 24px; border-top: 1px solid var(--line); background: rgba(8,12,22,0.6); }
.d-summary { display: flex; align-items: center; gap: 6px; font-size: 11px; color: var(--muted); }
</style>
