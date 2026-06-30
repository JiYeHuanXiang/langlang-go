package main

import (
	"sync"

	"github.com/jiyehuanxiang/langlang-go/internal/bot"
	"github.com/jiyehuanxiang/langlang-go/internal/bot/onebot11"
	"github.com/jiyehuanxiang/langlang-go/internal/bot/satori"
	"github.com/jiyehuanxiang/langlang-go/internal/bot/telegram"
	"github.com/jiyehuanxiang/langlang-go/internal/config"
	"github.com/jiyehuanxiang/langlang-go/internal/cron"
	"github.com/jiyehuanxiang/langlang-go/internal/db"
	"github.com/jiyehuanxiang/langlang-go/internal/log"
	"github.com/jiyehuanxiang/langlang-go/internal/mqtt"
	"github.com/jiyehuanxiang/langlang-go/internal/plugin"
	"github.com/jiyehuanxiang/langlang-go/internal/redlang"
	"github.com/jiyehuanxiang/langlang-go/internal/webui"
)

// App 是应用主结构，负责编排所有组件
type App struct {
	cfg     *config.Config
	plugins *plugin.Manager
	webui   *webui.Server
	botReg  *bot.Registry
	db      *db.Manager
	cron    *cron.Scheduler
	mqtt    *mqtt.Client

	botConnectors []bot.BotAdapter

	mu      sync.Mutex
	running bool
}

// NewApp 创建应用实例
func NewApp(cfg *config.Config, configPath string) *App {
	app := &App{
		cfg:    cfg,
		botReg: bot.GlobalRegistry,
	}

	// 创建插件管理器
	app.plugins = plugin.NewManager(cfg.Paths.Plugins)
	app.plugins.SetConfig(cfg)

	// 创建 Web UI 服务器
	app.webui = webui.NewServer(cfg, app.plugins)
	app.webui.SetConfigPath(configPath)
	app.webui.SetRebuildBotConnectors(app.rebuildBotConnectors)
	// db 和 botConnectors 会在后面的构造中赋值
	// (在 Start() 前通过 SetDB/SetBotControl 注入)

	// 创建数据库管理器
	app.db = db.NewManager(cfg.Postgres)

	// 创建定时任务调度器
	app.cron = cron.NewScheduler()

	// 创建 MQTT 集群客户端
	app.mqtt = mqtt.NewClient(cfg.MQTT)

	// 创建机器人连接器
	app.botConnectors = make([]bot.BotAdapter, 0)
	app.buildConnectors()

	return app
}

// buildConnectors 从当前 config 构建所有 bot 连接器，并放回 app.botConnectors
func (a *App) buildConnectors() {
	connectors := make([]bot.BotAdapter, 0)

	for _, ob := range a.cfg.Bot.OneBot11 {
		if ob.URL == "" {
			continue
		}
		if !ob.IsEnabled() {
			log.Info("跳过已禁用的 OneBot11 连接", "url", ob.URL, "self_id", ob.SelfID)
			continue
		}
		mode := ob.Mode
		if mode == "" {
			mode = "reverse"
		}
		conn := onebot11.NewConnector(mode, ob.URL, ob.AccessToken, ob.SelfID)
		connectors = append(connectors, conn)
	}

	for _, tg := range a.cfg.Bot.Telegram {
		if tg.Token == "" {
			continue
		}
		if !tg.IsEnabled() {
			log.Info("跳过已禁用的 Telegram 连接", "token_prefix", tg.Token[:min(8, len(tg.Token))]+"...")
			continue
		}
		conn := telegram.NewConnector(tg.Token)
		connectors = append(connectors, conn)
	}

	for _, sc := range a.cfg.Bot.Satori {
		if sc.URL == "" {
			continue
		}
		if !sc.IsEnabled() {
			log.Info("跳过已禁用的 Satori 连接", "url", sc.URL, "self_id", sc.SelfID)
			continue
		}
		conn := satori.NewConnector(sc.URL, sc.Token, sc.SelfID, sc.APIURL)
		connectors = append(connectors, conn)
	}

	a.botConnectors = connectors
}

// rebuildBotConnectors 停止旧连接，从 config 重建并重新注册所有 bot 连接器
func (a *App) rebuildBotConnectors() {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.running {
		return
	}

	log.Info("正在重建 bot 连接器...")

	// 停止旧连接并从注册表中注销
	for _, adapter := range a.botConnectors {
		a.botReg.Unregister(adapter.Platform(), adapter.SelfID())
		adapter.Stop()
	}

	// 重建
	a.buildConnectors()

	// 同步给 WebUI
	a.webui.SetBotControl(a.botReg, a.botConnectors)

	// 重新启动
	a.startBotConnections()

	log.Info("bot 连接器重建完成", "count", len(a.botConnectors))
}

// Start 启动应用
func (a *App) Start() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.running {
		return nil
	}

	log.Info("正在启动各组件...")

	// 0. 注入 DB / Bot 引用到 Web UI
	a.webui.SetDB(a.db)
	a.webui.SetBotControl(a.botReg, a.botConnectors)

	// 接线脚本运行时的 BotAPI（之前是 stub）
	redlang.GlobalApp.BotAPI = func(platform, selfID, action string, params map[string]any) (map[string]any, error) {
		return a.botReg.CallAPI(platform, selfID, action, params)
	}

	// 1. 启动数据库
	if err := a.db.Open(); err != nil {
		log.Warn("数据库启动失败（不影响主流程）", "error", err)
	} else {
		log.Info("数据库已就绪")
	}

	// 2. 启动 Web UI
	if err := a.webui.Start(); err != nil {
		return err
	}
	log.Info("Web UI 已启动", "listen", a.cfg.Web.Listen)

	// 3. 启动 MQTT 集群（如果启用）
	if a.cfg.MQTT.Enabled {
		if err := a.mqtt.Start(); err != nil {
			log.Warn("MQTT 启动失败", "error", err)
		} else {
			log.Info("MQTT 集群已就绪")
		}
	}

	// 4. 加载插件
	if err := a.plugins.LoadAll(); err != nil {
		log.Warn("插件加载有错误", "error", err)
	}
	log.Info("插件已加载", "count", a.plugins.Count())

	// 5. 启动机器人连接
	a.startBotConnections()

	// 6. 启动定时任务
	a.cron.AddJob("插件重载", "0 */30 * * * *", func() error {
		log.Info("定时重载插件")
		return a.plugins.LoadAll()
	})
	if err := a.cron.Start(); err != nil {
		log.Warn("定时任务启动失败", "error", err)
	} else {
		log.Info("定时任务已就绪")
	}

	a.running = true
	log.Info("LangLang-Go 启动完成")
	return nil
}

// Shutdown 关闭应用
func (a *App) Shutdown() {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.running {
		return
	}

	log.Info("正在关闭...")

	// 停止机器人连接
	for _, adapter := range a.botConnectors {
		adapter.Stop()
	}
	log.Info("机器人连接已停止")

	// 停止定时任务
	a.cron.Stop()

	// 停止 MQTT
	a.mqtt.Stop()

	// 停止 Web UI
	a.webui.Stop()

	// 保存插件数据
	a.plugins.SaveAll()

	// 关闭数据库
	a.db.Close()

	log.Info("LangLang-Go 已停止")
	a.running = false
}

// startBotConnections 启动所有机器人连接并注册到全局注册表
func (a *App) startBotConnections() {
	for _, adapter := range a.botConnectors {
		// 先注册到全局注册表
		a.botReg.Register(adapter)

		// 启动连接
		conn := adapter
		go func() {
			log.Info("正在启动机器人连接",
				"platform", conn.Platform(),
				"self_id", conn.SelfID(),
			)
			if err := conn.Start(); err != nil {
				log.Error("机器人连接启动失败",
					"platform", conn.Platform(),
					"self_id", conn.SelfID(),
					"error", err,
				)
			}
		}()
	}
}
