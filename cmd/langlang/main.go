package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/super1207/langlang-go/internal/config"
	"github.com/super1207/langlang-go/internal/log"
)

func main() {
	cfgPath := flag.String("config", "config.json", "path to config file (JSON)")
	flag.Parse()

	// 加载配置
	cfg, err := config.Load(*cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "加载配置失败: %v\n", err)
		os.Exit(1)
	}

	// 初始化日志
	if err := log.Init(cfg.Log.Level, cfg.Log.File); err != nil {
		fmt.Fprintf(os.Stderr, "初始化日志失败: %v\n", err)
		os.Exit(1)
	}
	log.Info("LangLang-Go 启动中...", "config", *cfgPath)

	// 初始化资源目录
	ensureDirs(cfg)

	// 启动组件
	app := NewApp(cfg, *cfgPath)
	if err := app.Start(); err != nil {
		log.Fatal("启动应用失败", "error", err)
	}

	// 等待退出信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	log.Info("收到退出信号", "signal", sig)

	app.Shutdown()
	log.Info("LangLang-Go 已退出")
}

func ensureDirs(cfg *config.Config) {
	dirs := []string{
		cfg.Paths.Data,
		cfg.Paths.Plugins,
		cfg.Paths.Logs,
		cfg.Paths.Tmp,
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			fmt.Fprintf(os.Stderr, "创建目录失败 %s: %v\n", d, err)
			os.Exit(1)
		}
	}
}
