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

func BroadcastPlayerOnline(playerID, ip string) {
	log.Printf("[事件] 广播玩家上线: playerID=%s, ip=%s", playerID, ip)
	event := &pb.PlayerStatusChanged{
		PlayerId:  playerID,
		Ip:        ip,
		Event:     pb.PlayerStatusEventType_ONLINE,
		EventTime: time.Now().Unix(),
	}
	kafka.BroadcastPlayerStatusChanged(event)
}

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
