package route

import (
	"log"
	"sync"
)

// logRouteUpdate 记录路由更新日志
func logRouteUpdate(action, playerID, gateID string) {
	routeCount := len(playerRouteMap)
	log.Printf("[Route] %s player=%s gate=%s total_routes=%d", action, playerID, gateID, routeCount)

	// 如果路由表较小，可以打印完整路由表
	if routeCount <= 10 {
		log.Printf("[Route] Current routing table: %v", playerRouteMap)
	}
}

var (
	playerRouteMap = make(map[string]string) // playerID -> gatesvrID
	mu             sync.RWMutex
)

// Set 设置路由
func Set(playerID, gateID string) {
	mu.Lock()
	defer mu.Unlock()
	playerRouteMap[playerID] = gateID
	logRouteUpdate("Set", playerID, gateID)
}

// Get 获取路由
func Get(playerID string) (gateID string, ok bool) {
	mu.RLock()
	defer mu.RUnlock()
	gateID, ok = playerRouteMap[playerID]
	return
}

// Delete 删除路由
func Delete(playerID string) {
	mu.Lock()
	defer mu.Unlock()
	delete(playerRouteMap, playerID)
	logRouteUpdate("Delete", playerID, "")
}

// DeleteByGate 清理所有属于某个gate的玩家
func DeleteByGate(gateID string) (affected []string) {
	mu.Lock()
	defer mu.Unlock()
	for pid, gid := range playerRouteMap {
		if gid == gateID {
			delete(playerRouteMap, pid)
			affected = append(affected, pid)
		}
	}
	if len(affected) > 0 {
		log.Printf("[Route] DeleteByGate gate=%s affected_players=%d total_routes=%d", gateID, len(affected), len(playerRouteMap))
	}
	return
}

// CleanByOnlineGates 清理所有不在在线gate列表中的玩家
func CleanByOnlineGates(onlineGates map[string]struct{}) (affected []string) {
	mu.Lock()
	defer mu.Unlock()
	for pid, gid := range playerRouteMap {
		if _, ok := onlineGates[gid]; !ok {
			delete(playerRouteMap, pid)
			affected = append(affected, pid)
		}
	}
	if len(affected) > 0 {
		log.Printf("[Route] CleanByOnlineGates affected_players=%d total_routes=%d", len(affected), len(playerRouteMap))
	}
	return
}

// GetAll 获取所有路由表内容（只读副本）
func GetAll() map[string]string {
	mu.RLock()
	defer mu.RUnlock()
	copy := make(map[string]string, len(playerRouteMap))
	for k, v := range playerRouteMap {
		copy[k] = v
	}
	return copy
}
