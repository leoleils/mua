package conn

import (
	"fmt"
	"log"
	"mua/gatesvr/config"
	"mua/gatesvr/internal/auth"
	"mua/gatesvr/internal/forwarder"
	"mua/gatesvr/internal/nacos"
	commonpb "mua/gatesvr/internal/pb"
	"mua/gatesvr/internal/route"
	"mua/gatesvr/internal/rpc"
	"mua/gatesvr/internal/session"
	"sync"
	"time"

	"google.golang.org/protobuf/proto"
)

// 协议类型常量
const (
	ProtocolTCP = "tcp"
	ProtocolWS  = "ws"
)

// 消息类型常量
const (
	MessageTypeWebSocket = 1 // WebSocket Binary Message
	MessageTypeTCP       = 2 // TCP Custom Binary Message
)

// ConnectionContext 连接上下文
type ConnectionContext struct {
	Adapter       ConnAdapter
	Protocol      string
	PlayerID      string
	IP            string
	Config        config.ConnectionConfig
	EnableAuth    bool
	KickCh        chan string
	LastHeartbeat time.Time
}

// HandleConnection 处理连接 - 现代化版本
func HandleConnection(
	adapter ConnAdapter,
	protocol string,
	enableTokenCheck bool,
) {
	defer adapter.Close()

	// 初始化连接上下文
	ctx := &ConnectionContext{
		Adapter:    adapter,
		Protocol:   protocol,
		IP:         adapter.RemoteIP(),
		Config:     config.GetConnectionConfig(),
		EnableAuth: enableTokenCheck,
		KickCh:     make(chan string, 1),
	}

	// 确保在函数退出时清理连接
	defer func() {
		if ctx.PlayerID != "" {
			cleanupPlayerConnection(ctx.PlayerID)
		}
	}()

	// 处理首条消息（连接建立）
	if err := handleFirstMessage(ctx); err != nil {
		logConnectionError(ctx, "首条消息处理失败", err)
		return
	}

	// 设置玩家连接和会话
	if err := setupPlayerSession(ctx); err != nil {
		logConnectionError(ctx, "会话设置失败", err)
		return
	}

	// 进入消息处理循环
	handleMessageLoop(ctx)
}

// handleFirstMessage 处理首条消息（连接验证）
func handleFirstMessage(ctx *ConnectionContext) error {
	// 设置首条消息超时
	timeout := time.Duration(ctx.Config.FirstMessageTimeoutSec) * time.Second
	ctx.Adapter.SetReadDeadline(time.Now().Add(timeout))

	// 读取首条消息
	msgType, data, err := ctx.Adapter.ReadMessage()
	if err != nil {
		return fmt.Errorf("未及时收到首条消息: %v", err)
	}

	// 验证消息类型
	if !isValidMessageType(msgType) {
		return fmt.Errorf("首条消息类型非法: %d", msgType)
	}

	// 解析消息
	var gm commonpb.GameMessage
	if err := proto.Unmarshal(data, &gm); err != nil {
		return fmt.Errorf("首条消息反序列化失败: %v", err)
	}

	// 验证心跳消息
	if gm.MsgType != commonpb.MessageType_HEARTBEAT {
		return fmt.Errorf("首条消息类型非法: %v", gm.MsgType)
	}

	// 提取玩家ID
	if gm.MsgHead == nil || gm.MsgHead.PlayerId == "" {
		return fmt.Errorf("首条消息缺少player_id")
	}
	ctx.PlayerID = gm.MsgHead.PlayerId

	// 连接认证
	if ctx.EnableAuth {
		token := ""
		if gm.MsgHead != nil {
			token = gm.MsgHead.Token
		}

		authResult := auth.ValidateConnection(ctx.PlayerID, token)
		if !authResult.Success {
			// 发送认证失败响应
			sendErrorResponse(ctx, "CONNECTION_AUTH_FAILED", authResult.ErrorCode, authResult.Reason)
			return fmt.Errorf("连接认证失败: %s", authResult.Reason)
		}
		logStructured("连接认证成功", map[string]interface{}{
			"player_id": ctx.PlayerID,
			"ip":        ctx.IP,
		})
	}

	ctx.LastHeartbeat = time.Now()
	return nil
}

// setupPlayerSession 设置玩家连接和会话
func setupPlayerSession(ctx *ConnectionContext) error {
	localInstanceID := nacos.GetLocalInstanceID()

	// 处理异地登录
	if err := handleRemoteLogin(ctx.PlayerID, localInstanceID); err != nil {
		return fmt.Errorf("处理异地登录失败: %v", err)
	}

	// 设置路由
	route.Set(ctx.PlayerID, localInstanceID)

	// 设置踢人函数
	kickFunc := func(reason string) {
		select {
		case ctx.KickCh <- reason:
		default:
		}
		ctx.Adapter.Close()
	}

	// 存储连接
	connWrapper := &PlayerConnWrapper{
		Adapter: ctx.Adapter,
		Proto:   ctx.Protocol,
	}
	playerConnMap.Store(ctx.PlayerID, connWrapper)

	// 存储会话
	session.StoreSession(ctx.PlayerID, ctx.IP, localInstanceID, kickFunc)
	session.BroadcastPlayerOnline(ctx.PlayerID, ctx.IP, localInstanceID, nacos.GetLocalIP())

	logStructured("玩家上线", map[string]interface{}{
		"player_id": ctx.PlayerID,
		"ip":        ctx.IP,
		"protocol":  ctx.Protocol,
	})

	return nil
}

// handleRemoteLogin 处理异地登录
func handleRemoteLogin(playerID, localInstanceID string) error {
	routeGateSvrID, exists := route.Get(playerID)
	if !exists {
		return nil
	}

	if routeGateSvrID != localInstanceID {
		// 远程踢人
		addr := nacos.GetGatesvrAddrByInstanceID(routeGateSvrID)
		if err := rpc.KickPlayerRemote(addr, playerID, "异地远程登录"); err != nil {
			return fmt.Errorf("远程踢人失败: %v", err)
		}
		route.Delete(playerID)
	} else {
		// 本地踢人
		session.KickPlayer(playerID, "本地重连接")
	}

	return nil
}

// handleMessageLoop 消息处理循环
func handleMessageLoop(ctx *ConnectionContext) {
	readTimeout := time.Duration(ctx.Config.ReadTimeoutSec) * time.Second

	for {
		ctx.Adapter.SetReadDeadline(time.Now().Add(readTimeout))

		// 检查踢人信号
		select {
		case reason := <-ctx.KickCh:
			logStructured("玩家被踢下线", map[string]interface{}{
				"player_id": ctx.PlayerID,
				"reason":    reason,
			})
			return
		default:
		}

		// 读取消息
		msgType, data, err := ctx.Adapter.ReadMessage()
		if err != nil {
			logStructured("连接断开", map[string]interface{}{
				"player_id": ctx.PlayerID,
				"error":     err.Error(),
			})
			break
		}

		// 验证消息类型
		if !isValidMessageType(msgType) {
			logStructured("非法消息类型", map[string]interface{}{
				"player_id":    ctx.PlayerID,
				"message_type": msgType,
			})
			continue
		}

		// 解析消息
		var gm commonpb.GameMessage
		if err := proto.Unmarshal(data, &gm); err != nil {
			logStructured("消息反序列化失败", map[string]interface{}{
				"player_id": ctx.PlayerID,
				"error":     err.Error(),
			})
			continue
		}

		// 处理消息
		if err := processGameMessage(ctx, &gm); err != nil {
			logStructured("消息处理失败", map[string]interface{}{
				"player_id": ctx.PlayerID,
				"msg_type":  gm.MsgType.String(),
				"error":     err.Error(),
			})
		}
	}
}

// processGameMessage 处理游戏消息
func processGameMessage(ctx *ConnectionContext, gm *commonpb.GameMessage) error {
	// 确保消息头包含玩家ID
	ensureMessageHeader(gm, ctx.PlayerID)

	// 消息认证
	if ctx.EnableAuth && !isHeartbeatMessage(gm) {
		if err := authenticateMessage(ctx, gm); err != nil {
			return fmt.Errorf("消息认证失败: %v", err)
		}
	}

	// 根据消息类型处理
	return dispatchMessage(ctx, gm)
}

// authenticateMessage 认证消息
func authenticateMessage(ctx *ConnectionContext, gm *commonpb.GameMessage) error {
	authResult := auth.AuthenticateGameMessage(gm)
	if !authResult.Success {
		sendErrorResponse(ctx, "MSG_AUTH_FAILED", authResult.ErrorCode, authResult.Reason)
		return fmt.Errorf("认证失败: %s", authResult.Reason)
	}

	// 处理Token刷新
	if authResult.NeedRefresh {
		logStructured("Token需要刷新", map[string]interface{}{
			"player_id": ctx.PlayerID,
		})
		// 这里可以添加Token刷新逻辑
	}

	return nil
}

// dispatchMessage 分发消息
func dispatchMessage(ctx *ConnectionContext, gm *commonpb.GameMessage) error {
	switch gm.MsgType {
	case commonpb.MessageType_HEARTBEAT:
		return handleHeartbeat(ctx, gm)
	case commonpb.MessageType_SERVICE_MESSAGE:
		return handleServiceMessage(ctx, gm)
	case commonpb.MessageType_CLIENT_MESSAGE:
		return handleClientMessage(ctx, gm)
	case commonpb.MessageType_BROADCAST_MESSAGE:
		return handleBroadcastMessage(ctx, gm)
	default:
		return fmt.Errorf("未知消息类型: %v", gm.MsgType)
	}
}

// handleHeartbeat 处理心跳消息
func handleHeartbeat(ctx *ConnectionContext, gm *commonpb.GameMessage) error {
	session.UpdateHeartbeat(ctx.PlayerID)
	ctx.LastHeartbeat = time.Now()

	if ctx.Config.EnableStructuredLog {
		logStructured("收到心跳", map[string]interface{}{
			"player_id": ctx.PlayerID,
			"ip":        ctx.IP,
		})
	}

	return nil
}

// handleServiceMessage 处理服务消息
func handleServiceMessage(ctx *ConnectionContext, gm *commonpb.GameMessage) error {
	serviceName := "unknown"
	if gm.MsgHead != nil {
		serviceName = gm.MsgHead.ServiceName
	}

	logStructured("转发服务消息", map[string]interface{}{
		"player_id":    ctx.PlayerID,
		"service_name": serviceName,
	})

	resp, err := forwarder.ForwardServiceMessage(gm)
	if err != nil {
		sendErrorResponse(ctx, "SERVICE_ERROR", 5001, err.Error())
		return fmt.Errorf("服务转发失败: %v", err)
	}

	// 发送响应
	responseMsg := createServiceResponse(resp)
	if !SendToPlayer(ctx.PlayerID, responseMsg) {
		return fmt.Errorf("发送服务响应失败")
	}

	return nil
}

// handleClientMessage 处理客户端消息
func handleClientMessage(ctx *ConnectionContext, gm *commonpb.GameMessage) error {
	logStructured("收到客户端消息", map[string]interface{}{
		"player_id": ctx.PlayerID,
	})
	// 这里可以添加客户端消息的处理逻辑
	return nil
}

// handleBroadcastMessage 处理广播消息
func handleBroadcastMessage(ctx *ConnectionContext, gm *commonpb.GameMessage) error {
	logStructured("收到广播消息", map[string]interface{}{
		"player_id": ctx.PlayerID,
	})
	// 这里可以添加广播消息的处理逻辑
	return nil
}

// 工具函数

// isValidMessageType 验证消息类型
func isValidMessageType(msgType int) bool {
	return msgType == MessageTypeWebSocket || msgType == MessageTypeTCP
}

// isHeartbeatMessage 判断是否为心跳消息
func isHeartbeatMessage(gm *commonpb.GameMessage) bool {
	return gm.MsgType == commonpb.MessageType_HEARTBEAT
}

// ensureMessageHeader 确保消息头包含玩家ID
func ensureMessageHeader(gm *commonpb.GameMessage, playerID string) {
	if gm.MsgHead == nil {
		gm.MsgHead = &commonpb.HeadMessage{
			PlayerId: playerID,
		}
	} else if gm.MsgHead.PlayerId == "" {
		gm.MsgHead.PlayerId = playerID
	}
}

// sendErrorResponse 发送错误响应
func sendErrorResponse(ctx *ConnectionContext, errorType string, errorCode int, reason string) {
	errorResp := &commonpb.GameMessage{
		MsgHead: &commonpb.HeadMessage{
			PlayerId: ctx.PlayerID,
		},
		MsgType: commonpb.MessageType_CLIENT_MESSAGE,
		Payload: []byte(fmt.Sprintf("%s:%d:%s", errorType, errorCode, reason)),
	}

	if !SendToPlayer(ctx.PlayerID, errorResp) {
		logStructured("发送错误响应失败", map[string]interface{}{
			"player_id":  ctx.PlayerID,
			"error_type": errorType,
			"reason":     reason,
		})
	}
}

// createServiceResponse 创建服务响应
func createServiceResponse(resp *commonpb.GameMessageResponse) *commonpb.GameMessage {
	return &commonpb.GameMessage{
		MsgHead: resp.MsgHead,
		MsgType: commonpb.MessageType_SERVICE_MESSAGE,
		Payload: func() []byte {
			if resp.Payload != nil {
				switch payload := resp.Payload.(type) {
				case *commonpb.GameMessageResponse_Data:
					return payload.Data
				case *commonpb.GameMessageResponse_Reason:
					return []byte(payload.Reason)
				default:
					return []byte("unknown payload type")
				}
			}
			return []byte("success")
		}(),
	}
}

// logStructured 结构化日志
func logStructured(message string, fields map[string]interface{}) {
	log.Printf("[%s] %v", message, fields)
}

// logConnectionError 记录连接错误
func logConnectionError(ctx *ConnectionContext, message string, err error) {
	logStructured(message, map[string]interface{}{
		"player_id": ctx.PlayerID,
		"ip":        ctx.IP,
		"protocol":  ctx.Protocol,
		"error":     err.Error(),
	})
}

// cleanupPlayerConnection 清理玩家连接
func cleanupPlayerConnection(playerID string) {
	playerConnMap.Delete(playerID)
	session.PlayerOffline(playerID)
	logStructured("连接已清理", map[string]interface{}{
		"player_id": playerID,
	})
}

// PlayerConnWrapper 连接包装器
type PlayerConnWrapper struct {
	Adapter ConnAdapter
	Proto   string
}

var playerConnMap sync.Map // playerID -> *PlayerConnWrapper

// SendToPlayer 向玩家发送消息
func SendToPlayer(playerID string, gm *commonpb.GameMessage) bool {
	val, ok := playerConnMap.Load(playerID)
	if !ok {
		return false
	}
	wrapper := val.(*PlayerConnWrapper)
	data, err := proto.Marshal(gm)
	if err != nil {
		log.Printf("SendToPlayer 消息序列化失败: %v", err)
		return false
	}

	// 根据协议类型选择正确的消息类型
	var msgType int
	if wrapper.Proto == ProtocolWS {
		msgType = MessageTypeWebSocket
	} else {
		msgType = MessageTypeTCP
	}

	err = wrapper.Adapter.WriteMessage(msgType, data)
	if err != nil {
		log.Printf("SendToPlayer 发送失败[%s]: %v", playerID, err)
		return false
	}
	return true
}
