import { ref, onUnmounted } from 'vue'
import { getRecentLogs, type LogEntry } from '../api/logs'

export type StreamMode = 'ws' | 'polling'

export function useLogStream() {
  const logs = ref<LogEntry[]>([])
  const connected = ref(false)
  const mode = ref<StreamMode>('ws')

  let ws: WebSocket | null = null
  let pollTimer: ReturnType<typeof setInterval> | null = null
  let reconnectTimer: ReturnType<typeof setTimeout> | null = null
  let closed = false

  function append(entry: LogEntry) {
    logs.value.push(entry)
    if (logs.value.length > 500) {
      logs.value = logs.value.slice(-300)
    }
  }

  function stopPolling() {
    if (pollTimer) {
      clearInterval(pollTimer)
      pollTimer = null
    }
  }

  function startPolling() {
    if (pollTimer) return
    mode.value = 'polling'
    pollTimer = setInterval(async () => {
      try {
        const res = await getRecentLogs()
        const existing = new Set(logs.value.map(l => l.time + l.msg))
        for (const entry of res.logs) {
          if (!existing.has(entry.time + entry.msg)) {
            append(entry)
          }
        }
      } catch { /* ignore */ }
    }, 2000)
  }

  function connectWs() {
    if (closed) return
    disconnectWs()

    const proto = location.protocol === 'https:' ? 'wss:' : 'ws:'
    const url = `${proto}//${location.host}/ws`

    try {
      ws = new WebSocket(url)
      ws.onopen = () => {
        connected.value = true
        mode.value = 'ws'
        stopPolling()
        append({ level: 'info', msg: '✅ 已连接到日志流', time: new Date().toLocaleTimeString() })
      }
      ws.onmessage = (e) => {
        try {
          const data = JSON.parse(e.data)
          if (data.type === 'log') {
            append({ level: data.level || 'info', msg: data.msg || '', time: data.time || '' })
          }
        } catch { /* parse error */ }
      }
      ws.onerror = () => {
        connected.value = false
        append({ level: 'error', msg: '❌ WebSocket 连接失败，切换到 HTTP 轮询', time: new Date().toLocaleTimeString() })
        startPolling()
      }
      ws.onclose = () => {
        connected.value = false
        ws = null
        if (!closed) {
          reconnectTimer = setTimeout(connectWs, 3000)
        }
      }
    } catch {
      startPolling()
    }
  }

  function disconnectWs() {
    if (reconnectTimer) {
      clearTimeout(reconnectTimer)
      reconnectTimer = null
    }
    if (ws) {
      ws.onclose = null
      ws.onerror = null
      ws.close()
      ws = null
    }
  }

  function clear() {
    logs.value = []
  }

  // 连接前先加载环形缓冲区中的已有日志
  async function loadInitial() {
    try {
      const res = await getRecentLogs()
      if (res.logs && res.logs.length > 0) {
        logs.value = res.logs
      }
    } catch { /* ignore */ }
  }

  // start on creation
  loadInitial()
  connectWs()

  onUnmounted(() => {
    closed = true
    disconnectWs()
    stopPolling()
  })

  return { logs, connected, mode, clear, append }
}
