export function fmtNum(v: number | undefined | null, digits = 1): string {
  if (v === undefined || v === null || Number.isNaN(v)) return '-'
  return v.toFixed(digits)
}

export function fmtPct(v: number | undefined | null): string {
  if (v === undefined || v === null || Number.isNaN(v)) return '-'
  return `${v.toFixed(1)}%`
}

export function fmtMB(v: number | undefined | null): string {
  if (!v) return '0 MB'
  if (v >= 1024) return `${(v / 1024).toFixed(2)} GB`
  return `${v.toFixed(1)} MB`
}

export function fmtGB(v: number | undefined | null): string {
  if (!v) return '0 GB'
  if (v >= 1024) return `${(v / 1024).toFixed(2)} TB`
  return `${v.toFixed(1)} GB`
}

export function fmtTime(v?: string | number | null): string {
  if (!v) return '-'
  const d = new Date(v)
  if (Number.isNaN(d.getTime())) return '-'
  return d.toLocaleString('zh-CN', { hour12: false })
}

export function timeAgo(v?: string | number | null): string {
  if (!v) return '-'
  const t = new Date(v).getTime()
  if (Number.isNaN(t)) return '-'
  const diff = Math.max(0, Date.now() - t)
  const min = Math.floor(diff / 60000)
  if (min < 1) return '刚刚'
  if (min < 60) return `${min} 分钟前`
  const hour = Math.floor(min / 60)
  if (hour < 24) return `${hour} 小时前`
  return `${Math.floor(hour / 24)} 天前`
}

export function fmtUptime(seconds?: number): string {
  if (!seconds) return '-'
  const d = Math.floor(seconds / 86400)
  const h = Math.floor((seconds % 86400) / 3600)
  const m = Math.floor((seconds % 3600) / 60)
  if (d > 0) return `${d} 天 ${h} 小时`
  if (h > 0) return `${h} 小时 ${m} 分`
  return `${m} 分钟`
}

export const statusMeta: Record<string, { label: string; cls: string; badge: string }> = {
  running: { label: '运行中', cls: 'running', badge: 'ok' },
  stopped: { label: '已停止', cls: 'stopped', badge: 'neutral' },
  error: { label: '异常', cls: 'error', badge: 'bad' },
  pending: { label: '排队中', cls: 'pending', badge: 'warn' },
  pulling: { label: '拉取镜像', cls: 'pending', badge: 'info' },
  installing: { label: '下载模型', cls: 'pending', badge: 'info' },
  starting: { label: '启动中', cls: 'pending', badge: 'info' },
  removing: { label: '删除中', cls: 'pending', badge: 'warn' },
}

export function statusOf(status?: string) {
  return statusMeta[status || ''] || { label: status || '未知', cls: 'stopped', badge: 'neutral' }
}

export function roleLabel(role?: string): string {
  return { admin: '超级管理员', operator: '运维操作员', viewer: '只读观察者' }[role || ''] || role || '-'
}

export function copyText(text: string): Promise<void> {
  if (navigator.clipboard?.writeText) return navigator.clipboard.writeText(text)
  const el = document.createElement('textarea')
  el.value = text
  document.body.appendChild(el)
  el.select()
  document.execCommand('copy')
  document.body.removeChild(el)
  return Promise.resolve()
}

export function shortId(id?: string, len = 12): string {
  if (!id) return '-'
  return id.length > len ? id.slice(0, len) : id
}
