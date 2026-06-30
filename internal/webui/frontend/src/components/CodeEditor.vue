<script setup lang="ts">
import { ref, nextTick, onMounted, onUnmounted, watch } from 'vue'
import { EditorView, basicSetup } from 'codemirror'
import { EditorState } from '@codemirror/state'
import { StreamLanguage, type StringStream } from '@codemirror/language'
import { lua } from '@codemirror/legacy-modes/mode/lua'
import { javascript } from '@codemirror/legacy-modes/mode/javascript'
import { oneDark } from '@codemirror/theme-one-dark'

const props = defineProps<{
  modelValue: string
  lang: 'redlang' | 'lua' | 'javascript'
  readonly?: boolean
}>()

const emit = defineEmits<{
  'update:modelValue': [value: string]
}>()

const container = ref<HTMLElement | null>(null)
let view: EditorView | null = null

const redlangLanguage = StreamLanguage.define({
  startState: (): null => null,
  token: (stream: StringStream, _state: null): string | null => {
    if (stream.eatSpace()) return null
    if (stream.match(/^\/\/.*/) || stream.match(/^#.*/)) return 'comment'
    if (stream.match(/^【[^】]*】/)) return 'keyword'
    if (stream.match(/^@/)) return 'atom'
    if (stream.match(/^[^【@#\n\r]+/)) return 'string'
    stream.next()
    return null
  },
  languageData: {
    commentTokens: { line: '//' },
  },
})

function createEditor() {
  if (!container.value) return

  const langExt = props.lang === 'lua'
    ? StreamLanguage.define(lua)
    : props.lang === 'javascript'
      ? StreamLanguage.define(javascript)
      : redlangLanguage
  const extensions = [
    basicSetup,
    oneDark,
    langExt,
    EditorView.lineWrapping,
    EditorView.updateListener.of((update) => {
      if (update.docChanged) {
        emit('update:modelValue', update.state.doc.toString())
      }
    }),
  ]

  if (props.readonly) {
    extensions.push(EditorView.editable.of(false))
  }

  const state = EditorState.create({
    doc: props.modelValue,
    extensions,
  })

  view = new EditorView({ state, parent: container.value })
}

function destroyEditor() {
  if (view) {
    view.destroy()
    view = null
  }
}

watch(() => props.modelValue, (newVal) => {
  if (view) {
    const current = view.state.doc.toString()
    if (newVal !== current) {
      view.dispatch({
        changes: { from: 0, to: current.length, insert: newVal },
      })
    }
  }
})

watch(() => props.lang, async () => {
  destroyEditor()
  await nextTick()
  createEditor()
})

onMounted(createEditor)
onUnmounted(destroyEditor)
</script>

<template>
  <div ref="container" class="h-full min-h-[400px] overflow-hidden rounded-lg border border-zinc-200" />
</template>
