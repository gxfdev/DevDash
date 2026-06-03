<template>
  <app-layout>
    <div class="page">
      <div class="page-header">
        <h2>Web 终端</h2>
        <n-space>
          <n-select v-model:value="selectedShell" :options="shellOptions" style="width:180px" placeholder="选择 Shell" />
          <n-tag :type="statusTagType" size="small" round>{{ statusText }}</n-tag>
          <n-button size="small" @click="reconnect">重连</n-button>
          <n-button size="small" type="error" @click="disconnect">断开</n-button>
        </n-space>
      </div>

      <div class="terminal-box">
        <div ref="termRef" class="terminal-screen" />
      </div>
    </div>
  </app-layout>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted, watch, nextTick, computed } from 'vue'
import { Terminal } from 'xterm'
import { FitAddon } from 'xterm-addon-fit'
import 'xterm/css/xterm.css'
import AppLayout from '@/components/AppLayout.vue'
import client from '@/api/client'

const selectedShell = ref('')
const termRef = ref<HTMLDivElement>()
const connStatus = ref<'disconnected' | 'connecting' | 'connected' | 'error'>('disconnected')
let term: Terminal | null = null
let fitAddon: FitAddon | null = null
let ws: WebSocket | null = null
let dataDisposable: { dispose(): void } | null = null
let resizeDisposable: { dispose(): void } | null = null
let reconnectTimer: ReturnType<typeof setTimeout> | null = null
let connectTimeoutTimer: ReturnType<typeof setTimeout> | null = null
let reconnectAttempts = 0
let watchReady = false
const MAX_RECONNECT_ATTEMPTS = 5
const CONNECT_TIMEOUT_MS = 10000

interface ShellOption {
  name: string
  path: string
}

const shellList = ref<ShellOption[]>([])

const shellOptions = computed(() => {
  const opts = shellList.value.map(s => ({ label: `${s.name} (${s.path})`, value: s.path }))
  return [{ label: '默认 Shell', value: '' }, ...opts]
})

const statusText = computed(() => {
  switch (connStatus.value) {
    case 'connected': return '已连接'
    case 'connecting': return '连接中...'
    case 'error': return '连接失败'
    default: return '未连接'
  }
})

const statusTagType = computed(() => {
  switch (connStatus.value) {
    case 'connected': return 'success'
    case 'connecting': return 'warning'
    case 'error': return 'error'
    default: return 'default'
  }
})

async function fetchShells() {
  try {
    const { data } = await client.get<ShellOption[]>('/terminal/shells')
    shellList.value = Array.isArray(data) ? data : []
  } catch {
    shellList.value = []
  }
}

function createTerminal() {
  disposeTerminal()

  if (!termRef.value) return

  term = new Terminal({
    theme: { background: '#0d1117', foreground: '#e6edf3', cursor: '#e6edf3', cursorAccent: '#0d1117', selectionBackground: '#388bfd40' },
    fontSize: 14,
    fontFamily: '"Cascadia Code", "JetBrains Mono", Consolas, "Courier New", monospace',
    cursorBlink: true,
    scrollback: 10000,
    allowProposedApi: true,
    convertEol: false,
    windowsMode: false,
  })

  fitAddon = new FitAddon()
  term.loadAddon(fitAddon)

  try {
    term.open(termRef.value)
    fitAddon.fit()
  } catch (e) {
    console.warn('[terminal] open error:', (e as Error)?.message || e)
  }
}

function disposeTerminal() {
  if (reconnectTimer) {
    clearTimeout(reconnectTimer)
    reconnectTimer = null
  }
  if (connectTimeoutTimer) {
    clearTimeout(connectTimeoutTimer)
    connectTimeoutTimer = null
  }

  if (resizeDisposable) {
    try { resizeDisposable.dispose() } catch {}
    resizeDisposable = null
  }
  if (dataDisposable) {
    try { dataDisposable.dispose() } catch {}
    dataDisposable = null
  }
  if (fitAddon) {
    try { fitAddon.dispose() } catch {}
    fitAddon = null
  }
  if (term) {
    try { term.dispose() } catch {}
    term = null
  }
}

function buildWSUrl(): string {
  const proto = location.protocol === 'https:' ? 'wss:' : 'ws:'
  const token = localStorage.getItem('token') || ''
  let url = `${proto}//${location.host}/ws/terminal/self?token=${encodeURIComponent(token)}`
  if (selectedShell.value) {
    url += `&shell=${encodeURIComponent(selectedShell.value)}`
  }
  return url
}

function sendResize(cols: number, rows: number) {
  if (ws && ws.readyState === WebSocket.OPEN) {
    ws.send(JSON.stringify({ type: 'resize', cols, rows }))
  }
}

function connectWS() {
  disconnectWS()
  connStatus.value = 'connecting'

  if (!term) {
    createTerminal()
    if (!term) {
      connStatus.value = 'error'
      return
    }
  }

  const token = localStorage.getItem('token')
  if (!token) {
    term.write('\r\n\x1b[1;31m✗ 未登录，请先登录\x1b[0m\r\n')
    connStatus.value = 'error'
    return
  }

  const url = buildWSUrl()
  console.log('[terminal] connecting to', url.replace(/token=[^&]+/, 'token=[REDACTED]'))

  try {
    ws = new WebSocket(url)
  } catch (e: unknown) {
    console.warn('[terminal] WebSocket create error:', (e as Error)?.message || e)
    term.write('\r\n\x1b[1;31m✗ WebSocket 创建失败\x1b[0m\r\n')
    connStatus.value = 'error'
    scheduleReconnect()
    return
  }

  ws.binaryType = 'arraybuffer'

  connectTimeoutTimer = setTimeout(() => {
    if (ws && ws.readyState === WebSocket.CONNECTING) {
      term?.write('\r\n\x1b[1;31m✗ 连接超时 (10s)\x1b[0m\r\n')
      ws.close()
      ws = null
      connStatus.value = 'error'
      scheduleReconnect()
    }
  }, CONNECT_TIMEOUT_MS)

  ws.onopen = () => {
    if (connectTimeoutTimer) {
      clearTimeout(connectTimeoutTimer)
      connectTimeoutTimer = null
    }
    connStatus.value = 'connected'
    reconnectAttempts = 0
    term?.write('\x1b[1;32m✓ 连接成功\x1b[0m\r\n')
    try {
      fitAddon?.fit()
      if (term) {
        sendResize(term.cols, term.rows)
      }
    } catch {}
  }

  ws.onmessage = (event) => {
    if (typeof event.data === 'string') {
      term?.write(event.data)
    } else {
      term?.write(new Uint8Array(event.data as ArrayBuffer))
    }
  }

  ws.onclose = (event) => {
    if (connectTimeoutTimer) {
      clearTimeout(connectTimeoutTimer)
      connectTimeoutTimer = null
    }
    console.log('[terminal] closed:', event.code, event.reason)
    ws = null
    connStatus.value = 'disconnected'

    if (event.code === 1000 || event.code === 1001) {
      return
    }

    if (event.code === 4001 || event.code === 4010) {
      term?.write('\r\n\x1b[1;31m✗ 认证失败，请重新登录\x1b[0m\r\n')
      connStatus.value = 'error'
    } else if (event.code === 4030) {
      term?.write('\r\n\x1b[1;31m✗ 权限不足\x1b[0m\r\n')
      connStatus.value = 'error'
    } else {
      term?.write(`\r\n\x1b[1;31m✗ 连接关闭 (code: ${event.code})\x1b[0m\r\n`)
      scheduleReconnect()
    }
  }

  ws.onerror = () => {
    connStatus.value = 'error'
    term?.write('\r\n\x1b[1;31m✗ 连接错误，请检查网络或后端服务\x1b[0m\r\n')
  }
}

function scheduleReconnect() {
  if (reconnectTimer) return
  if (reconnectAttempts >= MAX_RECONNECT_ATTEMPTS) {
    term?.write('\r\n\x1b[1;31m✗ 已达到最大重连次数，请手动重连\x1b[0m\r\n')
    return
  }

  reconnectAttempts++
  const delay = Math.min(3000 * reconnectAttempts, 15000)

  term?.write(`\r\n\x1b[1;33m⚠ ${delay / 1000}秒后重连 (${reconnectAttempts}/${MAX_RECONNECT_ATTEMPTS})...\x1b[0m\r\n`)

  reconnectTimer = setTimeout(() => {
    reconnectTimer = null
    if (!ws || ws.readyState === WebSocket.CLOSED) {
      connectWS()
    }
  }, delay)
}

function setupDataHandler() {
  if (!term || dataDisposable) return
  dataDisposable = term.onData((data: string) => {
    if (ws && ws.readyState === WebSocket.OPEN) {
      ws.send(data)
    }
  })
}

function setupResizeHandler() {
  if (!term || resizeDisposable) return
  resizeDisposable = term.onResize(({ cols, rows }) => {
    sendResize(cols, rows)
  })
}

function disconnectWS() {
  if (reconnectTimer) {
    clearTimeout(reconnectTimer)
    reconnectTimer = null
  }
  if (connectTimeoutTimer) {
    clearTimeout(connectTimeoutTimer)
    connectTimeoutTimer = null
  }

  if (ws) {
    try { ws.close(1000, 'User disconnected') } catch {}
    ws = null
  }
  connStatus.value = 'disconnected'
}

function connect() {
  connectWS()
  setupDataHandler()
  setupResizeHandler()
}

function disconnect() {
  disconnectWS()
  reconnectAttempts = 0
  term?.write('\r\n\x1b[1;33m已断开\x1b[0m\r\n')
}

function reconnect() {
  disconnectWS()
  reconnectAttempts = 0
  term?.clear()
  connect()
}

function handleResize() {
  try {
    fitAddon?.fit()
  } catch {}
}

onMounted(async () => {
  await fetchShells()
  await nextTick()

  createTerminal()
  window.addEventListener('resize', handleResize)

  await nextTick()
  connect()
  watchReady = true
})

watch(selectedShell, () => {
  if (!watchReady) return
  if (ws && ws.readyState === WebSocket.OPEN) {
    disconnectWS()
    term?.clear()
    connect()
  }
})

onUnmounted(() => {
  disconnectWS()
  window.removeEventListener('resize', handleResize)
  disposeTerminal()
})
</script>

<style scoped>
.page { padding: 24px; height: 100%; display: flex; flex-direction: column; }
.page-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 16px; flex-shrink: 0; flex-wrap: wrap; gap: 8px; }
h2 { font-size: 20px; font-weight: 600; margin: 0; }
.terminal-box { flex: 1; background: #0d1117; border: 1px solid #30363d; border-radius: 8px; overflow: hidden; padding: 8px; min-height: 500px; position: relative; }
.terminal-screen { width: 100%; height: 100%; }
</style>
