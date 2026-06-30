<script setup lang="ts">
import { useRoute, useRouter } from 'vue-router'
import { computed } from 'vue'
import { LayoutDashboard, Package, Pen, ScrollText, Send, Bug, Settings, type LucideIcon } from 'lucide-vue-next'

interface NavItem {
  path: string
  label: string
  icon: LucideIcon
}

const navItems: NavItem[] = [
  { path: '/dashboard', label: '仪表盘', icon: LayoutDashboard },
  { path: '/plugins', label: '插件管理', icon: Package },
  { path: '/editor', label: '脚本编辑', icon: Pen },
  { path: '/logs', label: '运行日志', icon: ScrollText },
  { path: '/send', label: '发送消息', icon: Send },
  { path: '/debug', label: '调试', icon: Bug },
  { path: '/settings', label: '系统设置', icon: Settings },
]

const route = useRoute()
const router = useRouter()

const activePath = computed(() => route.path)

function navigate(path: string) {
  router.push(path)
}
</script>

<template>
  <aside class="flex w-56 shrink-0 flex-col bg-zinc-900 text-zinc-300">
    <div class="border-b border-zinc-700 px-5 py-5">
      <div class="flex items-center gap-2.5">
        <span class="text-xl">🔴</span>
        <div>
          <div class="text-sm font-bold text-white">LangLang</div>
          <div class="text-[10px] text-zinc-500">管理控制台</div>
        </div>
      </div>
    </div>

    <nav class="flex-1 space-y-0.5 px-2.5 py-3">
      <button
        v-for="item in navItems"
        :key="item.path"
        @click="navigate(item.path)"
        :class="[
          'flex w-full items-center gap-3 rounded-lg px-3 py-2.5 text-sm transition-colors',
          activePath === item.path
            ? 'bg-red-700 text-white'
            : 'text-zinc-400 hover:bg-zinc-800 hover:text-zinc-200'
        ]"
      >
        <component :is="item.icon" :size="18" />
        <span>{{ item.label }}</span>
      </button>
    </nav>

    <div class="border-t border-zinc-700 px-5 py-3">
      <div class="flex items-center gap-2 text-xs text-zinc-500">
        <span class="h-2 w-2 rounded-full bg-green-500" />
        <span>v0.2.0</span>
      </div>
    </div>
  </aside>
</template>
