package main

import (
	"sync"

	"github.com/super1207/langlang-go/internal/bot"
	"github.com/super1207/langlang-go/internal/bot/onebot11"
	"github.com/super1207/langlang-go/internal/bot/satori"
	"github.com/super1207/langlang-go/internal/bot/telegram"
	"github.com/super1207/langlang-go/internal/config"
	"github.com/super1207/langlang-go/internal/cron"
	"github.com/super1207/langlang-go/internal/db"
	"github.com/super1207/langlang-go/internal/log"
	"github.com/super1207/langlang-go/internal/mqtt"
	"github.com/super1207/langlang-go/internal/plugin"
	"github.com/super1207/langlang-go/internal/webui"
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

	for _, ob := range cfg.Bot.OneBot11 {
		if ob.URL == "" {
			continue
		}
		mode := ob.Mode
		if mode == "" {
			mode = "reverse"
		}
		conn := onebot11.NewConnector(mode, ob.URL, ob.AccessToken, ob.SelfID)
		app.botConnectors = append(app.botConnectors, conn)
	}

	for _, token := range cfg.Bot.Telegram {
		if token == "" {
			continue
		}
		conn := telegram.NewConnector(token)
		app.botConnectors = append(app.botConnectors, conn)
	}

	for _, sc := range cfg.Bot.Satori {
		if sc.URL == "" {
			continue
		}
		conn := satori.NewConnector(sc.URL, sc.Token, sc.SelfID, sc.APIURL)
		app.botConnectors = append(app.botConnectors, conn)
	}

	return app
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
