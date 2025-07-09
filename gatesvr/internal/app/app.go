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

type commonServiceServer struct {
	pb.UnimplementedCommonServiceServer
}

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
	pb.RegisterCommonServiceServer(a.grpcSrv, &commonServiceServer{})
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

// 以下为原 main.go 的辅助函数
func initAuth() {}

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

func initRouteTable() {
	// route.Init() // route 包无初始化方法，如有需要可补充
}

func registerBusinessHandlers() {
	// 业务 handler 注册
}

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

func (s *commonServiceServer) SendMessage(ctx context.Context, req *pb.GameMessage) (*pb.GameMessageResponse, error) {
	log.Printf("[SendMessage] 收到消息: 类型=%v, 玩家=%s", req.MsgType, req.MsgHead.GetPlayerId())

	// 根据消息类型处理
	switch req.MsgType {
	case pb.MessageType_HEARTBEAT:
		return s.handleHeartbeat(ctx, req)
	case pb.MessageType_SERVICE_MESSAGE:
		return s.handleServiceMessage(ctx, req)
	case pb.MessageType_CLIENT_MESSAGE:
		return s.handleClientMessage(ctx, req)
	case pb.MessageType_BROADCAST_MESSAGE:
		return s.handleBroadcastMessage(ctx, req)
	default:
		return &pb.GameMessageResponse{
			MsgHead:           req.MsgHead,
			Ret:               1,
			Payload:           &pb.GameMessageResponse_Reason{Reason: "未知的消息类型"},
			ResponseTimestamp: time.Now().UnixMilli(),
		}, nil
	}
}

// handleHeartbeat 处理心跳消息
func (s *commonServiceServer) handleHeartbeat(ctx context.Context, req *pb.GameMessage) (*pb.GameMessageResponse, error) {
	log.Printf("[心跳] 玩家: %s", req.MsgHead.GetPlayerId())

	return &pb.GameMessageResponse{
		MsgHead:           req.MsgHead,
		Ret:               0,
		Payload:           &pb.GameMessageResponse_Reason{Reason: "心跳OK"},
		ResponseTimestamp: time.Now().UnixMilli(),
	}, nil
}

// handleServiceMessage 处理服务消息（转发到后端服务）
func (s *commonServiceServer) handleServiceMessage(ctx context.Context, req *pb.GameMessage) (*pb.GameMessageResponse, error) {
	head := req.MsgHead
	if head == nil {
		return &pb.GameMessageResponse{
			MsgHead:           req.MsgHead,
			Ret:               1,
			Payload:           &pb.GameMessageResponse_Reason{Reason: "消息头为空"},
			ResponseTimestamp: time.Now().UnixMilli(),
		}, nil
	}

	serviceName := head.ServiceName
	if serviceName == "" {
		return &pb.GameMessageResponse{
			MsgHead:           req.MsgHead,
			Ret:               1,
			Payload:           &pb.GameMessageResponse_Reason{Reason: "服务名不能为空"},
			ResponseTimestamp: time.Now().UnixMilli(),
		}, nil
	}

	log.Printf("[服务消息] 玩家=%s, 服务=%s, 分组=%s, 实例=%s, 类型=%v",
		head.PlayerId, head.ServiceName, head.Group, head.InstanceId, head.ServiceMsgType)

	// 使用转发器转发消息
	return forwarder.ForwardServiceMessage(req)
}

// handleClientMessage 处理客户端消息（网关内部处理）
func (s *commonServiceServer) handleClientMessage(ctx context.Context, req *pb.GameMessage) (*pb.GameMessageResponse, error) {
	log.Printf("[客户端消息] 玩家: %s", req.MsgHead.GetPlayerId())

	// 这里可以实现网关内部的业务逻辑
	// 例如：玩家状态查询、连接管理等

	return &pb.GameMessageResponse{
		MsgHead:           req.MsgHead,
		Ret:               0,
		Payload:           &pb.GameMessageResponse_Reason{Reason: "客户端消息处理完成"},
		ResponseTimestamp: time.Now().UnixMilli(),
	}, nil
}

// handleBroadcastMessage 处理广播消息
func (s *commonServiceServer) handleBroadcastMessage(ctx context.Context, req *pb.GameMessage) (*pb.GameMessageResponse, error) {
	log.Printf("[广播消息] 玩家: %s", req.MsgHead.GetPlayerId())

	// 这里可以实现广播逻辑
	// 例如：全服公告、世界聊天等

	return &pb.GameMessageResponse{
		MsgHead:           req.MsgHead,
		Ret:               0,
		Payload:           &pb.GameMessageResponse_Reason{Reason: "广播消息处理完成"},
		ResponseTimestamp: time.Now().UnixMilli(),
	}, nil
}

func New() *App {
	return &App{}
}
