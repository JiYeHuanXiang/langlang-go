<script lang="ts">
export interface LogLine {
  time: string
  type: 'info' | 'output' | 'error' | 'success'
  msg: string
}
</script>

<script setup lang="ts">
import { ref, computed, watch, nextTick } from 'vue'
import { ChevronDown, ChevronRight, Trash2 } from 'lucide-vue-next'

const props = defineProps<{
  open: boolean
  status: 'idle' | 'running' | 'success' | 'error' | 'cancelled' | 'timeout'
  logs: LogLine[]
  executionTime: number
}>()

const emit = defineEmits<{
  'update:open': [value: boolean]
  clear: []
}>()

const statusConfig = computed(() => {
  const map = {
    idle:      { color: 'bg-zinc-400',   text: 'text-zinc-500',   label: '就绪' },
    running:   { color: 'bg-blue-400 animate-pulse', text: 'text-blue-400', label: '运行中' },
    success:   { color: 'bg-green-400',  text: 'text-green-400',  label: '完成' },
    error:     { color: 'bg-red-400',    text: 'text-red-400',    label: '错误' },
    cancelled: { color: 'bg-yellow-400', text: 'text-yellow-400', label: '已取消' },
    timeout:   { color: 'bg-orange-400', text: 'text-orange-400', label: '超时' },
  } as const
  return map[props.status] || map.idle
})

const logTypeClass = (type: string) => {
  const map: Record<string, string> = { info: 'text-blue-400', output: 'text-zinc-200', error: 'text-red-400', success: 'text-green-400' }
  return map[type] || 'text-zinc-400'
}

const scrollRef = ref<HTMLElement | null>(null)

watch(() => props.logs.length, () => {
  nextTick(() => {
    if (scrollRef.value) {
      scrollRef.value.scrollTop = scrollRef.value.scrollHeight
    }
  })
})
</script>

<template>
  <div class="flex flex-col overflow-hidden border-t border-zinc-700 bg-zinc-900">
    <!-- Header -->
    <div
      class="flex items-center gap-2 px-3 py-1.5 cursor-pointer select-none hover:bg-zinc-800"
      @click="emit('update:open', !open)"
    >
      <component :is="open ? ChevronDown : ChevronRight" :size="14" class="text-zinc-500" />
      <span class="text-xs text-zinc-400 font-medium">输出</span>
      <span class="flex items-center gap-1.5">
        <span :class="['inline-block w-1.5 h-1.5 rounded-full', statusConfig.color]" />
        <span :class="['text-[11px]', statusConfig.text]">{{ statusConfig.label }}</span>
      </span>
      <span v-if="executionTime > 0" class="text-[11px] text-zinc-500 ml-1">{{ executionTime }}ms</span>
      <span v-if="logs.length > 0" class="text-[11px] text-zinc-600 ml-1">{{ logs.length }} 行</span>
      <div class="flex-1" />
      <button
        v-if="logs.length > 0 && open"
        class="flex items-center gap-1 text-[11px] text-zinc-500 hover:text-zinc-300 px-1.5 py-0.5 rounded hover:bg-zinc-700"
        @click.stop="emit('clear')"
      >
        <Trash2 :size="11" />
        清除
      </button>
    </div>
    <!-- Log lines -->
    <div ref="scrollRef" v-show="open" class="flex-1 overflow-y-auto px-3 pb-2 font-mono text-[12px] leading-5 min-h-0">
      <div v-if="logs.length === 0" class="text-zinc-600 py-2">暂无输出</div>
      <div v-for="(log, idx) in logs" :key="idx" class="flex gap-2 py-px">
        <span class="text-zinc-600 shrink-0 w-16 text-right">{{ log.time }}</span>
        <span :class="logTypeClass(log.type)">{{ log.msg }}</span>
      </div>
    </div>
  </div>
</template>
