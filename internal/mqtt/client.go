// Package mqtt 提供 MQTT 集群支持。
// 多个 langlang-go 实例可以通过 MQTT 共享适配器和插件。
package mqtt

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	paho "github.com/eclipse/paho.mqtt.golang"

	"github.com/jiyehuanxiang/langlang-go/internal/config"
	"github.com/jiyehuanxiang/langlang-go/internal/log"
)

const (
	topicHeartbeat = "langlang/cluster/heartbeat"
	topicEvent     = "langlang/cluster/event"
	topicRPCCall   = "langlang/cluster/rpc/call"
	topicRPCResp   = "langlang/cluster/rpc/response"
)

// Client MQTT 集群客户端
type Client struct {
	cfg     config.MQTTConfig
	mu      sync.Mutex
	running bool
	client  paho.Client
	subCh   chan struct{}
}

// NewClient 创建 MQTT 客户端
func NewClient(cfg config.MQTTConfig) *Client {
	return &Client{
		cfg: cfg,
	}
}

// Start 启动 MQTT 连接
func (c *Client) Start() error {
	if !c.cfg.Enabled {
		return nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.running {
		return nil
	}

	log.Info("MQTT 集群正在连接", "broker", c.cfg.Broker, "client_id", c.cfg.ClientID)

	opts := paho.NewClientOptions()
	opts.AddBroker(c.cfg.Broker)
	opts.SetClientID(c.cfg.ClientID)
	opts.SetUsername(c.cfg.Username)
	opts.SetPassword(c.cfg.Password)
	opts.SetKeepAlive(30 * time.Second)
	opts.SetPingTimeout(10 * time.Second)
	opts.SetAutoReconnect(true)
	opts.SetMaxReconnectInterval(60 * time.Second)
	opts.SetOnConnectHandler(func(_ paho.Client) {
		log.Info("MQTT 集群已连接")
		c.subscribe()
	})
	opts.SetConnectionLostHandler(func(_ paho.Client, err error) {
		log.Warn("MQTT 连接断开", "error", err)
	})

	client := paho.NewClient(opts)
	token := client.Connect()
	if token.WaitTimeout(15*time.Second) && token.Error() != nil {
		return fmt.Errorf("MQTT 连接失败: %w", token.Error())
	}

	c.client = client
	c.running = true
	return nil
}

// Stop 停止 MQTT 连接
func (c *Client) Stop() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.running {
		return
	}

	if c.client != nil {
		c.client.Disconnect(1000)
	}
	c.running = false
	log.Info("MQTT 集群已断开")
}

// subscribe 订阅集群主题
func (c *Client) subscribe() {
	if c.client == nil {
		return
	}

	topics := []string{topicHeartbeat, topicEvent, topicRPCCall}
	for _, topic := range topics {
		token := c.client.Subscribe(topic, 1, c.handleMessage)
		token.WaitTimeout(5 * time.Second)
		if token.Error() != nil {
			log.Warn("MQTT 订阅失败", "topic", topic, "error", token.Error())
		}
	}
	log.Info("MQTT 主题订阅完成")
}

// handleMessage 处理收到的 MQTT 消息
func (c *Client) handleMessage(_ paho.Client, msg paho.Message) {
	log.Info("MQTT 收到消息", "topic", msg.Topic(), "size", len(msg.Payload()))
}

// Publish 发布消息到集群
func (c *Client) Publish(topic string, payload []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.running || c.client == nil {
		return fmt.Errorf("MQTT 未连接")
	}

	token := c.client.Publish(topic, 1, false, payload)
	token.WaitTimeout(5 * time.Second)
	return token.Error()
}

// PublishEvent 发布事件到集群
func (c *Client) PublishEvent(platform string, data []byte) error {
	payload := map[string]any{
		"platform": platform,
		"data":     json.RawMessage(data),
		"time":     time.Now().Unix(),
	}
	body, _ := json.Marshal(payload)
	return c.Publish(topicEvent, body)
}

// RemoteCall 调用远程节点的 API
func (c *Client) RemoteCall(platform, selfID, passiveID, remoteID string, payload map[string]any) (map[string]any, error) {
	return nil, fmt.Errorf("MQTT 远程调用尚未实现")
}
