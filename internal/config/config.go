// Package config 提供 langlang-go 的配置管理。
// 配置格式为 JSON，支持热重载。
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Config 是全局配置结构
type Config struct {
	Core     CoreConfig     `json:"core"`
	Log      LogConfig      `json:"log"`
	Paths    PathsConfig    `json:"paths"`
	Web      WebConfig      `json:"web"`
	Bot      BotConfig      `json:"bot"`
	MQTT     MQTTConfig     `json:"mqtt"`
	Postgres PostgresConfig `json:"postgres"`
	Audio    AudioConfig    `json:"audio"`
}

type CoreConfig struct {
	WebPort         int      `json:"web_port"`
	Platforms       []string `json:"platforms"`
	SkipMsgMinutes  int64    `json:"skip_msg_minutes"`
}

type LogConfig struct {
	Level string `json:"level"`
	File  string `json:"file"`
}

type PathsConfig struct {
	Data    string `json:"data"`
	Plugins string `json:"plugins"`
	Logs    string `json:"logs"`
	Tmp     string `json:"tmp"`
}

type WebConfig struct {
	Listen      string `json:"listen"`
	StaticDir   string `json:"static_dir"`
	AccessToken string `json:"access_token"`
}

type BotConfig struct {
	OneBot11 []OneBot11Config  `json:"onebot11"`
	Telegram []TelegramConfig  `json:"telegram"`
	Satori   []SatoriConfig    `json:"satori"`
}

type OneBot11Config struct {
	Mode        string `json:"mode"`
	URL         string `json:"url"`
	AccessToken string `json:"access_token"`
	SelfID      string `json:"self_id"`
	Enabled     *bool  `json:"enabled,omitempty"`
}

// IsEnabled 返回连接器是否启用（nil 视为启用，向后兼容）
func (c OneBot11Config) IsEnabled() bool {
	return c.Enabled == nil || *c.Enabled
}

// TelegramConfig Telegram Bot 连接配置
type TelegramConfig struct {
	Token   string `json:"token"`
	Enabled *bool  `json:"enabled,omitempty"`
}

// UnmarshalJSON 支持向后兼容：旧格式为纯字符串 "token"，新格式为对象 {"token":"...","enabled":true}
func (c *TelegramConfig) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		c.Token = s
		return nil
	}
	type Alias TelegramConfig
	var a Alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	*c = TelegramConfig(a)
	return nil
}

// IsEnabled 返回连接器是否启用（nil 视为启用，向后兼容）
func (c TelegramConfig) IsEnabled() bool {
	return c.Enabled == nil || *c.Enabled
}

// SatoriConfig Satori 协议连接配置
type SatoriConfig struct {
	URL     string `json:"url"`
	Token   string `json:"token"`
	SelfID  string `json:"self_id"`
	APIURL  string `json:"api_url"`
	Enabled *bool  `json:"enabled,omitempty"`
}

// IsEnabled 返回连接器是否启用（nil 视为启用，向后兼容）
func (c SatoriConfig) IsEnabled() bool {
	return c.Enabled == nil || *c.Enabled
}

type MQTTConfig struct {
	Enabled  bool   `json:"enabled"`
	Broker   string `json:"broker"`
	ClientID string `json:"client_id"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type PostgresConfig struct {
	Enabled bool   `json:"enabled"`
	ConnStr string `json:"conn_str"`
}

type AudioConfig struct {
	EnabledFormats []string `json:"enabled_formats"`
}

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	exeDir, _ := os.Executable()
	baseDir := filepath.Dir(exeDir)

	return &Config{
		Core: CoreConfig{
			WebPort:        2397,
			Platforms:      []string{},
			SkipMsgMinutes: 10,
		},
		Log: LogConfig{
			Level: "info",
			File:  "",
		},
		Paths: PathsConfig{
			Data:    filepath.Join(baseDir, "data"),
			Plugins: filepath.Join(baseDir, "plugins"),
			Logs:    filepath.Join(baseDir, "logs"),
			Tmp:     filepath.Join(baseDir, "tmp"),
		},
		Web: WebConfig{
			Listen:    ":2397",
			StaticDir: "", // 静态文件已嵌入二进制，不再需要本地目录
		},
		MQTT: MQTTConfig{Enabled: false},
		Postgres: PostgresConfig{Enabled: false},
	}
}

// Load 从 JSON 文件加载配置，文件不存在则写入默认配置并返回
func Load(path string) (*Config, error) {
	cfg := DefaultConfig()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			if err := cfg.Save(path); err != nil {
				return nil, fmt.Errorf("写入默认配置失败: %w", err)
			}
			return cfg, nil
		}
		return nil, fmt.Errorf("读取配置失败: %w", err)
	}

	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("解析配置失败: %w", err)
	}

	return cfg, nil
}

// Save 将配置写入 JSON 文件
func (c *Config) Save(path string) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化配置失败: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}
