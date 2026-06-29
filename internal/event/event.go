package event

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/jiyehuanxiang/langlang-go/internal/log"
)

type Event struct {
	Platform    string          `json:"platform"`
	SelfID      string          `json:"self_id"`
	Time        int64           `json:"time"`
	EventType   string          `json:"event_type"`
	MessageType string          `json:"message_type"`
	MessageID   string          `json:"message_id"`
	UserID      string          `json:"user_id"`
	GroupID     string          `json:"group_id"`
	Message     string          `json:"message"`
	RawMessage  string          `json:"raw_message"`
	Font        int             `json:"font"`
	Raw         json.RawMessage `json:"raw"`
}

type Handler func(evt *Event) error

type Dispatcher struct {
	mu           sync.RWMutex
	handlers     []Handler
	typeHandlers map[string][]Handler
}

func NewDispatcher() *Dispatcher {
	return &Dispatcher{
		typeHandlers: make(map[string][]Handler),
	}
}

func (d *Dispatcher) RegisterHandler(h Handler) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.handlers = append(d.handlers, h)
}

func (d *Dispatcher) RegisterTypeHandler(eventType string, h Handler) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.typeHandlers[eventType] = append(d.typeHandlers[eventType], h)
}

func (d *Dispatcher) Dispatch(evt *Event) error {
	d.mu.RLock()
	handlers := make([]Handler, len(d.handlers))
	copy(handlers, d.handlers)

	typeHandlers := make([]Handler, 0)
	if hs, ok := d.typeHandlers[evt.EventType]; ok {
		typeHandlers = append(typeHandlers, hs...)
	}
	d.mu.RUnlock()

	for _, h := range handlers {
		if err := h(evt); err != nil {
			log.Error("事件处理器错误", "error", err)
		}
	}

	for _, h := range typeHandlers {
		if err := h(evt); err != nil {
			log.Error("事件类型处理器错误", "type", evt.EventType, "error", err)
		}
	}

	return nil
}

func (d *Dispatcher) DispatchJSON(platform string, data []byte) error {
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	evt := &Event{
		Platform:  platform,
		Time:      time.Now().Unix(),
		Raw:       data,
		EventType: "message",
	}

	if v, ok := raw["self_id"]; ok {
		evt.SelfID = anyToString(v)
	}
	if v, ok := raw["message_id"]; ok {
		evt.MessageID = anyToString(v)
	}
	if v, ok := raw["user_id"]; ok {
		evt.UserID = anyToString(v)
	}
	if v, ok := raw["group_id"]; ok {
		evt.GroupID = anyToString(v)
	}
	if v, ok := raw["message"]; ok {
		s := anyToString(v)
		evt.Message = s
		evt.RawMessage = s
	}
	if v, ok := raw["message_type"]; ok {
		evt.MessageType = anyToString(v)
	}
	if v, ok := raw["post_type"]; ok {
		evt.EventType = anyToString(v)
	}
	if v, ok := raw["time"]; ok {
		if f, ok := v.(float64); ok {
			evt.Time = int64(f)
		}
	}

	return d.Dispatch(evt)
}

// GlobalDispatcher 全局事件分发器
var GlobalDispatcher = NewDispatcher()

// anyToString 将 any 安全转换为 string
// 修复：原 formatFloat 无限递归 BUG
func anyToString(v any) string {
	switch s := v.(type) {
	case string:
		return s
	case float64:
		// JSON 数字转字符串，去掉末尾 .0
		if s == float64(int64(s)) {
			return strings.TrimRight(strings.TrimRight(
				fmt.Sprintf("%.0f", s), "0"), ".")
		}
		return strings.TrimRight(strings.TrimRight(
			fmt.Sprintf("%f", s), "0"), ".")
	case json.Number:
		return s.String()
	case bool:
		if s {
			return "true"
		}
		return "false"
	case nil:
		return ""
	default:
		b, _ := json.Marshal(v)
		return string(b)
	}
}
