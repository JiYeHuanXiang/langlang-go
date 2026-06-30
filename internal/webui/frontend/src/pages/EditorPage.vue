<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { usePluginsStore } from '../stores/plugins'
import { useToast } from '../composables/useToast'
import { runScript, stopScript } from '../api/script'
import CodeEditor from '../components/CodeEditor.vue'
import ConfirmModal from '../components/ConfirmModal.vue'
import OutputPanel from '../components/OutputPanel.vue'
import type { LogLine } from '../components/OutputPanel.vue'
import { Play, Square, Clock, FileCode, Upload } from 'lucide-vue-next'

const route = useRoute()
const router = useRouter()
const pluginsStore = usePluginsStore()
const toast = useToast()

const EXAMPLE_RED = `【赋值】@问候@你好，世界！
【输出】@【取变量】@问候
【计次循环】@3
  【输出】@第【取循环次数】次
【计次循环尾】
`

const EXAMPLE_LUA = `local greeting = "Hello, World!"
print(greeting)

for i = 1, 3 do
    print("Count: " .. i)
end
`

const EXAMPLE_JS = `const greeting = "Hello, World!";
console.log(greeting);

for (let i = 1; i <= 3; i++) {
    console.log("Count: " + i);
}
`

// Plugin state
const pluginName = ref('')
const code = ref('')
const lang = ref<'redlang' | 'lua' | 'javascript'>('redlang')
const dirty = ref(false)
const search = ref('')
const newName = ref('')
const newLang = ref<'redlang' | 'lua' | 'javascript'>('redlang')
const insertExample = ref(false)
const showNewDialog = ref(false)
const deleteTarget = ref<string | null>(null)
const saving = ref(false)
const selectedPkg = ref<any>(null)
const importFileInput = ref<HTMLInputElement | null>(null)

// Script run state
const scriptRunning = ref(false)
const scriptStatus = ref<'idle' | 'running' | 'success' | 'error' | 'cancelled' | 'timeout'>('idle')
const scriptLogs = ref<LogLine[]>([])
const scriptExecTime = ref(0)
const showConsole = ref(false)

const filteredPlugins = computed(() => {
  if (!search.value) return pluginsStore.plugins
  const q = search.value.toLowerCase()
  return pluginsStore.plugins.filter(p => p.name.toLowerCase().includes(q))
})

// Timeout
const runTimeout = ref(10)

onMounted(async () => {
  await pluginsStore.fetchAll()
  const name = (route.query.name as string) || ''
  if (name) {
    pluginName.value = name
    await loadPlugin(name)
  }
})

watch(() => route.query.name, async (name) => {
  if (name && name !== pluginName.value) {
    if (dirty.value && !confirm('当前脚本有未保存的修改，确定切换吗？')) {
      router.replace(`/editor?name=${encodeURIComponent(pluginName.value)}`)
      return
    }
    pluginName.value = name as string
    await loadPlugin(name as string)
  }
})

async function loadPlugin(name: string) {
  await pluginsStore.fetchAll()
  const pkg = pluginsStore.plugins.find(p => p.name === name)
  if (pkg) {
    code.value = pkg.code || ''
    lang.value = (pkg.lang as any) || 'redlang'
    selectedPkg.value = pkg
    dirty.value = false
  } else {
    code.value = ''
    selectedPkg.value = null
    dirty.value = false
  }
}

function onCodeChange(_newCode: string) {
  if (!dirty.value) dirty.value = true
}

function selectPlugin(name: string) {
  if (dirty.value && name !== pluginName.value) {
    if (!confirm('当前脚本有未保存的修改，确定切换吗？')) return
  }
  router.push(`/editor?name=${encodeURIComponent(name)}`)
}

async function save() {
  if (!pluginName.value.trim()) {
    toast.show('请输入插件名称', 'error')
    return
  }
  saving.value = true
  try {
    await pluginsStore.save(pluginName.value.trim(), code.value, lang.value)
    dirty.value = false
    await pluginsStore.fetchAll()
    const pkg = pluginsStore.plugins.find(p => p.name === pluginName.value.trim())
    if (pkg) selectedPkg.value = pkg
    toast.show(`插件 "${pluginName.value}" 已保存`, 'success')
  } catch (e: any) {
    toast.show(e.message || '保存失败', 'error')
  } finally {
    saving.value = false
  }
}

async function validate() {
  try {
    await pluginsStore.validate(code.value, lang.value)
    toast.show('语法验证通过 ✅', 'success')
  } catch (e: any) {
    toast.show(e.message || '验证失败', 'error')
  }
}

function format() {
  code.value = code.value.replace(/\n{3,}/g, '\n\n').trim() + '\n'
  toast.show('已格式化', 'info')
}

async function createPlugin() {
  if (!newName.value.trim()) {
    toast.show('请输入插件名称', 'error')
    return
  }
  const initialCode = insertExample.value
    ? (newLang.value === 'lua' ? EXAMPLE_LUA : newLang.value === 'javascript' ? EXAMPLE_JS : EXAMPLE_RED)
    : ''
  try {
    await pluginsStore.save(newName.value.trim(), initialCode, newLang.value)
    toast.show(`插件 "${newName.value}" 已创建`, 'success')
    showNewDialog.value = false
    newName.value = ''
    newLang.value = 'redlang'
    insertExample.value = false
    await pluginsStore.fetchAll()
    router.push(`/editor?name=${encodeURIComponent(newName.value.trim())}`)
  } catch (e: any) {
    toast.show(e.message || '创建失败', 'error')
  }
}

async function deletePlugin() {
  if (!deleteTarget.value) return
  try {
    await pluginsStore.remove(deleteTarget.value)
    toast.show(`插件 "${deleteTarget.value}" 已删除`, 'success')
    const next = pluginsStore.plugins.find(p => p.name !== deleteTarget.value)
    if (next) {
      router.push(`/editor?name=${encodeURIComponent(next.name)}`)
    } else {
      router.push('/editor')
      pluginName.value = ''
      code.value = ''
      selectedPkg.value = null
      dirty.value = false
    }
  } catch (e: any) {
    toast.show(e.message || '删除失败', 'error')
  } finally {
    deleteTarget.value = null
  }
}

function formatTime(t: string) {
  if (!t) return '-'
  return new Date(t).toLocaleString()
}

// ── Script Execution ──

function addLog(type: LogLine['type'], msg: string) {
  const now = new Date()
  const time = `${String(now.getHours()).padStart(2, '0')}:${String(now.getMinutes()).padStart(2, '0')}:${String(now.getSeconds()).padStart(2, '0')}`
  scriptLogs.value.push({ time, type, msg })
}

async function runScriptNow() {
  if (scriptRunning.value) return
  if (!code.value.trim()) {
    toast.show('请输入脚本代码', 'error')
    return
  }

  scriptRunning.value = true
  scriptStatus.value = 'running'
  scriptLogs.value = []
  scriptExecTime.value = 0
  showConsole.value = true

  const langLabel = lang.value === 'lua' ? 'Lua' : lang.value === 'javascript' ? 'JavaScript' : 'RedLang'
  addLog('info', `▶ 开始执行 (${langLabel}, 超时 ${runTimeout.value}s)`)

  try {
    const res = await runScript(
      code.value,
      lang.value === 'redlang' ? '' : lang.value,
      runTimeout.value,
    )

    scriptExecTime.value = res.duration_ms || 0

    if (res.output) {
      for (const line of res.output.split('\n')) {
        addLog('output', line)
      }
    }
    if (res.error) {
      addLog('error', res.error)
    }

    if (res.cancelled) {
      scriptStatus.value = 'cancelled'
      addLog('info', '⊘ 已取消')
    } else if (res.timeout) {
      scriptStatus.value = 'timeout'
      addLog('error', `⏱ 超时 (${runTimeout.value}s)`)
    } else if (res.code === 0 && !res.error) {
      scriptStatus.value = 'success'
      addLog('success', `✓ 执行成功 (${res.duration_ms}ms)`)
    } else {
      scriptStatus.value = 'error'
      addLog('error', `✗ 执行失败 (code=${res.code})`)
    }
  } catch (e: any) {
    scriptStatus.value = 'error'
    addLog('error', `请求失败: ${e.message || e}`)
  } finally {
    scriptRunning.value = false
  }
}

async function handleStop() {
  try {
    await stopScript()
    addLog('info', '⊘ 正在停止...')
  } catch {
    // ignore
  }
}

function clearLogs() {
  scriptLogs.value = []
  scriptStatus.value = 'idle'
  scriptExecTime.value = 0
}

function triggerImportFile() {
  importFileInput.value?.click()
}

function onImportFile(e: Event) {
  const input = e.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) return
  const reader = new FileReader()
  reader.onload = () => {
    code.value = reader.result as string
    dirty.value = true
    // Detect language from file extension
    const ext = file.name.split('.').pop()?.toLowerCase()
    if (ext === 'lua') lang.value = 'lua'
    else if (ext === 'js') lang.value = 'javascript'
    else lang.value = 'redlang'
    toast.show(`已导入 ${file.name}`, 'success')
  }
  reader.readAsText(file)
  input.value = ''
}

onUnmounted(() => {
  // Clean up if script is running when navigating away
  if (scriptRunning.value) {
    stopScript().catch(() => {})
  }
})
</script>

<template>
  <div class="flex h-[calc(100vh-8rem)] gap-4">
    <!-- Left Sidebar: Plugin List -->
    <div class="flex w-60 shrink-0 flex-col rounded-xl border border-zinc-200 bg-white">
      <div class="border-b border-zinc-100 p-3">
        <div class="flex items-center gap-2">
          <input
            v-model="search"
            placeholder="🔍 搜索..."
            class="w-full rounded-lg border border-zinc-200 px-2.5 py-1.5 text-xs outline-none focus:border-red-400"
          />
          <button
            @click="showNewDialog = true"
            class="shrink-0 rounded-lg bg-red-700 px-2.5 py-1.5 text-xs text-white hover:bg-red-800"
            title="新建脚本"
          >
            ＋
          </button>
        </div>
      </div>
      <div class="flex-1 overflow-auto py-1">
        <div
          v-if="filteredPlugins.length === 0"
          class="px-3 py-6 text-center text-xs text-zinc-400"
        >
          {{ search ? '无匹配结果' : '暂无脚本，点击 ＋ 创建' }}
        </div>
        <button
          v-for="p in filteredPlugins"
          :key="p.name"
          @click="selectPlugin(p.name)"
          :class="[
            'w-full text-left px-3 py-2 text-sm transition-colors',
            p.name === pluginName
              ? 'bg-red-50 text-red-700 font-medium'
              : 'text-zinc-600 hover:bg-zinc-50',
          ]"
        >
          <div class="flex items-center gap-1.5">
            <span class="truncate">{{ p.name }}</span>
            <span
              v-if="p.lang && p.lang !== 'redlang'"
              class="shrink-0 rounded bg-zinc-100 px-1.5 py-px text-[10px] text-zinc-500"
            >
              {{ p.lang }}
            </span>
          </div>
          <div class="mt-0.5 text-[10px] text-zinc-400">
            {{ formatTime(p.updated_at) }}
          </div>
        </button>
      </div>
    </div>

    <!-- Right: Editor + Output -->
    <div class="flex flex-1 flex-col gap-3 overflow-hidden min-w-0">
      <!-- Toolbar Row 1: Plugin controls -->
      <div class="flex flex-wrap items-center gap-2">
        <input
          v-model="pluginName"
          placeholder="插件名称"
          class="w-44 rounded-lg border border-zinc-200 px-3 py-2 text-sm font-mono outline-none focus:border-red-400"
        />
        <select
          v-model="lang"
          class="w-28 rounded-lg border border-zinc-200 px-2 py-2 text-xs outline-none"
        >
          <option value="redlang">RedLang</option>
          <option value="lua">Lua</option>
          <option value="javascript">JavaScript</option>
        </select>
        <button
          @click="save"
          :disabled="saving"
          class="rounded-lg bg-red-700 px-4 py-2 text-sm text-white hover:bg-red-800 disabled:opacity-50"
        >
          {{ saving ? '保存中…' : '💾 保存' }}
        </button>
        <button
          @click="format"
          class="rounded-lg border border-zinc-200 px-4 py-2 text-sm hover:bg-zinc-50"
        >
          ✨ 格式化
        </button>
        <button
          @click="validate"
          class="rounded-lg border border-zinc-200 px-4 py-2 text-sm hover:bg-zinc-50"
        >
          ✅ 验证
        </button>
        <button
          v-if="pluginName"
          @click="deleteTarget = pluginName"
          class="rounded-lg border border-red-200 px-4 py-2 text-sm text-red-600 hover:bg-red-50"
        >
          🗑 删除
        </button>
        <button
          @click="triggerImportFile"
          class="inline-flex items-center gap-1.5 rounded-lg border border-zinc-200 px-3 py-2 text-sm hover:bg-zinc-50 transition-colors"
        >
          <Upload :size="14" />
          导入文件
        </button>
        <input
          ref="importFileInput"
          type="file"
          accept=".txt,.red,.lua,.js,.json,.redlang,*"
          class="hidden"
          @change="onImportFile"
        />
        <span v-if="dirty" class="text-xs text-amber-600 font-medium">● 未保存</span>
        <span v-if="selectedPkg" class="ml-auto text-[10px] text-zinc-400">
          创建 {{ formatTime(selectedPkg.created_at) }} · 更新 {{ formatTime(selectedPkg.updated_at) }}
        </span>
      </div>

      <!-- Toolbar Row 2: Run controls -->
      <div class="flex items-center gap-2 -mt-1">
        <button
          v-if="!scriptRunning"
          @click="runScriptNow"
          class="inline-flex items-center gap-1.5 rounded-lg bg-emerald-600 px-4 py-1.5 text-sm text-white hover:bg-emerald-700 transition-colors"
        >
          <Play :size="14" />
          运行
        </button>
        <button
          v-else
          @click="handleStop"
          class="inline-flex items-center gap-1.5 rounded-lg bg-red-600 px-4 py-1.5 text-sm text-white hover:bg-red-700 animate-pulse transition-colors"
        >
          <Square :size="14" />
          停止
        </button>
        <div class="flex items-center gap-1.5 text-xs text-zinc-500">
          <Clock :size="12" />
          <select
            v-model.number="runTimeout"
            class="rounded border border-zinc-200 px-1.5 py-1 text-xs outline-none w-14"
          >
            <option :value="5">5s</option>
            <option :value="10">10s</option>
            <option :value="30">30s</option>
            <option :value="60">60s</option>
          </select>
        </div>
        <button
          @click="code = lang === 'lua' ? EXAMPLE_LUA : lang === 'javascript' ? EXAMPLE_JS : EXAMPLE_RED"
          class="inline-flex items-center gap-1.5 rounded-lg border border-zinc-200 px-3 py-1.5 text-xs hover:bg-zinc-50 transition-colors"
        >
          <FileCode :size="12" />
          插入示例
        </button>
      </div>

      <!-- Editor + Output Panel (CSS Grid for reliable height) -->
      <div class="flex-1 min-h-0 grid" :class="showConsole ? 'grid-rows-[1fr_192px]' : 'grid-rows-[1fr_0fr]'">
        <!-- Editor -->
        <div class="min-h-0 overflow-hidden">
          <CodeEditor v-model="code" :lang="lang" @update:model-value="onCodeChange" />
        </div>
        <!-- Output Panel -->
        <div class="overflow-hidden min-h-0 rounded-lg">
          <OutputPanel
            v-model:open="showConsole"
            :status="scriptStatus"
            :logs="scriptLogs"
            :execution-time="scriptExecTime"
            @clear="clearLogs"
          />
        </div>
      </div>
    </div>

    <!-- New Plugin Dialog -->
    <ConfirmModal
      :open="showNewDialog"
      title="新建脚本"
      @close="showNewDialog = false"
      @confirm="createPlugin"
    >
      <div class="space-y-3">
        <div>
          <label class="mb-1 block text-xs font-medium text-zinc-500">脚本名称</label>
          <input
            v-model="newName"
            placeholder="my-plugin"
            class="w-full rounded-lg border border-zinc-200 px-3 py-2 text-sm font-mono outline-none focus:border-red-400"
          />
        </div>
        <div>
          <label class="mb-1 block text-xs font-medium text-zinc-500">脚本语言</label>
          <select
            v-model="newLang"
            class="w-full rounded-lg border border-zinc-200 px-3 py-2 text-sm outline-none focus:border-red-400"
          >
            <option value="redlang">RedLang</option>
            <option value="lua">Lua</option>
            <option value="javascript">JavaScript</option>
          </select>
        </div>
        <label class="flex items-center gap-2 text-sm text-zinc-600 cursor-pointer">
          <input v-model="insertExample" type="checkbox" class="accent-red-600" />
          插入示例代码模板
        </label>
      </div>
    </ConfirmModal>

    <!-- Delete Confirm Modal -->
    <ConfirmModal
      :open="deleteTarget !== null"
      title="删除脚本"
      @close="deleteTarget = null"
      @confirm="deletePlugin"
    >
      <p class="text-sm text-zinc-600">
        确定要删除脚本 <span class="font-mono font-medium text-red-700">{{ deleteTarget }}</span> 吗？此操作不可撤销。
      </p>
    </ConfirmModal>
  </div>
</template>

<style scoped>
:deep(.cm-editor) {
  height: 100% !important;
}
:deep(.cm-scroller) {
  flex: 1 !important;
  min-height: 0 !important;
}
</style>
