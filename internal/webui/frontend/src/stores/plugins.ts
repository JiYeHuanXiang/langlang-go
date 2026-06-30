import { defineStore } from 'pinia'
import { ref } from 'vue'
import { getPlugins, savePlugin, deletePlugin, reloadPlugins, validateCode, togglePlugin, type PluginInfo } from '../api/plugins'

export const usePluginsStore = defineStore('plugins', () => {
  const plugins = ref<PluginInfo[]>([])
  const loading = ref(false)

  async function fetchAll() {
    loading.value = true
    try {
      const res = await getPlugins()
      plugins.value = res.plugins || []
    } finally {
      loading.value = false
    }
  }

  async function save(name: string, code: string, lang: string) {
    await savePlugin(name, code, lang)
    await fetchAll()
  }

  async function remove(name: string) {
    await deletePlugin(name)
    await fetchAll()
  }

  async function toggle(name: string, enabled: boolean) {
    await togglePlugin(name, enabled)
    await fetchAll()
  }

  async function reload() {
    await reloadPlugins()
    await fetchAll()
  }

  async function validate(code: string, lang: string) {
    return validateCode(code, lang)
  }

  return { plugins, loading, fetchAll, save, remove, reload, validate, toggle }
})
