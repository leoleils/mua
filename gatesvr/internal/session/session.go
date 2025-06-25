package session

import (
	"log"
	"sync"
	"time"
	"mua/gatesvr/internal/kafka"
	"mua/gatesvr/internal/pb"
)

type Session struct {
	PlayerID      string
	IP            string
	LastHeartbeat time.Time
	GateSvrID     string // 当前接入的gatesvr实例ID
	KickFunc      func(reason string) // 踢下线回调
}

var (
	sessions sync.Map // playerID -> *Session
	ip2pid   sync.Map // ip -> playerID
)

// 玩家上线，返回是否为异地登录
func PlayerOnline(playerID, ip, gatesvrID string, kickFunc func(string)) (isOtherPlace bool, oldSession *Session) {
	val, loaded := sessions.LoadOrStore(playerID, &Session{
		PlayerID:      playerID,
		IP:            ip,
		LastHeartbeat: time.Now(),
		GateSvrID:     gatesvrID,
		KickFunc:      kickFunc,
	})
	ip2pid.Store(ip, playerID)
	if loaded {
		sess := val.(*Session)
		if sess.IP != ip || sess.GateSvrID != gatesvrID {
			// 异地登录
			return true, sess
		}
	}
	// 广播上线
	event := &pb.PlayerStatusChanged{
		PlayerId:  playerID,
		Ip:        ip,
		Event:     pb.PlayerStatusEventType_ONLINE,
		EventTime: time.Now().Unix(),
	}
	kafka.BroadcastPlayerStatusChanged(event)
	return false, nil
}

// 玩家下线
func PlayerOffline(playerID string) {
	if val, ok := sessions.Load(playerID); ok {
		sess := val.(*Session)
		event := &pb.PlayerStatusChanged{
			PlayerId:  playerID,
			Ip:        sess.IP,
			Event:     pb.PlayerStatusEventType_OFFLINE,
			EventTime: time.Now().Unix(),
		}
		kafka.BroadcastPlayerStatusChanged(event)
		ip2pid.Delete(sess.IP)
	}
	sessions.Delete(playerID)
	log.Printf("玩家[%s]下线", playerID)
}

// 心跳更新
func UpdateHeartbeat(playerID string) {
	if val, ok := sessions.Load(playerID); ok {
		sess := val.(*Session)
		sess.LastHeartbeat = time.Now()
	}
}

// 踢下线
func KickPlayer(playerID, reason string) {
	if val, ok := sessions.Load(playerID); ok {
		sess := val.(*Session)
		if sess.KickFunc != nil {
			sess.KickFunc(reason)
		}
		sessions.Delete(playerID)
		log.Printf("玩家[%s]被踢下线: %s", playerID, reason)
	}
}

// 获取Session
func GetSession(playerID string) (*Session, bool) {
	val, ok := sessions.Load(playerID)
	if !ok {
		return nil, false
	}
	return val.(*Session), true
}

// 通过ip查找playerID
func GetPlayerIDByIP(ip string) (string, bool) {
	val, ok := ip2pid.Load(ip)
	if !ok {
		return "", false
	}
	return val.(string), true
} 