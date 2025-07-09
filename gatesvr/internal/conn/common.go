package conn

import (
	"log"
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
		if gm.MsgType == commonpb.MessageType_HEARTBEAT {
			session.UpdateHeartbeat(playerID)
			log.Printf("收到玩家[%s]心跳，IP: %s", playerID, ip)
			continue
		}

		if handler := registry.GetHandler(int32(gm.MsgType)); handler != nil {
			handler(playerID, &gm)
		} else {
			log.Printf("收到玩家[%s]未知类型消息: %v", playerID, gm.MsgType)
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
