/* ===== LangLang-Go API 层 ===== */
const API = (() => {
  const BASE = '';

  // 通用 fetch 封装
  async function request(method, path, body) {
    const opts = {
      method,
      headers: { 'Content-Type': 'application/json' },
    };
    if (body !== undefined) {
      opts.body = JSON.stringify(body);
    }
    try {
      const resp = await fetch(BASE + path, opts);
      const data = await resp.json();
      if (data.code !== 0) {
        throw new Error(data.msg || '请求失败');
      }
      return data;
    } catch (err) {
      if (err.message === 'Failed to fetch') {
        throw new Error('无法连接到服务器');
      }
      throw err;
    }
  }

  return {
    // 仪表盘
    async getStatus() {
      return request('GET', '/api/status');
    },

    // 插件列表
    async getPlugins() {
      return request('GET', '/api/plugins');
    },

    // 获取单个插件
    async getPlugin(name) {
      return request('GET', `/api/plugin/${encodeURIComponent(name)}`);
    },

    // 保存插件
    async savePlugin(name, code) {
      return request('POST', `/api/plugin/${encodeURIComponent(name)}`, { code });
    },

    // 删除插件
    async deletePlugin(name) {
      return request('DELETE', `/api/plugin/${encodeURIComponent(name)}`);
    },

    // 创建新插件
    async createPlugin(name, code) {
      return request('POST', `/api/plugin/${encodeURIComponent(name)}`, { code });
    },

    // 获取配置
    async getConfig() {
      return request('GET', '/api/config');
    },

    // 保存配置
    async saveConfig(cfg) {
      return request('POST', '/api/config', cfg);
    },

    // 重载插件
    async reload() {
      return request('POST', '/api/reload');
    },

    // 验证脚本语法
    async validate(code) {
      return request('POST', '/api/validate', { code });
    },

    // WebSocket URL
    wsUrl() {
      const proto = location.protocol === 'https:' ? 'wss:' : 'ws:';
      return `${proto}//${location.host}/ws`;
    },
  };
})();
