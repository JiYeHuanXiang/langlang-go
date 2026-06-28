import { ref } from 'vue'

const toasts = ref<{ id: number; msg: string; type: 'success' | 'error' | 'info' | 'warn' }[]>([])
let nextId = 0

export function useToast() {
  function show(msg: string, type: 'success' | 'error' | 'info' | 'warn' = 'info') {
    const id = nextId++
    toasts.value.push({ id, msg, type })
    setTimeout(() => {
      toasts.value = toasts.value.filter(t => t.id !== id)
    }, 3500)
  }

  return { toasts, show }
}
