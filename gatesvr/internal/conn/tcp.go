package conn

import (
	"log"
	"mua/gatesvr/config"
	"mua/gatesvr/internal/pb"
	"mua/gatesvr/internal/route"
	"mua/gatesvr/internal/session"
	"net"
	"strings"
	"time"

	"google.golang.org/protobuf/proto"
)

func init() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
}

const (
	HeartbeatInterval = 30 * time.Second
	HeartbeatTimeout  = 60 * time.Second
)

// 业务分发函数类型
type HandlerFunc func(playerID string, msg *pb.GameMessage)

// 新增：通用 handler 注册表实现
var handlers = make(map[int32]HandlerFuncGeneric)

type tcpHandlerRegistry struct{}

func (tcpHandlerRegistry) GetHandler(msgType int32) HandlerFuncGeneric {
	return handlers[msgType]
}

// 注册业务分发
func RegisterHandler(msgType int32, handler HandlerFuncGeneric) {
	handlers[msgType] = handler
}

// 启动TCP监听
func StartTCPServer(addr string) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("TCP监听失败: %v", err)
	}
	log.Printf("TCP服务已启动，监听: %s", addr)
	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Printf("接受连接失败: %v", err)
			continue
		}
		go func(c net.Conn) {
			adapter := NewTCPConnAdapter(c)
			HandleConnection(adapter, tcpHandlerRegistry{}, config.GetConfig().EnableTokenCheck, config.GetConfig().EnableIPWhitelist)
		}(conn)
	}
}

func handleTCPConn(conn net.Conn) {
	defer conn.Close()
	remoteAddr := conn.RemoteAddr().String()
	ip := strings.Split(remoteAddr, ":")[0]

	cfg := config.GetConfig()
	// IP白名单检查
	if cfg.EnableIPWhitelist {
		// 已移除auth依赖
	}

	// 1. 连接建立后，等待5秒内收到心跳或认证消息
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	lenBuf := make([]byte, 4)
	_, err := conn.Read(lenBuf)
	if err != nil {
		log.Printf("连接建立后未及时收到首条消息: %v", err)
		return
	}
	msgLen := int(lenBuf[0]) | int(lenBuf[1])<<8 | int(lenBuf[2])<<16 | int(lenBuf[3])<<24
	if msgLen <= 0 || msgLen > 10*1024 {
		log.Printf("首条消息长度非法: %d", msgLen)
		return
	}
	data := make([]byte, msgLen)
	_, err = conn.Read(data)
	if err != nil {
		log.Printf("首条消息读取失败: %v", err)
		return
	}
	var gm pb.GameMessage
	if err := proto.Unmarshal(data, &gm); err != nil {
		log.Printf("首条消息反序列化失败: %v", err)
		return
	}
	if gm.MsgType != 0 && gm.MsgType != 100 {
		log.Printf("首条消息类型非法: %d", gm.MsgType)
		return
	}
	if gm.MsgHead == nil || gm.MsgHead.PlayerId == "" {
		log.Printf("首条消息缺少player_id")
		return
	}
	playerID := gm.MsgHead.PlayerId
	gatesvrID := config.GetGatesvrID()

	kickCh := make(chan string, 1)
	kickFunc := func(reason string) {
		kickCh <- reason
	}
	isOtherPlace, oldSession := session.PlayerOnline(playerID, ip, gatesvrID, kickFunc)
	if isOtherPlace && oldSession != nil {
		if oldSession.KickFunc != nil {
			oldSession.KickFunc("异地登录")
		}
	}
	log.Printf("玩家[%s]上线，IP: %s", playerID, ip)

	// 处理首条消息（心跳/认证）
	if gm.MsgType == 0 {
		session.UpdateHeartbeat(playerID)
		log.Printf("收到玩家[%s]心跳", playerID)
		log.Printf("[路由] 当前路由表: %+v", route.GetAll())
	} else if gm.MsgType == 100 {
		// 已移除auth依赖
	}

	// 进入正常消息循环
	for {
		conn.SetReadDeadline(time.Now().Add(HeartbeatTimeout))
		select {
		case reason := <-kickCh:
			log.Printf("玩家[%s]被踢下线: %s", playerID, reason)
			return
		default:
		}
		lenBuf := make([]byte, 4)
		_, err := conn.Read(lenBuf)
		if err != nil {
			log.Printf("玩家[%s]连接断开: %v", playerID, err)
			break
		}
		msgLen := int(lenBuf[0]) | int(lenBuf[1])<<8 | int(lenBuf[2])<<16 | int(lenBuf[3])<<24
		if msgLen <= 0 || msgLen > 10*1024 {
			log.Printf("玩家[%s]消息长度非法: %d", playerID, msgLen)
			break
		}
		data := make([]byte, msgLen)
		_, err = conn.Read(data)
		if err != nil {
			log.Printf("玩家[%s]消息读取失败: %v", playerID, err)
			break
		}
		var gm pb.GameMessage
		if err := proto.Unmarshal(data, &gm); err != nil {
			log.Printf("玩家[%s]消息反序列化失败: %v", playerID, err)
			continue
		}
		if gm.MsgType == 0 {
			session.UpdateHeartbeat(playerID)
			log.Printf("收到玩家[%s]心跳", playerID)
			log.Printf("[路由] 当前路由表: %+v", route.GetAll())
			continue
		}
		if gm.MsgType == 100 {
			// 已移除auth依赖
		}
		if gm.MsgType != 0 && gm.MsgType != 100 && cfg.EnableTokenCheck {
			// 已移除auth依赖
		}
		if handler, ok := handlers[gm.MsgType]; ok {
			handler(playerID, &gm)
		} else {
			log.Printf("收到玩家[%s]未知类型消息: %d", playerID, gm.MsgType)
		}
	}
	session.PlayerOffline(playerID)
	log.Printf("玩家[%s]连接已关闭", playerID)
}
