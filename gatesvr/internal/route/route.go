package route

import "sync"

var (
	playerRouteMap = make(map[string]string) // playerID -> gatesvrID
	mu             sync.RWMutex
)

// 设置路由
func Set(playerID, gateID string) {
	mu.Lock()
	defer mu.Unlock()
	playerRouteMap[playerID] = gateID
}

// 获取路由
func Get(playerID string) (gateID string, ok bool) {
	mu.RLock()
	defer mu.RUnlock()
	gateID, ok = playerRouteMap[playerID]
	return
}

// 删除路由
func Delete(playerID string) {
	mu.Lock()
	defer mu.Unlock()
	delete(playerRouteMap, playerID)
}

// 清理所有属于某个gate的玩家
func DeleteByGate(gateID string) (affected []string) {
	mu.Lock()
	defer mu.Unlock()
	for pid, gid := range playerRouteMap {
		if gid == gateID {
			delete(playerRouteMap, pid)
			affected = append(affected, pid)
		}
	}
	return
}

// 清理所有不在在线gate列表中的玩家
func CleanByOnlineGates(onlineGates map[string]struct{}) (affected []string) {
	mu.Lock()
	defer mu.Unlock()
	for pid, gid := range playerRouteMap {
		if _, ok := onlineGates[gid]; !ok {
			delete(playerRouteMap, pid)
			affected = append(affected, pid)
		}
	}
	return
}

// 获取所有路由表内容（只读副本）
func GetAll() map[string]string {
	mu.RLock()
	defer mu.RUnlock()
	copy := make(map[string]string, len(playerRouteMap))
	for k, v := range playerRouteMap {
		copy[k] = v
	}
	return copy
}
