import { createApp } from 'vue'
import { createPinia } from 'pinia'
import { createRouter, createWebHashHistory } from 'vue-router'
import App from './App.vue'
import './style.css'

import DashboardPage from './pages/DashboardPage.vue'
import PluginsPage from './pages/PluginsPage.vue'
import EditorPage from './pages/EditorPage.vue'
import LogsPage from './pages/LogsPage.vue'
import DebugPage from './pages/DebugPage.vue'
import SettingsPage from './pages/SettingsPage.vue'

const routes = [
  { path: '/dashboard', component: DashboardPage },
  { path: '/plugins', component: PluginsPage },
  { path: '/editor', component: EditorPage },
  { path: '/logs', component: LogsPage },
  { path: '/debug', component: DebugPage },
  { path: '/settings', component: SettingsPage },
  { path: '/:pathMatch(.*)*', redirect: '/dashboard' },
]

const router = createRouter({
  history: createWebHashHistory(),
  routes,
})

const app = createApp(App)
app.use(createPinia())
app.use(router)
app.mount('#app')
