package app

import (
	"context"
	"fmt"
	"log"
	"mua/gatesvr/config"
	"mua/gatesvr/internal/conn"
	"mua/gatesvr/internal/forwarder"
	"mua/gatesvr/internal/kafka"
	"mua/gatesvr/internal/nacos"
	"mua/gatesvr/internal/pb"
	"mua/gatesvr/internal/route"
	"mua/gatesvr/internal/rpc"
	"mua/gatesvr/internal/session"
	"net"
	"strconv"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
)

// App 应用程序结构体
type App struct {
	ip       string
	grpcPort uint64
	grpcSrv  *grpc.Server
	grpcLis  net.Listener
	cancel   context.CancelFunc
}

type server struct {
	pb.UnimplementedGateSvrServer
}

// Init 初始化应用程序，包括配置加载、各种服务初始化等
func (a *App) Init() error {
	if err := config.LoadConfig(); err != nil {
		log.Fatalf("配置加载失败: %v", err)
	}
	config.WatchConfig()
	initAuth()
	kafka.Init()
	a.ip, a.grpcPort = initNacosAndRegister()
	initRouteTable()
	registerBusinessHandlers()

	// 初始化消息转发器
	log.Println("初始化消息转发器...")
	forwarder.Init()

	return nil
}

// Run 运行应用程序，启动TCP、WebSocket和gRPC服务
func (a *App) Run() error {
	// 启动TCP接入服务
	go conn.StartTCPServer(":6001")
	// 启动WebSocket接入服务
	go conn.StartWSServer(":6002")

	// 启动gRPC服务
	lis, err := net.Listen("tcp", fmt.Sprintf("%s:%d", a.ip, a.grpcPort))
	if err != nil {
		return err
	}
	a.grpcLis = lis
	a.grpcSrv = grpc.NewServer()
	pb.RegisterGateSvrServer(a.grpcSrv, &server{})
	// 移除 CommonService 注册，因为不再需要 SendMessage RPC 接口
	// pb.RegisterCommonServiceServer(a.grpcSrv, &commonServiceServer{})
	ctx, cancel := context.WithCancel(context.Background())
	a.cancel = cancel
	go func() {
		if err := a.grpcSrv.Serve(lis); err != nil {
			log.Printf("gRPC服务异常退出: %v", err)
		}
	}()
	<-ctx.Done() // 阻塞直到 Stop 被调用
	return nil
}

// Stop 停止应用程序，优雅关闭所有服务
func (a *App) Stop() {
	if a.cancel != nil {
		a.cancel()
	}
	if a.grpcSrv != nil {
		a.grpcSrv.GracefulStop()
	}
	if a.grpcLis != nil {
		a.grpcLis.Close()
	}
	log.Println("服务已优雅退出")
}

// 初始化认证模块
func initAuth() {}

// 初始化Nacos并注册服务实例
func initNacosAndRegister() (ip string, grpcPort uint64) {
	instanceID := "gatesvr-" + strconv.FormatInt(time.Now().UnixNano(), 10)

	// 设置本地实例ID到配置和kafka
	config.SetGatesvrID(instanceID)
	kafka.SetLocalGateSvrID(instanceID)

	ip = getLocalIP()
	kafka.SetLocalGateSvrIP(ip)
	grpcPort = 50051
	nacos.Register(instanceID, ip, grpcPort)
	log.Printf("服务实例ID: %s, IP: %s, gRPC端口: %d", instanceID, ip, grpcPort)
	return
}

// 初始化路由表
func initRouteTable() {
	// route.Init() // route 包无初始化方法，如有需要可补充
}

// 注册业务处理器
func registerBusinessHandlers() {
	// 业务 handler 注册
}

// 获取本地IP地址
func getLocalIP() string {
	// 获取本地非回环IP地址
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		log.Printf("获取本地IP失败: %v", err)
		return "127.0.0.1"
	}

	for _, addr := range addrs {
		if ipNet, ok := addr.(*net.IPNet); ok && !ipNet.IP.IsLoopback() {
			if ipNet.IP.To4() != nil {
				ip := ipNet.IP.String()
				log.Printf("检测到本地IP: %s", ip)
				return ip
			}
		}
	}

	log.Printf("未找到合适的本地IP，使用默认值")
	return "127.0.0.1"
}

// gRPC 业务实现（可根据 main.go 迁移）
func (s *server) KickPlayer(ctx context.Context, req *pb.KickPlayerRequest) (*pb.KickPlayerResponse, error) {
	playerID := req.GetPlayerId()
	reason := req.GetReason()
	if _, ok := session.GetSession(playerID); ok {
		session.KickPlayer(playerID, reason)
		return &pb.KickPlayerResponse{
			Success: true,
			Message: "本节点踢下线成功",
		}, nil
	}
	if gatesvrID, ok := route.Get(playerID); ok {
		addr := nacos.GetGatesvrAddrByInstanceID(gatesvrID)
		err := rpc.KickPlayerRemote(addr, playerID, reason)
		if err == nil {
			return &pb.KickPlayerResponse{Success: true, Message: "远程踢下线成功"}, nil
		}
	}
	return &pb.KickPlayerResponse{Success: false, Message: "未找到目标玩家路由"}, nil
}

func (s *server) ForwardMessage(ctx context.Context, req *pb.ForwardMessageRequest) (*pb.ForwardMessageResponse, error) {
	playerID := req.GetPlayerId()
	payload := req.GetPayload()
	var gm pb.GameMessage
	if err := proto.Unmarshal(payload, &gm); err != nil {
		return &pb.ForwardMessageResponse{Success: false, Message: "消息反序列化失败"}, nil
	}

	if gatesvrID, ok := route.Get(playerID); ok {
		if gatesvrID == config.GetGatesvrID() {
			//本节点接入的玩家
			if conn.SendToPlayer(playerID, &gm) {
				log.Printf("[本节点] 推送消息到玩家: %s", playerID)
				return &pb.ForwardMessageResponse{Success: true, Message: "推送成功"}, nil
			}
		} else {
			// 远程节点接入的玩家，根据实例ID获取地址
			addr := nacos.GetGatesvrAddrByInstanceID(gatesvrID)
			if addr != "" {
				data, _ := proto.Marshal(&gm)
				err := rpc.ForwardMessageRemote(addr, playerID, data)
				if err == nil {
					return &pb.ForwardMessageResponse{Success: true, Message: "远程转发成功"}, nil
				}
				log.Printf("[远程转发失败] addr=%s, err=%v", addr, err)
			} else {
				log.Printf("[远程转发失败] 未找到实例地址: %s", gatesvrID)
			}
		}
	}
	return &pb.ForwardMessageResponse{Success: false, Message: "未找到目标玩家"}, nil
}

func (s *server) PushToClient(ctx context.Context, req *pb.PushRequest) (*pb.PushResponse, error) {
	playerID := req.GetPlayerId()
	ip := req.GetIp()
	cbType := req.GetCbType()
	gm := req.GetMessage()
	if playerID == "" && ip != "" {
		if pid, ok := session.GetPlayerIDByIP(ip); ok {
			playerID = pid
		}
	}
	if playerID == "" {
		log.Printf("[PushToClient] 未指定playerID或ip，消息丢弃")
		return &pb.PushResponse{Success: false, Message: "未指定playerID或ip"}, nil
	}

	// 先尝试本地推送
	if conn.SendToPlayer(playerID, gm) {
		log.Printf("[PushToClient] 本节点推送成功: %s", playerID)
		if cbType == pb.CallbackType_SYNC {
			return &pb.PushResponse{Success: true, Message: "本节点推送成功"}, nil
		}
		if cbType == pb.CallbackType_ASYNC || cbType == pb.CallbackType_PUSH {
			go func() { /* 可扩展异步后处理 */ }()
			return &pb.PushResponse{Success: true, Message: "本节点异步/推送成功"}, nil
		}
	}

	// 本地推送失败，尝试远程推送
	if gatesvrID, ok := route.Get(playerID); ok {
		addr := nacos.GetGatesvrAddrByInstanceID(gatesvrID)
		if addr != "" {
			if cbType == pb.CallbackType_SYNC {
				err := rpc.PushToClientRemote(addr, req)
				if err == nil {
					return &pb.PushResponse{Success: true, Message: "远程推送成功"}, nil
				}
				log.Printf("[远程推送失败] addr=%s, err=%v", addr, err)
				return &pb.PushResponse{Success: false, Message: "远程推送失败"}, nil
			}
			if cbType == pb.CallbackType_ASYNC || cbType == pb.CallbackType_PUSH {
				go rpc.PushToClientRemote(addr, req)
				return &pb.PushResponse{Success: true, Message: "远程异步/推送成功"}, nil
			}
		} else {
			log.Printf("[远程推送失败] 未找到实例地址: %s", gatesvrID)
		}
	}
	log.Printf("[PushToClient] 玩家[%s] 不在线，消息丢弃", playerID)
	return &pb.PushResponse{Success: false, Message: "玩家不在线，消息丢弃"}, nil
}

// New 创建新的应用实例
func New() *App {
	return &App{}
}
