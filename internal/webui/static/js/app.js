/* ===== LangLang-Go 应用主逻辑 ===== */
(function() {
  'use strict';

  // ==================== 状态 ====================
  const state = {
    currentPage: 'dashboard',
    plugins: [],
    logs: [],
    ws: null,
    wsReconnectTimer: null,
  };

  // ==================== DOM 引用 ====================
  const $ = (sel) => document.querySelector(sel);
  const $$ = (sel) => document.querySelectorAll(sel);
  const content = document.getElementById('content');
  const pageTitle = document.getElementById('pageTitle');

  // ==================== Toast ====================
  function showToast(msg, type = 'info') {
    let container = document.querySelector('.toast-container');
    if (!container) {
      container = document.createElement('div');
      container.className = 'toast-container';
      document.body.appendChild(container);
    }
    const el = document.createElement('div');
    el.className = `toast ${type}`;
    el.textContent = msg;
    container.appendChild(el);
    setTimeout(() => {
      el.style.opacity = '0';
      el.style.transition = 'opacity 0.3s';
      setTimeout(() => el.remove(), 300);
    }, 3000);
  }

  // ==================== Modal ====================
  function showModal({ title, body, footer }) {
    const overlay = document.createElement('div');
    overlay.className = 'modal-overlay';
    overlay.innerHTML = `
      <div class="modal">
        <div class="modal-header">
          <h3>${title}</h3>
          <button class="modal-close" onclick="this.closest('.modal-overlay').remove()">×</button>
        </div>
        <div class="modal-body">${body}</div>
        ${footer ? `<div class="modal-footer">${footer}</div>` : ''}
      </div>`;
    document.body.appendChild(overlay);
    overlay.addEventListener('click', (e) => {
      if (e.target === overlay) overlay.remove();
    });
    return overlay;
  }

  function confirmModal(msg) {
    return new Promise((resolve) => {
      const overlay = showModal({
        title: '确认',
        body: `<p>${msg}</p>`,
        footer: `
          <button class="btn btn-secondary" onclick="this.closest('.modal-overlay').remove(); window._confirmRes(false)">取消</button>
          <button class="btn btn-danger" onclick="this.closest('.modal-overlay').remove(); window._confirmRes(true)">确定</button>`,
      });
      window._confirmRes = resolve;
    });
  }

  // ==================== 加载状态 ====================
  function showLoading() {
    content.innerHTML = '<div class="loading"><div class="spinner"></div><p style="margin-top:12px">加载中...</p></div>';
  }

  function showError(msg) {
    content.innerHTML = `<div class="empty-state"><div class="empty-icon">⚠️</div><p>${msg}</p></div>`;
  }

  // ==================== 路由 ====================
  const pages = {
    dashboard: { title: '仪表盘', render: renderDashboard },
    plugins:   { title: '插件管理', render: renderPlugins },
    editor:    { title: '脚本编辑', render: renderEditor },
    logs:      { title: '运行日志', render: renderLogs },
    settings:  { title: '系统设置', render: renderSettings },
  };

  function navigate(page) {
    if (!pages[page]) page = 'dashboard';
    state.currentPage = page;

    // 更新导航高亮
    $$('.nav-item').forEach(el => {
      el.classList.toggle('active', el.dataset.page === page);
    });

    // 更新标题
    pageTitle.textContent = pages[page].title;

    // 渲染页面
    showLoading();
    setTimeout(() => {
      try {
        pages[page].render();
      } catch (err) {
        showError(`渲染失败: ${err.message}`);
      }
    }, 50);
  }

  // 监听 hash 变化
  window.addEventListener('hashchange', () => {
    const page = location.hash.replace('#/', '') || 'dashboard';
    navigate(page);
  });

  // 侧边栏切换（移动端）
  window.toggleSidebar = function() {
    document.getElementById('sidebar').classList.toggle('open');
  };

  // ==================== 仪表盘 ====================
  async function renderDashboard() {
    try {
      const [statusRes, pluginsRes] = await Promise.all([
        API.getStatus(),
        API.getPlugins(),
      ]);

      const st = statusRes;
      const pluginCount = pluginsRes.plugins ? pluginsRes.plugins.length : 0;
      const enabledCount = pluginsRes.plugins ? pluginsRes.plugins.filter(p => p.enabled).length : 0;
      const testModeOn = st.test_mode === 'on';

      content.innerHTML = `
        <div class="stats-grid">
          <div class="stat-card">
            <div class="stat-value">${pluginCount}</div>
            <div class="stat-label">📦 插件总数</div>
          </div>
          <div class="stat-card">
            <div class="stat-value">${enabledCount}</div>
            <div class="stat-label">✅ 启用中</div>
          </div>
          <div class="stat-card">
            <div class="stat-value">${st.version || '0.1.0'}</div>
            <div class="stat-label">🏷️ 版本号</div>
          </div>
          <div class="stat-card">
            <div class="stat-value">${st.uptime || '刚刚启动'}</div>
            <div class="stat-label">⏱️ 运行时间</div>
          </div>
        </div>

        <div class="card">
          <div class="card-header" style="display:flex;justify-content:space-between;align-items:center">
            <div>
              <span style="font-weight:600">🧪 测试模式</span>
              ${testModeOn ? '<span style="background:#e74c3c;color:#fff;padding:2px 8px;border-radius:4px;font-size:12px;margin-left:8px">开启</span>' : '<span style="background:#666;color:#fff;padding:2px 8px;border-radius:4px;font-size:12px;margin-left:8px">关闭</span>'}
            </div>
            <label class="testmode-toggle" style="cursor:pointer;position:relative;display:inline-block;width:44px;height:24px">
              <input type="checkbox" id="testModeToggle" ${testModeOn ? 'checked' : ''} onchange="toggleTestMode()" style="opacity:0;width:0;height:0" >
              <span style="position:absolute;cursor:pointer;top:0;left:0;right:0;bottom:0;background:${testModeOn ? '#e74c3c' : '#666'};border-radius:24px;transition:.3s">
                <span style="position:absolute;height:18px;width:18px;left:3px;bottom:3px;background:#fff;border-radius:50%;transition:.3s;${testModeOn ? 'transform:translateX(20px)' : ''}"></span>
              </span>
            </label>
          </div>
          <div class="card-body">
            <p style="color:var(--text-secondary);font-size:14px">
              开启后，收到消息不会向外发送，只在日志界面输出结果。
            </p>
          </div>
        </div>

        <div class="card">
          <div class="card-header" style="display:flex;justify-content:space-between;align-items:center">
            <h3>🔗 机器人连接</h3>
            <button class="btn btn-secondary btn-sm" onclick="location.hash='#/settings'">设置</button>
          </div>
          <div class="card-body">
            ${st.bots && st.bots.length > 0
              ? `<div class="table-wrap"><table>
                  <thead><tr><th>平台</th><th>ID</th><th>状态</th><th>操作</th></tr></thead>
                  <tbody>
                    ${st.bots.map(b => `
                      <tr>
                        <td><strong>${b.platform}</strong></td>
                        <td style="font-family:monospace;font-size:13px">${b.self_id || '-'}</td>
                        <td>${b.running
                          ? '<span class="badge badge-success">● 运行中</span>'
                          : '<span class="badge badge-danger">● 已停止</span>'
                        }</td>
                        <td>
                          ${b.running
                            ? `<button class="btn btn-danger btn-sm" onclick="toggleBot('stop','${b.platform}','${b.self_id}')">停止</button>`
                            : `<button class="btn btn-primary btn-sm" onclick="toggleBot('start','${b.platform}','${b.self_id}')">启动</button>`
                          }
                        </td>
                      </tr>`).join('')}
                  </tbody>
                </table></div>`
              : '<div class="empty-state"><p>暂无已配置的机器人连接</p></div>'
            }
          </div>
        </div>

        <div class="card">
          <div class="card-header">
            <h3>🔴 LangLang-Go</h3>
            <div class="btn-group">
              <button class="btn btn-primary btn-sm" onclick="location.hash='#/plugins'">管理插件</button>
              <button class="btn btn-secondary btn-sm" onclick="location.hash='#/logs'">查看日志</button>
            </div>
          </div>
          <div class="card-body">
            <p>欢迎使用 LangLang-Go 管理控制台。</p>
            <p style="margin-top:8px;color:var(--text-secondary)">
              本项目来自 <a href="https://github.com/super1207/redreply" target="_blank" style="color:var(--red-primary)">redlang</a>，
              为方便想法实施而进行 Go 语言重制。
            </p>
            <div style="margin-top:16px;display:flex;gap:12px;flex-wrap:wrap">
              <button class="btn btn-secondary btn-sm" onclick="reloadPlugins()">🔄 重载插件</button>
            </div>
          </div>
        </div>

        <div class="card">
          <div class="card-header"><h3>📋 最近插件</h3></div>
          <div class="card-body">
            ${pluginCount === 0
              ? '<div class="empty-state"><p>暂无插件，点击右上角添加</p></div>'
              : `<div class="table-wrap"><table>
                  <thead><tr><th>名称</th><th>状态</th><th>更新</th><th>操作</th></tr></thead>
                  <tbody>
                    ${(pluginsRes.plugins || []).slice(0, 5).map(p => `
                      <tr>
                        <td><strong>${p.name}</strong></td>
                        <td>${p.enabled ? '<span class="badge badge-success">启用</span>' : '<span class="badge badge-danger">禁用</span>'}</td>
                        <td>${p.updated_at ? new Date(p.updated_at).toLocaleString() : '-'}</td>
                        <td><button class="btn btn-primary btn-sm" onclick="location.hash='#/editor?name=${encodeURIComponent(p.name)}'">编辑</button></td>
                      </tr>`).join('')}
                  </tbody>
                </table></div>`
            }
          </div>
        </div>`;
    } catch (err) {
      showError(`加载仪表盘失败: ${err.message}`);
    }
  }

  // ==================== 插件管理 ====================
  async function renderPlugins() {
    try {
      const res = await API.getPlugins();
      const plugins = res.plugins || [];

      content.innerHTML = `
        <div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:16px;flex-wrap:wrap;gap:8px">
          <div style="display:flex;gap:8px;align-items:center">
            <input class="form-control" id="pluginSearch" placeholder="🔍 搜索插件..." style="width:200px" oninput="filterPlugins()">
          </div>
          <button class="btn btn-primary" onclick="showNewPluginDialog()">➕ 新建插件</button>
        </div>
        <div class="card">
          <div class="card-header">
            <h3>📦 插件列表</h3>
            <span style="color:var(--text-muted);font-size:12px">共 ${plugins.length} 个</span>
          </div>
          <div class="card-body">
            ${plugins.length === 0
              ? '<div class="empty-state"><div class="empty-icon">📦</div><p>还没有插件，点击右上角创建一个</p></div>'
              : `<div class="table-wrap"><table id="pluginTable">
                  <thead><tr><th>名称</th><th>状态</th><th>创建时间</th><th>更新时间</th><th>操作</th></tr></thead>
                  <tbody>
                    ${plugins.map(p => `
                      <tr class="plugin-row" data-name="${p.name}">
                        <td><strong>${p.name}</strong></td>
                        <td>${p.enabled ? '<span class="badge badge-success">启用</span>' : '<span class="badge badge-danger">禁用</span>'}</td>
                        <td>${p.created_at ? new Date(p.created_at).toLocaleString() : '-'}</td>
                        <td>${p.updated_at ? new Date(p.updated_at).toLocaleString() : '-'}</td>
                        <td>
                          <div class="btn-group">
                            <button class="btn btn-primary btn-sm" onclick="location.hash='#/editor?name=${encodeURIComponent(p.name)}'">编辑</button>
                            <button class="btn btn-danger btn-sm" onclick="deletePlugin('${encodeURIComponent(p.name)}')">删除</button>
                          </div>
                        </td>
                      </tr>`).join('')}
                  </tbody>
                </table></div>`
            }
          </div>
        </div>`;
    } catch (err) {
      showError(`加载插件列表失败: ${err.message}`);
    }
  }

  // 搜索过滤
  window.filterPlugins = function() {
    const q = document.getElementById('pluginSearch')?.value?.toLowerCase() || '';
    document.querySelectorAll('.plugin-row').forEach(row => {
      const name = row.dataset.name.toLowerCase();
      row.style.display = name.includes(q) ? '' : 'none';
    });
  };

  // 新建插件
  window.showNewPluginDialog = function() {
    showModal({
      title: '新建插件',
      body: `
        <div class="form-group">
          <label>插件名称</label>
          <input class="form-control" id="newPluginName" placeholder="my-plugin" style="font-family:monospace">
        </div>
        <div class="form-group">
          <label>初始代码（可选）</label>
          <textarea class="form-control" id="newPluginCode" rows="6" placeholder="【输出】@你好世界"></textarea>
        </div>`,
      footer: `
        <button class="btn btn-secondary" onclick="this.closest('.modal-overlay').remove()">取消</button>
        <button class="btn btn-primary" onclick="doCreatePlugin()">创建</button>`,
    });
  };

  window.doCreatePlugin = async function() {
    const name = document.getElementById('newPluginName')?.value?.trim();
    if (!name) { showToast('请输入插件名称', 'error'); return; }
    const code = document.getElementById('newPluginCode')?.value || '';
    try {
      await API.createPlugin(name, code);
      showToast(`插件 "${name}" 创建成功`, 'success');
      document.querySelector('.modal-overlay')?.remove();
      renderPlugins();
    } catch (err) {
      showToast(`创建失败: ${err.message}`, 'error');
    }
  };

  // 删除插件
  window.deletePlugin = async function(name) {
    name = decodeURIComponent(name);
    const ok = await confirmModal(`确定删除插件 "${name}" 吗？此操作不可撤销。`);
    if (!ok) return;
    try {
      await API.deletePlugin(name);
      showToast(`插件 "${name}" 已删除`, 'success');
      renderPlugins();
    } catch (err) {
      showToast(`删除失败: ${err.message}`, 'error');
    }
  };

  // 重载插件
  window.reloadPlugins = async function() {
    try {
      await API.reload();
      showToast('插件已重载', 'success');
    } catch (err) {
      showToast(`重载失败: ${err.message}`, 'error');
    }
  };

  // ==================== 脚本编辑 ====================
  let currentEditorPlugin = '';
  let editorCode = '';
  let editDirty = false;

  async function renderEditor() {
    const params = new URLSearchParams(location.hash.split('?')[1] || '');
    const pluginName = params.get('name') || '';

    // 获取插件列表供选择
    let plugins = [];
    try {
      const res = await API.getPlugins();
      plugins = res.plugins || [];
    } catch (_) {}

    let code = '';
    if (pluginName) {
      try {
        const res = await API.getPlugin(pluginName);
        code = res.data?.code || '';
      } catch (_) {}
    }

    currentEditorPlugin = pluginName;
    editorCode = code;

    content.innerHTML = `
      <div class="editor-container">
        <div class="editor-toolbar">
          <select class="form-control" id="editorPluginSelect" onchange="switchEditorPlugin()">
            <option value="">— 选择或输入插件名 —</option>
            ${plugins.map(p => `<option value="${p.name}" ${p.name === pluginName ? 'selected' : ''}>${p.name}</option>`).join('')}
          </select>
          <input class="form-control" id="editorPluginName" placeholder="插件名称" style="width:180px;font-family:monospace" value="${pluginName}">
          <div class="btn-group">
            <button class="btn btn-primary" onclick="saveEditorCode()">💾 保存</button>
            <button class="btn btn-secondary" onclick="formatEditorCode()">✨ 格式化</button>
            <button class="btn btn-secondary" onclick="validateEditorCode()">✅ 验证</button>
          </div>
        </div>
        <div class="editor-area">
          <textarea id="editorTextarea" spellcheck="false" oninput="onEditorChange()">${escapeHtml(code)}</textarea>
        </div>
        <div id="editorStatus" style="padding:8px 0;font-size:12px;color:var(--text-muted);flex-shrink:0"></div>
      </div>`;

    document.getElementById('editorTextarea').focus();
  }

  window.switchEditorPlugin = function() {
    const name = document.getElementById('editorPluginSelect').value;
    const currentCode = document.getElementById('editorTextarea')?.value;
    // 有未保存修改时先提示
    if (editDirty && currentCode !== undefined && currentCode !== editorCode) {
      if (!confirm('当前脚本有未保存的修改，确定切换吗？')) return;
    }
    editDirty = false;
    document.getElementById('editorPluginName').value = name;
    if (name) {
      location.hash = `#/editor?name=${encodeURIComponent(name)}`;
    }
  };

  window.onEditorChange = function() {
    editDirty = true;
    editorCode = document.getElementById('editorTextarea').value;
    const status = document.getElementById('editorStatus');
    const lines = editorCode.split('\n').length;
    const bytes = new Blob([editorCode]).size;
    status.textContent = `📄 ${lines} 行 | ${bytes} 字节`;
  };

  window.saveEditorCode = async function() {
    const name = document.getElementById('editorPluginName').value.trim();
    if (!name) { showToast('请输入插件名称', 'error'); return; }
    const code = document.getElementById('editorTextarea').value;
    try {
      await API.savePlugin(name, code);
      editDirty = false;
      showToast(`插件 "${name}" 已保存`, 'success');
      location.hash = `#/editor?name=${encodeURIComponent(name)}`;
    } catch (err) {
      showToast(`保存失败: ${err.message}`, 'error');
    }
  };

  window.validateEditorCode = async function() {
    const code = document.getElementById('editorTextarea').value;
    try {
      await API.validate(code);
      showToast('语法验证通过 ✅', 'success');
    } catch (err) {
      showToast(`语法错误: ${err.message}`, 'error');
    }
  };

  window.formatEditorCode = function() {
    // 简单格式化：统一缩进
    const ta = document.getElementById('editorTextarea');
    let code = ta.value;
    // 去除多余空行
    code = code.replace(/\n{3,}/g, '\n\n');
    ta.value = code;
    showToast('已格式化', 'info');
  };

  // ==================== 日志查看 ====================
  let logLines = [];

  async function renderLogs() {
    content.innerHTML = `
      <div>
        <div class="log-toolbar">
          <button class="btn btn-secondary btn-sm" onclick="clearLogs()">🗑️ 清除</button>
          <button class="btn btn-secondary btn-sm" onclick="copyLogs()">📋 复制</button>
          <label style="display:flex;align-items:center;gap:4px;font-size:12px;color:var(--text-secondary)">
            <input type="checkbox" id="logAutoScroll" checked> 自动滚动
          </label>
          <label style="display:flex;align-items:center;gap:4px;font-size:12px;color:var(--text-secondary)">
            <input type="checkbox" id="logFilterInfo" checked> INFO
          </label>
          <label style="display:flex;align-items:center;gap:4px;font-size:12px;color:var(--text-secondary)">
            <input type="checkbox" id="logFilterWarn" checked> WARN
          </label>
          <label style="display:flex;align-items:center;gap:4px;font-size:12px;color:var(--text-secondary)">
            <input type="checkbox" id="logFilterError" checked> ERROR
          </label>
        </div>
        <div class="log-container" id="logContainer"></div>
      </div>`;

    logLines = [];
    connectWs();
  }

  window.clearLogs = function() {
    logLines = [];
    document.getElementById('logContainer').innerHTML = '';
  };

  window.copyLogs = function() {
    const text = logLines.map(l => l.text).join('\n');
    navigator.clipboard.writeText(text).then(() => showToast('已复制到剪贴板', 'success'));
  };

  function appendLog(level, msg, time) {
    const line = { level, msg, time, text: `[${time}] [${level.toUpperCase()}] ${msg}` };
    logLines.push(line);

    const container = document.getElementById('logContainer');
    if (!container) return;

    // 过滤
    const showInfo = document.getElementById('logFilterInfo')?.checked !== false;
    const showWarn = document.getElementById('logFilterWarn')?.checked !== false;
    const showError = document.getElementById('logFilterError')?.checked !== false;
    if (level === 'info' && !showInfo) return;
    if (level === 'warn' && !showWarn) return;
    if (level === 'error' && !showError) return;

    const el = document.createElement('div');
    el.className = `log-line ${level}`;
    el.textContent = line.text;
    container.appendChild(el);

    // 自动滚动
    if (document.getElementById('logAutoScroll')?.checked) {
      container.scrollTop = container.scrollHeight;
    }
  }

  // ==================== WebSocket ====================
  let wsCloseRequested = false;

  function disconnectWs() {
    wsCloseRequested = true;
    if (state.ws) {
      state.ws.onclose = null;
      state.ws.close();
      state.ws = null;
    }
    if (state.wsReconnectTimer) {
      clearTimeout(state.wsReconnectTimer);
      state.wsReconnectTimer = null;
    }
  }

  function connectWs() {
    // 先断旧连接，避免重复
    disconnectWs();
    wsCloseRequested = false;

    try {
      const ws = new WebSocket(API.wsUrl());
      state.ws = ws;

      ws.onopen = () => {
        appendLog('info', '已连接到日志流', new Date().toLocaleTimeString());
      };

      ws.onmessage = (e) => {
        try {
          const data = JSON.parse(e.data);
          if (data.type === 'log') {
            appendLog(data.level || 'info', data.msg || '', data.time || '');
          }
        } catch (_) {}
      };

      ws.onclose = () => {
        state.ws = null;
        if (!wsCloseRequested && state.currentPage === 'logs') {
          state.wsReconnectTimer = setTimeout(() => {
            state.wsReconnectTimer = null;
            connectWs();
          }, 3000);
        }
      };

      ws.onerror = () => {
        ws.close();
      };
    } catch (_) {}
  }

  // ==================== 设置 ====================
  async function renderSettings() {
    content.innerHTML = `
      <div class="card" style="max-width:600px">
        <div class="card-header"><h3>⚙️ 系统设置</h3></div>
        <div class="card-body">
          <div class="loading"><div class="spinner"></div><p style="margin-top:12px">加载配置...</p></div>
        </div>
      </div>`;

    try {
      const [res, tmRes] = await Promise.all([
        API.getConfig(),
        API.getTestMode(),
      ]);
      const cfg = res.config || {};
      const testOn = tmRes.test_mode === 'on';

      document.querySelector('.card-body').innerHTML = `
        <div class="settings-section">
          <div class="form-group">
            <label>Web 监听地址</label>
            <input class="form-control" id="cfgListen" value="${cfg.web?.listen || ':2397'}">
          </div>
          <div class="form-group">
            <label>测试模式开关</label>
            <button class="btn ${testOn ? 'btn-danger' : 'btn-secondary'} btn-sm" id="testModeBtn" onclick="toggleTestModeSettings()">
              ${testOn ? '🧪 关闭测试模式' : '🧪 开启测试模式'}
            </button>
            <span style="font-size:12px;color:var(--text-muted);margin-left:8px">开启后不向外发送消息</span>
          </div>
          <div class="form-group">
            <label>访问令牌（留空=无鉴权）</label>
            <input class="form-control" id="cfgToken" value="${cfg.web?.access_token || ''}" placeholder="留空则不设密码">
          </div>
          <div class="form-group">
            <label>日志级别</label>
            <select class="form-control" id="cfgLogLevel">
              <option value="debug" ${cfg.log?.level === 'debug' ? 'selected' : ''}>DEBUG</option>
              <option value="info" ${!cfg.log?.level || cfg.log.level === 'info' ? 'selected' : ''}>INFO</option>
              <option value="warn" ${cfg.log?.level === 'warn' ? 'selected' : ''}>WARN</option>
              <option value="error" ${cfg.log?.level === 'error' ? 'selected' : ''}>ERROR</option>
            </select>
          </div>
          <div class="form-group">
            <label>跳过 N 分钟前的消息</label>
            <input class="form-control" id="cfgSkipMsg" type="number" value="${cfg.core?.skip_msg_minutes || 10}">
          </div>
          <div class="form-group">
            <label>数据目录</label>
            <input class="form-control" id="cfgDataDir" value="${cfg.paths?.data || 'data'}">
          </div>
          <div style="margin-top:20px">
            <button class="btn btn-primary" onclick="saveSettings()">💾 保存设置</button>
            <span id="saveSettingsResult" style="margin-left:8px;font-size:12px"></span>
          </div>
        </div>`;
    } catch (err) {
      document.querySelector('.card-body').innerHTML = `<p style="color:var(--red-primary)">加载配置失败: ${err.message}</p>`;
    }
  }

  // ==================== 测试模式 ====================
  // ==================== Bot 控制 ====================
  window.toggleBot = async function(action, platform, selfID) {
    try {
      await API.botControl(action, platform, selfID);
      showToast(action === 'stop' ? '⏹️ 已停止' : '▶️ 启动中');
      setTimeout(() => navigate('dashboard'), 1000);
    } catch (err) {
      showToast(`操作失败: ${err.message}`, 'error');
    }
  };

  window.toggleTestMode = async function() {
    const toggle = document.getElementById('testModeToggle');
    // onchange fires after the checkbox has already toggled, so checked is the new state
    const enabled = toggle ? toggle.checked : true;
    try {
      await API.setTestMode(enabled);
      showToast(enabled ? '🧪 测试模式已开启 — 不会向外发送消息' : '✅ 测试模式已关闭 — 恢复正常发送');
      if (state.currentPage === 'dashboard') {
        navigate('dashboard');
      }
    } catch (err) {
      showToast(`切换失败: ${err.message}`, 'error');
    }
  };

  window.toggleTestModeSettings = async function() {
    const btn = document.getElementById('testModeBtn');
    const currentText = btn ? btn.textContent.trim() : '';
    const nowOn = currentText.includes('关闭');
    const enabled = !nowOn;
    try {
      await API.setTestMode(enabled);
      showToast(enabled ? '🧪 测试模式已开启' : '✅ 测试模式已关闭');
      navigate('settings');
    } catch (err) {
      showToast(`切换失败: ${err.message}`, 'error');
    }
  };

  window.saveSettings = async function() {
    const btn = document.querySelector('.btn-primary');
    btn.disabled = true;
    btn.textContent = '保存中...';

    try {
      const cfg = {
        web: {
          listen: document.getElementById('cfgListen')?.value || ':2397',
          access_token: document.getElementById('cfgToken')?.value || '',
        },
        log: {
          level: document.getElementById('cfgLogLevel')?.value || 'info',
        },
        core: {
          skip_msg_minutes: parseInt(document.getElementById('cfgSkipMsg')?.value) || 10,
        },
        paths: {
          data: document.getElementById('cfgDataDir')?.value || 'data',
        },
      };
      await API.saveConfig(cfg);
      const el = document.getElementById('saveSettingsResult');
      el.textContent = '✅ 已保存';
      el.style.color = '#27ae60';
      setTimeout(() => { el.textContent = ''; }, 3000);
    } catch (err) {
      const el = document.getElementById('saveSettingsResult');
      el.textContent = `❌ ${err.message}`;
      el.style.color = '#e74c3c';
    } finally {
      btn.disabled = false;
      btn.textContent = '💾 保存设置';
    }
  };

  // ==================== 工具函数 ====================
  function escapeHtml(str) {
    if (!str) return '';
    return str.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
  }

  // ==================== 初始化 ====================
  function init() {
    const page = location.hash.replace('#/', '') || 'dashboard';
    navigate(page);
  }

  // 页面完全加载后初始化
  if (document.readyState === 'complete') {
    init();
  } else {
    window.addEventListener('load', init);
  }

})();
