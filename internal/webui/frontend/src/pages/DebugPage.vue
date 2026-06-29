<script setup lang="ts">
import { ref, watch, onMounted } from 'vue'
import { debugMessage } from '../api/debug'
import { useBotsStore } from '../stores/bots'
import { useToast } from '../composables/useToast'

const botsStore = useBotsStore()
const toast = useToast()

const LS_KEY_DEBUG = 'debug_form'
const LS_KEY_HISTORY = 'debug_history'

function loadDebugForm() {
  try {
    const saved = localStorage.getItem(LS_KEY_DEBUG)
    if (saved) return JSON.parse(saved)
  } catch { /* ignore */ }
  return {}
}

const savedForm = loadDebugForm()
const platform = ref(savedForm.platform ?? 'debug')
const messageType = ref(savedForm.messageType ?? 'private')
const userId = ref(savedForm.userId ?? 'debug_user')
const groupId = ref(savedForm.groupId ?? 'debug_group')
const message = ref('')
const sending = ref(false)

watch([platform, messageType, userId, groupId], () => {
  localStorage.setItem(LS_KEY_DEBUG, JSON.stringify({
    platform: platform.value,
    messageType: messageType.value,
    userId: userId.value,
    groupId: groupId.value,
  }))
}, { deep: true })

interface HistoryItem {
  platform: string
  messageType: string
  userId: string
  groupId: string
  msgPreview: string
  time: string
}

function loadHistory(): HistoryItem[] {
  try {
    const saved = localStorage.getItem(LS_KEY_HISTORY)
    if (saved) return JSON.parse(saved)
  } catch { /* ignore */ }
  return []
}

const history = ref<HistoryItem[]>(loadHistory())

watch(history, (v) => {
  // 只保留最近 50 条
  const trimmed = v.slice(-50)
  localStorage.setItem(LS_KEY_HISTORY, JSON.stringify(trimmed))
  if (trimmed.length < v.length) {
    history.value = trimmed
  }
}, { deep: true })

onMounted(() => botsStore.fetchStatus())

async function send() {
  if (!message.value.trim()) {
    toast.show('请输入消息内容', 'error')
    return
  }
  sending.value = true
  try {
    await debugMessage(platform.value, message.value.trim(), userId.value.trim(), groupId.value.trim(), messageType.value)
    toast.show('✅ 调试消息已发送', 'success')

    history.value.push({
      platform: platform.value,
      messageType: messageType.value,
      userId: userId.value.trim(),
      groupId: groupId.value.trim(),
      msgPreview: message.value.length > 40 ? message.value.slice(0, 40) + '...' : message.value,
      time: new Date().toLocaleTimeString(),
    })
    message.value = ''
  } catch (e: any) {
    toast.show(e.message || '发送失败', 'error')
  } finally {
    sending.value = false
  }
}

const platformLabels: Record<string, string> = {
  onebot11: 'OneBot11',
  telegram: 'Telegram',
  satori: 'Satori',
  debug: 'Debug（通用）',
}

// 系统支持的所有平台（用于调试页的选项列表）
const KNOWN_PLATFORMS = ['onebot11', 'telegram', 'satori', 'debug']
</script>

<template>
  <div class="grid grid-cols-1 gap-6 lg:grid-cols-2">
    <!-- Form -->
    <div class="rounded-xl border border-zinc-200 bg-white shadow-sm">
      <div class="border-b border-zinc-100 px-5 py-3">
        <h3 class="text-sm font-semibold">🐛 本地调试</h3>
      </div>
      <div class="space-y-4 p-5">
        <div>
          <label class="mb-1 block text-xs font-medium text-zinc-500">模拟平台</label>
          <select v-model="platform" class="w-full rounded-lg border border-zinc-200 px-3 py-2 text-sm outline-none focus:border-red-400">
            <option v-for="p in KNOWN_PLATFORMS" :key="p" :value="p">
              {{ platformLabels[p] || p }}
            </option>
          </select>
        </div>
        <div>
          <label class="mb-1 block text-xs font-medium text-zinc-500">消息类型</label>
          <select v-model="messageType" class="w-full rounded-lg border border-zinc-200 px-3 py-2 text-sm outline-none focus:border-red-400">
            <option value="private">私聊 (private)</option>
            <option value="group">群组 (group)</option>
          </select>
        </div>
        <div class="grid grid-cols-2 gap-3">
          <div>
            <label class="mb-1 block text-xs font-medium text-zinc-500">用户 ID</label>
            <input v-model="userId" class="w-full rounded-lg border border-zinc-200 px-3 py-2 text-sm outline-none focus:border-red-400" />
          </div>
          <div>
            <label class="mb-1 block text-xs font-medium text-zinc-500">群组 ID</label>
            <input v-model="groupId" class="w-full rounded-lg border border-zinc-200 px-3 py-2 text-sm outline-none focus:border-red-400" />
          </div>
        </div>
        <div>
          <label class="mb-1 block text-xs font-medium text-zinc-500">消息内容</label>
          <textarea
            v-model="message"
            rows="5"
            placeholder="【输出】@你好世界"
            class="w-full rounded-lg border border-zinc-200 px-3 py-2 text-sm font-mono outline-none focus:border-red-400 resize-none"
          />
        </div>
        <button
          @click="send"
          :disabled="sending"
          class="rounded-lg bg-red-700 px-5 py-2 text-sm text-white hover:bg-red-800 disabled:opacity-50"
        >
          {{ sending ? '发送中...' : '🚀 发送调试消息' }}
        </button>
      </div>
    </div>

    <!-- History -->
    <div class="rounded-xl border border-zinc-200 bg-white shadow-sm">
      <div class="flex items-center justify-between border-b border-zinc-100 px-5 py-3">
        <h3 class="text-sm font-semibold">📋 发送历史</h3>
        <button @click="history = []" class="rounded-lg border border-zinc-200 px-3 py-1 text-xs hover:bg-zinc-50">
          清除
        </button>
      </div>
      <div class="p-5">
        <div v-if="history.length === 0" class="py-8 text-center text-sm text-zinc-400">
          暂无发送记录
        </div>
        <div v-else class="space-y-3">
          <div
            v-for="(item, i) in [...history].reverse()"
            :key="i"
            class="rounded-lg border border-zinc-100 p-3"
          >
            <div class="flex items-center gap-2">
              <span class="rounded bg-red-100 px-1.5 py-0.5 text-[10px] font-medium text-red-700">
                {{ item.platform }}
              </span>
              <span class="rounded bg-zinc-100 px-1.5 py-0.5 text-[10px] font-medium text-zinc-600">
                {{ item.messageType }}
              </span>
            </div>
            <div class="mt-1.5 font-mono text-xs text-zinc-700">{{ item.msgPreview }}</div>
            <div class="mt-1 text-[10px] text-zinc-400">
              user={{ item.userId }} group={{ item.groupId }} · {{ item.time }}
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
