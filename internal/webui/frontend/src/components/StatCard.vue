<script setup lang="ts">
defineProps<{
  label: string
  value: string | number
}>()
</script>

<template>
  <div class="rounded-xl border border-zinc-200 bg-white p-5 shadow-sm">
    <div class="text-2xl font-bold text-red-700">{{ value }}</div>
    <div class="mt-1 text-sm text-zinc-500">{{ label }}</div>
  </div>
</template>
