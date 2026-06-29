// Package satori 实现 Satori 协议连接器。
// 使用反向 WebSocket 连接 Satori 协议网关，接收事件并通过 REST API 调用动作。
package satori

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"github.com/jiyehuanxiang/langlang-go/internal/bot"
	"github.com/jiyehuanxiang/langlang-go/internal/log"
)

// Satori 协议操作码
const (
	opEvent = 0  // 事件推送
	opPing  = 1  // 服务端心跳
	opPong  = 2  // 客户端心跳回复
	opReady = 3  // 客户端鉴权
)

// Connector Satori 协议连接器
type Connector struct {
	url       string // WebSocket 网关地址
	token     string // 鉴权令牌
	selfID    string // 机器人 ID（可为空，启动后从事件中获取）
	apiURL    string // REST API 基础地址
	conn      *websocket.Conn
	client    *http.Client
	mu        sync.Mutex
	running   bool
	stopCh    chan struct{}
	dialer    *websocket.Dialer
}

// NewConnector 创建 Satori 连接器
func NewConnector(wsURL, token, selfID, apiURL string) *Connector {
	if apiURL == "" {
		// 从 WS URL 推导 REST API 地址
		if u, err := url.Parse(wsURL); err == nil {
			scheme := "http"
			if u.Scheme == "wss" || u.Scheme == "https" {
				scheme = "https"
			}
			apiURL = fmt.Sprintf("%s://%s", scheme, u.Host)
		}
	}
	return &Connector{
		url:    wsURL,
		token:  token,
		selfID: selfID,
		apiURL: apiURL,
		stopCh: make(chan struct{}),
		dialer: &websocket.Dialer{
			HandshakeTimeout: 10 * time.Second,
		},
		client: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// Platform 返回平台标识
func (c *Connector) Platform() string { return "satori" }

// SelfID 返回机器人 ID
func (c *Connector) SelfID() string { return c.selfID }

// Running 返回连接是否正在运行
func (c *Connector) Running() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.running
}

// Start 启动反向 WebSocket 连接
func (c *Connector) Start() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.running {
		return fmt.Errorf("already running")
	}

	c.running = true
	go c.connectLoop()

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
}

// CallAPI 调用 Satori REST API
func (c *Connector) CallAPI(action string, params map[string]any) (map[string]any, error) {
	// 测试模式：仅记录日志，不真实发送
	if bot.TestMode {
		log.Info("[测试模式] 拦截 API 调用",
			"platform", c.Platform(),
			"self_id", c.SelfID(),
			"action", action,
			"params", fmt.Sprintf("%+v", params),
		)
		return map[string]any{"ok": true, "data": map[string]any{}}, nil
	}

	apiURL := fmt.Sprintf("%s/api/%s", c.apiURL, action)
	body, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("序列化参数失败: %w", err)
	}

	resp, err := c.client.Post(apiURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("API 请求失败: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	var result map[string]any
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}
	return result, nil
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
			log.Warn("Satori 连接失败，即将重试",
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

// connect 建立 WebSocket 连接、鉴权并进入消息读取循环
func (c *Connector) connect() error {
	u, err := url.Parse(c.url)
	if err != nil {
		return fmt.Errorf("解析 URL 失败: %w", err)
	}

	scheme := "ws"
	if u.Scheme == "wss" || u.Scheme == "https" {
		scheme = "wss"
	}
	wsURL := url.URL{Scheme: scheme, Host: u.Host, Path: u.Path}
	if u.RawQuery != "" {
		wsURL.RawQuery = u.RawQuery
	}

	log.Info("正在连接 Satori", "url", wsURL.String())

	conn, _, err := c.dialer.Dial(wsURL.String(), nil)
	if err != nil {
		return fmt.Errorf("WebSocket 握手失败: %w", err)
	}

	c.mu.Lock()
	c.conn = conn
	c.mu.Unlock()

	// 发送鉴权帧
	if err := c.sendReady(conn); err != nil {
		conn.Close()
		return fmt.Errorf("发送鉴权失败: %w", err)
	}

	log.Info("Satori 连接成功", "url", wsURL.String())

	// 设置 ping/pong 检测
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	// 消息读取循环（同步阻塞，断开时返回）
	c.readLoop(conn)

	return nil
}

// sendReady 发送鉴权帧 (op=3)
func (c *Connector) sendReady(conn *websocket.Conn) error {
	readyPayload := map[string]any{
		"op": opReady,
		"body": map[string]string{
			"token": c.token,
		},
	}
	return conn.WriteJSON(readyPayload)
}

// satoriFrame Satori 协议帧结构
type satoriFrame struct {
	Op   int              `json:"op"`
	Body *json.RawMessage `json:"body,omitempty"`
}

// readLoop 读取 WebSocket 消息并处理协议帧
func (c *Connector) readLoop(conn *websocket.Conn) {
	defer func() {
		c.mu.Lock()
		if c.conn == conn {
			c.conn = nil
		}
		c.mu.Unlock()
		conn.Close()
		log.Warn("Satori 连接已断开", "url", c.url)
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
			log.Warn("Satori 读取消息失败", "error", err)
			return
		}

		c.handleFrame(message)
	}
}

// handleFrame 处理 Satori 协议帧
func (c *Connector) handleFrame(data []byte) {
	var frame satoriFrame
	if err := json.Unmarshal(data, &frame); err != nil {
		log.Error("Satori 解析帧失败", "error", err)
		return
	}

	switch frame.Op {
	case opPing:
		c.handlePing()
	case opEvent:
		if frame.Body != nil {
			c.handleEvent(*frame.Body)
		}
	default:
		// 忽略其他操作码
	}
}

// handlePing 响应服务端心跳 (op=1 -> op=2)
func (c *Connector) handlePing() {
	c.mu.Lock()
	conn := c.conn
	c.mu.Unlock()

	if conn == nil {
		return
	}

	pong := map[string]any{"op": opPong}
	if err := conn.WriteJSON(pong); err != nil {
		log.Warn("Satori 发送 Pong 失败", "error", err)
	}
}

// satoriEvent Satori 事件结构（关键字段）
type satoriEvent struct {
	ID        string           `json:"id"`
	Type      string           `json:"type"`
	Platform  string           `json:"platform"`
	SelfID    string           `json:"self_id"`
	Timestamp float64          `json:"timestamp"`
	Channel   *satoriChannel   `json:"channel,omitempty"`
	Guild     *satoriGuild     `json:"guild,omitempty"`
	Member    *satoriMember    `json:"member,omitempty"`
	User      *satoriUser      `json:"user,omitempty"`
	Content   string           `json:"content,omitempty"`
	Raw       *json.RawMessage `json:"_raw,omitempty"`
}

type satoriChannel struct {
	ID   string `json:"id"`
	Type string `json:"type"`
}

type satoriGuild struct {
	ID string `json:"id"`
}

type satoriMember struct {
	User *satoriUser `json:"user,omitempty"`
}

type satoriUser struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// handleEvent 处理 Satori 事件 (op=0)
func (c *Connector) handleEvent(body json.RawMessage) {
	var evt satoriEvent
	if err := json.Unmarshal(body, &evt); err != nil {
		log.Error("Satori 解析事件失败", "error", err)
		return
	}

	// 从事件中获取 self_id（如果尚未设置）
	if c.selfID == "" && evt.SelfID != "" {
		c.mu.Lock()
		c.selfID = evt.SelfID
		c.mu.Unlock()
	}

	// 映射为 OneBot 风格的事件字段
	evtMap := map[string]any{
		"self_id":     evt.SelfID,
		"platform":    "satori",
		"time":        int64(evt.Timestamp),
		"post_type":   "message",
		"message_id":  evt.ID,
		"raw_message": evt.Content,
		"message":     evt.Content,
	}

	// 用户信息
	if evt.User != nil {
		evtMap["user_id"] = evt.User.ID
	} else if evt.Member != nil && evt.Member.User != nil {
		evtMap["user_id"] = evt.Member.User.ID
	}

	// 群组/频道信息
	if evt.Guild != nil {
		evtMap["group_id"] = evt.Guild.ID
	} else if evt.Channel != nil {
		evtMap["group_id"] = evt.Channel.ID
	}

	// 消息类型
	if evt.Channel != nil {
		if evt.Channel.Type == "direct" || evt.Channel.Type == "private" {
			evtMap["message_type"] = "private"
		} else {
			evtMap["message_type"] = "group"
		}
	}

	// 事件类型映射
	switch evt.Type {
	case "message-created":
		evtMap["post_type"] = "message"
	case "message-deleted":
		return // 忽略撤回事件
	case "guild-added":
		evtMap["post_type"] = "notice"
		evtMap["notice_type"] = "group_increase"
		return
	case "guild-removed":
		evtMap["post_type"] = "notice"
		evtMap["notice_type"] = "group_decrease"
		return
	case "friend-added":
		evtMap["post_type"] = "notice"
		evtMap["notice_type"] = "friend_add"
		return
	default:
		evtMap["post_type"] = "message"
	}

	eventData, err := json.Marshal(evtMap)
	if err != nil {
		log.Error("Satori 序列化事件失败", "error", err)
		return
	}

	if err := bot.DispatchAdapter(c, eventData); err != nil {
		log.Error("Satori 分发事件失败", "error", err)
	}
}
