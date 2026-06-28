import { defineStore } from 'pinia'
import { ref } from 'vue'
import { getConfig, saveConfig, getTestMode, setTestMode } from '../api/config'

export const useConfigStore = defineStore('config', () => {
  const config = ref<Record<string, any>>({})
  const testMode = ref(false)
  const loading = ref(false)

  async function fetchConfig() {
    loading.value = true
    try {
      const [cfgRes, tmRes] = await Promise.all([getConfig(), getTestMode()])
      config.value = cfgRes.config || {}
      testMode.value = tmRes.test_mode === 'on'
    } finally {
      loading.value = false
    }
  }

  async function save(cfg: Record<string, any>) {
    await saveConfig(cfg)
    await fetchConfig()
  }

  async function toggleTestMode(enabled: boolean) {
    await setTestMode(enabled)
    testMode.value = enabled
  }

  return { config, testMode, loading, fetchConfig, save, toggleTestMode }
})
