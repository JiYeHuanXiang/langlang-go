<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useConfigStore } from '../stores/config'
import { useToast } from '../composables/useToast'

const configStore = useConfigStore()
const toast = useToast()

const saving = ref(false)

const form = ref({
  listen: ':2397',
  accessToken: '',
  logLevel: 'info',
  skipMsgMinutes: 10,
  dataDir: 'data',
})

onMounted(async () => {
  await configStore.fetchConfig()
  const cfg = configStore.config
  form.value.listen = cfg.web?.listen || ':2397'
  form.value.accessToken = cfg.web?.access_token || ''
  form.value.logLevel = cfg.log?.level || 'info'
  form.value.skipMsgMinutes = cfg.core?.skip_msg_minutes || 10
  form.value.dataDir = cfg.paths?.data || 'data'
})

async function save() {
  saving.value = true
  try {
    await configStore.save({
      web: { listen: form.value.listen, access_token: form.value.accessToken },
      log: { level: form.value.logLevel },
      core: { skip_msg_minutes: Number(form.value.skipMsgMinutes) },
      paths: { data: form.value.dataDir },
    })
    toast.show('配置已保存', 'success')
  } catch (e: any) {
    toast.show(e.message || '保存失败', 'error')
  } finally {
    saving.value = false
  }
}

async function toggleTestMode() {
  try {
    const newVal = !configStore.testMode
    await configStore.toggleTestMode(newVal)
    toast.show(newVal ? '测试模式已开启' : '测试模式已关闭', 'info')
  } catch (e: any) {
    toast.show(e.message || '操作失败', 'error')
  }
}
</script>

<template>
  <div class="max-w-lg space-y-6">
    <div class="rounded-xl border border-zinc-200 bg-white p-5 shadow-sm">
      <h3 class="mb-4 text-sm font-semibold">⚙️ 系统设置</h3>

      <div class="space-y-4">
        <div>
          <label class="mb-1 block text-xs font-medium text-zinc-500">Web 监听地址</label>
          <input
            v-model="form.listen"
            class="w-full rounded-lg border border-zinc-200 px-3 py-2 text-sm font-mono outline-none focus:border-red-400"
          />
        </div>

        <div>
          <label class="mb-1 block text-xs font-medium text-zinc-500">访问令牌（留空=无鉴权）</label>
          <input
            v-model="form.accessToken"
            class="w-full rounded-lg border border-zinc-200 px-3 py-2 text-sm font-mono outline-none focus:border-red-400"
            placeholder="留空则不设密码"
          />
        </div>

        <div class="flex items-center justify-between">
          <div>
            <label class="text-xs font-medium text-zinc-500">测试模式</label>
            <p class="text-[11px] text-zinc-400">开启后不向外发送消息</p>
          </div>
          <button
            @click="toggleTestMode"
            :class="configStore.testMode
              ? 'bg-red-600 hover:bg-red-700'
              : 'bg-zinc-400 hover:bg-zinc-500'"
            class="relative inline-flex h-6 w-11 items-center rounded-full transition-colors shrink-0"
          >
            <span
              :class="configStore.testMode ? 'translate-x-6' : 'translate-x-1'"
              class="inline-block h-4 w-4 rounded-full bg-white transition-transform"
            />
          </button>
        </div>

        <div>
          <label class="mb-1 block text-xs font-medium text-zinc-500">日志级别</label>
          <select v-model="form.logLevel" class="w-full rounded-lg border border-zinc-200 px-3 py-2 text-sm outline-none focus:border-red-400">
            <option value="debug">DEBUG</option>
            <option value="info">INFO</option>
            <option value="warn">WARN</option>
            <option value="error">ERROR</option>
          </select>
        </div>

        <div>
          <label class="mb-1 block text-xs font-medium text-zinc-500">跳过 N 分钟前的消息</label>
          <input
            v-model.number="form.skipMsgMinutes"
            type="number"
            class="w-full rounded-lg border border-zinc-200 px-3 py-2 text-sm outline-none focus:border-red-400"
          />
        </div>

        <div>
          <label class="mb-1 block text-xs font-medium text-zinc-500">数据目录</label>
          <input
            v-model="form.dataDir"
            class="w-full rounded-lg border border-zinc-200 px-3 py-2 text-sm font-mono outline-none focus:border-red-400"
          />
        </div>

        <button
          @click="save"
          :disabled="saving"
          class="rounded-lg bg-red-700 px-5 py-2 text-sm text-white hover:bg-red-800 disabled:opacity-50"
        >
          {{ saving ? '保存中...' : '💾 保存设置' }}
        </button>
      </div>
    </div>
  </div>
</template>
