package conn

import (
	"log"
	"net"
	"strings"
	"sync"
	"time"
	"mua/gatesvr/internal/session"
	"mua/gatesvr/internal/pb"
	"google.golang.org/protobuf/proto"
)

const (
	HeartbeatInterval = 30 * time.Second
	HeartbeatTimeout  = 60 * time.Second
)

type PlayerConn struct {
	Conn     net.Conn
	PlayerID string
	LastHeartbeat time.Time
	IP       string
}

var (
	playerConns sync.Map // playerID -> *PlayerConn
)

// 业务分发函数类型
type HandlerFunc func(playerID string, msg *pb.GameMessage)

var handlers = make(map[int32]HandlerFunc)

// 注册业务分发
func RegisterHandler(msgType int32, handler HandlerFunc) {
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
		go handleTCPConn(conn)
	}
}

// 向指定玩家推送消息
func SendToPlayer(playerID string, gm *pb.GameMessage) bool {
	val, ok := playerConns.Load(playerID)
	if !ok {
		return false
	}
	pc := val.(*PlayerConn)
	data, err := proto.Marshal(gm)
	if err != nil {
		return false
	}
	// 发送长度前缀+数据
	lenBuf := []byte{byte(len(data)), byte(len(data) >> 8), byte(len(data) >> 16), byte(len(data) >> 24)}
	_, err = pc.Conn.Write(append(lenBuf, data...))
	return err == nil
}

func handleTCPConn(conn net.Conn) {
	defer conn.Close()
	remoteAddr := conn.RemoteAddr().String()
	ip := strings.Split(remoteAddr, ":")[0]
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
	log.Printf("玩家[%s]上线，IP: %s", playerID, ip)

	pc := &PlayerConn{
		Conn:     conn,
		PlayerID: playerID,
		LastHeartbeat: time.Now(),
		IP:       ip,
	}
	playerConns.Store(playerID, pc)

	for {
		conn.SetReadDeadline(time.Now().Add(HeartbeatTimeout))
		select {
		case reason := <-kickCh:
			log.Printf("玩家[%s]被踢下线: %s", playerID, reason)
			return
		default:
		}
		// 读取长度前缀+protobuf二进制
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
			// 心跳
			session.UpdateHeartbeat(playerID)
			log.Printf("收到玩家[%s]心跳", playerID)
			continue
		}
		if handler, ok := handlers[gm.MsgType]; ok {
			handler(playerID, &gm)
		} else {
			log.Printf("收到玩家[%s]未知类型消息: %d", playerID, gm.MsgType)
		}
	}
	playerConns.Delete(playerID)
	session.PlayerOffline(playerID)
	log.Printf("玩家[%s]连接已关闭", playerID)
} 