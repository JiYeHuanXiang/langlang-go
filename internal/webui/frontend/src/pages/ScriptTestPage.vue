<script setup lang="ts">
import { ref, watch, nextTick } from 'vue'
import { runScript, stopScript } from '../api/script'
import { useToast } from '../composables/useToast'
import CodeEditor from '../components/CodeEditor.vue'

const toast = useToast()

const LS_KEY = 'script_test_form'

function loadForm() {
  try {
    const saved = localStorage.getItem(LS_KEY)
    if (saved) return JSON.parse(saved)
  } catch { /* ignore */ }
  return {}
}

const savedForm = loadForm()
const lang = ref<'redlang' | 'lua' | 'javascript'>(savedForm.lang ?? 'redlang')
const timeout = ref(savedForm.timeout ?? 10)

// 分语言持久化代码
const LS_CODE_KEY = 'script_test_code'
function loadCodes() {
  try {
    const saved = localStorage.getItem(LS_CODE_KEY)
    if (saved) return JSON.parse(saved)
  } catch { /* ignore */ }
  return {}
}
const savedCodes = loadCodes()
const code = ref(savedCodes[lang.value] ?? '')

function saveForm() {
  localStorage.setItem(LS_KEY, JSON.stringify({ lang: lang.value, timeout: timeout.value }))
}
function saveCodes() {
  const codes = loadCodes()
  codes[lang.value] = code.value
  localStorage.setItem(LS_CODE_KEY, JSON.stringify(codes))
}

// 切换语言时恢复对应代码
watch(lang, (newLang, oldLang) => {
  // 保存当前语言代码
  if (oldLang) {
    const codes = loadCodes()
    codes[oldLang] = code.value
    localStorage.setItem(LS_CODE_KEY, JSON.stringify(codes))
  }
  // 恢复新语言代码
  const codes = loadCodes()
  code.value = codes[newLang] ?? ''
  saveForm()
}, { immediate: false })

watch(timeout, saveForm)
watch(code, saveCodes)

// 执行状态
const running = ref(false)
const output = ref('')
const errorMsg = ref('')
const duration = ref(0)
const status = ref<'idle' | 'running' | 'success' | 'error' | 'cancelled' | 'timeout'>('idle')

interface LogLine {
  type: 'info' | 'output' | 'error' | 'success'
  text: string
  time: string
}
const outputLines = ref<LogLine[]>([])
const outputRef = ref<HTMLElement | null>(null)

function now() {
  return new Date().toLocaleTimeString('zh-CN', { hour12: false })
}

function scrollOutput() {
  nextTick(() => {
    if (outputRef.value) {
      outputRef.value.scrollTop = outputRef.value.scrollHeight
    }
  })
}

// 示例代码
const samples: Record<string, string> = {
  redlang: `【输出】@你好，世界！
【令】@x@【加】@10@20
【输出】@计算结果：【到文本】@【变量】@x
【计次循环】@3
  【输出】@循环第【到文本】@【变量】@循环次数@次
【计次循环尾】`,
  lua: `-- Lua 示例脚本
print("你好，Lua！")

local sum = 0
for i = 1, 10 do
    sum = sum + i
end
print("1 到 10 的和 =", sum)

-- 使用数学函数
print("圆周率 =", math.pi)
print("sqrt(2) =", math.sqrt(2))`,
  javascript: `// JavaScript 示例脚本
console.log("你好，JavaScript！");

let sum = 0;
for (let i = 1; i <= 10; i++) {
    sum += i;
}
console.log("1 到 10 的和 =", sum);

// 使用数学函数
console.log("圆周率 =", Math.PI);
console.log("sqrt(2) =", Math.sqrt(2));`,
}

function insertSample() {
  code.value = samples[lang.value] || ''
  toast.show('已插入示例代码', 'info')
}

// 运行脚本
async function run() {
  if (!code.value.trim()) {
    toast.show('请输入脚本代码', 'error')
    return
  }

  running.value = true
  output.value = ''
  errorMsg.value = ''
  duration.value = 0
  status.value = 'running'
  const langLabel = lang.value === 'lua' ? 'Lua' : lang.value === 'javascript' ? 'JavaScript' : 'RedLang'
  outputLines.value = [
    { type: 'info', text: `▶ 开始执行 ${langLabel} 脚本...`, time: now() },
  ]
  scrollOutput()

  try {
    const res = await runScript(code.value, lang.value, timeout.value) as any

    if (res.cancelled) {
      status.value = 'cancelled'
      outputLines.value.push({ type: 'error', text: `■ 执行已取消（${res.duration_ms}ms）`, time: now() })
    } else if (res.timeout) {
      status.value = 'timeout'
      errorMsg.value = res.error || '执行超时'
      outputLines.value.push({ type: 'error', text: `⏰ ${res.error}（${res.duration_ms}ms）`, time: now() })
    } else if (res.code !== 0 || res.error) {
      status.value = 'error'
      errorMsg.value = res.error || '执行失败'
      outputLines.value.push({ type: 'error', text: `✗ 错误: ${res.error}`, time: now() })
      outputLines.value.push({ type: 'info', text: `  耗时 ${res.duration_ms}ms`, time: now() })
    } else {
      status.value = 'success'
      output.value = res.output || ''
      const lines = (res.output || '').split('\n')
      lines.forEach((line: string) => {
        outputLines.value.push({ type: 'output', text: line, time: '' })
      })
      outputLines.value.push({ type: 'success', text: `✓ 执行成功（${res.duration_ms}ms）`, time: now() })
    }
    duration.value = res.duration_ms || 0
  } catch (e: any) {
    status.value = 'error'
    errorMsg.value = e.message || '执行失败'
    outputLines.value.push({ type: 'error', text: `✗ ${e.message || '请求失败'}`, time: now() })
  } finally {
    running.value = false
    scrollOutput()
  }
}

async function stop() {
  try {
    await stopScript()
    toast.show('已发送停止信号', 'info')
  } catch (e: any) {
    toast.show(e.message || '操作失败', 'error')
  }
}

function clearOutput() {
  output.value = ''
  errorMsg.value = ''
  duration.value = 0
  status.value = 'idle'
  outputLines.value = []
}

const statusLabel: Record<string, string> = {
  idle: '',
  running: '执行中...',
  success: '执行成功',
  error: '执行出错',
  cancelled: '已取消',
  timeout: '执行超时',
}

const statusColor: Record<string, string> = {
  idle: '',
  running: 'text-blue-400',
  success: 'text-green-400',
  error: 'text-red-400',
  cancelled: 'text-yellow-400',
  timeout: 'text-orange-400',
}

const langLabel: Record<string, string> = {
  redlang: 'RedLang',
  lua: 'Lua',
  javascript: 'JavaScript',
}
</script>

<template>
  <!-- 与 EditorPage 完全一致的两层 flex 结构：行 flex → 列 flex + overflow-hidden -->
  <div class="flex h-[calc(100vh-8rem)] gap-0">
    <div class="flex flex-1 flex-col gap-3 overflow-hidden">
      <!-- 工具栏 -->
      <div class="flex flex-wrap items-center gap-2">
        <!-- 语言选择 -->
        <div class="flex items-center gap-1.5">
          <span class="text-xs font-medium text-zinc-500">语言</span>
          <select
            v-model="lang"
            class="rounded-lg border border-zinc-200 px-2.5 py-1.5 text-sm font-medium outline-none focus:border-red-400"
          >
            <option value="redlang">RedLang</option>
            <option value="lua">Lua</option>
            <option value="javascript">JavaScript</option>
          </select>
        </div>

        <div class="h-5 w-px bg-zinc-200" />

        <!-- 运行/停止按钮 -->
        <button
          v-if="!running"
          @click="run"
          class="flex items-center gap-1.5 rounded-lg bg-green-600 px-4 py-1.5 text-sm font-medium text-white shadow-sm hover:bg-green-700 active:bg-green-800 transition-colors"
        >
          <svg class="h-3.5 w-3.5" viewBox="0 0 16 16" fill="currentColor"><path d="M4 2l10 6-10 6V2z"/></svg>
          运行
        </button>
        <button
          v-else
          @click="stop"
          class="flex items-center gap-1.5 rounded-lg bg-red-600 px-4 py-1.5 text-sm font-medium text-white shadow-sm hover:bg-red-700 active:bg-red-800 transition-colors animate-pulse"
        >
          <svg class="h-3.5 w-3.5" viewBox="0 0 16 16" fill="currentColor"><rect x="3" y="3" width="10" height="10" rx="1"/></svg>
          停止
        </button>

        <div class="h-5 w-px bg-zinc-200" />

        <!-- 超时设置 -->
        <div class="flex items-center gap-1.5">
          <span class="text-xs text-zinc-500">超时</span>
          <select
            v-model.number="timeout"
            class="rounded-lg border border-zinc-200 px-2 py-1.5 text-sm outline-none focus:border-red-400"
          >
            <option :value="5">5秒</option>
            <option :value="10">10秒</option>
            <option :value="30">30秒</option>
            <option :value="60">60秒</option>
          </select>
        </div>

        <div class="h-5 w-px bg-zinc-200" />

        <!-- 辅助按钮 -->
        <button
          @click="insertSample"
          class="rounded-lg border border-zinc-200 px-3 py-1.5 text-xs hover:bg-zinc-50 transition-colors"
        >
          插入示例
        </button>
        <button
          @click="clearOutput"
          class="rounded-lg border border-zinc-200 px-3 py-1.5 text-xs hover:bg-zinc-50 transition-colors"
        >
          清除输出
        </button>

        <!-- 状态指示 -->
        <div class="ml-auto flex items-center gap-2">
          <span v-if="status !== 'idle'" :class="['text-xs font-medium', statusColor[status]]">
            {{ statusLabel[status] }}
          </span>
          <span v-if="duration > 0" class="text-[11px] text-zinc-400">
            {{ duration }}ms
          </span>
        </div>
      </div>

      <!-- 编辑器 -->
      <div class="flex-1 min-h-0">
        <CodeEditor :key="lang" v-model="code" :lang="lang" />
      </div>

      <!-- 输出控制台 -->
      <div class="flex h-48 shrink-0 flex-col rounded-lg border border-zinc-700 bg-zinc-900">
        <!-- 控制台头部 -->
        <div class="flex items-center justify-between border-b border-zinc-700/60 px-4 py-2">
          <div class="flex items-center gap-2">
            <span class="text-xs font-medium text-zinc-400">输出控制台</span>
            <span
              v-if="status === 'running'"
              class="inline-flex h-2 w-2 rounded-full bg-blue-500 animate-pulse"
            />
            <span
              v-else-if="status === 'success'"
              class="inline-flex h-2 w-2 rounded-full bg-green-500"
            />
            <span
              v-else-if="status === 'error' || status === 'timeout'"
              class="inline-flex h-2 w-2 rounded-full bg-red-500"
            />
          </div>
          <div class="flex items-center gap-3">
            <span class="text-[10px] text-zinc-500 font-mono">
              {{ langLabel[lang] }}
            </span>
            <span v-if="outputLines.length > 0" class="text-[10px] text-zinc-500">
              {{ outputLines.length }} 行
            </span>
          </div>
        </div>

        <!-- 控制台内容 -->
        <div ref="outputRef" class="flex-1 overflow-auto p-3 font-mono text-sm">
          <div
            v-if="outputLines.length === 0"
            class="flex h-full items-center justify-center text-xs text-zinc-600"
          >
            编写脚本后点击「运行」查看输出结果
          </div>

          <template v-else>
            <div
              v-for="(line, i) in outputLines"
              :key="i"
              :class="[
                'flex gap-2 leading-relaxed',
                line.type === 'output' ? 'text-zinc-200' : '',
                line.type === 'error' ? 'text-red-400' : '',
                line.type === 'success' ? 'text-green-400' : '',
                line.type === 'info' ? 'text-blue-400' : '',
              ]"
            >
              <span v-if="line.time" class="shrink-0 text-zinc-600 select-none">{{ line.time }}</span>
              <span class="whitespace-pre-wrap break-all">{{ line.text }}</span>
            </div>
          </template>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
:deep(.cm-editor) {
  height: 100%;
}
:deep(.cm-scroller) {
  flex: 1;
  min-height: 0;
}
</style>
