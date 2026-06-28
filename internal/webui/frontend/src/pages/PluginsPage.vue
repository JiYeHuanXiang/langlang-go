<script setup lang="ts">
import { onMounted, ref } from 'vue'
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
          <label class="mb-1 block text-xs font-medium text-zinc-500">初始代码（可选）</label>
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
  </div>
</template>
