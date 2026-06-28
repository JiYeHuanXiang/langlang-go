// Package webui 提供 Web 管理界面后端。
// 基于 net/http，提供 RESTful API 和 WebSocket 日志推送。
package webui

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"github.com/super1207/langlang-go/internal/bot"
	"github.com/super1207/langlang-go/internal/config"
	"github.com/super1207/langlang-go/internal/db"
	"github.com/super1207/langlang-go/internal/event"
	"github.com/super1207/langlang-go/internal/log"
	"github.com/super1207/langlang-go/internal/lua"
	"github.com/super1207/langlang-go/internal/plugin"
	"github.com/super1207/langlang-go/internal/redlang"
)

//go:embed all:static
var staticFiles embed.FS

// Server Web UI 服务器
type Server struct {
	cfg          *config.Config
	configPath   string
	plugins      *plugin.Manager
	db           *db.Manager
	botRegistry  *bot.Registry
	botAdapters  []bot.BotAdapter
	server       *http.Server
	hub          *Hub
	mu           sync.Mutex
	running      bool
}

// NewServer 创建 Web UI 服务器
func NewServer(cfg *config.Config, pm *plugin.Manager) *Server {
	return &Server{
		cfg:     cfg,
		plugins: pm,
		hub:     NewHub(),
	}
}

// Start 启动服务器
func (s *Server) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running {
		return nil
	}

	// 注册日志广播器，使 log.Info/Warn/Error 通过 WebSocket 推送到前端
	log.SetBroadcaster(s.BroadcastLog)

	mux := http.NewServeMux()

	// API 路由（cors → auth → handler）
	apiAuth := func(h http.HandlerFunc) http.Handler {
		return s.cors(s.auth(http.HandlerFunc(h)))
	}
	mux.Handle("/api/status", apiAuth(s.handleStatus))
	mux.Handle("/api/plugins", apiAuth(s.handlePlugins))
	mux.Handle("/api/plugin/", apiAuth(s.handlePlugin))
	mux.Handle("/api/config", apiAuth(s.handleConfig))
	mux.Handle("/api/reload", apiAuth(s.handleReload))
	mux.Handle("/api/validate", apiAuth(s.handleValidate))
	mux.Handle("/api/messages", apiAuth(s.handleMessages))
	mux.Handle("/api/bot/", s.cors(http.HandlerFunc(s.handleBot)))
	mux.Handle("/api/testmode", apiAuth(s.handleTestMode))
	mux.Handle("/ws", s.cors(http.HandlerFunc(s.handleWebSocket)))
	mux.Handle("/api/debug/message", apiAuth(s.handleDebugMessage))
	mux.Handle("/api/log/recent", apiAuth(s.handleLogRecent))

	// 静态文件（从嵌入的 FS 中读取，无需外部目录）
	mux.Handle("/", s.cors(http.HandlerFunc(s.serveStatic)))

	s.server = &http.Server{
		Addr:              s.cfg.Web.Listen,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go s.hub.Run()

	go func() {
		log.Info("Web UI 启动", "addr", s.cfg.Web.Listen)
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("HTTP Server 错误", "error", err)
		}
	}()

	s.running = true
	startTime = time.Now()
	return nil
}

// Stop 停止服务器
func (s *Server) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		s.server.Shutdown(ctx)
		s.server = nil
	}
	s.running = false
}

// BroadcastLog 广播日志到所有 WebSocket 客户端
// SetDB 设置数据库管理器引用
func (s *Server) SetDB(database *db.Manager) {
	s.db = database
}

// SetBotControl 设置机器人连接器列表引用
func (s *Server) SetBotControl(registry *bot.Registry, adapters []bot.BotAdapter) {
	s.botRegistry = registry
	s.botAdapters = adapters
}

// SetConfigPath 设置配置文件路径（用于保存时写回正确文件）
func (s *Server) SetConfigPath(path string) {
	s.configPath = path
}

func (s *Server) BroadcastLog(level, msg string) {
	data, _ := json.Marshal(map[string]string{
		"type":  "log",
		"level": level,
		"msg":   msg,
		"time":  time.Now().Format("15:04:05.000"),
	})
	select {
	case s.hub.Broadcast <- data:
	default:
	}
}

var startTime time.Time

// ==================== CORS 中间件 ====================

func (s *Server) cors(next http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	}
}

// ==================== Auth 中间件 ====================

func (s *Server) auth(next http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// access_token 为空时不校验（本地部署）
		if s.cfg.Web.AccessToken == "" {
			next.ServeHTTP(w, r)
			return
		}

		header := r.Header.Get("Authorization")
		if header == "" {
			writeJSON(w, http.StatusUnauthorized, map[string]any{"code": -1, "msg": "missing Authorization header"})
			return
		}

		// 支持 "Bearer <token>" 和直接 "<token>" 两种格式
		token := header
		if len(header) > 7 && header[:7] == "Bearer " {
			token = header[7:]
		}

		if token != s.cfg.Web.AccessToken {
			writeJSON(w, http.StatusUnauthorized, map[string]any{"code": -1, "msg": "invalid access token"})
			return
		}

		next.ServeHTTP(w, r)
	}
}



// ==================== API: 系统状态 ====================

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"code": -1, "msg": "method not allowed"})
		return
	}

	uptime := "刚刚启动"
	if !startTime.IsZero() {
		d := time.Since(startTime)
		uptime = fmt.Sprintf("%dh %dm %ds", int(d.Hours()), int(d.Minutes())%60, int(d.Seconds())%60)
	}

	var testModeStr string
	if bot.TestMode {
		testModeStr = "on"
	} else {
		testModeStr = "off"
	}

	// 收集机器人连接状态
	type botInfo struct {
		Platform string `json:"platform"`
		SelfID   string `json:"self_id"`
		Running  bool   `json:"running"`
	}
	bots := make([]botInfo, 0, len(s.botAdapters))
	for _, a := range s.botAdapters {
		bots = append(bots, botInfo{
			Platform: a.Platform(),
			SelfID:   a.SelfID(),
			Running:  a.Running(),
		})
	}

		// 收集已配置的平台列表
	configuredPlatforms := make([]string, 0)
	if len(s.cfg.Bot.OneBot11) > 0 {
		configuredPlatforms = append(configuredPlatforms, "onebot11")
	}
	if len(s.cfg.Bot.Telegram) > 0 {
		configuredPlatforms = append(configuredPlatforms, "telegram")
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"code":                 0,
		"version":              "0.1.0",
		"uptime":               uptime,
		"plugins":              s.plugins.Count(),
		"test_mode":            testModeStr,
		"bots":                 bots,
		"configured_platforms": configuredPlatforms,
	})
}

// ==================== API: 插件列表 ====================

func (s *Server) handlePlugins(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"code": -1, "msg": "method not allowed"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"code":    0,
		"plugins": s.plugins.List(),
	})
}

// ==================== API: 单个插件 ====================

func (s *Server) handlePlugin(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/api/plugin/")
	name = strings.TrimSuffix(name, "/")
	if name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": -1, "msg": "missing plugin name"})
		return
	}

	switch r.Method {
	case http.MethodGet:
		pkg := s.plugins.Get(name)
		if pkg == nil {
			writeJSON(w, http.StatusNotFound, map[string]any{"code": -1, "msg": "plugin not found"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"code": 0, "data": pkg})

	case http.MethodPost:
		body, err := io.ReadAll(r.Body)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"code": -1, "msg": "read body failed"})
			return
		}
		var req struct {
			Code string `json:"code"`
			Lang string `json:"lang"`
		}
		if err := json.Unmarshal(body, &req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"code": -1, "msg": "invalid json"})
			return
		}
		if err := s.plugins.Save(name, req.Code, req.Lang); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"code": -1, "msg": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"code": 0, "msg": "saved"})

	case http.MethodDelete:
		s.plugins.Delete(name)
		writeJSON(w, http.StatusOK, map[string]any{"code": 0, "msg": "deleted"})

	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"code": -1, "msg": "method not allowed"})
	}
}

// ==================== API: 配置 ====================

func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]any{
			"code":   0,
			"config": s.cfg,
		})

	case http.MethodPost:
		body, err := io.ReadAll(r.Body)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"code": -1, "msg": "read body failed"})
			return
		}
		// 先用 raw map 判断哪些 key 被显式传入（区分 "设为空" 与 "未提供"）
		var raw map[string]json.RawMessage
		if err := json.Unmarshal(body, &raw); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"code": -1, "msg": "invalid json"})
			return
		}

		if webData, ok := raw["web"]; ok {
			var wc config.WebConfig
			if err := json.Unmarshal(webData, &wc); err == nil {
				if _, exists := hasKey(webData, "listen"); exists {
					s.cfg.Web.Listen = wc.Listen
				}
				if _, exists := hasKey(webData, "access_token"); exists {
					s.cfg.Web.AccessToken = wc.AccessToken
				}
			}
		}

		if logData, ok := raw["log"]; ok {
			var lc config.LogConfig
			if err := json.Unmarshal(logData, &lc); err == nil {
				if _, exists := hasKey(logData, "level"); exists {
					s.cfg.Log.Level = lc.Level
				}
			}
		}

		if coreData, ok := raw["core"]; ok {
			var cc config.CoreConfig
			if err := json.Unmarshal(coreData, &cc); err == nil {
				if _, exists := hasKey(coreData, "skip_msg_minutes"); exists {
					s.cfg.Core.SkipMsgMinutes = cc.SkipMsgMinutes
				}
			}
		}

		if pathsData, ok := raw["paths"]; ok {
			var pc config.PathsConfig
			if err := json.Unmarshal(pathsData, &pc); err == nil {
				if _, exists := hasKey(pathsData, "data"); exists {
					s.cfg.Paths.Data = pc.Data
				}
			}
		}

		if botData, ok := raw["bot"]; ok {
			if onebot11Raw, has := hasKey(botData, "onebot11"); has {
				var onebotCfgs []config.OneBot11Config
				if err := json.Unmarshal(onebot11Raw, &onebotCfgs); err == nil {
					s.cfg.Bot.OneBot11 = onebotCfgs
				}
			}
			if telegramRaw, has := hasKey(botData, "telegram"); has {
				var telegrams []string
				if err := json.Unmarshal(telegramRaw, &telegrams); err == nil {
					s.cfg.Bot.Telegram = telegrams
				}
			}
		}

		if err := s.cfg.Save(s.configPath); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"code": -1, "msg": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"code": 0, "msg": "配置已保存"})

	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"code": -1, "msg": "method not allowed"})
	}
}

// ==================== API: 重载 ====================

func (s *Server) handleReload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"code": -1, "msg": "method not allowed"})
		return
	}
	if err := s.plugins.LoadAll(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"code": -1, "msg": err.Error()})
		return
	}
	s.BroadcastLog("info", "插件已重载")
	writeJSON(w, http.StatusOK, map[string]any{"code": 0, "msg": "reloaded"})
}

// ==================== API: 验证 ====================

func (s *Server) handleValidate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"code": -1, "msg": "method not allowed"})
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": -1, "msg": "read body failed"})
		return
	}
	var req struct {
		Code string `json:"code"`
		Lang string `json:"lang"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": -1, "msg": "invalid json"})
		return
	}
	var validateErr error
	switch req.Lang {
	case "lua":
		validateErr = lua.ValidateLua(req.Code)
	default:
		// 默认使用 RedLang 校验（兼容旧版本未传 lang 字段）
		validateErr = redlang.ValidateCode(req.Code)
	}
	if validateErr != nil {
		writeJSON(w, http.StatusOK, map[string]any{"code": -1, "msg": validateErr.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": 0, "msg": "ok"})
}

// ==================== API: 消息查询 ====================

func (s *Server) handleMessages(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"code": -1, "msg": "method not allowed"})
		return
	}
	if s.db == nil {
		writeJSON(w, http.StatusOK, map[string]any{"code": -1, "msg": "database not available"})
		return
	}

	q := r.URL.Query()
	filter := make(map[string]any)
	for _, key := range []string{"platform", "user_id", "group_id", "event_type", "message_type"} {
		if v := q.Get(key); v != "" {
			filter[key] = v
		}
	}

	msgs, err := s.db.QueryMsg(filter)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"code": -1, "msg": err.Error()})
		return
	}

	// 分页
	limit := 50
	offset := 0
	if v := q.Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 200 {
			limit = n
		}
	}
	if v := q.Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}

	if offset > len(msgs) {
		msgs = nil
	} else if offset+limit > len(msgs) {
		msgs = msgs[offset:]
	} else {
		msgs = msgs[offset : offset+limit]
	}

	if msgs == nil {
		msgs = []map[string]any{}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"code": 0,
		"data": msgs,
	})
}

// ==================== API: Bot 控制 ====================

func (s *Server) handleBot(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"code": -1, "msg": "method not allowed"})
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": -1, "msg": "read body failed"})
		return
	}
	var req struct {
		Action   string `json:"action"`   // "start" 或 "stop"
		Platform string `json:"platform"`
		SelfID   string `json:"self_id"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": -1, "msg": "invalid json"})
		return
	}

	if req.Action == "" || req.Platform == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": -1, "msg": "action and platform required"})
		return
	}

	if s.botRegistry == nil {
		writeJSON(w, http.StatusOK, map[string]any{"code": -1, "msg": "bot registry not available"})
		return
	}

	adapter, ok := s.botRegistry.Get(req.Platform, req.SelfID)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{"code": -1, "msg": "adapter not found"})
		return
	}

	switch req.Action {
	case "stop":
		adapter.Stop()
		writeJSON(w, http.StatusOK, map[string]any{"code": 0, "msg": "stopped"})
	case "start":
		go func() {
			if err := adapter.Start(); err != nil {
				log.Error("Bot 启动失败", "platform", req.Platform, "self_id", req.SelfID, "error", err)
			}
		}()
		writeJSON(w, http.StatusOK, map[string]any{"code": 0, "msg": "starting"})
	default:
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": -1, "msg": "unknown action: " + req.Action})
	}
}

// ==================== API: 测试模式 ====================

func (s *Server) handleTestMode(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		status := "off"
		if bot.TestMode {
			status = "on"
		}
		writeJSON(w, http.StatusOK, map[string]any{"code": 0, "test_mode": status})

	case http.MethodPost:
		body, err := io.ReadAll(r.Body)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"code": -1, "msg": "read body failed"})
			return
		}
		var req struct {
			Enabled bool `json:"enabled"`
		}
		if err := json.Unmarshal(body, &req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"code": -1, "msg": "invalid json"})
			return
		}

		bot.TestMode = req.Enabled
		status := "off"
		if bot.TestMode {
			status = "on"
		}
		log.Info("测试模式已切换", "test_mode", status)
		writeJSON(w, http.StatusOK, map[string]any{"code": 0, "test_mode": status})

	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"code": -1, "msg": "method not allowed"})
	}
}

// ==================== API: 调试消息 ====================

func (s *Server) handleDebugMessage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"code": -1, "msg": "method not allowed"})
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": -1, "msg": "read body failed"})
		return
	}

	var req struct {
		Platform    string `json:"platform"`
		Message     string `json:"message"`
		UserID      string `json:"user_id"`
		GroupID     string `json:"group_id"`
		MessageType string `json:"message_type"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": -1, "msg": "invalid json"})
		return
	}

	if req.Message == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": -1, "msg": "message is required"})
		return
	}

	platform := req.Platform
	if platform == "" {
		platform = "debug"
	}
	userID := req.UserID
	if userID == "" {
		userID = "debug_user"
	}
	groupID := req.GroupID
	if groupID == "" {
		groupID = "debug_group"
	}
	messageType := req.MessageType
	if messageType == "" {
		messageType = "private"
	}

	// 构造事件 JSON，适配 onebot11 / telegram / debug 格式
	var evtMap map[string]any
	switch platform {
	case "telegram":
		evtMap = map[string]any{
			"self_id":      "debug_bot",
			"message_id":   fmt.Sprintf("debug_%d", time.Now().UnixNano()),
			"user_id":      userID,
			"group_id":     groupID,
			"message":      req.Message,
			"raw_message":  req.Message,
			"message_type": messageType,
			"post_type":    "message",
			"platform":     "telegram",
			"time":         time.Now().Unix(),
		}
	default:
		// onebot11 / debug 通用格式
		evtMap = map[string]any{
			"self_id":      "debug_bot",
			"message_id":   fmt.Sprintf("debug_%d", time.Now().UnixNano()),
			"user_id":      userID,
			"group_id":     groupID,
			"message":      req.Message,
			"raw_message":  req.Message,
			"message_type": messageType,
			"post_type":    "message",
			"platform":     platform,
			"time":         time.Now().Unix(),
		}
	}

	data, err := json.Marshal(evtMap)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"code": -1, "msg": "marshal event failed"})
		return
	}

	// 分发到事件系统
	if err := event.GlobalDispatcher.DispatchJSON(platform, data); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"code": -1, "msg": fmt.Sprintf("dispatch failed: %v", err)})
		return
	}

	log.Info("调试消息已分发",
		"platform", platform,
		"user_id", userID,
		"group_id", groupID,
		"message_type", messageType,
		"message", req.Message,
	)

	writeJSON(w, http.StatusOK, map[string]any{
		"code": 0,
		"msg":  "调试消息已发送",
	})
}

// ==================== API: 最近日志 ====================

func (s *Server) handleLogRecent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"code": -1, "msg": "method not allowed"})
		return
	}
	entries := log.RecentEntries(100)
	writeJSON(w, http.StatusOK, map[string]any{
		"code": 0,
		"logs": entries,
	})
}

// ==================== WebSocket ====================

var wsUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Error("WebSocket 握手失败", "error", err)
		return
	}

	client := &WSClient{
		conn: conn,
		send: make(chan []byte, 64),
	}

	s.hub.Register <- client

	// 写协程：发送日志帧
	go func() {
		defer func() {
			conn.Close()
			s.hub.Unregister <- client
		}()
		for msg := range client.send {
			conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		}
	}()

	// 读协程：维持连接，自动处理 ping/pong/close 帧
	conn.SetReadLimit(4096)
	for {
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		if _, _, err := conn.ReadMessage(); err != nil {
			break
		}
	}
}

// ==================== 工具函数 ====================

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// hasKey 检查一段 JSON 是否包含指定 key
func hasKey(data json.RawMessage, key string) (json.RawMessage, bool) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, false
	}
	v, ok := m[key]
	return v, ok
}

// serveStatic 从嵌入的 embed.FS 中读取静态文件
func (s *Server) serveStatic(w http.ResponseWriter, r *http.Request) {
	filePath := r.URL.Path
	if filePath == "/" {
		filePath = "/index.html"
	}
	// 嵌入路径前缀 static/
	data, err := staticFiles.ReadFile("static" + filePath)
	if err != nil {
		if os.IsNotExist(err) {
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprintf(w, "404 not found: %s", r.URL.Path)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "500: %v", err)
		return
	}
	// 根据扩展名设置 Content-Type
	ext := path.Ext(filePath)
	switch ext {
	case ".html":
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
	case ".css":
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
	case ".js":
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	case ".png":
		w.Header().Set("Content-Type", "image/png")
	case ".ico":
		w.Header().Set("Content-Type", "image/x-icon")
	case ".svg":
		w.Header().Set("Content-Type", "image/svg+xml")
	}
	w.Write(data)
}

// ==================== WebSocket 辅助 ====================

// Hub 管理 WebSocket 客户端
type Hub struct {
	clients    map[*WSClient]bool
	Broadcast  chan []byte
	Register   chan *WSClient
	Unregister chan *WSClient
}

type WSClient struct {
	conn *websocket.Conn
	send chan []byte
}

func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*WSClient]bool),
		Broadcast:  make(chan []byte, 256),
		Register:   make(chan *WSClient),
		Unregister: make(chan *WSClient),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.Register:
			h.clients[client] = true

		case client := <-h.Unregister:
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}

		case message := <-h.Broadcast:
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					delete(h.clients, client)
					close(client.send)
				}
			}
		}
	}
}


