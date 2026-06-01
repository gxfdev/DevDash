<template>
  <div class="terminal-view">
    <n-space justify="space-between" align="center" style="margin-bottom: 8px">
      <n-space align="center">
        <n-select v-model:value="selectedShell" :options="shellOptions" style="width: 200px" size="small" />
        <n-button type="primary" size="small" @click="connectWS" :disabled="connStatus === 'connected'">连接</n-button>
        <n-button size="small" @click="disconnectWS" :disabled="connStatus === 'disconnected'">断开</n-button>
      </n-space>
      <n-tag :type="statusType" size="small">{{ statusText }}</n-tag>
    </n-space>
    <div ref="terminalRef" class="terminal-container"></div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted, nextTick, computed } from 'vue'
import { Terminal } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import { WebLinksAddon } from '@xterm/addon-web-links'
import '@xterm/xterm/css/xterm.css'

const terminalRef = ref<HTMLElement>()
const selectedShell = ref('/bin/bash')
const connStatus = ref<'disconnected' | 'connecting' | 'connected' | 'error'>('disconnected')

let term: Terminal | null = null
let fitAddon: FitAddon | null = null
let ws: WebSocket | null = null

const shellOptions = [
  { label: '/bin/bash', value: '/bin/bash' },
  { label: '/bin/sh', value: '/bin/sh' },
  { label: '/bin/zsh', value: '/bin/zsh' },
]

const statusType = computed(() => {
  const map: Record<string, string> = { connected: 'success', connecting: 'warning', error: 'error', disconnected: 'default' }
  return map[connStatus.value] || 'default'
})
const statusText = computed(() => {
  const map: Record<string, string> = { connected: '已连接', connecting: '连接中...', error: '连接失败', disconnected: '未连接' }
  return map[connStatus.value] || '未知'
})

function connectWS() {
  if (ws) {
    ws.close()
    ws = null
  }

  connStatus.value = 'connecting'
  const token = localStorage.getItem('token') || ''
  const proto = location.protocol === 'https:' ? 'wss:' : 'ws:'
  const wsUrl = `${proto}//${location.host}/ws/terminal?token=${encodeURIComponent(token)}&shell=${encodeURIComponent(selectedShell.value)}`

  ws = new WebSocket(wsUrl)
  ws.binaryType = 'arraybuffer'

  ws.onopen = () => {
    connStatus.value = 'connected'
    term?.focus()
    nextTick(() => fitAddon?.fit())
  }

  ws.onmessage = (event) => {
    const data = event.data instanceof ArrayBuffer ? new TextDecoder().decode(event.data) : event.data
    term?.write(data)
  }

  ws.onclose = (event) => {
    connStatus.value = 'disconnected'
    ws = null
    if (event.code !== 1000 && event.code !== 1001) {
      term?.write(`\r\n\x1b[1;31m✗ 连接关闭 (code: ${event.code})\x1b[0m\r\n`)
    }
  }

  ws.onerror = () => {
    connStatus.value = 'error'
    term?.write('\r\n\x1b[1;31m✗ 连接错误\x1b[0m\r\n')
  }
}

function disconnectWS() {
  if (ws) {
    ws.close(1000, 'user disconnect')
    ws = null
  }
  connStatus.value = 'disconnected'
}

function sendResize() {
  if (ws && ws.readyState === WebSocket.OPEN && term) {
    ws.send(JSON.stringify({ rows: term.rows, cols: term.cols }))
  }
}

onMounted(async () => {
  await nextTick()

  term = new Terminal({
    cursorBlink: true,
    fontSize: 14,
    fontFamily: "'Cascadia Code', 'Fira Code', 'JetBrains Mono', Menlo, Monaco, 'Courier New', monospace",
    theme: {
      background: '#1a1a2e',
      foreground: '#e0e0e0',
      cursor: '#0f0',
      selectionBackground: 'rgba(0, 255, 0, 0.3)',
    },
    rows: 30,
    cols: 100,
  })

  fitAddon = new FitAddon()
  term.loadAddon(fitAddon)
  term.loadAddon(new WebLinksAddon())

  if (terminalRef.value) {
    term.open(terminalRef.value)
    fitAddon.fit()
  }

  term.onData((data) => {
    if (ws && ws.readyState === WebSocket.OPEN) {
      ws.send(data)
    }
  })

  term.onResize(() => {
    sendResize()
  })

  window.addEventListener('resize', handleResize)
})

function handleResize() {
  fitAddon?.fit()
  sendResize()
}

onUnmounted(() => {
  window.removeEventListener('resize', handleResize)
  disconnectWS()
  term?.dispose()
})
</script>

<style scoped>
.terminal-view {
  height: 100%;
  display: flex;
  flex-direction: column;
}
.terminal-container {
  flex: 1;
  border-radius: 6px;
  overflow: hidden;
  background: #1a1a2e;
}
</style>
