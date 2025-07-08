package session

import (
	"log"
	"mua/gatesvr/internal/event"
	"sync"
	"time"
)

type Session struct {
	PlayerID      string              // 玩家ID
	IP            string              // ip地址（含端口）
	LastHeartbeat time.Time           // 最后心跳时间
	GateSvrID     string              // 当前接入的gatesvr实例ID
	KickFunc      func(reason string) // 踢下线回调
}

var (
	sessions sync.Map // playerID -> *Session
	ip2pid   sync.Map // ip -> playerID
)

// 判断是否异地登录，返回是否异地登录和旧session
func IsOtherPlaceLogin(playerID, ip, gatesvrID string) (bool, *Session) {
	val, loaded := sessions.Load(playerID)
	// 如果session存在，则判断是否异地登录
	if loaded {
		sess := val.(*Session)
		if sess.IP != ip || sess.GateSvrID != gatesvrID {
			return true, sess
		}
	}
	return false, nil
}

// 踢人操作（本地或远程）
func KickSession(sess *Session, reason string) {
	if sess == nil {
		return
	}
	if sess.KickFunc != nil {
		sess.KickFunc(reason)
	}
	ip := sess.IP
	ip2pid.Delete(ip)
	sessions.Delete(sess.PlayerID)
}

// 存储/更新 session
func StoreSession(playerID, ip, gatesvrID string, kickFunc func(string)) {
	sessions.Store(playerID, &Session{
		PlayerID:      playerID,
		IP:            ip,
		LastHeartbeat: time.Now(),
		GateSvrID:     gatesvrID,
		KickFunc:      kickFunc,
	})
	ip2pid.Store(ip, playerID)
}

// 广播上线事件
func BroadcastPlayerOnline(playerID, ip, gatesvrID, gatesvrIP string) {
	event.BroadcastPlayerOnline(playerID, ip, gatesvrID, gatesvrIP)
}

// 玩家上线，返回是否为异地登录
// Deprecated: 请使用 IsOtherPlaceLogin + KickSession + StoreSession + BroadcastPlayerOnline
func PlayerOnline(playerID, ip, gatesvrID, gatesvrIP string, kickFunc func(string)) (isOtherPlace bool, oldSession *Session) {
	isOtherPlace, oldSession = IsOtherPlaceLogin(playerID, ip, gatesvrID)
	if isOtherPlace {
		KickSession(oldSession, "异地登录")
	}
	StoreSession(playerID, ip, gatesvrID, kickFunc)
	BroadcastPlayerOnline(playerID, ip, gatesvrID, gatesvrIP)
	return
}

// 玩家下线
func PlayerOffline(playerID string) {
	if val, ok := sessions.Load(playerID); ok {
		sess := val.(*Session)
		// 广播下线
		event.BroadcastPlayerOffline(playerID, sess.IP)
		ip2pid.Delete(sess.IP)
		log.Printf("玩家[%s]下线, ip=%s, gatesvrID=%s", playerID, sess.IP, sess.GateSvrID)
	}
	sessions.Delete(playerID)
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
			ip := sess.IP
			ip2pid.Delete(ip)
		}

		sessions.Delete(playerID)
		log.Printf("玩家[%s]会话下线: %s, 客户端ip=%s, gatesvrID=%s", playerID, reason, sess.IP, sess.GateSvrID)
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

// 供kafka消费玩家上下线历史数据时调用
func PlayerOnlineFromKafka(playerID, gatesvrID, ip string) {

}

func PlayerOfflineFromKafka(playerID string) {
}
