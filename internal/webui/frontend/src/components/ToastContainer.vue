<script setup lang="ts">
import { useToast } from '../composables/useToast'
import { CheckCircle, XCircle, Info, AlertTriangle } from 'lucide-vue-next'

const { toasts } = useToast()

const iconMap: Record<string, any> = {
  success: CheckCircle,
  error: XCircle,
  info: Info,
  warn: AlertTriangle,
}

const colorMap: Record<string, string> = {
  success: 'bg-green-600',
  error: 'bg-red-600',
  info: 'bg-blue-600',
  warn: 'bg-amber-500',
}
</script>

<template>
  <div class="fixed right-4 top-4 z-50 flex flex-col gap-2">
    <div
      v-for="t in toasts"
      :key="t.id"
      :class="colorMap[t.type]"
      class="flex items-center gap-2 rounded-lg px-4 py-2.5 text-sm text-white shadow-lg animate-[slideIn_0.25s_ease]"
    >
      <component :is="iconMap[t.type]" :size="16" />
      {{ t.msg }}
    </div>
  </div>
</template>

<style>
@keyframes slideIn {
  from { transform: translateX(100%); opacity: 0; }
  to { transform: translateX(0); opacity: 1; }
}
</style>
