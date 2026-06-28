import { defineStore } from 'pinia'
import { ref } from 'vue'
import { getStatus } from '../api/status'

interface BotInfo {
  platform: string
  self_id: string
  running: boolean
}

export const useBotsStore = defineStore('bots', () => {
  const bots = ref<BotInfo[]>([])
  const configuredPlatforms = ref<string[]>([])
  const uptime = ref('')
  const version = ref('')
  const pluginCount = ref(0)
  const loading = ref(false)

  async function fetchStatus() {
    loading.value = true
    try {
      const res = await getStatus()
      bots.value = res.bots || []
      configuredPlatforms.value = res.configured_platforms || []
      uptime.value = res.uptime || ''
      version.value = res.version || ''
      pluginCount.value = res.plugins || 0
    } finally {
      loading.value = false
    }
  }

  return { bots, configuredPlatforms, uptime, version, pluginCount, loading, fetchStatus }
})
