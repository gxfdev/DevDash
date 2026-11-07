<script setup lang="ts">
import { nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { Terminal } from 'xterm'
import { FitAddon } from 'xterm-addon-fit'
import 'xterm/css/xterm.css'
import { useAuthStore } from '@/stores/auth'

const props = withDefaults(defineProps<{ path: string; height?: string; notice?: string }>(), { height: '420px' })
const auth = useAuthStore()
const box = ref<HTMLDivElement>()
let term: Terminal | null = null
let fit: FitAddon | null = null
let ws: WebSocket | null = null
let ro: ResizeObserver | null = null

function connect() {
  if (!box.value) return
  term = new Terminal({
    cursorBlink: true,
    fontSize: 12.5,
    fontFamily: "'JetBrains Mono', Consolas, monospace",
    theme: {
      background: '#05080f',
      foreground: '#c9d4ee',
      cursor: '#7c5cff',
      selectionBackground: 'rgba(124,92,255,0.3)',
      black: '#0a0f1a', red: '#fb7185', green: '#34d399', yellow: '#fbbf24',
      blue: '#60a5fa', magenta: '#c084fc', cyan: '#22d3ee', white: '#e2e8f0',
    },
    scrollback: 4000,
    convertEol: true,
  })
  fit = new FitAddon()
  term.loadAddon(fit)
  term.open(box.value)
  nextTick(() => fit?.fit())

  const proto = location.protocol === 'https:' ? 'wss:' : 'ws:'
  const url = `${proto}//${location.host}${props.path}?token=${encodeURIComponent(auth.token)}&cols=${term.cols}&rows=${term.rows}`
  ws = new WebSocket(url)
  ws.onopen = () => {
    term?.write('\x1b[38;5;141m● AIDash 终端已连接\x1b[0m\r\n')
    if (props.notice) term?.write(`\x1b[38;5;244m${props.notice}\x1b[0m\r\n`)
  }
  ws.onmessage = (ev) => term?.write(typeof ev.data === 'string' ? ev.data : '')
  ws.onclose = () => term?.write('\r\n\x1b[38;5;203m● 连接已断开\x1b[0m\r\n')
  ws.onerror = () => term?.write('\r\n\x1b[38;5;203m● 连接异常\x1b[0m\r\n')
  term.onData((data) => {
    if (ws?.readyState === WebSocket.OPEN) ws.send(JSON.stringify({ type: 'input', data }))
  })
  term.onResize(({ cols, rows }) => {
    if (ws?.readyState === WebSocket.OPEN) ws.send(JSON.stringify({ type: 'resize', cols, rows }))
  })
  ro = new ResizeObserver(() => fit?.fit())
  ro.observe(box.value)
}

onMounted(connect)
onBeforeUnmount(() => {
  ro?.disconnect()
  ws?.close()
  term?.dispose()
  term = null
})
watch(() => props.path, () => {
  ws?.close()
  term?.dispose()
  term = null
  nextTick(connect)
})
</script>

<template>
  <div class="term-shell" :style="{ height }">
    <div ref="box" class="term-box" />
  </div>
</template>

<style scoped>
.term-shell { background: #05080f; border: 1px solid var(--line); border-radius: 12px; padding: 8px 10px; overflow: hidden; }
.term-box { width: 100%; height: 100%; }
</style>
