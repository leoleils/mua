package forwarder

import (
	"context"
	"fmt"
	"log"
	"time"

	"mua/gatesvr/config"
	"mua/gatesvr/internal/nacos"
	"mua/gatesvr/internal/pb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// MessageForwarder 消息转发器
type MessageForwarder struct {
	connections map[string]*grpc.ClientConn // 连接池
}

// NewMessageForwarder 创建消息转发器
func NewMessageForwarder() *MessageForwarder {
	return &MessageForwarder{
		connections: make(map[string]*grpc.ClientConn),
	}
}

// ForwardMessage 转发服务消息
func (f *MessageForwarder) ForwardMessage(msg *pb.GameMessage) (*pb.GameMessageResponse, error) {
	head := msg.MsgHead
	if head == nil {
		return nil, fmt.Errorf("消息头为空")
	}

	// 检查是否为服务消息
	if msg.MsgType != pb.MessageType_SERVICE_MESSAGE {
		return nil, fmt.Errorf("不是服务消息类型: %v", msg.MsgType)
	}

	serviceName := head.ServiceName
	if serviceName == "" {
		return nil, fmt.Errorf("服务名不能为空")
	}

	// 获取目标服务地址
	targetAddr, err := f.getTargetServiceAddr(head)
	if err != nil {
		return nil, fmt.Errorf("获取目标服务地址失败: %v", err)
	}

	// 根据消息类型选择处理方式
	switch head.ServiceMsgType {
	case pb.ServiceMessageType_SYNC:
		return f.forwardSync(targetAddr, msg)
	case pb.ServiceMessageType_ASYNC:
		return f.forwardAsync(targetAddr, msg)
	default:
		return nil, fmt.Errorf("未知的服务消息类型: %v", head.ServiceMsgType)
	}
}

// getTargetServiceAddr 获取目标服务地址
func (f *MessageForwarder) getTargetServiceAddr(head *pb.HeadMessage) (string, error) {
	serviceName := head.ServiceName
	groupName := head.Group
	instanceID := head.InstanceId

	// 如果指定了实例ID，直接通过实例ID获取地址
	if instanceID != "" {
		log.Printf("[消息转发] 使用指定实例ID: %s", instanceID)

		if groupName == "" {
			groupName = "DEFAULT_GROUP"
		}

		addr, err := nacos.GetRPCAddrByInstanceID(serviceName, groupName, instanceID)
		if err != nil {
			return "", fmt.Errorf("通过实例ID获取地址失败: %v", err)
		}
		return addr, nil
	}

	// 获取服务配置
	serviceConfig := config.GetServiceConfig(serviceName)

	// 使用配置中的分组（如果消息头没有指定）
	if groupName == "" {
		groupName = serviceConfig.Group
	}

	// 根据负载均衡策略选择实例
	strategy := f.getLoadBalanceStrategy(head, serviceConfig)

	var addr string
	var err error

	switch strategy {
	case "round_robin":
		if groupName == "DEFAULT_GROUP" {
			addr, err = nacos.GetRPCAddrRoundRobin(serviceName)
		} else {
			addr, err = nacos.GetRPCAddrRoundRobinWithGroup(serviceName, groupName)
		}
	case "weighted":
		if groupName == "DEFAULT_GROUP" {
			addr, err = nacos.GetRPCAddrWeighted(serviceName)
		} else {
			addr, err = nacos.GetRPCAddrWeightedWithGroup(serviceName, groupName)
		}
	default:
		return "", fmt.Errorf("不支持的负载均衡策略: %s", strategy)
	}

	if err != nil {
		return "", fmt.Errorf("负载均衡获取地址失败: %v", err)
	}

	log.Printf("[消息转发] 服务=%s@%s, 策略=%s, 地址=%s", serviceName, groupName, strategy, addr)
	return addr, nil
}

// getLoadBalanceStrategy 获取负载均衡策略
func (f *MessageForwarder) getLoadBalanceStrategy(head *pb.HeadMessage, serviceConfig config.ServiceConfig) string {
	// 优先使用消息头中指定的策略
	if head.LoadBalanceStrategy != "" {
		return head.LoadBalanceStrategy
	}

	// 使用服务配置中的策略
	return serviceConfig.LoadBalance
}

// forwardSync 同步转发（等待回包）
func (f *MessageForwarder) forwardSync(targetAddr string, msg *pb.GameMessage) (*pb.GameMessageResponse, error) {
	log.Printf("[同步转发] 目标地址: %s, 玩家: %s", targetAddr, msg.MsgHead.PlayerId)

	// 获取连接
	conn, err := f.getConnection(targetAddr)
	if err != nil {
		return nil, fmt.Errorf("获取连接失败: %v", err)
	}

	// 创建客户端
	client := pb.NewCommonServiceClient(conn)

	// 设置超时
	serviceConfig := config.GetServiceConfig(msg.MsgHead.ServiceName)
	timeout := time.Duration(serviceConfig.TimeoutMs) * time.Millisecond
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// 设置响应时间戳
	msg.MsgHead.Timestamp = time.Now().UnixMilli()

	// 发送请求
	resp, err := client.SendMessage(ctx, msg)
	if err != nil {
		return nil, fmt.Errorf("RPC调用失败: %v", err)
	}

	// 设置响应时间戳
	resp.ResponseTimestamp = time.Now().UnixMilli()

	log.Printf("[同步转发成功] 玩家: %s, 耗时: %dms",
		msg.MsgHead.PlayerId,
		resp.ResponseTimestamp-msg.MsgHead.Timestamp)

	return resp, nil
}

// forwardAsync 异步转发（不等待回包）
func (f *MessageForwarder) forwardAsync(targetAddr string, msg *pb.GameMessage) (*pb.GameMessageResponse, error) {
	log.Printf("[异步转发] 目标地址: %s, 玩家: %s", targetAddr, msg.MsgHead.PlayerId)

	// 异步执行转发
	go func() {
		conn, err := f.getConnection(targetAddr)
		if err != nil {
			log.Printf("[异步转发失败] 获取连接失败: %v", err)
			return
		}

		client := pb.NewCommonServiceClient(conn)

		// 异步请求使用较短的超时时间
		cfg := config.GetConfig()
		timeout := time.Duration(cfg.ServiceForwarding.AsyncTimeoutMs) * time.Millisecond
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		msg.MsgHead.Timestamp = time.Now().UnixMilli()

		_, err = client.SendMessage(ctx, msg)
		if err != nil {
			log.Printf("[异步转发失败] RPC调用失败: %v", err)
		} else {
			log.Printf("[异步转发成功] 玩家: %s", msg.MsgHead.PlayerId)
		}
	}()

	// 立即返回成功响应
	return &pb.GameMessageResponse{
		MsgHead:           msg.MsgHead,
		Ret:               0,
		Payload:           &pb.GameMessageResponse_Reason{Reason: "异步处理中"},
		ResponseTimestamp: time.Now().UnixMilli(),
	}, nil
}

// getConnection 获取或创建到目标服务的连接
func (f *MessageForwarder) getConnection(targetAddr string) (*grpc.ClientConn, error) {
	// 检查是否已有连接
	if conn, exists := f.connections[targetAddr]; exists {
		if conn.GetState().String() == "READY" || conn.GetState().String() == "IDLE" {
			return conn, nil
		}
		// 连接状态不正常，关闭旧连接
		conn.Close()
		delete(f.connections, targetAddr)
	}

	// 创建新连接
	conn, err := grpc.Dial(targetAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("创建gRPC连接失败: %v", err)
	}

	// 缓存连接
	f.connections[targetAddr] = conn
	log.Printf("[连接池] 创建新连接: %s", targetAddr)

	return conn, nil
}

// Close 关闭转发器，清理所有连接
func (f *MessageForwarder) Close() {
	for addr, conn := range f.connections {
		conn.Close()
		log.Printf("[连接池] 关闭连接: %s", addr)
	}
	f.connections = make(map[string]*grpc.ClientConn)
}

// 全局转发器实例
var globalForwarder *MessageForwarder

// Init 初始化全局转发器
func Init() {
	globalForwarder = NewMessageForwarder()
	log.Println("消息转发器初始化完成")
}

// GetForwarder 获取全局转发器
func GetForwarder() *MessageForwarder {
	if globalForwarder == nil {
		globalForwarder = NewMessageForwarder()
	}
	return globalForwarder
}

// ForwardServiceMessage 转发服务消息（便捷函数）
func ForwardServiceMessage(msg *pb.GameMessage) (*pb.GameMessageResponse, error) {
	return GetForwarder().ForwardMessage(msg)
}
