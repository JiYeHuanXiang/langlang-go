<script setup lang="ts">
import { onMounted, ref, computed } from 'vue'
import { useRouter } from 'vue-router'
import { usePluginsStore } from '../stores/plugins'
import { useToast } from '../composables/useToast'
import ConfirmModal from '../components/ConfirmModal.vue'

const router = useRouter()
const pluginsStore = usePluginsStore()
const toast = useToast()

const search = ref('')
const showCreate = ref(false)
const newName = ref('')
const newCode = ref('')
const deleteTarget = ref<string | null>(null)
const fileInput = ref<HTMLInputElement | null>(null)

function triggerFileSelect() {
  fileInput.value?.click()
}

function onFileSelected(e: Event) {
  const input = e.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) return
  const reader = new FileReader()
  reader.onload = () => {
    newCode.value = reader.result as string
    if (!newName.value.trim()) {
      newName.value = file.name.replace(/\.[^.]+$/, '')
    }
  }
  reader.readAsText(file)
  input.value = ''
}

// ── Export ──
const showExport = ref(false)
const exportSelected = ref<Set<string>>(new Set())

const LANG_EXT: Record<string, string> = {
  redlang: 'red',
  lua: 'lua',
  javascript: 'js',
}

const LANG_LABEL: Record<string, string> = {
  redlang: 'RedLang',
  lua: 'Lua',
  javascript: 'JavaScript',
}

const exportGroups = computed(() => {
  const groups: Record<string, typeof pluginsStore.plugins> = {}
  for (const p of pluginsStore.plugins) {
    const lang = p.lang || 'redlang'
    if (!groups[lang]) groups[lang] = []
    groups[lang].push(p)
  }
  return groups
})

function toggleExportAll(lang: string) {
  const items = exportGroups.value[lang] || []
  const allSelected = items.every(p => exportSelected.value.has(p.name))
  if (allSelected) {
    items.forEach(p => exportSelected.value.delete(p.name))
  } else {
    items.forEach(p => exportSelected.value.add(p.name))
  }
}

function toggleExportItem(name: string) {
  if (exportSelected.value.has(name)) {
    exportSelected.value.delete(name)
  } else {
    exportSelected.value.add(name)
  }
}

function isGroupAllSelected(lang: string) {
  const items = exportGroups.value[lang] || []
  return items.length > 0 && items.every(p => exportSelected.value.has(p.name))
}

function downloadPlugin(p: { name: string; code: string; lang: string }) {
  const ext = LANG_EXT[p.lang || 'redlang'] || 'txt'
  const blob = new Blob([p.code || ''], { type: 'text/plain;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = `${p.name}.${ext}`
  a.click()
  URL.revokeObjectURL(url)
}

function doExport() {
  const selected = pluginsStore.plugins.filter(p => exportSelected.value.has(p.name))
  if (selected.length === 0) {
    toast.show('请至少选择一个插件', 'error')
    return
  }
  for (const p of selected) {
    downloadPlugin(p)
  }
  toast.show(`已导出 ${selected.length} 个插件`, 'success')
  showExport.value = false
  exportSelected.value.clear()
}

function openExport() {
  exportSelected.value.clear()
  showExport.value = true
}

onMounted(() => pluginsStore.fetchAll())

const filteredPlugins = ref(() =>
  pluginsStore.plugins.filter(p =>
    p.name.toLowerCase().includes(search.value.toLowerCase())
  )
)

async function doCreate() {
  if (!newName.value.trim()) {
    toast.show('请输入插件名称', 'error')
    return
  }
  try {
    await pluginsStore.save(newName.value.trim(), newCode.value, 'redlang')
    toast.show(`插件 "${newName.value}" 创建成功`, 'success')
    showCreate.value = false
    newName.value = ''
    newCode.value = ''
  } catch (e: any) {
    toast.show(e.message || '创建失败', 'error')
  }
}

async function doDelete() {
  if (!deleteTarget.value) return
  try {
    await pluginsStore.remove(deleteTarget.value)
    toast.show(`插件 "${deleteTarget.value}" 已删除`, 'success')
    deleteTarget.value = null
  } catch (e: any) {
    toast.show(e.message || '删除失败', 'error')
  }
}

async function doReload() {
  try {
    await pluginsStore.reload()
    toast.show('插件已重载', 'success')
  } catch (e: any) {
    toast.show(e.message || '重载失败', 'error')
  }
}
</script>

<template>
  <div class="space-y-4">
    <!-- Toolbar -->
    <div class="flex flex-wrap items-center gap-3">
      <input
        v-model="search"
        placeholder="🔍 搜索插件..."
        class="w-52 rounded-lg border border-zinc-200 px-3 py-2 text-sm outline-none focus:border-red-400 focus:ring-2 focus:ring-red-100"
      />
      <button @click="showCreate = true" class="rounded-lg bg-red-700 px-4 py-2 text-sm text-white hover:bg-red-800">
        ➕ 新建插件
      </button>
      <button @click="doReload" class="rounded-lg border border-zinc-200 px-4 py-2 text-sm hover:bg-zinc-50">
        🔄 重载
      </button>
      <button @click="openExport" class="rounded-lg border border-zinc-200 px-4 py-2 text-sm hover:bg-zinc-50">
        📤 导出
      </button>
    </div>

    <!-- Table -->
    <div class="rounded-xl border border-zinc-200 bg-white shadow-sm">
      <div class="flex items-center justify-between border-b border-zinc-100 px-5 py-3">
        <h3 class="text-sm font-semibold">📦 插件列表</h3>
        <span class="text-xs text-zinc-400">共 {{ pluginsStore.plugins.length }} 个</span>
      </div>

      <div v-if="pluginsStore.plugins.length === 0" class="py-12 text-center text-sm text-zinc-400">
        📦 还没有插件，点击右上角创建一个
      </div>

      <table v-else class="w-full text-sm">
        <thead>
          <tr class="border-b border-zinc-100 text-left text-xs text-zinc-400">
            <th class="px-5 py-2 font-medium">名称</th>
            <th class="px-5 py-2 font-medium">状态</th>
            <th class="px-5 py-2 font-medium">创建时间</th>
            <th class="px-5 py-2 font-medium">更新时间</th>
            <th class="px-5 py-2 font-medium" />
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="p in pluginsStore.plugins.filter(p => p.name.toLowerCase().includes(search.toLowerCase()))"
            :key="p.name"
            class="border-b border-zinc-50 hover:bg-red-50/30"
          >
            <td class="px-5 py-2.5 font-medium">{{ p.name }}</td>
            <td class="px-5 py-2.5">
              <span
                :class="p.enabled ? 'bg-green-100 text-green-700' : 'bg-red-100 text-red-700'"
                class="rounded-full px-2 py-0.5 text-xs font-medium"
              >
                {{ p.enabled ? '启用' : '禁用' }}
              </span>
            </td>
            <td class="px-5 py-2.5 text-xs text-zinc-400">
              {{ p.created_at ? new Date(p.created_at).toLocaleString() : '-' }}
            </td>
            <td class="px-5 py-2.5 text-xs text-zinc-400">
              {{ p.updated_at ? new Date(p.updated_at).toLocaleString() : '-' }}
            </td>
            <td class="px-5 py-2.5">
              <div class="flex gap-2">
                <button
                  @click="pluginsStore.toggle(p.name, !p.enabled)"
                  :class="p.enabled ? 'border-red-200 text-red-600 hover:bg-red-50' : 'border-green-200 text-green-600 hover:bg-green-50'"
                  class="rounded-lg border px-3 py-1 text-xs"
                >
                  {{ p.enabled ? '停用' : '启用' }}
                </button>
                <button
                  @click="router.push(`/editor?name=${encodeURIComponent(p.name)}`)"
                  class="rounded-lg border border-zinc-200 px-3 py-1 text-xs hover:bg-zinc-50"
                >
                  编辑
                </button>
                <button
                  @click="deleteTarget = p.name"
                  class="rounded-lg border border-red-200 px-3 py-1 text-xs text-red-600 hover:bg-red-50"
                >
                  删除
                </button>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <!-- Create Modal -->
    <ConfirmModal :open="showCreate" title="新建插件" @close="showCreate = false" @confirm="doCreate">
      <div class="space-y-3">
        <div>
          <label class="mb-1 block text-xs font-medium text-zinc-500">插件名称</label>
          <input
            v-model="newName"
            placeholder="my-plugin"
            class="w-full rounded-lg border border-zinc-200 px-3 py-2 text-sm font-mono outline-none focus:border-red-400"
          />
        </div>
        <div>
          <div class="mb-1 flex items-center justify-between">
            <label class="text-xs font-medium text-zinc-500">初始代码（可选）</label>
            <button
              type="button"
              @click="triggerFileSelect"
              class="rounded border border-zinc-200 px-2 py-0.5 text-xs text-zinc-500 hover:bg-zinc-50"
            >
              📂 选择文件
            </button>
          </div>
          <input
            ref="fileInput"
            type="file"
            accept=".txt,.red,.lua,.js,.json,.redlang,*"
            class="hidden"
            @change="onFileSelected"
          />
          <textarea
            v-model="newCode"
            rows="5"
            placeholder="【输出】@你好世界"
            class="w-full rounded-lg border border-zinc-200 px-3 py-2 text-sm font-mono outline-none focus:border-red-400 resize-none"
          />
        </div>
      </div>
    </ConfirmModal>

    <!-- Delete Confirm -->
    <ConfirmModal :open="!!deleteTarget" title="确认删除" @close="deleteTarget = null" @confirm="doDelete">
      <p class="text-sm text-zinc-600">确定删除插件 "{{ deleteTarget }}" 吗？此操作不可撤销。</p>
    </ConfirmModal>

    <!-- Export Modal -->
    <Teleport to="body">
      <div
        v-if="showExport"
        class="fixed inset-0 z-50 flex items-center justify-center bg-black/40"
        @click.self="showExport = false"
      >
        <div class="w-full max-w-lg rounded-xl bg-white shadow-xl">
          <div class="flex items-center justify-between border-b border-zinc-100 px-5 py-4">
            <h3 class="text-sm font-semibold">📤 导出插件</h3>
            <button @click="showExport = false" class="text-lg leading-none text-zinc-400 hover:text-zinc-600">&times;</button>
          </div>
          <div class="max-h-80 overflow-auto px-5 py-4">
            <div v-if="pluginsStore.plugins.length === 0" class="py-6 text-center text-sm text-zinc-400">
              暂无可导出的插件
            </div>
            <div v-else class="space-y-4">
              <div v-for="(items, langKey) in exportGroups" :key="langKey">
                <div class="mb-1.5 flex items-center gap-2">
                  <input
                    type="checkbox"
                    :checked="isGroupAllSelected(langKey)"
                    @change="toggleExportAll(langKey)"
                    class="accent-red-600"
                  />
                  <span class="text-xs font-semibold text-zinc-600">
                    {{ LANG_LABEL[langKey] || langKey }}
                  </span>
                  <span class="rounded bg-zinc-100 px-1.5 py-px text-[10px] text-zinc-400">
                    .{{ LANG_EXT[langKey] || 'txt' }}
                  </span>
                  <span class="text-[10px] text-zinc-400">{{ items.length }} 个</span>
                </div>
                <div class="ml-5 space-y-1">
                  <label
                    v-for="p in items"
                    :key="p.name"
                    class="flex cursor-pointer items-center gap-2 rounded px-2 py-1 text-sm hover:bg-zinc-50"
                  >
                    <input
                      type="checkbox"
                      :checked="exportSelected.has(p.name)"
                      @change="toggleExportItem(p.name)"
                      class="accent-red-600"
                    />
                    <span class="font-mono text-xs">{{ p.name }}</span>
                    <span
                      :class="p.enabled ? 'bg-green-100 text-green-600' : 'bg-zinc-100 text-zinc-400'"
                      class="rounded-full px-1.5 py-px text-[10px]"
                    >
                      {{ p.enabled ? '启用' : '禁用' }}
                    </span>
                  </label>
                </div>
              </div>
            </div>
          </div>
          <div class="flex items-center justify-between border-t border-zinc-100 px-5 py-3">
            <span class="text-xs text-zinc-400">
              已选 {{ exportSelected.size }} / {{ pluginsStore.plugins.length }}
            </span>
            <div class="flex gap-2">
              <button
                @click="showExport = false"
                class="rounded-lg border border-zinc-200 px-4 py-2 text-sm hover:bg-zinc-50"
              >
                取消
              </button>
              <button
                @click="doExport"
                class="rounded-lg bg-red-700 px-4 py-2 text-sm text-white hover:bg-red-800"
              >
                导出
              </button>
            </div>
          </div>
        </div>
      </div>
    </Teleport>
  </div>
</template>
