package gateclient

import (
	"context"
	"fmt"
	"log"

	commonpb "mua/gatesvr/pb"
	gatesvrpb "mua/gatesvr/pb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// GateSvrClient 封装gatesvr的gRPC客户端
type GateSvrClient interface {
	PushToClient(ctx context.Context, req *gatesvrpb.PushRequest) (*gatesvrpb.PushResponse, error)
}

// gateSvrClient gatesvr客户端实现
type gateSvrClient struct {
	client gatesvrpb.GateSvrClient
}

// NewGateSvrClient 创建gatesvr客户端
func NewGateSvrClient(conn *grpc.ClientConn) GateSvrClient {
	return &gateSvrClient{
		client: gatesvrpb.NewGateSvrClient(conn),
	}
}

// PushToClient 调用gatesvr的PushToClient方法
func (c *gateSvrClient) PushToClient(ctx context.Context, req *gatesvrpb.PushRequest) (*gatesvrpb.PushResponse, error) {
	// 直接调用gRPC客户端
	resp, err := c.client.PushToClient(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("gRPC调用失败: %v", err)
	}

	log.Printf("[GateClient] 推送调用成功: 玩家=%s, 成功=%t, 消息=%s", 
		req.GetPlayerId(), resp.GetSuccess(), resp.GetMessage())
	
	return resp, nil
}

// CreateConnection 创建到gatesvr的gRPC连接
func CreateConnection(addr string) (*grpc.ClientConn, error) {
	conn, err := grpc.Dial(addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(4*1024*1024)),
	)
	if err != nil {
		return nil, fmt.Errorf("创建gRPC连接失败: %v", err)
	}

	log.Printf("[GateClient] 连接创建成功: %s", addr)
	return conn, nil
}

// 便捷方法：构造PushRequest
func NewPushRequest(playerID string, gameMsg *commonpb.GameMessage) *gatesvrpb.PushRequest {
	return &gatesvrpb.PushRequest{
		PlayerId: playerID,
		Ip:       "", // 通常为空，让gatesvr自动检测
		CbType:   gatesvrpb.CallbackType_PUSH, // 推送类型
		Message:  gameMsg,
	}
}

// 便捷方法：构造GameMessage
func NewGameMessage(playerID, method string, payload []byte) *commonpb.GameMessage {
	return &commonpb.GameMessage{
		MsgHead: &commonpb.HeadMessage{
			PlayerId:    playerID,
			ServiceName: "gomokusvr",
			RequestId:   method,
			Timestamp:   0, // 可以设置时间戳
		},
		MsgType: commonpb.MessageType_CLIENT_MESSAGE, // 客户端消息
		Payload: payload,
	}
}
