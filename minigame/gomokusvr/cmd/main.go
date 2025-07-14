package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"mua/minigame/gomokusvr/internal/config"
	gatesvrpb "mua/gatesvr/pb"
	"mua/minigame/gomokusvr/internal/handler"
	"mua/minigame/gomokusvr/internal/nacos"
	"mua/minigame/gomokusvr/internal/pb"
	"mua/minigame/gomokusvr/internal/room"

	"google.golang.org/grpc"
)

func main() {
	log.Println("五子棋服务启动中...")

	// 1. 加载配置
	if err := config.LoadConfig(); err != nil {
		log.Fatalf("配置加载失败: %v", err)
	}

	// 2. 初始化房间管理器
	room.InitRoomManager()

	// 3. 初始化Nacos客户端（如果启用）
	nacosConfig := config.GetNacosConfig()
	if nacosConfig.EnableRegister {
		if err := nacos.InitNacos(); err != nil {
			log.Fatalf("Nacos初始化失败: %v", err)
		}

		// 4. 注册服务到Nacos
		if err := nacos.RegisterService(); err != nil {
			log.Fatalf("服务注册失败: %v", err)
		}
		log.Println("Nacos服务注册成功")
	} else {
		log.Println("Nacos注册已禁用，跳过服务注册")
	}

	// 5. 启动gRPC服务器
	serviceConfig := config.GetServiceConfig()
	listen, err := net.Listen("tcp", fmt.Sprintf(":%d", serviceConfig.Port))
	if err != nil {
		log.Fatalf("监听端口失败: %v", err)
	}

	server := grpc.NewServer()
	gameHandler := handler.NewGameHandler()
	
	// 注册gomoku自己的服务
	gatewayHandler := handler.NewGatewayHandler(gameHandler)
	pb.RegisterCommonServiceServer(server, gatewayHandler)
	log.Println("已注册 gomoku.CommonService")

	// 注册gatesvr兼容的common服务
	gatesvrAdapter := handler.NewGatesvrAdapter(gatewayHandler)
	gatesvrpb.RegisterCommonServiceServer(server, gatesvrAdapter)
	log.Println("已注册 common.CommonService (gatesvr兼容)")

	// 启动服务器
	go func() {
		log.Printf("五子棋服务已启动，监听端口: %d", serviceConfig.Port)
		if err := server.Serve(listen); err != nil {
			log.Fatalf("gRPC服务启动失败: %v", err)
		}
	}()

	// 6. 等待退出信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("正在关闭五子棋服务...")

	// 7. 优雅关闭
	server.GracefulStop()

	// 8. 注销服务（如果启用了注册）
	if nacosConfig.EnableRegister {
		if err := nacos.DeregisterService(); err != nil {
			log.Printf("服务注销失败: %v", err)
		} else {
			log.Println("服务注销成功")
		}
	}

	log.Println("五子棋服务已关闭")
}
