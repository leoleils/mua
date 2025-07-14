package handler

import (
	"context"
	"log"
	"strings"
	"time"

	gatesvrpb "mua/gatesvr/pb"
	"mua/minigame/gomokusvr/internal/pb"
)

// GatesvrAdapter 实现gatesvr的common.CommonService接口
type GatesvrAdapter struct {
	gatesvrpb.UnimplementedCommonServiceServer
	gatewayHandler *GatewayHandler
}

// NewGatesvrAdapter 创建gatesvr适配器
func NewGatesvrAdapter(gatewayHandler *GatewayHandler) *GatesvrAdapter {
	return &GatesvrAdapter{
		gatewayHandler: gatewayHandler,
	}
}

// SendMessage 实现gatesvr的common.CommonService.SendMessage
func (a *GatesvrAdapter) SendMessage(ctx context.Context, req *gatesvrpb.GameMessage) (*gatesvrpb.GameMessageResponse, error) {
	log.Printf("收到gatesvr消息 - 玩家: %s, 方法: %s", req.MsgHead.PlayerId, req.MsgHead.RequestId)

	// 转换方法名（去掉Response后缀）
	method := convertMethodName(req.MsgHead.RequestId)
	log.Printf("转换方法名: %s -> %s", req.MsgHead.RequestId, method)

	// 转换gatesvr格式到gomoku格式
	gomokuReq := &pb.GameMessage{
		Head: &pb.MessageHead{
			MsgId:       req.MsgHead.RequestId,
			PlayerId:    req.MsgHead.PlayerId,
			ServiceName: req.MsgHead.ServiceName,
			Method:      method, // 使用转换后的Method
		},
		MsgType:   convertMessageType(req.MsgType),
		Payload:   req.Payload,
		Timestamp: req.MsgHead.Timestamp,
	}

	// 调用gomoku的gateway handler
	gomokuResp, err := a.gatewayHandler.SendMessage(ctx, gomokuReq)
	if err != nil {
		log.Printf("调用gomoku handler失败: %v", err)
		return &gatesvrpb.GameMessageResponse{
			MsgHead:           req.MsgHead,
			Ret:               500,
			Payload:           &gatesvrpb.GameMessageResponse_Reason{Reason: err.Error()},
			ResponseTimestamp: time.Now().UnixMilli(),
		}, nil
	}

	// 转换gomoku响应回gatesvr格式
	gatesvrResp := &gatesvrpb.GameMessageResponse{
		MsgHead:           req.MsgHead,
		Ret:               gomokuResp.Code,
		ResponseTimestamp: gomokuResp.Timestamp,
	}

	if gomokuResp.Code == 0 {
		gatesvrResp.Payload = &gatesvrpb.GameMessageResponse_Data{Data: gomokuResp.Payload}
	} else {
		gatesvrResp.Payload = &gatesvrpb.GameMessageResponse_Reason{Reason: gomokuResp.Message}
	}

	log.Printf("返回gatesvr响应 - 玩家: %s, 状态: %d, 数据长度: %d", req.MsgHead.PlayerId, gatesvrResp.Ret, len(gomokuResp.Payload))
	return gatesvrResp, nil
}

// convertMethodName 转换方法名，去掉Response后缀
func convertMethodName(methodName string) string {
	if strings.HasSuffix(methodName, "Response") {
		return strings.TrimSuffix(methodName, "Response")
	}
	return methodName
}

// convertMessageType 转换消息类型
func convertMessageType(gatesvrType gatesvrpb.MessageType) pb.MessageType {
	switch gatesvrType {
	case gatesvrpb.MessageType_HEARTBEAT:
		return pb.MessageType_UNKNOWN
	case gatesvrpb.MessageType_SERVICE_MESSAGE:
		return pb.MessageType_REQUEST
	case gatesvrpb.MessageType_CLIENT_MESSAGE:
		return pb.MessageType_REQUEST
	case gatesvrpb.MessageType_BROADCAST_MESSAGE:
		return pb.MessageType_NOTIFY
	default:
		return pb.MessageType_REQUEST
	}
}
