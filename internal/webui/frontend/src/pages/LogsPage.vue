<script setup lang="ts">
import { ref, computed, watch, nextTick, onMounted, onUnmounted } from 'vue'
import { useLogStream } from '../composables/useLogStream'
import { useToast } from '../composables/useToast'
import { Wifi, WifiOff, Trash2, Copy, Download } from 'lucide-vue-next'

const { logs, connected, mode, clear } = useLogStream()
const toast = useToast()

const LS_KEY_FILTERS = 'logs_filters'
const LS_KEY_AUTOSCROLL = 'logs_autoscroll'

function loadFilters() {
  try {
    const saved = localStorage.getItem(LS_KEY_FILTERS)
    if (saved) return JSON.parse(saved)
  } catch { /* ignore */ }
  return { info: true, warn: true, error: true, debug: false }
}

function loadAutoScroll(): boolean {
  try {
    const saved = localStorage.getItem(LS_KEY_AUTOSCROLL)
    if (saved !== null) return JSON.parse(saved)
  } catch { /* ignore */ }
  return true
}

const logContainer = ref<HTMLElement | null>(null)
const autoScroll = ref(loadAutoScroll())
const filters = ref(loadFilters())

watch(filters, (v) => {
  localStorage.setItem(LS_KEY_FILTERS, JSON.stringify(v))
}, { deep: true })

watch(autoScroll, (v) => {
  localStorage.setItem(LS_KEY_AUTOSCROLL, JSON.stringify(v))
})

const filteredLogs = computed(() =>
  logs.value.filter(l => {
    const level = l.level || 'info'
    return (filters.value as any)[level] ?? true
  })
)

watch(() => logs.value.length, async () => {
  if (autoScroll.value) {
    await nextTick()
    if (logContainer.value) {
      logContainer.value.scrollTop = logContainer.value.scrollHeight
    }
  }
})

function copyLogs() {
  const text = logs.value.map(l => `[${l.time}] [${(l.level || 'info').toUpperCase()}] ${l.msg}`).join('\n')
  navigator.clipboard.writeText(text).then(() => toast.show('已复制到剪贴板', 'success'))
}

function isWithinRetention(timeStr: string, retentionMinutes: number): boolean {
  if (retentionMinutes <= 0) return true
  const parts = timeStr.split(/[:.]/)
  if (parts.length < 3) return true
  const now = new Date()
  const logDate = new Date(now.getFullYear(), now.getMonth(), now.getDate(), +parts[0], +parts[1], +parts[2])
  return (now.getTime() - logDate.getTime()) / 60000 <= retentionMinutes
}

function saveLogs() {
  const retention = Number(localStorage.getItem('logs_retention_minutes') ?? '0')
  const lines = logs.value
    .filter(l => isWithinRetention(l.time, retention))
    .map(l => `[${l.time}] [${(l.level || 'info').toUpperCase()}] ${l.msg}`)
  if (lines.length === 0) {
    toast.show('没有可保存的日志', 'error')
    return
  }
  const text = lines.join('\n')
  const blob = new Blob([text], { type: 'text/plain;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = `langlang-logs-${new Date().toISOString().slice(0, 19).replace(/[:-]/g, '')}.log`
  document.body.appendChild(a)
  a.click()
  document.body.removeChild(a)
  URL.revokeObjectURL(url)
  toast.show(`已保存 ${lines.length} 条日志`, 'success')
}

// ===== 自动保存定时器 =====

const LS_KEY_ARCHIVE = 'logs_archive'
const AUTO_SAVE_INTERVAL_MS = 5000
const ARCHIVE_MAX = 5000

let autoSaveTimer: ReturnType<typeof setInterval> | null = null

function formatLogLine(l: { time: string; level?: string; msg: string }): string {
  return `[${l.time}] [${(l.level || 'info').toUpperCase()}] ${l.msg}`
}

function loadArchive(): string[] {
  try {
    const saved = localStorage.getItem(LS_KEY_ARCHIVE)
    if (saved) return JSON.parse(saved)
  } catch { /* ignore */ }
  return []
}

function saveArchive() {
  const retention = Number(localStorage.getItem('logs_retention_minutes') ?? '0')
  const existing = loadArchive()
  const existingSet = new Set(existing)

  // 合并当前流日志（去重）
  const current = logs.value
    .filter(l => isWithinRetention(l.time, retention))
    .map(formatLogLine)
  for (const line of current) {
    existingSet.add(line)
  }

  // 按时间排序取最新的 N 条
  const merged = Array.from(existingSet)
  const sorted = merged.sort().reverse().slice(0, ARCHIVE_MAX)

  localStorage.setItem(LS_KEY_ARCHIVE, JSON.stringify(sorted))
}

onMounted(() => {
  // 恢复之前自动保存的日志
  const archived = loadArchive()
  if (archived.length > 0) {
    const restored = archived.map(line => {
      // 解析格式: [HH:mm:ss.xxx] [LEVEL] msg
      const match = line.match(/^\[([^\]]+)\]\s+\[([^\]]+)\]\s+(.*)/)
      if (match) {
        return { time: match[1], level: match[2].toLowerCase(), msg: match[3] }
      }
      return { time: '', level: 'info', msg: line }
    })
    // 插入到日志列表开头（不覆盖实时流）
    logs.value = [...restored, ...logs.value]
    if (logs.value.length > 500) {
      logs.value = logs.value.slice(-500)
    }
  }

  // 启动自动保存定时器
  autoSaveTimer = setInterval(saveArchive, AUTO_SAVE_INTERVAL_MS)
})

onUnmounted(() => {
  if (autoSaveTimer) {
    clearInterval(autoSaveTimer)
    autoSaveTimer = null
  }
})

const levelColors: Record<string, string> = {
  info: 'text-green-400',
  warn: 'text-amber-400',
  error: 'text-red-400',
  debug: 'text-zinc-500',
}
</script>

<template>
  <div class="flex h-[calc(100vh-8rem)] flex-col gap-3">
    <!-- Toolbar -->
    <div class="flex flex-wrap items-center gap-3">
      <button @click="clear" class="inline-flex items-center gap-1.5 rounded-lg border border-zinc-200 px-3 py-1.5 text-xs hover:bg-zinc-50">
        <Trash2 :size="14" /> 清除
      </button>
      <button @click="copyLogs" class="inline-flex items-center gap-1.5 rounded-lg border border-zinc-200 px-3 py-1.5 text-xs hover:bg-zinc-50">
        <Copy :size="14" /> 复制
      </button>
      <button @click="saveLogs" class="inline-flex items-center gap-1.5 rounded-lg border border-zinc-200 px-3 py-1.5 text-xs hover:bg-zinc-50">
        <Download :size="14" /> 保存
      </button>
      <label class="flex items-center gap-1.5 text-xs text-zinc-500">
        <input v-model="autoScroll" type="checkbox" class="accent-red-700" /> 自动滚动
      </label>
      <div class="h-4 w-px bg-zinc-200" />
      <label v-for="lvl in ['info','warn','error','debug']" :key="lvl" class="flex items-center gap-1 text-xs text-zinc-500 capitalize">
        <input v-model="(filters as any)[lvl]" type="checkbox" class="accent-red-700" /> {{ lvl }}
      </label>
      <div class="ml-auto flex items-center gap-1.5 text-xs">
        <component :is="connected ? Wifi : WifiOff" :size="14" :class="connected ? 'text-green-500' : 'text-red-400'" />
        <span class="text-zinc-400">{{ mode === 'ws' ? 'WebSocket' : 'HTTP 轮询' }}</span>
      </div>
    </div>

    <!-- Log Viewer -->
    <div
      ref="logContainer"
      class="flex-1 overflow-auto rounded-lg bg-zinc-900 p-4 font-mono text-xs leading-relaxed"
    >
      <div v-if="filteredLogs.length === 0" class="py-12 text-center text-zinc-600">
        ⏳ 等待日志...
      </div>
      <div
        v-for="(l, i) in filteredLogs"
        :key="i"
        :class="levelColors[l.level || 'info']"
        class="py-0.5 hover:bg-white/5"
      >
        <span class="text-zinc-600">[{{ l.time }}]</span>
        <span class="ml-1 font-semibold">[{{ (l.level || 'info').toUpperCase() }}]</span>
        <span class="ml-1">{{ l.msg }}</span>
      </div>
    </div>
  </div>
</template>
