// Package webui 提供 Web 管理界面后端。
// 基于 net/http，提供 RESTful API 和 WebSocket 日志推送。
package webui

import (
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/super1207/langlang-go/internal/config"
	"github.com/super1207/langlang-go/internal/log"
	"github.com/super1207/langlang-go/internal/plugin"
	"github.com/super1207/langlang-go/internal/redlang"
)

const wsGUID = "258EAFA5-E914-47DA-95CA-5AB5E6D3C4FD"

// Server Web UI 服务器
type Server struct {
	cfg     *config.Config
	plugins *plugin.Manager
	server  *http.Server
	hub     *Hub
	mu      sync.Mutex
	running bool
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
	mux.Handle("/ws", s.cors(http.HandlerFunc(s.handleWebSocket)))

	// 静态文件
	fileServer := http.FileServer(neuteredFileSystem{http.Dir(s.cfg.Web.StaticDir)})
	mux.Handle("/", s.cors(fileServer))

	s.server = &http.Server{
		Addr:    s.cfg.Web.Listen,
		Handler: mux,
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
		s.server.Close()
	}
	s.running = false
}

// BroadcastLog 广播日志到所有 WebSocket 客户端
func (s *Server) BroadcastLog(level, msg string) {
	data, _ := json.Marshal(map[string]string{
		"type":  "log",
		"level": level,
		"msg":   msg,
		"time":  time.Now().Format("15:04:05.000"),
	})
	s.hub.Broadcast <- data
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

	writeJSON(w, http.StatusOK, map[string]any{
		"code":    0,
		"version": "0.1.0",
		"uptime":  uptime,
		"plugins": s.plugins.Count(),
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
		}
		if err := json.Unmarshal(body, &req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"code": -1, "msg": "invalid json"})
			return
		}
		if err := s.plugins.Save(name, req.Code); err != nil {
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

		if err := s.cfg.Save("config.json"); err != nil {
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
	}
	if err := json.Unmarshal(body, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": -1, "msg": "invalid json"})
		return
	}
	if err := redlang.ValidateCode(req.Code); err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"code": -1, "msg": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": 0, "msg": "ok"})
}

// ==================== WebSocket ====================

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	hj, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "hijacking not supported", http.StatusInternalServerError)
		return
	}

	conn, bufrw, err := hj.Hijack()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 读取客户端 WebSocket 握手请求，提取 Sec-WebSocket-Key
	req, err := http.ReadRequest(bufrw.Reader)
	if err != nil {
		conn.Close()
		return
	}

	clientKey := req.Header.Get("Sec-WebSocket-Key")
	if clientKey == "" {
		conn.Close()
		return
	}

	// 计算 Sec-WebSocket-Accept
	h := sha1.New()
	h.Write([]byte(clientKey + wsGUID))
	acceptKey := base64.StdEncoding.EncodeToString(h.Sum(nil))

	// 发送符合 RFC 6455 的握手响应
	resp := "HTTP/1.1 101 Switching Protocols\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n" +
		"Sec-WebSocket-Accept: " + acceptKey + "\r\n" +
		"\r\n"

	if _, err := bufrw.WriteString(resp); err != nil {
		conn.Close()
		return
	}
	if err := bufrw.Flush(); err != nil {
		conn.Close()
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
			frame := encodeWSFrame(true, 0x1, msg)
			conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if _, err := conn.Write(frame); err != nil {
				return
			}
		}
	}()

	// 读协程：等待客户端发来的 close/ping 帧或连接断开
	readBuf := make([]byte, 1024)
	for {
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		n, err := conn.Read(readBuf)
		if err != nil {
			break
		}
		if n >= 2 {
			opcode := readBuf[0] & 0x0F
			if opcode == 0x8 {
				// 收到 close 帧，回一个 close 帧
				closeFrame := encodeWSFrame(true, 0x8, nil)
				conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
				conn.Write(closeFrame)
				break
			}
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

// neuteredFileSystem 防止直接目录列表
type neuteredFileSystem struct {
	fs http.FileSystem
}

func (nfs neuteredFileSystem) Open(path string) (http.File, error) {
	f, err := nfs.fs.Open(path)
	if err != nil {
		return nil, err
	}
	s, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, err
	}
	if s.IsDir() {
		// 检查是否有 index.html
		indexPath := filepath.Join(path, "index.html")
		if _, err := nfs.fs.Open(indexPath); os.IsNotExist(err) {
			f.Close()
			return nil, err
		}
	}
	return f, nil
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
	conn io.ReadWriteCloser
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

// encodeWSFrame 编码 WebSocket 数据帧（简单实现，不支持分片和掩码）
func encodeWSFrame(fin bool, opcode byte, payload []byte) []byte {
	var frame []byte
	// FIN + opcode
	b := opcode
	if fin {
		b |= 0x80
	}
	frame = append(frame, b)

	// 长度
	l := len(payload)
	if l < 126 {
		frame = append(frame, byte(l))
	} else if l < 65536 {
		frame = append(frame, 126)
		frame = append(frame, byte(l>>8), byte(l))
	} else {
		frame = append(frame, 127)
		for i := 7; i >= 0; i-- {
			frame = append(frame, byte(l>>(i*8)))
		}
	}

	frame = append(frame, payload...)
	return frame
}
