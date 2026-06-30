<script setup lang="ts">
import { onMounted, onUnmounted, ref, watch } from 'vue'
import { useConfigStore } from '../stores/config'
import { useToast } from '../composables/useToast'
import { saveConfig } from '../api/config'
import { botControl } from '../api/bots'
import { getStatus } from '../api/status'

interface BotConn {
  platform: 'onebot11' | 'telegram' | 'satori'
  mode?: string       // onebot11: reverse/forward
  url?: string
  access_token?: string
  self_id?: string
  token?: string      // telegram
  api_url?: string    // satori
  enabled: boolean    // false = 禁用（下次启动时不连接）
}

interface BotStatus {
  platform: string
  self_id: string
  running: boolean
}

const configStore = useConfigStore()
const toast = useToast()

const saving = ref(false)
const savingBot = ref(false)
const botStatuses = ref<BotStatus[]>([])
let statusTimer: ReturnType<typeof setInterval> | null = null

const LS_KEY_RETENTION = 'logs_retention_minutes'

function loadRetention(): number {
  try {
    const saved = localStorage.getItem(LS_KEY_RETENTION)
    if (saved !== null) return Number(saved)
  } catch { /* ignore */ }
  return 0
}

const logsRetention = ref(loadRetention())

watch(logsRetention, (t) => {
  localStorage.setItem(LS_KEY_RETENTION, String(t))
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
      let url = ob.url || ''
      const mode = ob.mode || 'reverse'
      // 反向 WS 模式不需要 ws:// 前缀，只保留监听地址方便编辑
      if (mode === 'forward') {
        url = url.replace(/^wss?:\/\//, '')
      }
      botConns.value.push({
        platform: 'onebot11',
        mode,
        url,
        access_token: ob.access_token || '',
        self_id: ob.self_id || '',
        enabled: ob.enabled !== false,
      })
    }
  }
  if (Array.isArray(bot.telegram)) {
    for (const t of bot.telegram) {
      const token = typeof t === 'string' ? t : (t.token || '')
      const enabled = typeof t === 'string' ? true : (t.enabled !== false)
      botConns.value.push({ platform: 'telegram', token, enabled })
    }
  }
  if (Array.isArray(bot.satori)) {
    for (const s of bot.satori) {
      botConns.value.push({
        platform: 'satori',
        url: s.url || '',
        token: s.token || '',
        self_id: s.self_id || '',
        api_url: s.api_url || '',
        enabled: s.enabled !== false,
      })
    }
  }
}

async function fetchBotStatus() {
  try {
    const res = await getStatus()
    botStatuses.value = res.bots || []
  } catch { /* ignore */ }
}

function getBotStatus(conn: BotConn): BotStatus | undefined {
  const connSelfId = conn.self_id || ''
  // 如果 self_id 为空（自动获取），按平台匹配第一个
  if (!connSelfId) {
    return botStatuses.value.find(b => b.platform === conn.platform)
  }
  return botStatuses.value.find(
    b => b.platform === conn.platform && b.self_id === connSelfId
  )
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
  await fetchBotStatus()
  statusTimer = setInterval(fetchBotStatus, 5000)
})

onUnmounted(() => {
  if (statusTimer) {
    clearInterval(statusTimer)
    statusTimer = null
  }
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
  botConns.value.push({ platform: 'onebot11', mode: 'reverse', url: '', access_token: '', self_id: '', enabled: true })
}

function removeBot(idx: number) {
  botConns.value.splice(idx, 1)
}

async function saveBot() {
  savingBot.value = true
  try {
    const onebot11: { mode?: string; url: string; access_token: string; self_id: string; enabled?: boolean }[] = []
    const telegram: { token: string; enabled?: boolean }[] = []
    const satori: { url: string; token: string; self_id: string; api_url: string; enabled?: boolean }[] = []
    for (const conn of botConns.value) {
      if (conn.platform === 'onebot11') {
        // 根据模式规范化 URL
        let url = conn.url || ''
        const mode = conn.mode || 'reverse'
        if (mode === 'forward') {
          url = url.replace(/^wss?:\/\//, '')   // forward 不需要 ws:// 前缀
        } else if (url && !/^wss?:\/\//.test(url)) {
          url = 'ws://' + url                    // reverse 必须带 ws://
        }
        onebot11.push({
          mode,
          url,
          access_token: conn.access_token || '',
          self_id: conn.self_id || '',
          ...(conn.enabled === false ? { enabled: false } : {}),
        })
      } else if (conn.platform === 'telegram') {
        if (conn.token) {
          telegram.push({
            token: conn.token,
            ...(conn.enabled === false ? { enabled: false } : {}),
          })
        }
      } else if (conn.platform === 'satori') {
        satori.push({
          url: conn.url || '',
          token: conn.token || '',
          self_id: conn.self_id || '',
          api_url: conn.api_url || '',
          ...(conn.enabled === false ? { enabled: false } : {}),
        })
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

async function connectBot(conn: BotConn) {
  try {
    await botControl('start', conn.platform, conn.self_id || '')
    toast.show('已发送连接指令', 'success')
    setTimeout(fetchBotStatus, 2000)
  } catch (e: any) {
    toast.show(e.message || '连接失败', 'error')
  }
}

async function disconnectBot(conn: BotConn) {
  try {
    await botControl('stop', conn.platform, conn.self_id || '')
    toast.show('已发送断开指令', 'success')
    setTimeout(fetchBotStatus, 1500)
  } catch (e: any) {
    toast.show(e.message || '断开失败', 'error')
  }
}

function toggleEnabled(conn: BotConn) {
  conn.enabled = !conn.enabled
}

function onModeChanged(conn: BotConn) {
  if (conn.mode === 'forward') {
    // 反向 WS 模式（等待服务端连接）：去掉 ws:// 前缀，保留监听地址
    conn.url = (conn.url || '').replace(/^wss?:\/\//, '')
    if (!conn.url) conn.url = ':6700'
  } else {
    // 正向 WS 模式（连接远程服务器）：如果没有 ws:// 前缀则加上
    if (conn.url && !/^wss?:\/\//.test(conn.url)) {
      conn.url = 'ws://' + conn.url
    }
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
          :class="conn.enabled ? 'border-zinc-100 bg-zinc-50/50' : 'border-zinc-100 bg-zinc-100/60 opacity-75'"
          class="rounded-lg border p-4"
        >
          <!-- 头部：平台选择 + 启用开关 + 状态 + 操作按钮 -->
          <div class="mb-3 flex items-center justify-between gap-2 flex-wrap">
            <div class="flex items-center gap-2">
              <select
                v-model="conn.platform"
                class="rounded-lg border border-zinc-200 px-3 py-1.5 text-xs outline-none focus:border-red-400"
              >
                <option value="onebot11">OneBot 11</option>
                <option value="telegram">Telegram</option>
                <option value="satori">Satori</option>
              </select>

              <!-- 启用/禁用 切换 -->
              <button
                @click="toggleEnabled(conn)"
                :class="conn.enabled
                  ? 'bg-green-600 hover:bg-green-700'
                  : 'bg-zinc-400 hover:bg-zinc-500'"
                class="relative inline-flex h-5 w-9 items-center rounded-full transition-colors shrink-0"
                :title="conn.enabled ? '点击禁用（下次启动不连接）' : '点击启用'"
              >
                <span
                  :class="conn.enabled ? 'translate-x-5' : 'translate-x-0.5'"
                  class="inline-block h-3.5 w-3.5 rounded-full bg-white transition-transform"
                />
              </button>

              <!-- 状态徽章 -->
              <template v-if="!conn.enabled">
                <span class="rounded-full bg-zinc-200 text-zinc-500 px-2 py-0.5 text-[11px] font-medium">已禁用</span>
              </template>
              <template v-else>
                <span
                  v-if="getBotStatus(conn)?.running"
                  class="rounded-full bg-green-100 text-green-700 px-2 py-0.5 text-[11px] font-medium"
                >● 已连接</span>
                <span
                  v-else-if="getBotStatus(conn)"
                  class="rounded-full bg-red-100 text-red-700 px-2 py-0.5 text-[11px] font-medium"
                >● 未连接</span>
                <span
                  v-else
                  class="rounded-full bg-zinc-100 text-zinc-400 px-2 py-0.5 text-[11px] font-medium"
                >未启动</span>
              </template>
            </div>

            <div class="flex items-center gap-1.5">
              <!-- 连接/断开 按钮 -->
              <template v-if="conn.enabled">
                <button
                  v-if="getBotStatus(conn)?.running"
                  @click="disconnectBot(conn)"
                  class="rounded-lg border border-red-200 text-red-600 px-2.5 py-1 text-xs font-medium hover:bg-red-50 transition-colors"
                >断开</button>
                <button
                  v-else
                  @click="connectBot(conn)"
                  class="rounded-lg border border-green-200 text-green-600 px-2.5 py-1 text-xs font-medium hover:bg-green-50 transition-colors"
                >连接</button>
              </template>
              <template v-else>
                <span class="text-[11px] text-zinc-400 italic">需启用后连接</span>
              </template>

              <!-- 删除 -->
              <button
                @click="removeBot(idx)"
                class="text-xs text-red-500 hover:text-red-700 px-1"
                title="删除此连接"
              >✕</button>
            </div>
          </div>

          <!-- OneBot 11 字段 -->
          <template v-if="conn.platform === 'onebot11'">
            <div class="mb-2">
              <label class="mb-1 block text-[11px] font-medium text-zinc-500">连接模式</label>
              <select
                v-model="conn.mode"
                @change="onModeChanged(conn)"
                class="w-full rounded-lg border border-zinc-200 px-3 py-1.5 text-xs outline-none focus:border-red-400"
              >
                <option value="reverse">正向 WS（连接远程服务器）</option>
                <option value="forward">反向 WS（等待服务端连接）</option>
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
        <p class="text-[11px] text-zinc-400">禁用/启用需保存后重启生效；连接/断开为即时操作</p>
      </div>
    </div>
  </div>
</template>
