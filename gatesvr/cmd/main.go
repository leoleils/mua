package main

import (
	"log"
	"mua/gatesvr/internal/app"
	"os"
	"os/signal"
	"syscall"
)

func main() {
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
