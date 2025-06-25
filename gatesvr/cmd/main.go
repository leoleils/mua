package main

import (
	"log"
	"net"

	"google.golang.org/grpc"
	"mua/gatesvr/internal/pb"
	"mua/gatesvr/internal/conn"
	"context"
	"mua/gatesvr/internal/session"
	"mua/gatesvr/internal/kafka"
	"mua/gatesvr/internal/nacos"
	"strconv"
	"time"
	"mua/gatesvr/internal/rpc"
	"google.golang.org/protobuf/proto"
)

// 实现gRPC服务
// 这里只做空实现，后续补充业务逻辑

type server struct {
	pb.UnimplementedGateSvrServer
}

var playerRouteMap = make(map[string]string) // playerID -> gatesvrID

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
	// 不在本节点，查路由并远程调用
	if gatesvrID, ok := playerRouteMap[playerID]; ok {
		addrList := nacos.GetAllInstances()
		for _, addr := range addrList {
			if addr == gatesvrID {
				err := rpc.KickPlayerRemote(addr, playerID, reason)
				if err == nil {
					return &pb.KickPlayerResponse{Success: true, Message: "远程踢下线成功"}, nil
				}
			}
		}
	}
	return &pb.KickPlayerResponse{Success: false, Message: "未找到目标玩家"}, nil
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
	if conn.SendToPlayerWS(playerID, &gm) {
		log.Printf("[本节点-WS] 推送消息到玩家: %s", playerID)
		return &pb.ForwardMessageResponse{Success: true, Message: "本节点WS推送成功"}, nil
	}
	// 不在本节点，查路由并远程调用
	if gatesvrID, ok := playerRouteMap[playerID]; ok {
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
	if conn.SendToPlayer(playerID, gm) || conn.SendToPlayerWS(playerID, gm) {
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
	if gatesvrID, ok := playerRouteMap[playerID]; ok {
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

func main() {
	// 注册到Nacos
	instanceID := "gatesvr-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	ip := "127.0.0.1" // 实际部署时应获取本机真实IP
	grpcPort := uint64(50051)
	nacos.Register(instanceID, ip, grpcPort)

	// 订阅Kafka玩家上线
	kafka.Subscribe(kafka.KafkaTopicOnline, func(playerID, gatesvrID string) {
		playerRouteMap[playerID] = gatesvrID
		log.Printf("[路由] 玩家[%s] 上线于 %s", playerID, gatesvrID)
	})
	// 订阅Kafka玩家下线
	kafka.Subscribe(kafka.KafkaTopicOffline, func(playerID, gatesvrID string) {
		if v, ok := playerRouteMap[playerID]; ok && v == gatesvrID {
			delete(playerRouteMap, playerID)
			log.Printf("[路由] 玩家[%s] 从 %s 下线", playerID, gatesvrID)
		}
	})

	// 注册TCP业务分发
	conn.RegisterHandler(1, func(playerID string, msg *pb.GameMessage) {
		log.Printf("[业务1][TCP] 玩家[%s] 数据: %s", playerID, string(msg.Payload))
	})
	conn.RegisterHandler(2, func(playerID string, msg *pb.GameMessage) {
		log.Printf("[业务2][TCP] 玩家[%s] 数据: %s", playerID, string(msg.Payload))
	})
	// 注册WebSocket业务分发
	conn.RegisterHandlerWS(1, func(playerID string, msg *pb.GameMessage) {
		log.Printf("[业务1][WS] 玩家[%s] 数据: %s", playerID, string(msg.Payload))
	})
	conn.RegisterHandlerWS(2, func(playerID string, msg *pb.GameMessage) {
		log.Printf("[业务2][WS] 玩家[%s] 数据: %s", playerID, string(msg.Payload))
	})

	// 启动gRPC服务
	go func() {
		lis, err := net.Listen("tcp", ":50051")
		if err != nil {
			log.Fatalf("failed to listen: %v", err)
		}
		grpcServer := grpc.NewServer()
		pb.RegisterGateSvrServer(grpcServer, &server{})
		log.Println("gRPC服务已启动，监听:50051")
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatalf("failed to serve: %v", err)
		}
	}()

	// 启动TCP接入服务
	go conn.StartTCPServer(":6001")

	// 启动WebSocket接入服务
	go conn.StartWSServer(":6002")

	// 阻塞主线程
	select {}
} 