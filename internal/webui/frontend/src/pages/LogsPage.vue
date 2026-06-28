<script setup lang="ts">
import { ref, computed, watch, nextTick } from 'vue'
import { useLogStream } from '../composables/useLogStream'
import { useToast } from '../composables/useToast'
import { Wifi, WifiOff, Trash2, Copy } from 'lucide-vue-next'

const { logs, connected, mode, clear } = useLogStream()
const toast = useToast()

const logContainer = ref<HTMLElement | null>(null)
const autoScroll = ref(true)
const filters = ref({ info: true, warn: true, error: true, debug: false })

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
