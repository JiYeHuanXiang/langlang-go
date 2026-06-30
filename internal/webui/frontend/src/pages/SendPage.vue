<script setup lang="ts">
import { ref, watch, onMounted, computed } from 'vue'
import { sendBotMessage } from '../api/send'
import { useBotsStore } from '../stores/bots'
import { useToast } from '../composables/useToast'
import { Send, Clock, Trash2, MessageSquare } from 'lucide-vue-next'

const botsStore = useBotsStore()
const toast = useToast()

const LS_KEY_FORM = 'send_form'
const LS_KEY_HISTORY = 'send_history'

function loadForm() {
  try {
    const saved = localStorage.getItem(LS_KEY_FORM)
    if (saved) return JSON.parse(saved)
  } catch { /* ignore */ }
  return {}
}

const savedForm = loadForm()
const selectedBot = ref(savedForm.selectedBot ?? '')
const targetType = ref<'private' | 'group'>(savedForm.targetType ?? 'group')
const targetId = ref(savedForm.targetId ?? '')
const message = ref('')
const sending = ref(false)

// 在线机器人列表
const onlineBots = computed(() => botsStore.bots.filter(b => b.running))

// 当前选中的机器人
const currentBot = computed(() => {
  if (!selectedBot.value) return null
  return onlineBots.value.find(b => `${b.platform}:${b.self_id}` === selectedBot.value)
})

// 平台标签
const platformLabels: Record<string, string> = {
  onebot11: 'OneBot11',
  telegram: 'Telegram',
  satori: 'Satori',
}

watch([selectedBot, targetType, targetId], () => {
  localStorage.setItem(LS_KEY_FORM, JSON.stringify({
    selectedBot: selectedBot.value,
    targetType: targetType.value,
    targetId: targetId.value,
  }))
}, { deep: true })

interface HistoryItem {
  platform: string
  selfId: string
  targetType: string
  targetId: string
  msgPreview: string
  time: string
  success: boolean
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
  const trimmed = v.slice(-50)
  localStorage.setItem(LS_KEY_HISTORY, JSON.stringify(trimmed))
  if (trimmed.length < v.length) {
    history.value = trimmed
  }
}, { deep: true })

onMounted(async () => {
  await botsStore.fetchStatus()
  // 自动选择第一个在线机器人
  if (!selectedBot.value && onlineBots.value.length > 0) {
    const bot = onlineBots.value[0]
    selectedBot.value = `${bot.platform}:${bot.self_id}`
  }
})

async function send() {
  if (!selectedBot.value) {
    toast.show('请选择机器人', 'error')
    return
  }
  if (!targetId.value.trim()) {
    toast.show('请输入目标 ID', 'error')
    return
  }
  if (!message.value.trim()) {
    toast.show('请输入消息内容', 'error')
    return
  }

  const bot = currentBot.value
  if (!bot) {
    toast.show('所选机器人不在线', 'error')
    return
  }

  sending.value = true
  try {
    await sendBotMessage({
      platform: bot.platform,
      self_id: bot.self_id,
      target_type: targetType.value,
      target_id: targetId.value.trim(),
      message: message.value.trim(),
    })
    toast.show('消息已发送', 'success')

    history.value.push({
      platform: bot.platform,
      selfId: bot.self_id,
      targetType: targetType.value,
      targetId: targetId.value.trim(),
      msgPreview: message.value.length > 40 ? message.value.slice(0, 40) + '...' : message.value,
      time: new Date().toLocaleTimeString(),
      success: true,
    })
    message.value = ''
  } catch (e: any) {
    toast.show(e.message || '发送失败', 'error')
    history.value.push({
      platform: bot.platform,
      selfId: bot.self_id,
      targetType: targetType.value,
      targetId: targetId.value.trim(),
      msgPreview: message.value.length > 40 ? message.value.slice(0, 40) + '...' : message.value,
      time: new Date().toLocaleTimeString(),
      success: false,
    })
  } finally {
    sending.value = false
  }
}
</script>

<template>
  <div class="grid grid-cols-1 gap-6 lg:grid-cols-2">
    <!-- 发送表单 -->
    <div class="rounded-xl border border-zinc-200 bg-white shadow-sm">
      <div class="border-b border-zinc-100 px-5 py-3">
        <h3 class="flex items-center gap-2 text-sm font-semibold">
          <MessageSquare :size="16" class="text-red-600" />
          发送消息
        </h3>
      </div>
      <div class="space-y-4 p-5">
        <!-- 机器人选择 -->
        <div>
          <label class="mb-1 block text-xs font-medium text-zinc-500">选择机器人</label>
          <select
            v-model="selectedBot"
            class="w-full rounded-lg border border-zinc-200 px-3 py-2 text-sm outline-none focus:border-red-400"
          >
            <option value="" disabled>-- 请选择 --</option>
            <option v-for="bot in onlineBots" :key="`${bot.platform}:${bot.self_id}`" :value="`${bot.platform}:${bot.self_id}`">
              {{ platformLabels[bot.platform] || bot.platform }} - {{ bot.self_id }}
            </option>
          </select>
          <div v-if="onlineBots.length === 0" class="mt-1 text-xs text-amber-600">
            没有在线的机器人，请先在系统设置中添加并启动机器人连接
          </div>
        </div>

        <!-- 目标类型 -->
        <div>
          <label class="mb-1 block text-xs font-medium text-zinc-500">目标类型</label>
          <div class="flex gap-2">
            <button
              @click="targetType = 'private'"
              :class="[
                'flex-1 rounded-lg border px-3 py-2 text-sm transition-colors',
                targetType === 'private'
                  ? 'border-red-300 bg-red-50 text-red-700'
                  : 'border-zinc-200 text-zinc-600 hover:bg-zinc-50'
              ]"
            >
              私聊
            </button>
            <button
              @click="targetType = 'group'"
              :class="[
                'flex-1 rounded-lg border px-3 py-2 text-sm transition-colors',
                targetType === 'group'
                  ? 'border-red-300 bg-red-50 text-red-700'
                  : 'border-zinc-200 text-zinc-600 hover:bg-zinc-50'
              ]"
            >
              群组
            </button>
          </div>
        </div>

        <!-- 目标 ID -->
        <div>
          <label class="mb-1 block text-xs font-medium text-zinc-500">
            {{ targetType === 'group' ? '群组 ID' : '用户 ID' }}
          </label>
          <input
            v-model="targetId"
            :placeholder="targetType === 'group' ? '输入群组 ID' : '输入用户 ID'"
            class="w-full rounded-lg border border-zinc-200 px-3 py-2 text-sm outline-none focus:border-red-400"
          />
        </div>

        <!-- 消息内容 -->
        <div>
          <label class="mb-1 block text-xs font-medium text-zinc-500">消息内容</label>
          <textarea
            v-model="message"
            rows="6"
            placeholder="输入要发送的消息..."
            class="w-full resize-none rounded-lg border border-zinc-200 px-3 py-2 font-mono text-sm outline-none focus:border-red-400"
            @keydown.ctrl.enter="send"
          />
          <div class="mt-1 text-xs text-zinc-400">按 Ctrl+Enter 快速发送</div>
        </div>

        <!-- 发送按钮 -->
        <button
          @click="send"
          :disabled="sending || !selectedBot || !targetId || !message.trim()"
          class="flex w-full items-center justify-center gap-2 rounded-lg bg-red-700 px-5 py-2.5 text-sm text-white hover:bg-red-800 disabled:opacity-50"
        >
          <Send :size="16" />
          {{ sending ? '发送中...' : '发送' }}
        </button>
      </div>
    </div>

    <!-- 发送历史 -->
    <div class="rounded-xl border border-zinc-200 bg-white shadow-sm">
      <div class="flex items-center justify-between border-b border-zinc-100 px-5 py-3">
        <h3 class="flex items-center gap-2 text-sm font-semibold">
          <Clock :size="16" class="text-zinc-500" />
          发送历史
        </h3>
        <button
          v-if="history.length > 0"
          @click="history = []"
          class="flex items-center gap-1 rounded-lg border border-zinc-200 px-3 py-1 text-xs hover:bg-zinc-50"
        >
          <Trash2 :size="12" />
          清除
        </button>
      </div>
      <div class="max-h-[600px] overflow-auto p-5">
        <div v-if="history.length === 0" class="py-8 text-center text-sm text-zinc-400">
          暂无发送记录
        </div>
        <div v-else class="space-y-3">
          <div
            v-for="(item, i) in [...history].reverse()"
            :key="i"
            :class="[
              'rounded-lg border p-3',
              item.success ? 'border-zinc-100' : 'border-red-100 bg-red-50/30'
            ]"
          >
            <div class="flex items-center gap-2">
              <span class="rounded bg-red-100 px-1.5 py-0.5 text-[10px] font-medium text-red-700">
                {{ platformLabels[item.platform] || item.platform }}
              </span>
              <span :class="[
                'rounded px-1.5 py-0.5 text-[10px] font-medium',
                item.targetType === 'group' ? 'bg-blue-100 text-blue-700' : 'bg-green-100 text-green-700'
              ]">
                {{ item.targetType === 'group' ? '群组' : '私聊' }}
              </span>
              <span v-if="!item.success" class="rounded bg-red-200 px-1.5 py-0.5 text-[10px] font-medium text-red-800">
                失败
              </span>
            </div>
            <div class="mt-1.5 font-mono text-xs text-zinc-700">{{ item.msgPreview }}</div>
            <div class="mt-1 text-[10px] text-zinc-400">
              → {{ item.targetId }} · {{ item.time }}
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
