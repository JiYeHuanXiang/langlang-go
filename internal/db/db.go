// Package db 提供数据持久化支持。
package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	_ "modernc.org/sqlite"

	"github.com/jiyehuanxiang/langlang-go/internal/config"
	"github.com/jiyehuanxiang/langlang-go/internal/log"
)

// Database 数据层接口
type Database interface {
	// Open 打开数据库连接
	Open() error
	// Close 关闭数据库
	Close()
	// InsertMsg 插入消息记录
	InsertMsg(msg map[string]any) error
	// QueryMsg 查询消息记录
	QueryMsg(filter map[string]any) ([]map[string]any, error)
}

// Manager 数据库管理器
type Manager struct {
	cfg    config.PostgresConfig
	db     Database
	mu     sync.Mutex
	opened bool
}

// NewManager 创建数据库管理器
func NewManager(cfg config.PostgresConfig) *Manager {
	return &Manager{
		cfg: cfg,
	}
}

// Open 打开数据库连接
func (m *Manager) Open() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.opened {
		return nil
	}

	if !m.cfg.Enabled {
		log.Info("数据库未启用，使用 SQLite 本地存储")
		m.db = newSQLiteStore()
	} else {
		log.Info("正在连接 PostgreSQL", "conn_str", maskConnStr(m.cfg.ConnStr))
		m.db = newPostgresStore(m.cfg.ConnStr)
	}

	if err := m.db.Open(); err != nil {
		return fmt.Errorf("打开数据库失败: %w", err)
	}

	m.opened = true
	return nil
}

// Close 关闭数据库
func (m *Manager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.db != nil {
		m.db.Close()
	}
	m.opened = false
}

// InsertMsg 插入消息
func (m *Manager) InsertMsg(msg map[string]any) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.db == nil {
		return fmt.Errorf("数据库未打开")
	}
	return m.db.InsertMsg(msg)
}

// QueryMsg 查询消息
func (m *Manager) QueryMsg(filter map[string]any) ([]map[string]any, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.db == nil {
		return nil, fmt.Errorf("数据库未打开")
	}
	return m.db.QueryMsg(filter)
}

func maskConnStr(s string) string {
	if len(s) > 20 {
		return s[:10] + "..." + s[len(s)-10:]
	}
	return s
}

// --- SQLite 实现（modernc.org/sqlite，纯 Go 无需 CGO） ---

type sqliteStore struct {
	db      *sql.DB
	dbPath  string
}

func newSQLiteStore() *sqliteStore {
	return &sqliteStore{dbPath: "data/langlang.db"}
}

func (s *sqliteStore) Open() error {
	db, err := sql.Open("sqlite", s.dbPath)
	if err != nil {
		return fmt.Errorf("打开 SQLite 失败: %w", err)
	}

	// 设置连接池
	db.SetMaxOpenConns(1) // SQLite 不支持并发写
	db.SetMaxIdleConns(1)

	// 建表
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS messages (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			platform TEXT NOT NULL DEFAULT '',
			self_id  TEXT NOT NULL DEFAULT '',
			event_type TEXT NOT NULL DEFAULT '',
			message_type TEXT NOT NULL DEFAULT '',
			message_id TEXT NOT NULL DEFAULT '',
			user_id  TEXT NOT NULL DEFAULT '',
			group_id TEXT NOT NULL DEFAULT '',
			message  TEXT NOT NULL DEFAULT '',
			raw_data TEXT NOT NULL DEFAULT '',
			created_at INTEGER NOT NULL DEFAULT (unixepoch())
		);
		CREATE INDEX IF NOT EXISTS idx_messages_platform ON messages(platform);
		CREATE INDEX IF NOT EXISTS idx_messages_user_id ON messages(user_id);
		CREATE INDEX IF NOT EXISTS idx_messages_created_at ON messages(created_at);
	`)
	if err != nil {
		db.Close()
		return fmt.Errorf("建表失败: %w", err)
	}

	s.db = db
	log.Info("SQLite 存储已打开", "path", s.dbPath)
	return nil
}

func (s *sqliteStore) Close() {
	if s.db != nil {
		s.db.Close()
		log.Info("SQLite 存储已关闭")
	}
}

func (s *sqliteStore) InsertMsg(msg map[string]any) error {
	if s.db == nil {
		return fmt.Errorf("数据库未打开")
	}

	raw, _ := json.Marshal(msg)

	_, err := s.db.Exec(`
		INSERT INTO messages (platform, self_id, event_type, message_type, message_id, user_id, group_id, message, raw_data, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		getStr(msg, "platform"),
		getStr(msg, "self_id"),
		getStr(msg, "event_type"),
		getStr(msg, "message_type"),
		getStr(msg, "message_id"),
		getStr(msg, "user_id"),
		getStr(msg, "group_id"),
		getStr(msg, "message"),
		string(raw),
		time.Now().Unix(),
	)
	return err
}

func (s *sqliteStore) QueryMsg(filter map[string]any) ([]map[string]any, error) {
	if s.db == nil {
		return nil, fmt.Errorf("数据库未打开")
	}

	query := "SELECT id, platform, self_id, event_type, message_type, message_id, user_id, group_id, message, raw_data, created_at FROM messages WHERE 1=1"
	args := []any{}

	if v, ok := filter["platform"]; ok {
		query += " AND platform = ?"
		args = append(args, v)
	}
	if v, ok := filter["user_id"]; ok {
		query += " AND user_id = ?"
		args = append(args, v)
	}
	if v, ok := filter["group_id"]; ok {
		query += " AND group_id = ?"
		args = append(args, v)
	}
	if v, ok := filter["message_type"]; ok {
		query += " AND message_type = ?"
		args = append(args, v)
	}

	// 限制条数
	limit := 100
	if v, ok := filter["limit"]; ok {
		if l, ok2 := v.(int); ok2 && l > 0 && l <= 1000 {
			limit = l
		}
	}
	query += " ORDER BY created_at DESC LIMIT ?"
	args = append(args, limit)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []map[string]any
	for rows.Next() {
		var id int64
		var platform, selfID, eventType, msgType, msgID, userID, groupID, message, rawData string
		var createdAt int64
		if err := rows.Scan(&id, &platform, &selfID, &eventType, &msgType, &msgID, &userID, &groupID, &message, &rawData, &createdAt); err != nil {
			return nil, err
		}
		results = append(results, map[string]any{
			"id":           id,
			"platform":     platform,
			"self_id":      selfID,
			"event_type":   eventType,
			"message_type": msgType,
			"message_id":   msgID,
			"user_id":      userID,
			"group_id":     groupID,
			"message":      message,
			"raw_data":     rawData,
			"created_at":   createdAt,
		})
	}
	return results, rows.Err()
}

func getStr(m map[string]any, key string) string {
	if v, ok := m[key]; ok {
		if s, ok2 := v.(string); ok2 {
			return s
		}
	}
	return ""
}

// --- PostgreSQL 实现（骨架，需引入 lib/pq 或 pgx） ---

type postgresStore struct {
	connStr string
}

func newPostgresStore(connStr string) *postgresStore {
	return &postgresStore{connStr: connStr}
}

func (s *postgresStore) Open() error {
	return fmt.Errorf("PostgreSQL 支持尚未实现：请设置 postgres.enabled=false 使用内置 SQLite 存储，或自行接入 lib/pq / pgx")
}

func (s *postgresStore) Close() {}

func (s *postgresStore) InsertMsg(msg map[string]any) error {
	return fmt.Errorf("PostgreSQL 支持尚未实现")
}

func (s *postgresStore) QueryMsg(filter map[string]any) ([]map[string]any, error) {
	return nil, fmt.Errorf("PostgreSQL 支持尚未实现")
}
