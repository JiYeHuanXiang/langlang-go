<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { usePluginsStore } from '../stores/plugins'
import { useToast } from '../composables/useToast'
import CodeEditor from '../components/CodeEditor.vue'
import ConfirmModal from '../components/ConfirmModal.vue'

const route = useRoute()
const router = useRouter()
const pluginsStore = usePluginsStore()
const toast = useToast()

const pluginName = ref('')
const code = ref('')
const lang = ref<'redlang' | 'lua'>('redlang')
const dirty = ref(false)
const search = ref('')
const newName = ref('')
const newLang = ref<'redlang' | 'lua'>('redlang')
const showNewDialog = ref(false)
const deleteTarget = ref<string | null>(null)
const saving = ref(false)
const selectedPkg = ref<any>(null)

const filteredPlugins = computed(() => {
  if (!search.value) return pluginsStore.plugins
  const q = search.value.toLowerCase()
  return pluginsStore.plugins.filter(p => p.name.toLowerCase().includes(q))
})

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
  try {
    await pluginsStore.save(newName.value.trim(), '', newLang.value)
    toast.show(`插件 "${newName.value}" 已创建`, 'success')
    showNewDialog.value = false
    newName.value = ''
    newLang.value = 'redlang'
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

    <!-- Right: Editor Area -->
    <div class="flex flex-1 flex-col gap-3 overflow-hidden">
      <!-- Toolbar -->
      <div class="flex flex-wrap items-center gap-2">
        <input
          v-model="pluginName"
          placeholder="插件名称"
          class="w-44 rounded-lg border border-zinc-200 px-3 py-2 text-sm font-mono outline-none focus:border-red-400"
        />
        <select
          v-model="lang"
          class="w-24 rounded-lg border border-zinc-200 px-2 py-2 text-xs outline-none"
        >
          <option value="redlang">RedLang</option>
          <option value="lua">Lua</option>
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
        <span v-if="dirty" class="text-xs text-amber-600 font-medium">● 未保存</span>
        <span v-if="selectedPkg" class="ml-auto text-[10px] text-zinc-400">
          创建 {{ formatTime(selectedPkg.created_at) }} · 更新 {{ formatTime(selectedPkg.updated_at) }}
        </span>
      </div>

      <!-- Editor -->
      <div class="flex-1 min-h-0">
        <CodeEditor v-model="code" :lang="lang" @update:model-value="onCodeChange" />
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
          </select>
        </div>
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