<script setup lang="ts">
import { onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useBotsStore } from '../stores/bots'
import { useConfigStore } from '../stores/config'
import { usePluginsStore } from '../stores/plugins'
import { useToast } from '../composables/useToast'
import { botControl } from '../api/bots'
import StatCard from '../components/StatCard.vue'

const router = useRouter()
const botsStore = useBotsStore()
const configStore = useConfigStore()
const pluginsStore = usePluginsStore()
const toast = useToast()

onMounted(async () => {
  await Promise.all([
    botsStore.fetchStatus(),
    pluginsStore.fetchAll(),
  ])
})

async function toggleBot(platform: string, selfId: string, running: boolean) {
  const action = running ? 'stop' : 'start'
  try {
    await botControl(action, platform, selfId)
    toast.show(action === 'stop' ? '已发送停止指令' : '已发送启动指令', 'success')
    setTimeout(() => botsStore.fetchStatus(), 1500)
  } catch (e: any) {
    toast.show(e.message || '操作失败', 'error')
  }
}

async function toggleTestMode() {
  try {
    const newVal = !configStore.testMode
    await configStore.toggleTestMode(newVal)
    toast.show(newVal ? '测试模式已开启' : '测试模式已关闭', 'info')
  } catch (e: any) {
    toast.show(e.message || '切换失败', 'error')
  }
}
</script>

<template>
  <div class="space-y-6">
    <!-- Stats -->
    <div class="grid grid-cols-2 gap-4 lg:grid-cols-4">
      <StatCard :value="pluginsStore.plugins.length" label="📦 插件总数" />
      <StatCard
        :value="pluginsStore.plugins.filter(p => p.enabled).length"
        label="✅ 启用中"
      />
      <StatCard :value="botsStore.version" label="🏷️ 版本号" />
      <StatCard :value="botsStore.uptime" label="⏱️ 运行时间" />
    </div>

    <!-- Test Mode -->
    <div class="rounded-xl border border-zinc-200 bg-white p-5 shadow-sm">
      <div class="flex items-center justify-between">
        <div>
          <h3 class="text-sm font-semibold">🧪 测试模式</h3>
          <p class="mt-1 text-xs text-zinc-500">
            开启后收到消息不向外发送，仅在日志界面输出结果
          </p>
        </div>
        <button
          @click="toggleTestMode"
          :class="configStore.testMode
            ? 'bg-red-600 hover:bg-red-700'
            : 'bg-zinc-400 hover:bg-zinc-500'"
          class="relative inline-flex h-6 w-11 items-center rounded-full transition-colors"
        >
          <span
            :class="configStore.testMode ? 'translate-x-6' : 'translate-x-1'"
            class="inline-block h-4 w-4 rounded-full bg-white transition-transform"
          />
        </button>
      </div>
    </div>

    <!-- Bot Connections -->
    <div class="rounded-xl border border-zinc-200 bg-white shadow-sm">
      <div class="flex items-center justify-between border-b border-zinc-100 px-5 py-3">
        <h3 class="text-sm font-semibold">🔗 机器人连接</h3>
        <button
          @click="router.push('/settings')"
          class="rounded-lg border border-zinc-200 px-3 py-1 text-xs hover:bg-zinc-50"
        >
          设置
        </button>
      </div>
      <div class="p-5">
        <div v-if="botsStore.bots.length === 0" class="py-8 text-center text-sm text-zinc-400">
          暂无已配置的机器人连接
        </div>
        <table v-else class="w-full text-sm">
          <thead>
            <tr class="border-b border-zinc-100 text-left text-xs text-zinc-400">
              <th class="pb-2 font-medium">平台</th>
              <th class="pb-2 font-medium">ID</th>
              <th class="pb-2 font-medium">状态</th>
              <th class="pb-2 font-medium">操作</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="bot in botsStore.bots" :key="bot.platform + bot.self_id" class="border-b border-zinc-50">
              <td class="py-2.5 font-medium">{{ bot.platform }}</td>
              <td class="py-2.5 font-mono text-xs text-zinc-500">{{ bot.self_id || '-' }}</td>
              <td class="py-2.5">
                <span
                  :class="bot.running ? 'bg-green-100 text-green-700' : 'bg-red-100 text-red-700'"
                  class="rounded-full px-2 py-0.5 text-xs font-medium"
                >
                  {{ bot.running ? '● 运行中' : '● 已停止' }}
                </span>
              </td>
              <td class="py-2.5">
                <button
                  @click="toggleBot(bot.platform, bot.self_id, bot.running)"
                  :class="bot.running
                    ? 'border-red-200 text-red-600 hover:bg-red-50'
                    : 'border-green-200 text-green-600 hover:bg-green-50'"
                  class="rounded-lg border px-3 py-1 text-xs font-medium transition-colors"
                >
                  {{ bot.running ? '停止' : '启动' }}
                </button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <!-- Recent Plugins -->
    <div class="rounded-xl border border-zinc-200 bg-white shadow-sm">
      <div class="border-b border-zinc-100 px-5 py-3">
        <h3 class="text-sm font-semibold">📋 最近插件</h3>
      </div>
      <div class="p-5">
        <div v-if="pluginsStore.plugins.length === 0" class="py-8 text-center text-sm text-zinc-400">
          暂无插件
        </div>
        <table v-else class="w-full text-sm">
          <thead>
            <tr class="border-b border-zinc-100 text-left text-xs text-zinc-400">
              <th class="pb-2 font-medium">名称</th>
              <th class="pb-2 font-medium">状态</th>
              <th class="pb-2 font-medium">更新</th>
              <th class="pb-2 font-medium" />
            </tr>
          </thead>
          <tbody>
            <tr v-for="p in pluginsStore.plugins.slice(0, 5)" :key="p.name" class="border-b border-zinc-50">
              <td class="py-2.5 font-medium">{{ p.name }}</td>
              <td class="py-2.5">
                <span
                  :class="p.enabled ? 'bg-green-100 text-green-700' : 'bg-red-100 text-red-700'"
                  class="rounded-full px-2 py-0.5 text-xs font-medium"
                >
                  {{ p.enabled ? '启用' : '禁用' }}
                </span>
              </td>
              <td class="py-2.5 text-xs text-zinc-400">
                {{ p.updated_at ? new Date(p.updated_at).toLocaleString() : '-' }}
              </td>
              <td class="py-2.5 text-right">
                <button
                  @click="router.push(`/editor?name=${encodeURIComponent(p.name)}`)"
                  class="text-xs font-medium text-red-700 hover:underline"
                >
                  编辑
                </button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
  </div>
</template>
