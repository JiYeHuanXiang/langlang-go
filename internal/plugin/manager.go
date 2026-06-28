// Package plugin 提供插件（脚本包）的管理与热重载。
package plugin

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/super1207/langlang-go/internal/config"
	"github.com/super1207/langlang-go/internal/log"
)

// Package 表示一个脚本包
type Package struct {
	Name      string    `json:"name"`
	Code      string    `json:"code"`
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Manager 插件管理器
type Manager struct {
	dir     string
	cfg     *config.Config
	mu      sync.RWMutex
	plugins map[string]*Package // key = pkg name
}

// NewManager 创建插件管理器
func NewManager(pluginDir string) *Manager {
	return &Manager{
		dir:     pluginDir,
		plugins: make(map[string]*Package),
	}
}

// SetConfig 设置配置引用
func (m *Manager) SetConfig(cfg *config.Config) {
	m.cfg = cfg
}

// LoadAll 从磁盘加载所有插件
func (m *Manager) LoadAll() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	entries, err := os.ReadDir(m.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		name := entry.Name()[:len(entry.Name())-5]
		data, err := os.ReadFile(filepath.Join(m.dir, entry.Name()))
		if err != nil {
			log.Warn("读取插件失败", "name", name, "error", err)
			continue
		}

		var pkg Package
		if err := json.Unmarshal(data, &pkg); err != nil {
			log.Warn("解析插件失败", "name", name, "error", err)
			continue
		}
		pkg.Name = name
		m.plugins[name] = &pkg
		log.Info("加载插件", "name", name, "enabled", pkg.Enabled)
	}

	return nil
}

// Save 保存插件到磁盘
func (m *Manager) Save(name, code string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	pkg, exists := m.plugins[name]
	if !exists {
		pkg = &Package{
			Name:      name,
			Enabled:   true,
			CreatedAt: now,
		}
		m.plugins[name] = pkg
	}
	pkg.Code = code
	pkg.UpdatedAt = now

	return m.writeToDisk(name)
}

// Delete 删除插件
func (m *Manager) Delete(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.plugins, name)
	path := filepath.Join(m.dir, name+".json")
	os.Remove(path)
}

// Get 获取插件
func (m *Manager) Get(name string) *Package {
	m.mu.RLock()
	defer m.mu.RUnlock()
	pkg, ok := m.plugins[name]
	if !ok {
		return nil
	}
	cp := *pkg
	return &cp
}

// List 列出所有插件
func (m *Manager) List() []*Package {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Package, 0, len(m.plugins))
	for _, pkg := range m.plugins {
		cp := *pkg
		result = append(result, &cp)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result
}

// Count 返回插件数量
func (m *Manager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.plugins)
}

// SaveAll 保存所有插件
func (m *Manager) SaveAll() {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for name := range m.plugins {
		if err := m.writeToDisk(name); err != nil {
			log.Error("保存插件失败", "name", name, "error", err)
		}
	}
}

func (m *Manager) writeToDisk(name string) error {
	if err := os.MkdirAll(m.dir, 0755); err != nil {
		return err
	}

	pkg := m.plugins[name]
	data, err := json.MarshalIndent(pkg, "", "  ")
	if err != nil {
		return err
	}

	path := filepath.Join(m.dir, name+".json")
	return os.WriteFile(path, data, 0644)
}
