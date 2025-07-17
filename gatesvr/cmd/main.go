package main

import (
	"flag"
	"log"
	"mua/gatesvr/config"
	"mua/gatesvr/internal/app"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	// 新增：解析 -config 启动参数
	configFile := flag.String("config", "", "配置文件路径（可选，默认使用config-local.yaml）")
	flag.Parse()
	if *configFile != "" {
		config.SetConfigPath(*configFile)
	}
	a := app.New()
	if err := a.Init(); err != nil {
		log.Fatalf("init error: %v", err)
	}
	go func() {
		if err := a.Run(); err != nil {
			log.Fatalf("run error: %v", err)
		}
	}()
	// 优雅退出
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	a.Stop()
}
