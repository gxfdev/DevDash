<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useMessage, useDialog } from 'naive-ui'
import {
  CubeOutline, PlayOutline, StopOutline, RefreshOutline, TrashOutline, SearchOutline,
  CloudDownloadOutline, DocumentTextOutline, LayersOutline, GitNetworkOutline, ServerOutline,
} from '@vicons/ionicons5'
import { dockerApi } from '@/api'
import type { ContainerView, ImageView, NetworkView, VolumeView } from '@/types'
import { fmtMB, fmtTime, shortId, fmtGB, copyText } from '@/utils/format'

const message = useMessage()
const dialog = useDialog()

const tab = ref('containers')
const containers = ref<ContainerView[]>([])
const images = ref<ImageView[]>([])
const networks = ref<NetworkView[]>([])
const volumes = ref<VolumeView[]>([])
const search = ref('')
const loading = ref(false)
const showAll = ref(true)
const logOpen = ref(false)
const logTitle = ref('')
const logContent = ref('')
const logLoading = ref(false)

const pullOpen = ref(false)
const pullImage = ref('')
const pullTag = ref('latest')
const pulling = ref(false)
const pullLogs = ref<string[]>([])
const pullBox = ref<HTMLElement>()

const filteredContainers = computed(() =>
  containers.value.filter((c) => {
    if (!showAll.value && c.state !== 'running') return false
    if (!search.value) return true
    const q = search.value.toLowerCase()
    return (c.names?.[0] || '').toLowerCase().includes(q) || c.image.toLowerCase().includes(q) || c.id.startsWith(q)
  }),
)
const filteredImages = computed(() =>
  images.value.filter((i) => !search.value || (i.tags || []).join(' ').toLowerCase().includes(search.value.toLowerCase())),
)

async function load() {
  loading.value = true
  try {
    if (tab.value === 'containers') containers.value = (await dockerApi.containers()).containers || []
    else if (tab.value === 'images') images.value = (await dockerApi.images()).images || []
    else if (tab.value === 'networks') networks.value = (await dockerApi.networks()).networks || []
    else volumes.value = (await dockerApi.volumes()).volumes || []
  } catch (e: any) {
    message.error(e.message)
  } finally {
    loading.value = false
  }
}

async function containerAction(c: ContainerView, action: string) {
  try {
    await dockerApi.action(c.id, action)
    message.success('操作完成')
    load()
  } catch (e: any) {
    message.error(e.message)
  }
}

async function showLogs(id: string, title: string) {
  logOpen.value = true
  logTitle.value = title
  logContent.value = '加载中…'
  logLoading.value = true
  try {
    const res = await dockerApi.logs(id, 400)
    logContent.value = res.logs || '（暂无日志）'
  } catch (e: any) {
    logContent.value = `获取日志失败：${e.message}`
  } finally {
    logLoading.value = false
  }
}

async function removeImage(img: ImageView) {
  const ref = (img.tags || [])[0] || img.id
  dialog.warning({
    title: '删除镜像',
    content: `确定删除镜像 ${ref}？`,
    positiveText: '删除',
    negativeText: '取消',
    onPositiveClick: async () => {
      try {
        await dockerApi.removeImage(img.id, true)
        message.success('镜像已删除')
        load()
      } catch (e: any) {
        message.error(e.message)
      }
    },
  })
}

async function startPull() {
  if (!pullImage.value.trim()) {
    message.warning('请输入镜像名称')
    return
  }
  pulling.value = true
  pullLogs.value = []
  try {
    const res = await fetch('/api/docker/images/pull', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        Authorization: `Bearer ${localStorage.getItem('aidash_token')}`,
      },
      body: JSON.stringify({ image: pullImage.value.trim(), tag: pullTag.value.trim() || 'latest' }),
    })
    if (!res.body) throw new Error('浏览器不支持流式读取')
    const reader = res.body.getReader()
    const decoder = new TextDecoder()
    let buffer = ''
    for (;;) {
      const { done, value } = await reader.read()
      if (done) break
      buffer += decoder.decode(value, { stream: true })
      const lines = buffer.split('\n')
      buffer = lines.pop() || ''
      for (const line of lines) {
        if (!line.trim()) continue
        try {
          const obj = JSON.parse(line)
          if (obj.done) {
            pullLogs.value.push('✔ 拉取完成')
          } else if (obj.error) {
            pullLogs.value.push(`✖ ${obj.error}`)
          } else {
            pullLogs.value.push(`${obj.status || ''} ${obj.id || ''} ${obj.progress || ''}`.trim())
          }
          if (pullBox.value) pullBox.value.scrollTop = pullBox.value.scrollHeight
        } catch { /* ignore partial */ }
      }
    }
    message.success('镜像拉取完成')
    load()
  } catch (e: any) {
    message.error(e.message)
  } finally {
    pulling.value = false
  }
}

function switchTab(t: string) {
  tab.value = t
  search.value = ''
  load()
}

onMounted(load)
</script>

<template>
  <div class="page">
    <div class="page-head">
      <div>
        <h1 class="page-title">容器引擎</h1>
        <p class="page-sub">直接管理单机 Docker：容器、镜像、网络与数据卷。AIDash 托管的实例会标记来源服务并保留隔离策略。</p>
      </div>
      <div class="row gap-8">
        <n-button size="small" quaternary :loading="loading" @click="load">刷新</n-button>
        <n-button size="small" type="primary" ghost @click="pullOpen = true">
          <template #icon><n-icon :component="CloudDownloadOutline" /></template>拉取镜像
        </n-button>
      </div>
    </div>

    <div class="tabs-line">
      <button class="tl" :class="{ on: tab === 'containers' }" @click="switchTab('containers')"><n-icon :component="CubeOutline" :size="15" />容器</button>
      <button class="tl" :class="{ on: tab === 'images' }" @click="switchTab('images')"><n-icon :component="LayersOutline" :size="15" />镜像</button>
      <button class="tl" :class="{ on: tab === 'networks' }" @click="switchTab('networks')"><n-icon :component="GitNetworkOutline" :size="15" />网络</button>
      <button class="tl" :class="{ on: tab === 'volumes' }" @click="switchTab('volumes')"><n-icon :component="ServerOutline" :size="15" />数据卷</button>
      <div class="spacer" />
      <n-input v-model:value="search" size="small" placeholder="搜索" clearable style="max-width: 220px">
        <template #prefix><n-icon :component="SearchOutline" /></template>
      </n-input>
      <n-checkbox v-if="tab === 'containers'" v-model:checked="showAll" @update:checked="load">显示已停止</n-checkbox>
    </div>

    <div v-if="tab === 'containers'" class="glass table-wrap">
      <div v-if="!filteredContainers.length" class="placeholder-panel">暂无容器</div>
      <table v-else class="tbl">
        <thead><tr><th>容器</th><th>镜像</th><th>状态</th><th>端口</th><th>来源</th><th style="text-align: right">操作</th></tr></thead>
        <tbody>
          <tr v-for="c in filteredContainers" :key="c.id">
            <td>
              <div class="c-name">{{ (c.names?.[0] || '').replace(/^\//, '') }}</div>
              <div class="c-id mono">{{ shortId(c.id) }}</div>
            </td>
            <td><span class="mono img">{{ c.image }}</span></td>
            <td><span class="badge" :class="c.state === 'running' ? 'ok' : 'neutral'">{{ c.state === 'running' ? '运行中' : c.state }}</span></td>
            <td class="mono">{{ c.ports?.map((p) => `${p.public_port || '-'}→${p.private_port}`).join(', ') || '-' }}</td>
            <td>
              <span v-if="c.managed" class="badge info">AIDash · {{ c.managed_for || c.managed_service_id }}</span>
              <span v-else class="muted tiny">手动</span>
              <span v-if="c.host_access" class="badge warn" style="margin-left: 6px">宿主机权限</span>
            </td>
            <td>
              <div class="row gap-8" style="justify-content: flex-end">
                <n-button size="tiny" quaternary @click="showLogs(c.id, (c.names?.[0] || '').replace(/^\//, ''))">
                  <template #icon><n-icon :component="DocumentTextOutline" /></template>
                </n-button>
                <n-button v-if="c.state !== 'running'" size="tiny" quaternary @click="containerAction(c, 'start')">
                  <template #icon><n-icon :component="PlayOutline" /></template>
                </n-button>
                <n-button v-else size="tiny" quaternary @click="containerAction(c, 'stop')">
                  <template #icon><n-icon :component="StopOutline" /></template>
                </n-button>
                <n-button size="tiny" quaternary @click="containerAction(c, 'restart')">
                  <template #icon><n-icon :component="RefreshOutline" /></template>
                </n-button>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <div v-else-if="tab === 'images'" class="glass table-wrap">
      <table class="tbl">
        <thead><tr><th>镜像</th><th>大小</th><th>使用中</th><th>创建时间</th><th style="text-align:right">操作</th></tr></thead>
        <tbody>
          <tr v-for="img in filteredImages" :key="img.id">
            <td>
              <span class="mono img">{{ (img.tags || [])[0] }}</span>
              <span v-if="img.is_ai_base" class="badge info" style="margin-left: 8px">AI 基础镜像</span>
            </td>
            <td>{{ img.size_mb.toFixed(0) }} MB</td>
            <td>{{ img.in_use ? `${img.in_use} 个容器` : '—' }}</td>
            <td class="muted">{{ fmtTime(img.created * 1000) }}</td>
            <td style="text-align:right">
              <n-button size="tiny" quaternary type="error" @click="removeImage(img)">
                <template #icon><n-icon :component="TrashOutline" /></template>删除
              </n-button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <div v-else-if="tab === 'networks'" class="glass table-wrap">
      <table class="tbl">
        <thead><tr><th>名称</th><th>驱动</th><th>子网</th><th>网关</th><th>ID</th></tr></thead>
        <tbody>
          <tr v-for="n in networks" :key="n.id">
            <td class="c-name">{{ n.name }}</td>
            <td><span class="badge neutral">{{ n.driver }}</span></td>
            <td class="mono">{{ n.subnet || '-' }}</td>
            <td class="mono">{{ n.gateway || '-' }}</td>
            <td class="mono muted">{{ shortId(n.id) }}</td>
          </tr>
        </tbody>
      </table>
    </div>

    <div v-else class="glass table-wrap">
      <table class="tbl">
        <thead><tr><th>数据卷</th><th>驱动</th><th>大小</th><th>挂载点</th></tr></thead>
        <tbody>
          <tr v-for="v in volumes" :key="v.name">
            <td class="c-name">{{ v.name }}</td>
            <td><span class="badge neutral">{{ v.driver }}</span></td>
            <td>{{ v.size_mb ? fmtMB(v.size_mb) : '-' }}</td>
            <td class="mono muted">{{ v.mountpoint }}</td>
          </tr>
        </tbody>
      </table>
    </div>

    <n-modal v-model:show="logOpen" preset="card" :title="`日志 · ${logTitle}`" style="width: 860px">
      <pre class="log-view">{{ logContent }}</pre>
    </n-modal>

    <n-modal v-model:show="pullOpen" preset="card" title="拉取镜像" style="width: 620px">
      <div class="row gap-8">
        <n-input v-model:value="pullImage" placeholder="镜像地址，例如 ollama/ollama 或 ghcr.io/open-webui/open-webui" />
        <n-input v-model:value="pullTag" placeholder="tag" style="width: 130px" />
      </div>
      <div ref="pullBox" class="log-view" style="height: 260px; margin-top: 12px">
        <div v-for="(l, i) in pullLogs" :key="i">{{ l }}</div>
        <div v-if="!pullLogs.length" class="muted">等待开始…</div>
      </div>
      <template #footer>
        <div class="row gap-12" style="justify-content: flex-end">
          <n-button quaternary @click="pullOpen = false">关闭</n-button>
          <n-button type="primary" :loading="pulling" @click="startPull">开始拉取</n-button>
        </div>
      </template>
    </n-modal>
  </div>
</template>

<style scoped>
.tabs-line { display: flex; align-items: center; gap: 8px; margin-bottom: 16px; flex-wrap: wrap; }
.tl {
  display: inline-flex; align-items: center; gap: 6px; font-size: 12px; padding: 7px 13px; border-radius: 9px;
  border: 1px solid var(--line); background: rgba(255,255,255,0.03); color: var(--muted); cursor: pointer; font-family: inherit;
}
.tl.on { color: #ede9fe; background: rgba(124,92,255,0.18); border-color: rgba(124,92,255,0.4); }
.table-wrap { overflow: auto; }
.tbl { width: 100%; border-collapse: collapse; font-size: 12px; }
.tbl th { text-align: left; font-size: 10.5px; color: #7d8bab; font-weight: 600; padding: 12px 16px; border-bottom: 1px solid var(--line); letter-spacing: 0.4px; white-space: nowrap; }
.tbl td { padding: 12px 16px; border-bottom: 1px solid rgba(255,255,255,0.045); color: #c3cee6; vertical-align: middle; }
.tbl tbody tr:hover { background: rgba(255,255,255,0.025); }
.c-name { font-weight: 600; color: var(--text); }
.c-id { font-size: 10px; color: #5d6a89; margin-top: 2px; }
.img { font-size: 11.5px; color: #a5b4fc; }
.log-view { background: #05080f; border: 1px solid var(--line); border-radius: 12px; padding: 12px; font-family: var(--mono); font-size: 11.5px; color: #b9c5dd; height: 460px; overflow: auto; white-space: pre-wrap; margin: 0; }
</style>
