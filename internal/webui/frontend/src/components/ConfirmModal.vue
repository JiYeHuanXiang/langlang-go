<script setup lang="ts">
defineProps<{
  open: boolean
  title: string
}>()

const emit = defineEmits<{
  close: []
  confirm: []
}>()
</script>

<template>
  <Teleport to="body">
    <div
      v-if="open"
      class="fixed inset-0 z-50 flex items-center justify-center bg-black/40"
      @click.self="emit('close')"
    >
      <div class="w-full max-w-md rounded-xl bg-white shadow-xl">
        <div class="flex items-center justify-between border-b border-zinc-100 px-5 py-4">
          <h3 class="text-sm font-semibold">{{ title }}</h3>
          <button @click="emit('close')" class="text-zinc-400 hover:text-zinc-600 text-lg leading-none">&times;</button>
        </div>
        <div class="px-5 py-4">
          <slot />
        </div>
        <div class="flex justify-end gap-2 border-t border-zinc-100 px-5 py-3">
          <button
            @click="emit('close')"
            class="rounded-lg border border-zinc-200 px-4 py-2 text-sm hover:bg-zinc-50"
          >
            取消
          </button>
          <button
            @click="emit('confirm')"
            class="rounded-lg bg-red-700 px-4 py-2 text-sm text-white hover:bg-red-800"
          >
            确定
          </button>
        </div>
      </div>
    </div>
  </Teleport>
</template>
