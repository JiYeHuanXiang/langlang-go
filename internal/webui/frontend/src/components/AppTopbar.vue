<script setup lang="ts">
import { useRoute } from 'vue-router'
import { computed } from 'vue'

const pageTitles: Record<string, string> = {
  '/dashboard': '仪表盘',
  '/plugins': '插件管理',
  '/editor': '脚本编辑',
  '/logs': '运行日志',
  '/send': '发送消息',
  '/debug': '调试',
  '/settings': '系统设置',
}

const route = useRoute()
const title = computed(() => pageTitles[route.path] || 'LangLang-Go')
</script>

<template>
  <header class="flex h-14 shrink-0 items-center border-b border-zinc-200 bg-white px-6">
    <h1 class="text-base font-semibold text-red-700">{{ title }}</h1>
    <div class="ml-auto text-xs text-zinc-400">LangLang-Go</div>
  </header>
</template>
