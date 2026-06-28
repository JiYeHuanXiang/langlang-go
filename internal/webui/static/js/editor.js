// CodeMirror 6 编辑器初始化模块
// 通过 importmap 从 CDN 加载 CM6 包

import { EditorView, basicSetup } from 'codemirror'
import { EditorState } from '@codemirror/state'
import { StreamLanguage } from '@codemirror/language'
import { lua } from '@codemirror/lang-lua'
import { oneDark } from '@codemirror/theme-one-dark'

// ========== RedLang StreamLanguage 模式 ==========
//
// RedLang 语法要点:
//   【命令名】@参数1@参数2    — 命令调用
//   // 或 # 到行尾           — 注释
//   其他文本                  — 纯文本
//
const redlangLanguage = StreamLanguage.define({
  startState: () => ({}),
  token: (stream) => {
    // 跳过空白
    if (stream.eatSpace()) return null

    // 注释: // 或 #
    if (stream.match(/^\/\/.*/) || stream.match(/^#.*/)) return 'comment'

    // 命令调用: 【...】
    if (stream.match(/^【[^】]*】/)) return 'keyword'

    // 参数分隔符 @
    if (stream.match(/^@/)) return 'atom'

    // 参数或文本（直到遇到特殊字符）
    if (stream.match(/^[^【@#\n\r]+/)) return 'string'

    // 单个字符兜底
    stream.next()
    return null
  },
  languageData: {
    commentTokens: { line: '//' }
  }
})

// ========== 语言模式映射 ==========
const langExtensions = {
  lua: lua(),
  redlang: redlangLanguage
}

// ========== 对外接口 ==========

/** 当前编辑器实例 */
let currentView = null

/**
 * 创建或替换编辑器
 * @param {HTMLElement} parent    容器元素
 * @param {string}      code      初始代码
 * @param {string}      lang      lua | redlang
 * @param {object}      opts      可选 { readonly, placeholder }
 * @returns {EditorView}
 */
export function createEditor(parent, code, lang, opts = {}) {
  // 销毁旧的编辑器
  destroyEditor()

  const langExt = langExtensions[lang] || redlangLanguage
  const extensions = [
    basicSetup,
    oneDark,
    langExt,
    EditorView.lineWrapping
  ]

  if (opts.readonly) {
    extensions.push(EditorView.editable.of(false))
  }

  const state = EditorState.create({
    doc: code || '',
    extensions
  })

  currentView = new EditorView({
    state,
    parent
  })

  return currentView
}

/** 销毁编辑器 */
export function destroyEditor() {
  if (currentView) {
    currentView.destroy()
    currentView = null
  }
}

/** 获取编辑器内容 */
export function getEditorCode() {
  return currentView ? currentView.state.doc.toString() : ''
}

/** 设置编辑器内容 */
export function setEditorCode(code) {
  if (!currentView) return
  const pos = currentView.state.doc.length
  currentView.dispatch({
    changes: { from: 0, to: pos, insert: code }
  })
}

/** 聚焦编辑器 */
export function focusEditor() {
  if (currentView) currentView.focus()
}

/** 当前语言是否有效 */
export function isValidLang(lang) {
  return lang === 'lua' || lang === 'redlang'
}

// ========== 暴露到 window 供 app.js 的非模块脚本调用 ==========
window.__CM = {
  createEditor,
  destroyEditor,
  getEditorCode,
  setEditorCode,
  focusEditor,
  isValidLang
}
