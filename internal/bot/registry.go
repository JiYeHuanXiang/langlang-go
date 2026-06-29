// Package bot 定义机器人平台适配器接口。
//
// 每个聊天平台只需要实现 BotAdapter 接口，
// 然后注册到 Registry 即可接入事件系统。
package bot

import (
	"sync"

	"github.com/jiyehuanxiang/langlang-go/internal/event"
)

// BotAdapter 是机器人平台适配器接口
type BotAdapter interface {
	// Platform 返回平台标识（如 onebot11、telegram、kook）
	Platform() string

	// Start 启动连接，返回错误
	Start() error

	// Stop 停止连接
	Stop()

	// CallAPI 调用平台 API
	CallAPI(action string, params map[string]any) (map[string]any, error)

	// SelfID 返回机器人自身 ID
	SelfID() string

	// Running 返回连接是否正在运行
	Running() bool
}

// Registry 是适配器注册表
type Registry struct {
	mu      sync.RWMutex
	adapters map[string]BotAdapter // key: platform+selfID
}

// GlobalRegistry 全局适配器注册表
var GlobalRegistry = NewRegistry()

// NewRegistry 创建适配器注册表
func NewRegistry() *Registry {
	return &Registry{
		adapters: make(map[string]BotAdapter),
	}
}

// Register 注册适配器
func (r *Registry) Register(adapter BotAdapter) {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := adapter.Platform() + ":" + adapter.SelfID()
	r.adapters[key] = adapter
}

// Unregister 注销适配器
func (r *Registry) Unregister(platform, selfID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := platform + ":" + selfID
	delete(r.adapters, key)
}

// Get 获取适配器
func (r *Registry) Get(platform, selfID string) (BotAdapter, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	key := platform + ":" + selfID
	a, ok := r.adapters[key]
	return a, ok
}

// CallAPI 调用指定平台的 API
func (r *Registry) CallAPI(platform, selfID, action string, params map[string]any) (map[string]any, error) {
	adapter, ok := r.Get(platform, selfID)
	if !ok {
		return nil, ErrAdapterNotFound
	}
	return adapter.CallAPI(action, params)
}

// DispatchAdapter 从适配器分发事件
func DispatchAdapter(adapter BotAdapter, data []byte) error {
	return event.GlobalDispatcher.DispatchJSON(adapter.Platform(), data)
}

// AdapterHost 是适配器宿主接口（适配器用来反向调用宿主）
type AdapterHost interface {
	Log(msg string)
	Warn(msg string)
	DispatchEvent(eventJSON string) error
	AppDir() string
}
