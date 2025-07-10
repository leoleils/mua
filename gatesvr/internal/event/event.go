package event

import (
	"log"
	"mua/gatesvr/internal/kafka"
	"mua/gatesvr/internal/pb"
	"time"
)

func init() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
}

// BroadcastPlayerOnline 广播玩家上线事件
func BroadcastPlayerOnline(playerID, ip, gatesvrID, gatesvrIP string) {
	log.Printf("[事件] 广播玩家上线: playerID=%s, ip=%s, gatesvrID=%s, gatesvrIP=%s", playerID, ip, gatesvrID, gatesvrIP)
	event := &pb.PlayerStatusChanged{
		PlayerId:  playerID,
		Ip:        ip,
		GatesvrId: gatesvrID,
		GatesvrIp: gatesvrIP,
		Event:     pb.PlayerStatusEventType_ONLINE,
		EventTime: time.Now().Unix(),
	}
	kafka.BroadcastPlayerStatusChanged(event)
}

// BroadcastPlayerOffline 广播玩家下线事件
func BroadcastPlayerOffline(playerID, ip string) {
	log.Printf("[事件] 广播玩家下线: playerID=%s, ip=%s", playerID, ip)
	event := &pb.PlayerStatusChanged{
		PlayerId:  playerID,
		Ip:        ip,
		Event:     pb.PlayerStatusEventType_OFFLINE,
		EventTime: time.Now().Unix(),
	}
	kafka.BroadcastPlayerStatusChanged(event)
}
