// Package telegram 实现 Telegram Bot API 长轮询连接。
package telegram

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/super1207/langlang-go/internal/bot"
	"github.com/super1207/langlang-go/internal/log"
)

const apiBase = "https://api.telegram.org/bot"
const pollTimeout = 30 // long-poll seconds

// Connector Telegram 连接器
type Connector struct {
	token    string
	selfID   string
	baseURL  string
	client   *http.Client
	mu       sync.Mutex
	running  bool
	stopCh   chan struct{}
	offset   int64 // getUpdates offset
}

// Update Telegram API 更新结构（只解析需要的字段）
type Update struct {
	UpdateID int64 `json:"update_id"`
	Message  *struct {
		MessageID int64 `json:"message_id"`
		From      *struct {
			ID       int64  `json:"id"`
			Username string `json:"username"`
		} `json:"from"`
		Chat *struct {
			ID   int64  `json:"id"`
			Type string `json:"type"`
		} `json:"chat"`
		Text string `json:"text"`
		Date int64  `json:"date"`
	} `json:"message"`
}

// NewConnector 创建 Telegram 连接器
func NewConnector(token string) *Connector {
	return &Connector{
		token:   token,
		baseURL: apiBase + token,
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
		stopCh: make(chan struct{}),
	}
}

// Platform 返回平台标识
func (c *Connector) Platform() string { return "telegram" }

// SelfID 返回机器人 ID
func (c *Connector) SelfID() string { return c.selfID }

// Running 返回连接是否正在运行
func (c *Connector) Running() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.running
}

// Start 启动长轮询
func (c *Connector) Start() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.running {
		return nil
	}

	// 先获取 bot 信息，拿到 selfID
	info, err := c.getMe()
	if err != nil {
		log.Warn("Telegram getMe 失败", "error", err)
	} else {
		c.selfID = strconv.FormatInt(info.ID, 10)
	}

	c.running = true
	go c.pollLoop()
	log.Info("Telegram 连接器已启动", "token_prefix", c.token[:8]+"...", "self_id", c.selfID)
	return nil
}

// Stop 停止连接
func (c *Connector) Stop() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.running {
		close(c.stopCh)
		c.running = false
	}
}

// CallAPI 调用 Telegram Bot API
func (c *Connector) CallAPI(action string, params map[string]any) (map[string]any, error) {
	// 测试模式：仅记录日志，不真实发送
	if bot.TestMode {
		log.Info("[测试模式] 拦截 API 调用",
			"platform", c.Platform(),
			"self_id", c.SelfID(),
			"action", action,
			"params", fmt.Sprintf("%+v", params),
		)
		return map[string]any{"ok": true, "result": map[string]any{}}, nil
	}

	body, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("序列化参数失败: %w", err)
	}

	apiURL := fmt.Sprintf("%s/%s", c.baseURL, action)
	resp, err := c.client.Post(apiURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("API 请求失败: %w", err)
	}
	defer resp.Body.Close()

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}
	return result, nil
}

// botUser 是 Telegram Bot API 返回的 User 对象
type botUser struct {
	ID int64 `json:"id"`
}

// getMe 获取机器人自身信息
func (c *Connector) getMe() (*botUser, error) {
	apiURL := fmt.Sprintf("%s/getMe", c.baseURL)
	resp, err := c.client.Get(apiURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var res struct {
		Ok     bool     `json:"ok"`
		Result *botUser `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, err
	}
	if !res.Ok || res.Result == nil {
		return nil, fmt.Errorf("getMe failed")
	}
	return res.Result, nil
}

// pollLoop 长轮询 getUpdates
func (c *Connector) pollLoop() {
	log.Info("Telegram 长轮询已启动")

	for {
		select {
		case <-c.stopCh:
			log.Info("Telegram 连接器已停止")
			return
		default:
		}

		updates, err := c.getUpdates()
		if err != nil {
			log.Warn("Telegram getUpdates 失败", "error", err)
			select {
			case <-c.stopCh:
				return
			case <-time.After(3 * time.Second):
			}
			continue
		}

		for _, upd := range updates {
			c.handleUpdate(upd)
			c.offset = upd.UpdateID + 1
		}
	}
}

// getUpdates 获取更新（长轮询）
func (c *Connector) getUpdates() ([]Update, error) {
	apiURL := fmt.Sprintf("%s/getUpdates?timeout=%d&offset=%d&allowed_updates=[\"message\"]",
		c.baseURL, pollTimeout, c.offset)

	resp, err := c.client.Get(apiURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result struct {
		Ok     bool     `json:"ok"`
		Result []Update `json:"result"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	if !result.Ok {
		return nil, fmt.Errorf("API 返回错误")
	}
	return result.Result, nil
}

// handleUpdate 处理单个 Telegram 更新
func (c *Connector) handleUpdate(upd Update) {
	if upd.Message == nil {
		return
	}

	msg := upd.Message
	chatID := strconv.FormatInt(msg.Chat.ID, 10)
	userID := strconv.FormatInt(msg.From.ID, 10)
	messageID := strconv.FormatInt(msg.MessageID, 10)

	// 构造标准事件
	evtMap := map[string]any{
		"self_id":      c.selfID,
		"message_id":   messageID,
		"user_id":      userID,
		"group_id":     chatID,
		"message":      msg.Text,
		"raw_message":  msg.Text,
		"message_type": "private",
		"post_type":    "message",
		"platform":     "telegram",
		"time":         msg.Date,
	}

	if msg.Chat.Type == "group" || msg.Chat.Type == "supergroup" {
		evtMap["message_type"] = "group"
	}

	data, _ := json.Marshal(evtMap)
	if err := bot.DispatchAdapter(c, data); err != nil {
		log.Error("分发 Telegram 事件失败", "error", err)
	}
}

// SendMessage 发送 Telegram 消息
func (c *Connector) SendMessage(chatID, text string) error {
	apiURL := fmt.Sprintf("%s/sendMessage", c.baseURL)
	payload := map[string]string{
		"chat_id": chatID,
		"text":    text,
	}
	body, _ := json.Marshal(payload)

	resp, err := c.client.Post(apiURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("发送消息失败: %w", err)
	}
	defer resp.Body.Close()

	return nil
}
