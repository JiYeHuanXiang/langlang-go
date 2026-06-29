<script setup lang="ts">
import { onMounted, ref, watch } from 'vue'
import { useConfigStore } from '../stores/config'
import { useToast } from '../composables/useToast'
import { saveConfig } from '../api/config'

interface BotConn {
  platform: 'onebot11' | 'telegram' | 'satori'
  mode?: string       // onebot11: reverse/forward
  url?: string
  access_token?: string
  self_id?: string
  token?: string      // telegram
  api_url?: string    // satori
}

const configStore = useConfigStore()
const toast = useToast()

const saving = ref(false)
const savingBot = ref(false)

const LS_KEY_RETENTION = 'logs_retention_minutes'

function loadRetention(): number {
  try {
    const saved = localStorage.getItem(LS_KEY_RETENTION)
    if (saved !== null) return Number(saved)
  } catch { /* ignore */ }
  return 0
}

const logsRetention = ref(loadRetention())

watch(logsRetention, (v) => {
  localStorage.setItem(LS_KEY_RETENTION, String(v))
})

const form = ref({
  listen: ':2397',
  accessToken: '',
  logLevel: 'info',
  skipMsgMinutes: 10,
  dataDir: 'data',
})

const botConns = ref<BotConn[]>([])

function loadBotConfig(cfg: Record<string, any>) {
  botConns.value = []
  const bot = cfg.bot || {}
  if (Array.isArray(bot.onebot11)) {
    for (const ob of bot.onebot11) {
      botConns.value.push({ platform: 'onebot11', mode: ob.mode || 'reverse', url: ob.url || '', access_token: ob.access_token || '', self_id: ob.self_id || '' })
    }
  }
  if (Array.isArray(bot.telegram)) {
    for (const t of bot.telegram) {
      botConns.value.push({ platform: 'telegram', token: t })
    }
  }
  if (Array.isArray(bot.satori)) {
    for (const s of bot.satori) {
      botConns.value.push({ platform: 'satori', url: s.url || '', token: s.token || '', self_id: s.self_id || '', api_url: s.api_url || '' })
    }
  }
}

onMounted(async () => {
  await configStore.fetchConfig()
  const cfg = configStore.config
  form.value.listen = cfg.web?.listen || ':2397'
  form.value.accessToken = cfg.web?.access_token || ''
  form.value.logLevel = cfg.log?.level || 'info'
  form.value.skipMsgMinutes = cfg.core?.skip_msg_minutes || 10
  form.value.dataDir = cfg.paths?.data || 'data'
  loadBotConfig(cfg)
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

function addBot() {
  botConns.value.push({ platform: 'onebot11', mode: 'reverse', url: '', access_token: '', self_id: '' })
}

function removeBot(idx: number) {
  botConns.value.splice(idx, 1)
}

async function saveBot() {
  savingBot.value = true
  try {
    const onebot11: { mode?: string; url: string; access_token: string; self_id: string }[] = []
    const telegram: string[] = []
    const satori: { url: string; token: string; self_id: string; api_url: string }[] = []
    for (const conn of botConns.value) {
      if (conn.platform === 'onebot11') {
        onebot11.push({ mode: conn.mode || 'reverse', url: conn.url || '', access_token: conn.access_token || '', self_id: conn.self_id || '' })
      } else if (conn.platform === 'telegram') {
        if (conn.token) telegram.push(conn.token)
      } else if (conn.platform === 'satori') {
        satori.push({ url: conn.url || '', token: conn.token || '', self_id: conn.self_id || '', api_url: conn.api_url || '' })
      }
    }
    await saveConfig({
      bot: {
        onebot11: onebot11.length > 0 ? onebot11 : null,
        telegram: telegram.length > 0 ? telegram : null,
        satori: satori.length > 0 ? satori : null,
      },
    })
    toast.show('机器人配置已保存', 'success')
  } catch (e: any) {
    toast.show(e.message || '保存失败', 'error')
  } finally {
    savingBot.value = false
  }
}
</script>

<template>
  <div class="max-w-2xl space-y-6">
    <!-- 系统设置 -->
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

        <div>
          <label class="mb-1 block text-xs font-medium text-zinc-500">日志保存时间范围</label>
          <select v-model="logsRetention" class="w-full rounded-lg border border-zinc-200 px-3 py-2 text-sm outline-none focus:border-red-400">
            <option :value="0">全部</option>
            <option :value="5">5 分钟</option>
            <option :value="15">15 分钟</option>
            <option :value="30">30 分钟</option>
            <option :value="60">1 小时</option>
            <option :value="120">2 小时</option>
          </select>
          <p class="mt-1 text-[11px] text-zinc-400">手动保存或自动保存时只保留此时间范围内的日志</p>
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

    <!-- 机器人连接配置 -->
    <div class="rounded-xl border border-zinc-200 bg-white p-5 shadow-sm">
      <div class="mb-4 flex items-center justify-between">
        <h3 class="text-sm font-semibold">🤖 机器人连接</h3>
        <button
          @click="addBot"
          class="rounded-lg border border-zinc-200 px-3 py-1 text-xs hover:bg-zinc-50"
        >
          ➕ 添加连接
        </button>
      </div>

      <div v-if="botConns.length === 0" class="py-8 text-center text-sm text-zinc-400">
        暂无连接配置，点击「添加连接」新增
      </div>

      <div v-else class="space-y-4">
        <div
          v-for="(conn, idx) in botConns"
          :key="idx"
          class="rounded-lg border border-zinc-100 bg-zinc-50/50 p-4"
        >
          <div class="mb-3 flex items-center justify-between">
            <select
              v-model="conn.platform"
              class="rounded-lg border border-zinc-200 px-3 py-1.5 text-xs outline-none focus:border-red-400"
            >
              <option value="onebot11">OneBot 11</option>
              <option value="telegram">Telegram</option>
              <option value="satori">Satori</option>
            </select>
            <button
              @click="removeBot(idx)"
              class="text-xs text-red-500 hover:text-red-700"
            >
              ✕ 删除
            </button>
          </div>

          <!-- OneBot 11 字段 -->
          <template v-if="conn.platform === 'onebot11'">
            <div class="mb-2">
              <label class="mb-1 block text-[11px] font-medium text-zinc-500">连接模式</label>
              <select
                v-model="conn.mode"
                class="w-full rounded-lg border border-zinc-200 px-3 py-1.5 text-xs outline-none focus:border-red-400"
              >
                <option value="reverse">反向 WS（连接远程服务器）</option>
                <option value="forward">正向 WS（等待客户端连接）</option>
              </select>
            </div>
            <div class="mb-2">
              <label class="mb-1 block text-[11px] font-medium text-zinc-500">{{ conn.mode === 'forward' ? '监听地址' : 'WebSocket 地址' }}</label>
              <input
                v-model="conn.url"
                :placeholder="conn.mode === 'forward' ? ':6700' : 'ws://127.0.0.1:6700'"
                class="w-full rounded-lg border border-zinc-200 px-3 py-1.5 text-xs font-mono outline-none focus:border-red-400"
              />
            </div>
            <div class="mb-2">
              <label class="mb-1 block text-[11px] font-medium text-zinc-500">Access Token</label>
              <input
                v-model="conn.access_token"
                placeholder="留空则无鉴权"
                class="w-full rounded-lg border border-zinc-200 px-3 py-1.5 text-xs font-mono outline-none focus:border-red-400"
              />
            </div>
            <div>
              <label class="mb-1 block text-[11px] font-medium text-zinc-500">Self ID（机器人 QQ 号）</label>
              <input
                v-model="conn.self_id"
                placeholder="留空则自动获取"
                class="w-full rounded-lg border border-zinc-200 px-3 py-1.5 text-xs font-mono outline-none focus:border-red-400"
              />
            </div>
          </template>

          <!-- Telegram 字段 -->
          <template v-if="conn.platform === 'telegram'">
            <div>
              <label class="mb-1 block text-[11px] font-medium text-zinc-500">Bot Token</label>
              <input
                v-model="conn.token"
                placeholder="123456:ABCdefGHIjklmNOPqrstUVwxyz"
                class="w-full rounded-lg border border-zinc-200 px-3 py-1.5 text-xs font-mono outline-none focus:border-red-400"
              />
            </div>
          </template>

          <!-- Satori 字段 -->
          <template v-if="conn.platform === 'satori'">
            <div class="mb-2">
              <label class="mb-1 block text-[11px] font-medium text-zinc-500">WebSocket 地址</label>
              <input
                v-model="conn.url"
                placeholder="ws://127.0.0.1:5500"
                class="w-full rounded-lg border border-zinc-200 px-3 py-1.5 text-xs font-mono outline-none focus:border-red-400"
              />
            </div>
            <div class="mb-2">
              <label class="mb-1 block text-[11px] font-medium text-zinc-500">Token</label>
              <input
                v-model="conn.token"
                placeholder="鉴权令牌"
                class="w-full rounded-lg border border-zinc-200 px-3 py-1.5 text-xs font-mono outline-none focus:border-red-400"
              />
            </div>
            <div class="mb-2">
              <label class="mb-1 block text-[11px] font-medium text-zinc-500">REST API 地址（可选）</label>
              <input
                v-model="conn.api_url"
                placeholder="留空则从 WS 地址推导 http://127.0.0.1:5500"
                class="w-full rounded-lg border border-zinc-200 px-3 py-1.5 text-xs font-mono outline-none focus:border-red-400"
              />
            </div>
            <div>
              <label class="mb-1 block text-[11px] font-medium text-zinc-500">Self ID（可选）</label>
              <input
                v-model="conn.self_id"
                placeholder="留空则自动获取"
                class="w-full rounded-lg border border-zinc-200 px-3 py-1.5 text-xs font-mono outline-none focus:border-red-400"
              />
            </div>
          </template>
        </div>

        <button
          @click="saveBot"
          :disabled="savingBot"
          class="rounded-lg bg-red-700 px-5 py-2 text-sm text-white hover:bg-red-800 disabled:opacity-50"
        >
          {{ savingBot ? '保存中...' : '💾 保存机器人配置' }}
        </button>
        <p class="text-[11px] text-zinc-400">修改后需重启程序生效</p>
      </div>
    </div>
  </div>
</template>
