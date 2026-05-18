<template>
  <app-layout>
    <div class="page">
      <div class="page-header">
        <h2>Web 终端</h2>
        <n-space>
          <n-select v-model:value="selectedNode" :options="nodeOptions" style="width:180px" />
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
import { useNodesStore } from '@/stores/nodes'
import client from '@/api/client'

const nodesStore = useNodesStore()

const selectedNode = ref<string | null>(null)
const termRef = ref<HTMLDivElement>()
let term: Terminal | null = null
let fitAddon: FitAddon | null = null
let ws: WebSocket | null = null
let dataDisposable: any = null

const nodeOptions = computed(() => nodesStore.nodes.map((n: any) => ({ label: n.name || n.hostname || n.ip, value: n.id })))

function createTerminal() {
  disposeTerminal()

  if (!termRef.value) return

  term = new Terminal({
    theme: { background: '#0d1117', foreground: '#e6edf3', cursor: '#e6edf3', cursorAccent: '#0d1117', selectionBackground: '#388bfd40' },
    fontSize: 13,
    fontFamily: 'Consolas, "Courier New", monospace',
    cursorBlink: true,
    scrollback: 5000,
    allowProposedApi: true,
    convertEol: true,
  })

  fitAddon = new FitAddon()
  term.loadAddon(fitAddon)

  try {
    term.open(termRef.value)
    fitAddon.fit()
  } catch (e) {
    console.error('[terminal] open error:', e)
  }

  term.writeln('\x1b[1;36mDevDash Web Terminal\x1b[0m')
  term.writeln('Waiting for connection...')
}

function disposeTerminal() {
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

function connectWS() {
  disconnectWS()

  if (!selectedNode.value || !term) {
    createTerminal()
    if (!selectedNode.value || !term) return
  }

  const proto = location.protocol === 'https:' ? 'wss:' : 'ws:'
  const token = localStorage.getItem('token') || ''
  const url = `${proto}//${location.host}/ws/terminal/${encodeURIComponent(selectedNode.value)}?token=${encodeURIComponent(token)}`
  console.log('[terminal] connecting to', url.replace(token, '[REDACTED]'))

  try {
    ws = new WebSocket(url)
  } catch (e: any) {
    console.error('[terminal] WebSocket create error:', e)
    term?.write('\r\n\x1b[1;31m✗ WebSocket 创建失败: ' + (e.message || e) + '\x1b[0m\r\n')
    return
  }

  ws.binaryType = 'arraybuffer'

  ws.onopen = () => {
    term?.write('\r\n\x1b[1;32m✓ Connected\x1b[0m\r\n')
    try { fitAddon?.fit() } catch {}
  }

  ws.onmessage = (event) => {
    if (typeof event.data === 'string') {
      term?.write(event.data)
    } else {
      term?.write(new Uint8Array(event.data as ArrayBuffer))
    }
  }

  ws.onclose = (event) => {
    term?.write('\r\n\x1b[1;33m⚠ Disconnected' + (event.code ? ` (code=${event.code})` : '') + '\x1b[0m\r\n')
    ws = null
  }

  ws.onerror = (event) => {
    console.error('[terminal] WS error:', event)
    term?.write('\r\n\x1b[1;31m✗ Connection Error\x1b[0m\r\n')
  }
}

function setupDataHandler() {
  if (!term || dataDisposable) return

  dataDisposable = term.onData((data: string) => {
    if (ws && ws.readyState === WebSocket.OPEN) {
      ws.send(data)
    } else {
      term?.write('\x1b[1;33m(未连接)\x1b[0m')
    }
  })
}

function disconnectWS() {
  if (ws) {
    try { ws.close(1000, 'User disconnected') } catch {}
    ws = null
  }
}

function connect() {
  connectWS()
  setupDataHandler()
}

function disconnect() {
  disconnectWS()
  term?.write('\r\n\x1b[1;33m已断开\x1b[0m\r\n')
}

function reconnect() {
  disconnectWS()
  term?.clear()
  term?.write('\x1b[1;36mReconnecting...\x1b[0m\r\n')
  setTimeout(() => connect(), 200)
}

function handleResize() { try { fitAddon?.fit() } catch {} }

onMounted(async () => {
  await nodesStore.fetchNodes()
  await nextTick()

  createTerminal()
  window.addEventListener('resize', handleResize)

  if (nodesStore.nodes.length) {
    selectedNode.value = nodesStore.nodes[0]?.id
    await nextTick()
    connect()
  }
})

watch(selectedNode, async () => {
  await nextTick()
  if (termRef.value) connect()
})

onUnmounted(() => {
  disconnectWS()
  window.removeEventListener('resize', handleResize)
  disposeTerminal()
})
</script>

<style scoped>
.page { padding: 24px; height: 100%; display: flex; flex-direction: column; }
.page-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 16px; flex-shrink: 0; }
h2 { font-size: 20px; font-weight: 600; margin: 0; }
.terminal-box { flex: 1; background: #0d1117; border: 1px solid #30363d; border-radius: 8px; overflow: hidden; padding: 8px; min-height: 500px; position: relative; }
.terminal-screen { width: 100%; height: 100%; }
</style>