package conn

import (
	"fmt"
	"log"
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

type HandlerFuncGeneric func(playerID string, msg *commonpb.GameMessage)

type HandlerRegistry interface {
	GetHandler(msgType int32) HandlerFuncGeneric
}

// HandleConnection 处理连接，支持指定协议类型
func HandleConnection(
	adapter ConnAdapter,
	registry HandlerRegistry,
	protocol string,
	enableTokenCheck bool,
	enableIPWhitelist bool,
) {
	defer adapter.Close()
	ip := adapter.RemoteIP()
	var playerID string // 声明 playerID 变量，在整个函数中使用

	// 确保在函数退出时清理连接
	defer func() {
		if playerID != "" {
			playerConnMap.Delete(playerID)
			session.PlayerOffline(playerID)
			log.Printf("玩家[%s]连接已关闭并清理", playerID)
		}
	}()

	// 1. 连接建立后，等待5秒内收到心跳
	adapter.SetReadDeadline(time.Now().Add(5 * time.Second))
	msgType, data, err := adapter.ReadMessage()
	if err != nil {
		log.Printf("连接建立后未及时收到首条消息: %v", err)
		return
	}
	if msgType != 2 && msgType != 1 { // 2: TCP自定义，1: websocket.BinaryMessage
		log.Printf("首条消息连接类型非法: %d", msgType)
		return
	}
	var gm commonpb.GameMessage
	if err := proto.Unmarshal(data, &gm); err != nil {
		log.Printf("首条消息反序列化失败: %v", err)
		return
	}
	if gm.MsgType != commonpb.MessageType_HEARTBEAT {
		log.Printf("首条消息类型非法: %v", gm.MsgType)
		return
	}
	if gm.MsgHead == nil || gm.MsgHead.PlayerId == "" {
		log.Printf("首条消息缺少player_id")
		return
	}
	playerID = gm.MsgHead.PlayerId // 设置 playerID

	// 2. 连接认证检查（如果启用）
	if enableTokenCheck {
		token := ""
		if gm.MsgHead != nil {
			token = gm.MsgHead.Token
		}

		authResult := auth.ValidateConnection(playerID, token)
		if !authResult.Success {
			log.Printf("连接认证失败 - 玩家: %s, 原因: %s", playerID, authResult.Reason)

			// 发送认证失败响应
			errorResp := &commonpb.GameMessage{
				MsgHead: &commonpb.HeadMessage{
					PlayerId: playerID,
				},
				MsgType: commonpb.MessageType_CLIENT_MESSAGE,
				Payload: []byte(fmt.Sprintf("CONNECTION_AUTH_FAILED:%d:%s", authResult.ErrorCode, authResult.Reason)),
			}

			respData, _ := proto.Marshal(errorResp)
			var respMsgType int
			if protocol == "ws" {
				respMsgType = 1
			} else {
				respMsgType = 2
			}
			adapter.WriteMessage(respMsgType, respData)
			return
		}
		log.Printf("连接认证成功 - 玩家: %s", playerID)
	}

	localInstancID := nacos.GetLocalInstanceID()

	//如果路由存在，则判断是否异地登录
	routeGateSvrID, ok := route.Get(playerID)
	if ok && routeGateSvrID != localInstancID {
		// 是其他实例已经登录了 发起远程rpc踢人
		err = rpc.KickPlayerRemote(nacos.GetGatesvrAddrByInstanceID(routeGateSvrID), playerID, "异地远程登录")
		if err != nil {
			log.Printf("远程踢人失败: %v", err)
			return
		}
		// 删除当前路由
		route.Delete(playerID)
	}
	if ok && routeGateSvrID == localInstancID {
		session.KickPlayer(playerID, "本地重连接")
	}

	// 设置路由 路由是全网的
	route.Set(playerID, localInstancID)

	// 踢人通道
	kickCh := make(chan string, 1)
	kickFunc := func(reason string) {
		kickCh <- reason
		adapter.Close() // 立即关闭底层连接，实现踢人立即生效
	}

	// 存储连接到 playerConnMap（关键修复）
	connWrapper := &PlayerConnWrapper{
		Adapter: adapter,
		Proto:   protocol, // 使用传入的协议类型
	}
	playerConnMap.Store(playerID, connWrapper)

	// 存储session 本地session
	session.StoreSession(playerID, ip, localInstancID, kickFunc)
	// 广播上线事件
	session.BroadcastPlayerOnline(playerID, ip, localInstancID, nacos.GetLocalIP())
	log.Printf("玩家[%s]上线，客户端IP: %s, 协议: %s", playerID, ip, protocol)

	// 处理首条消息（心跳）
	if gm.MsgType == commonpb.MessageType_HEARTBEAT {
		session.UpdateHeartbeat(playerID)
		log.Printf("收到玩家[%s]心跳，IP: %s", playerID, ip)
	}

	// 进入正常消息循环
	for {
		adapter.SetReadDeadline(time.Now().Add(60 * time.Second))
		select {
		case reason := <-kickCh:
			log.Printf("玩家[%s]被踢下线: %s", playerID, reason)
			return
		default:
		}
		msgType, data, err := adapter.ReadMessage()
		if err != nil {
			log.Printf("玩家[%s]连接断开: %v", playerID, err)
			break
		}
		if msgType != 2 && msgType != 1 {
			log.Printf("玩家[%s]非法消息类型: %d", playerID, msgType)
			continue
		}
		var gm commonpb.GameMessage
		if err := proto.Unmarshal(data, &gm); err != nil {
			log.Printf("玩家[%s]消息反序列化失败: %v", playerID, err)
			continue
		}

		// 确保消息头包含玩家ID
		if gm.MsgHead == nil {
			gm.MsgHead = &commonpb.HeadMessage{
				PlayerId: playerID,
			}
		} else if gm.MsgHead.PlayerId == "" {
			gm.MsgHead.PlayerId = playerID
		}

		// 消息认证检查（如果启用）
		if enableTokenCheck {
			authResult := auth.AuthenticateGameMessage(&gm)
			if !authResult.Success {
				log.Printf("消息认证失败 - 玩家: %s, 消息类型: %v, 原因: %s",
					playerID, gm.MsgType, authResult.Reason)

				// 发送认证失败响应
				errorResp := &commonpb.GameMessage{
					MsgHead: &commonpb.HeadMessage{
						PlayerId: playerID,
					},
					MsgType: commonpb.MessageType_CLIENT_MESSAGE,
					Payload: []byte(fmt.Sprintf("MSG_AUTH_FAILED:%d:%s", authResult.ErrorCode, authResult.Reason)),
				}

				if !SendToPlayer(playerID, errorResp) {
					log.Printf("向玩家[%s]发送认证失败响应失败", playerID)
				}
				continue
			}

			// 处理Token刷新
			if authResult.NeedRefresh {
				log.Printf("玩家[%s]需要刷新Token", playerID)
				// 这里可以触发Token刷新逻辑
			}
		}

		// 根据消息类型处理
		switch gm.MsgType {
		case commonpb.MessageType_HEARTBEAT:
			// 处理心跳消息
			session.UpdateHeartbeat(playerID)
			log.Printf("收到玩家[%s]心跳，IP: %s", playerID, ip)
			continue

		case commonpb.MessageType_SERVICE_MESSAGE:
			// 处理服务消息 - 转发到后端服务
			log.Printf("收到玩家[%s]服务消息，转发到后端服务: %s", playerID, gm.MsgHead.ServiceName)
			resp, err := forwarder.ForwardServiceMessage(&gm)
			if err != nil {
				log.Printf("玩家[%s]服务消息转发失败: %v", playerID, err)
				// 发送错误响应给客户端
				errorResp := &commonpb.GameMessage{
					MsgHead: gm.MsgHead,
					MsgType: commonpb.MessageType_SERVICE_MESSAGE,
					Payload: []byte(err.Error()),
				}
				if !SendToPlayer(playerID, errorResp) {
					log.Printf("向玩家[%s]发送错误响应失败", playerID)
				}
			} else {
				// 将后端服务的响应转换为 GameMessage 并发送给客户端
				responseMsg := &commonpb.GameMessage{
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
				if !SendToPlayer(playerID, responseMsg) {
					log.Printf("向玩家[%s]发送服务响应失败", playerID)
				}
			}
			continue

		case commonpb.MessageType_CLIENT_MESSAGE:
			// 处理客户端消息 - 网关内部处理
			log.Printf("收到玩家[%s]客户端消息", playerID)
			// 这里可以添加客户端消息的处理逻辑
			// 例如：连接状态查询、用户信息获取等

		case commonpb.MessageType_BROADCAST_MESSAGE:
			// 处理广播消息
			log.Printf("收到玩家[%s]广播消息", playerID)
			// 这里可以添加广播消息的处理逻辑

		default:
			// 其他消息类型，使用原有的 handler 机制
			if handler := registry.GetHandler(int32(gm.MsgType)); handler != nil {
				handler(playerID, &gm)
			} else {
				log.Printf("收到玩家[%s]未知类型消息: %v", playerID, gm.MsgType)
			}
		}
	}
}

// 统一的连接包装
// Adapter: 连接适配器，Proto: "tcp" 或 "ws"
type PlayerConnWrapper struct {
	Adapter ConnAdapter
	Proto   string
}

var playerConnMap sync.Map // playerID -> *PlayerConnWrapper

// 统一推送接口
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
	if wrapper.Proto == "ws" {
		msgType = 1 // WebSocket BinaryMessage
	} else {
		msgType = 2 // TCP 自定义二进制消息
	}

	err = wrapper.Adapter.WriteMessage(msgType, data)
	if err != nil {
		log.Printf("SendToPlayer 发送失败[%s]: %v", playerID, err)
		return false
	}
	return true
}
