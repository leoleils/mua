package main

import (
	"fmt"
	"log"
	"net"

	"context"
	"mua/gatesvr/config"
	"mua/gatesvr/internal/conn"
	"mua/gatesvr/internal/kafka"
	"mua/gatesvr/internal/nacos"
	pb "mua/gatesvr/internal/pb"
	"mua/gatesvr/internal/route"
	"mua/gatesvr/internal/rpc"
	"mua/gatesvr/internal/session"
	"strconv"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
)

func init() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
}

// 实现gRPC服务
// 这里只做空实现，后续补充业务逻辑

type server struct {
	pb.UnimplementedGateSvrServer
}

// 新增通用服务实现

type commonServiceServer struct {
	pb.UnimplementedCommonServiceServer
}

// 踢下线实现
func (s *server) KickPlayer(ctx context.Context, req *pb.KickPlayerRequest) (*pb.KickPlayerResponse, error) {
	playerID := req.GetPlayerId()
	reason := req.GetReason()
	// 判断玩家是否在本节点
	if _, ok := session.GetSession(playerID); ok {
		session.KickPlayer(playerID, reason)
		return &pb.KickPlayerResponse{
			Success: true,
			Message: "本节点踢下线成功",
		}, nil
	}
	// 不在本节点，查路由并远程调用（只通过nacos实例遍历）
	if gatesvrID, ok := route.Get(playerID); ok {
		addr := nacos.GetAddrByInstanceID(gatesvrID)
		err := rpc.KickPlayerRemote(addr, playerID, reason)
		if err == nil {
			return &pb.KickPlayerResponse{Success: true, Message: "远程踢下线成功"}, nil
		}
	}
	return &pb.KickPlayerResponse{Success: false, Message: "未找到目标玩家路由"}, nil
}

// 消息转发实现
func (s *server) ForwardMessage(ctx context.Context, req *pb.ForwardMessageRequest) (*pb.ForwardMessageResponse, error) {
	playerID := req.GetPlayerId()
	payload := req.GetPayload()
	var gm pb.GameMessage
	if err := proto.Unmarshal(payload, &gm); err != nil {
		return &pb.ForwardMessageResponse{Success: false, Message: "消息反序列化失败"}, nil
	}
	if conn.SendToPlayer(playerID, &gm) {
		log.Printf("[本节点-TCP] 推送消息到玩家: %s", playerID)
		return &pb.ForwardMessageResponse{Success: true, Message: "本节点TCP推送成功"}, nil
	}
	if conn.SendToPlayer(playerID, &gm) {
		log.Printf("[本节点-WS] 推送消息到玩家: %s", playerID)
		return &pb.ForwardMessageResponse{Success: true, Message: "本节点WS推送成功"}, nil
	}
	// 不在本节点，查路由并远程调用
	if gatesvrID, ok := route.Get(playerID); ok {
		addrList := nacos.GetAllInstances()
		for _, addr := range addrList {
			if addr == gatesvrID {
				data, _ := proto.Marshal(&gm)
				err := rpc.ForwardMessageRemote(addr, playerID, data)
				if err == nil {
					return &pb.ForwardMessageResponse{Success: true, Message: "远程转发成功"}, nil
				}
			}
		}
	}
	return &pb.ForwardMessageResponse{Success: false, Message: "未找到目标玩家"}, nil
}

// PushToClient实现
func (s *server) PushToClient(ctx context.Context, req *pb.PushRequest) (*pb.PushResponse, error) {
	playerID := req.GetPlayerId()
	ip := req.GetIp()
	cbType := req.GetCbType()
	gm := req.GetMessage()

	// 支持通过ip查找playerID
	if playerID == "" && ip != "" {
		if pid, ok := session.GetPlayerIDByIP(ip); ok {
			playerID = pid
		}
	}
	if playerID == "" {
		log.Printf("[PushToClient] 未指定playerID或ip，消息丢弃")
		return &pb.PushResponse{Success: false, Message: "未指定playerID或ip"}, nil
	}

	// 本地推送
	if conn.SendToPlayer(playerID, gm) || conn.SendToPlayer(playerID, gm) {
		log.Printf("[PushToClient] 本节点推送成功: %s", playerID)
		if cbType == pb.CallbackType_SYNC {
			return &pb.PushResponse{Success: true, Message: "本节点推送成功"}, nil
		}
		if cbType == pb.CallbackType_ASYNC || cbType == pb.CallbackType_PUSH {
			go func() { /* 可扩展异步后处理 */ }()
			return &pb.PushResponse{Success: true, Message: "本节点异步/推送成功"}, nil
		}
	}

	// 远程推送
	if gatesvrID, ok := route.Get(playerID); ok {
		addrList := nacos.GetAllInstances()
		for _, addr := range addrList {
			if addr == gatesvrID {
				if cbType == pb.CallbackType_SYNC {
					err := rpc.PushToClientRemote(addr, req)
					if err == nil {
						return &pb.PushResponse{Success: true, Message: "远程推送成功"}, nil
					}
					return &pb.PushResponse{Success: false, Message: "远程推送失败"}, nil
				}
				if cbType == pb.CallbackType_ASYNC || cbType == pb.CallbackType_PUSH {
					go rpc.PushToClientRemote(addr, req)
					return &pb.PushResponse{Success: true, Message: "远程异步/推送成功"}, nil
				}
			}
		}
	}
	log.Printf("[PushToClient] 玩家[%s] 不在线，消息丢弃", playerID)
	return &pb.PushResponse{Success: false, Message: "玩家不在线，消息丢弃"}, nil
}

// 通用的发送消息接口实现
func (s *commonServiceServer) SendMessage(ctx context.Context, req *pb.GameMessage) (*pb.GameMessageResponse, error) {
	resp := &pb.GameMessageResponse{
		MsgHead: req.MsgHead,
		Ret:     0,
		Payload: &pb.GameMessageResponse_Reason{Reason: "OK"},
	}
	return resp, nil
}

func main() {
	loadAndWatchConfig()
	initAuth()
	initKafka()
	ip, grpcPort := initNacosAndRegister()
	initRouteTable()
	registerBusinessHandlers()

	// 启动TCP接入服务
	go conn.StartTCPServer(":6001")
	// 启动WebSocket接入服务
	go conn.StartWSServer(":6002")

	startGRPCServer(ip, grpcPort)
}

// 初始化配置和热更
func loadAndWatchConfig() {
	if err := config.LoadConfig(); err != nil {
		log.Fatalf("配置加载失败: %v", err)
	}
	config.WatchConfig()
}

// 初始化认证
func initAuth() {}

// 初始化Kafka
func initKafka() {
	kafka.Init()
}

// 初始化Nacos并注册服务
func initNacosAndRegister() (ip string, grpcPort uint64) {
	instanceID := "gatesvr-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	kafka.SetLocalGateSvrID(instanceID)
	ip = getLocalIP()
	kafka.SetLocalGateSvrIP(ip)
	grpcPort = 50051
	nacos.Register(instanceID, ip, grpcPort)
	return
}

// 初始化路由表维护逻辑
func initRouteTable() {
	// 启动Kafka历史玩家上下线消费，维护route
	kafka.StartPlayerEventConsumer(
		config.GetConfig().Kafka.Brokers,
		config.GetConfig().Kafka.Topic,
		config.GetConfig().Kafka.GroupID,
		func(evt *pb.PlayerStatusChanged, gateOnline bool) {
			switch evt.Event {
			case pb.PlayerStatusEventType_ONLINE:
				if gateOnline {
					// 如果是本地接入的消息，则忽略
					if evt.GatesvrId == nacos.GetLocalInstanceID() {
						return
					}
					route.Set(evt.PlayerId, evt.GatesvrId)
					log.Printf("[路由] 玩家[%s] 上线，路由到[%s]", evt.PlayerId, evt.GatesvrId)
				} else {

				}
			case pb.PlayerStatusEventType_OFFLINE:
				if v, ok := route.Get(evt.PlayerId); ok && v == evt.GatesvrId {
					route.Delete(evt.PlayerId)
				}
			}
		},
	)

	// 监听 gatesvr 实例变更，清理 route
	nacos.SubscribeServiceChange(func(onlineAddrs []string) {
		onlineSet := make(map[string]struct{})
		for _, addr := range onlineAddrs {
			onlineSet[addr] = struct{}{}
		}
		affected := route.CleanByOnlineGates(onlineSet)
		for _, pid := range affected {
			// 也可调用 session.PlayerOfflineFromKafka(pid) 做进一步清理
			log.Printf("[路由] 清理玩家[%s]，因 gatesvr 下线", pid)
		}
	})
}

// 注册业务 handler
func registerBusinessHandlers() {
	conn.RegisterHandler(1, func(playerID string, msg *pb.GameMessage) {
		log.Printf("[业务1][TCP] 玩家[%s] 数据: %s", playerID, string(msg.Payload))
	})
	conn.RegisterHandler(2, func(playerID string, msg *pb.GameMessage) {
		log.Printf("[业务2][TCP] 玩家[%s] 数据: %s", playerID, string(msg.Payload))
	})
}

// 启动gRPC服务
func startGRPCServer(ip string, grpcPort uint64) {
	lis, err := net.Listen("tcp", fmt.Sprintf("%s:%d", ip, grpcPort))
	if err != nil {
		log.Fatalf("监听端口失败: %v", err)
	}
	grpcServer := grpc.NewServer()
	pb.RegisterGateSvrServer(grpcServer, &server{})
	pb.RegisterCommonServiceServer(grpcServer, &commonServiceServer{})
	log.Printf("gRPC服务启动: %s:%d", ip, grpcPort)
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("gRPC服务启动失败: %v", err)
	}
}

func getLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "127.0.0.1"
	}
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() && ipnet.IP.To4() != nil {
			return ipnet.IP.String()
		}
	}
	return "127.0.0.1"
}
