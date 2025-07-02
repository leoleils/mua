package conn

import (
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"mua/gatesvr/config"
	"mua/gatesvr/internal/auth"
	"mua/gatesvr/internal/pb"
	"mua/gatesvr/internal/session"

	"github.com/gorilla/websocket"
	"google.golang.org/protobuf/proto"
)

func init() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

var (
	wsConns sync.Map // playerID -> *websocket.Conn
)

// WebSocket玩家连接结构
// 可与TCP共用PlayerConn结构

// 业务分发函数类型
type HandlerFuncWS func(playerID string, msg *pb.GameMessage)

var handlersWS = make(map[int32]HandlerFuncWS)

// 注册业务分发
func RegisterHandlerWS(msgType int32, handler HandlerFuncWS) {
	handlersWS[msgType] = handler
}

// 启动WebSocket服务
func StartWSServer(addr string) {
	http.HandleFunc("/ws", wsHandler)
	log.Printf("WebSocket服务已启动，监听: %s/ws", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("WebSocket监听失败: %v", err)
	}
}

// 发送GameMessage到玩家
func SendToPlayerWS(playerID string, gm *pb.GameMessage) bool {
	val, ok := wsConns.Load(playerID)
	if !ok {
		return false
	}
	conn := val.(*websocket.Conn)
	data, err := proto.Marshal(gm)
	if err != nil {
		return false
	}
	conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	return conn.WriteMessage(websocket.BinaryMessage, data) == nil
}

func wsHandler(w http.ResponseWriter, r *http.Request) {
	remoteAddr := r.RemoteAddr
	ip := strings.Split(remoteAddr, ":")[0]

	cfg := config.GetConfig()
	// IP白名单检查
	if cfg.EnableIPWhitelist {
		if !auth.IsIPAllowed(ip) {
			log.Printf("[认证] IP %s 不在白名单中，拒绝WebSocket连接", ip)
			http.Error(w, "IP not allowed", http.StatusForbidden)
			return
		}
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket升级失败: %v", err)
		return
	}
	defer conn.Close()
	playerID := remoteAddr
	gatesvrID := "gatesvr-1"

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
	log.Printf("玩家[%s] WebSocket上线，IP: %s", playerID, ip)

	wsConns.Store(playerID, conn)

	for {
		conn.SetReadDeadline(time.Now().Add(HeartbeatTimeout))
		select {
		case reason := <-kickCh:
			log.Printf("玩家[%s] WebSocket被踢下线: %s", playerID, reason)
			return
		default:
		}
		msgType, data, err := conn.ReadMessage()
		if err != nil {
			log.Printf("玩家[%s] WebSocket断开: %v", playerID, err)
			break
		}
		if msgType != websocket.BinaryMessage {
			log.Printf("玩家[%s] 非法消息类型: %d", playerID, msgType)
			continue
		}
		var gm pb.GameMessage
		if err := proto.Unmarshal(data, &gm); err != nil {
			log.Printf("玩家[%s] WebSocket消息反序列化失败: %v", playerID, err)
			continue
		}
		if gm.MsgType == 0 {
			session.UpdateHeartbeat(playerID)
			log.Printf("收到玩家[%s] WebSocket心跳", playerID)
			continue
		}
		// 认证消息特殊处理
		if gm.MsgType == 100 {
			// 为认证消息添加IP信息到payload
			authPayload := append([]byte(ip+":"), gm.Payload...)
			authMsg := &pb.GameMessage{
				MsgHead: gm.MsgHead,
				MsgType: gm.MsgType,
				Payload: authPayload,
				MsgTap:  gm.MsgTap,
				GameId:  gm.GameId,
			}
			if handler, ok := handlersWS[gm.MsgType]; ok {
				handler(playerID, authMsg)
			}
			continue
		}
		// 令牌校验（除了心跳和认证消息）
		if gm.MsgType != 0 && gm.MsgType != 100 && cfg.EnableTokenCheck {
			if gm.MsgHead == nil || gm.MsgHead.Token == "" {
				log.Printf("[认证] 玩家[%s] WebSocket消息缺少令牌，拒绝处理", playerID)
				continue
			}
			if !auth.ValidateToken(gm.MsgHead.Token, playerID, ip) {
				log.Printf("[认证] 玩家[%s] WebSocket令牌验证失败", playerID)
				continue
			}
		}
		if handler, ok := handlersWS[gm.MsgType]; ok {
			handler(playerID, &gm)
		} else {
			log.Printf("收到玩家[%s] WebSocket未知类型消息: %d", playerID, gm.MsgType)
		}
	}
	wsConns.Delete(playerID)
	session.PlayerOffline(playerID)
	log.Printf("玩家[%s] WebSocket连接已关闭", playerID)
}
