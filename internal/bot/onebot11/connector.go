// Package onebot11 实现 OneBot 11 协议。
// 同时支持反向 WebSocket（连接远程服务器）和正向 WebSocket（等待客户端连接）。
package onebot11

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"github.com/super1207/langlang-go/internal/bot"
	"github.com/super1207/langlang-go/internal/log"
)

// Connector OneBot 11 连接器
type Connector struct {
	mode        string // "reverse" 或 "forward"
	url         string // reverse: ws://..., forward: 监听地址
	accessToken string
	selfID      string
	conn        *websocket.Conn
	mu          sync.Mutex
	running     bool
	stopCh      chan struct{}
	dialer      *websocket.Dialer
	server      *http.Server // forward 模式用的 HTTP 服务

	// echo 应答机制
	seqCounter     int64
	pendingMu      sync.Mutex
	pendingReqs    map[int64]chan map[string]any
}

// NewConnector 创建 OneBot 11 连接器
// mode: "reverse"（反向 WS）或 "forward"（正向 WS）
func NewConnector(mode, addr, accessToken, selfID string) *Connector {
	return &Connector{
		mode:        mode,
		url:         addr,
		accessToken: accessToken,
		selfID:      selfID,
		stopCh:      make(chan struct{}),
		dialer: &websocket.Dialer{
			HandshakeTimeout: 10 * time.Second,
		},
		pendingReqs: make(map[int64]chan map[string]any),
	}
}

// Platform 返回平台标识
func (c *Connector) Platform() string { return "onebot11" }

// SelfID 返回机器人 ID
func (c *Connector) SelfID() string { return c.selfID }

// Running 返回连接是否正在运行
func (c *Connector) Running() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.running
}

// Start 启动连接
// reverse 模式：连接远程 WebSocket 服务器
// forward 模式：启动本地 WebSocket 服务等待客户端连接
func (c *Connector) Start() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.running {
		return fmt.Errorf("already running")
	}

	c.running = true
	if c.mode == "forward" {
		go c.serveForward()
	} else {
		go c.connectLoop()
	}

	return nil
}

// Stop 停止连接
func (c *Connector) Stop() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.running {
		return
	}
	c.running = false
	close(c.stopCh)
	if c.conn != nil {
		c.conn.Close()
	}
	if c.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		c.server.Shutdown(ctx)
		c.server = nil
	}

	// 清理所有等待中的请求
	c.pendingMu.Lock()
	for _, ch := range c.pendingReqs {
		close(ch)
	}
	c.pendingReqs = make(map[int64]chan map[string]any)
	c.pendingMu.Unlock()
}

// CallAPI 调用 OneBot 11 API（通过 WebSocket 发送请求并等待响应）
func (c *Connector) CallAPI(action string, params map[string]any) (map[string]any, error) {
	// 测试模式：仅记录日志，不真实发送
	if bot.TestMode {
		log.Info("[测试模式] 拦截 API 调用",
			"platform", c.Platform(),
			"self_id", c.SelfID(),
			"action", action,
			"params", fmt.Sprintf("%+v", params),
		)
		return map[string]any{"status": "ok", "data": map[string]any{}}, nil
	}

	c.mu.Lock()
	conn := c.conn
	c.mu.Unlock()

	if conn == nil {
		return nil, fmt.Errorf("WebSocket 未连接")
	}

	// 分配唯一 echo
	c.pendingMu.Lock()
	c.seqCounter++
	echo := c.seqCounter
	ch := make(chan map[string]any, 1)
	c.pendingReqs[echo] = ch
	c.pendingMu.Unlock()

	defer func() {
		c.pendingMu.Lock()
		delete(c.pendingReqs, echo)
		c.pendingMu.Unlock()
	}()

	req := map[string]any{
		"action": action,
		"params": params,
		"echo":   echo,
	}

	if err := conn.WriteJSON(req); err != nil {
		return nil, fmt.Errorf("发送 API 请求失败: %w", err)
	}

	// 等待响应（最多 10 秒）
	select {
	case resp := <-ch:
		if status, ok := resp["status"]; ok {
			if s, ok2 := status.(string); ok2 && s == "ok" {
				return resp, nil
			}
			if s, ok2 := status.(float64); ok2 && s == 0 {
				return resp, nil
			}
		}
		return resp, nil
	case <-time.After(10 * time.Second):
		return nil, fmt.Errorf("API 请求超时: %s", action)
	}
}

// connectLoop 连接循环（自动重连 + 指数退避）
func (c *Connector) connectLoop() {
	backoff := 1 * time.Second
	maxBackoff := 60 * time.Second

	for {
		select {
		case <-c.stopCh:
			return
		default:
		}

		if err := c.connect(); err != nil {
			log.Warn("OneBot11 连接失败，即将重试",
				"url", c.url,
				"error", err,
				"backoff", backoff)
			select {
			case <-c.stopCh:
				return
			case <-time.After(backoff):
			}
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
			continue
		}

		backoff = 1 * time.Second
	}
}

// connect 建立一次 WebSocket 连接并处理消息
func (c *Connector) connect() error {
	u, err := url.Parse(c.url)
	if err != nil {
		return fmt.Errorf("解析 URL 失败: %w", err)
	}

	// 根据协议选择 ws/wss
	scheme := "ws"
	if u.Scheme == "wss" || u.Scheme == "https" {
		scheme = "wss"
	}
	wsURL := url.URL{Scheme: scheme, Host: u.Host, Path: u.Path}

	header := http.Header{}
	if c.accessToken != "" {
		header.Set("Authorization", "Bearer "+c.accessToken)
	}

	log.Info("正在连接 OneBot11", "url", wsURL.String())

	conn, _, err := c.dialer.Dial(wsURL.String(), header)
	if err != nil {
		return fmt.Errorf("WebSocket 握手失败: %w", err)
	}

	c.mu.Lock()
	c.conn = conn
	c.mu.Unlock()

	log.Info("OneBot11 连接成功", "url", wsURL.String())

	// 设置 ping/pong 检测
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	// 消息读取循环
	go c.readLoop(conn)

	return nil
}

// serveForward 启动正向 WebSocket 服务，等待客户端连接
func (c *Connector) serveForward() {
	mux := http.NewServeMux()
	mux.HandleFunc("/", c.handleWSUpgrade)

	c.mu.Lock()
	c.server = &http.Server{
		Addr:    c.url,
		Handler: mux,
	}
	c.mu.Unlock()

	log.Info("OneBot11 正向 WS 服务启动", "listen", c.url)
	err := c.server.ListenAndServe()
	if err != nil && err != http.ErrServerClosed {
		log.Error("OneBot11 正向 WS 服务异常", "error", err)
	}
}

// handleWSUpgrade 处理正向 WS 的 HTTP 升级请求
func (c *Connector) handleWSUpgrade(w http.ResponseWriter, r *http.Request) {
	upgrader := websocket.Upgrader{
		HandshakeTimeout: 10 * time.Second,
		CheckOrigin: func(r *http.Request) bool {
			return true // 允许任意来源
		},
	}

	// 验证 Access Token
	if c.accessToken != "" {
		token := r.Header.Get("Authorization")
		if token != "Bearer "+c.accessToken {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Error("OneBot11 WS 升级失败", "error", err)
		return
	}

	c.mu.Lock()
	// 关闭旧连接（允许重新连接）
	if c.conn != nil {
		c.conn.Close()
	}
	c.conn = conn
	c.mu.Unlock()

	log.Info("OneBot11 正向 WS 客户端已连接")

	// 设置 ping/pong 检测
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	c.readLoop(conn)
}

// readLoop 读取 WebSocket 消息并分发到事件系统
func (c *Connector) readLoop(conn *websocket.Conn) {
	defer func() {
		c.mu.Lock()
		if c.conn == conn {
			c.conn = nil
		}
		c.mu.Unlock()
		conn.Close()
		log.Warn("OneBot11 连接已断开", "url", c.url)
	}()

	for {
		select {
		case <-c.stopCh:
			return
		default:
		}

		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		_, message, err := conn.ReadMessage()
		if err != nil {
			log.Warn("OneBot11 读取消息失败", "error", err)
			return
		}

		c.handleMessage(message)
	}
}

// handleMessage 处理收到的消息
func (c *Connector) handleMessage(data []byte) {
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		log.Error("解析消息失败", "error", err)
		return
	}

	// 检查是否是 API 调用的响应（含有 echo 字段）
	if echoVal, ok := raw["echo"]; ok {
		var echo int64
		switch v := echoVal.(type) {
		case float64:
			echo = int64(v)
		case int64:
			echo = v
		case json.Number:
			echo, _ = v.Int64()
		case string:
			// 某些实现可能序列化为字符串
			parsed, err := strconv.ParseInt(v, 10, 64)
			if err != nil {
				return
			}
			echo = parsed
		default:
			return
		}

		c.pendingMu.Lock()
		ch, ok := c.pendingReqs[echo]
		delete(c.pendingReqs, echo)
		c.pendingMu.Unlock()

		if ok {
			select {
			case ch <- raw:
			default:
			}
			close(ch)
		}
		return
	}

	// 忽略原生心跳等元事件
	if postType, ok := raw["post_type"].(string); ok && postType == "meta_event" {
		return
	}

	// 分发到事件系统
	if err := bot.DispatchAdapter(c, data); err != nil {
		log.Error("分发事件失败", "error", err)
	}
}
