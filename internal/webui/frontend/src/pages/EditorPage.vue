<script setup lang="ts">
import { ref, onMounted, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { usePluginsStore } from '../stores/plugins'
import { useToast } from '../composables/useToast'
import CodeEditor from '../components/CodeEditor.vue'

const route = useRoute()
const router = useRouter()
const pluginsStore = usePluginsStore()
const toast = useToast()

const pluginName = ref('')
const code = ref('')
const lang = ref<'redlang' | 'lua'>('redlang')
const dirty = ref(false)

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
    pluginName.value = name as string
    await loadPlugin(name as string)
  }
})

async function loadPlugin(name: string) {
  try {
    const res = await pluginsStore.fetchAll()
    const pkg = pluginsStore.plugins.find(p => p.name === name)
    if (pkg) {
      code.value = pkg.code || ''
      lang.value = (pkg.lang as any) || 'redlang'
      dirty.value = false
    } else {
      code.value = ''
      dirty.value = false
    }
  } catch { /* ignore */ }
}

function onCodeChange(newCode: string) {
  if (!dirty.value) dirty.value = true
}

async function save() {
  if (!pluginName.value.trim()) {
    toast.show('请输入插件名称', 'error')
    return
  }
  try {
    await pluginsStore.save(pluginName.value.trim(), code.value, lang.value)
    dirty.value = false
    toast.show(`插件 "${pluginName.value}" 已保存`, 'success')
  } catch (e: any) {
    toast.show(e.message || '保存失败', 'error')
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

function switchPlugin(name: string) {
  if (dirty && !confirm('当前脚本有未保存的修改，确定切换吗？')) return
  router.push(`/editor?name=${encodeURIComponent(name)}`)
}
</script>

<template>
  <div class="flex h-[calc(100vh-8rem)] flex-col gap-3">
    <!-- Toolbar -->
    <div class="flex flex-wrap items-center gap-2">
      <select
        @change="(e: any) => switchPlugin((e.target as HTMLSelectElement).value)"
        class="rounded-lg border border-zinc-200 px-3 py-2 text-sm outline-none focus:border-red-400"
      >
        <option value="">— 选择或输入插件名 —</option>
        <option
          v-for="p in pluginsStore.plugins"
          :key="p.name"
          :value="p.name"
          :selected="p.name === pluginName"
        >
          {{ p.name }}{{ p.lang && p.lang !== 'redlang' ? ' (' + p.lang + ')' : '' }}
        </option>
      </select>
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
      <div class="flex gap-2">
        <button @click="save" class="rounded-lg bg-red-700 px-4 py-2 text-sm text-white hover:bg-red-800">
          💾 保存
        </button>
        <button @click="format" class="rounded-lg border border-zinc-200 px-4 py-2 text-sm hover:bg-zinc-50">
          ✨ 格式化
        </button>
        <button @click="validate" class="rounded-lg border border-zinc-200 px-4 py-2 text-sm hover:bg-zinc-50">
          ✅ 验证
        </button>
      </div>
      <span v-if="dirty" class="text-xs text-amber-600">● 未保存</span>
    </div>

    <!-- Editor -->
    <div class="flex-1">
      <CodeEditor v-model="code" :lang="lang" @update:model-value="onCodeChange" />
    </div>
  </div>
</template>
