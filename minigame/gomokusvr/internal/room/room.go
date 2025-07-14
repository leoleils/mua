package room

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"sync"
	"time"

	"mua/minigame/gomokusvr/internal/config"
	"mua/minigame/gomokusvr/internal/game"
	"mua/minigame/gomokusvr/internal/pb"
)

// Room 游戏房间
type Room struct {
	ID          string                    `json:"id"`
	Name        string                    `json:"name"`
	Password    string                    `json:"password"`
	Status      pb.RoomStatus             `json:"status"`
	OwnerID     string                    `json:"owner_id"`
	Players     map[string]*pb.PlayerInfo `json:"players"`
	Game        *game.Game                `json:"game"`
	CreatedTime int64                     `json:"created_time"`
	mu          sync.RWMutex              `json:"-"`
}

// RoomManager 房间管理器
type RoomManager struct {
	rooms      map[string]*Room
	mu         sync.RWMutex
	maxRooms   int
	maxPlayers int
}

var globalRoomManager *RoomManager

// InitRoomManager 初始化房间管理器
func InitRoomManager() {
	gameConfig := config.GetGameConfig()
	globalRoomManager = &RoomManager{
		rooms:      make(map[string]*Room),
		maxRooms:   gameConfig.Room.MaxRooms,
		maxPlayers: gameConfig.Room.MaxPlayersPerRoom,
	}

	// 启动房间清理协程
	go globalRoomManager.startCleanup()
	log.Println("房间管理器初始化完成")
}

// GetRoomManager 获取房间管理器
func GetRoomManager() *RoomManager {
	if globalRoomManager == nil {
		InitRoomManager()
	}
	return globalRoomManager
}

// generateRoomID 生成房间ID
func generateRoomID() string {
	bytes := make([]byte, 4)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// CreateRoom 创建房间
func (rm *RoomManager) CreateRoom(ownerID, roomName, password string) (*Room, error) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	// 检查房间数量限制
	if len(rm.rooms) >= rm.maxRooms {
		return nil, fmt.Errorf("房间数量已达上限: %d", rm.maxRooms)
	}

	// 检查房间名长度 (设置默认最大长度为32)
	const maxRoomNameLength = 32
	if len(roomName) > maxRoomNameLength {
		return nil, fmt.Errorf("房间名长度不能超过 %d 个字符", maxRoomNameLength)
	}

	// 生成房间ID
	roomID := generateRoomID()
	for rm.rooms[roomID] != nil {
		roomID = generateRoomID()
	}

	// 创建房间
	room := &Room{
		ID:          roomID,
		Name:        roomName,
		Password:    password,
		Status:      pb.RoomStatus_WAITING,
		OwnerID:     ownerID,
		Players:     make(map[string]*pb.PlayerInfo),
		Game:        game.NewGame(),
		CreatedTime: time.Now().UnixMilli(),
	}

	// 房主加入房间
	ownerInfo := &pb.PlayerInfo{
		PlayerId: ownerID,
		Username: ownerID,              // 这里简化处理，实际可以从用户系统获取
		Color:    pb.PlayerColor_BLACK, // 房主默认黑子
		IsReady:  true,
		JoinTime: time.Now().UnixMilli(),
	}
	room.Players[ownerID] = ownerInfo

	rm.rooms[roomID] = room

	log.Printf("房间创建成功 - ID: %s, 名称: %s, 房主: %s", roomID, roomName, ownerID)
	return room, nil
}

// JoinRoom 加入房间
func (rm *RoomManager) JoinRoom(playerID, roomID, password string) (*Room, error) {
	rm.mu.RLock()
	room := rm.rooms[roomID]
	rm.mu.RUnlock()

	if room == nil {
		return nil, fmt.Errorf("房间不存在: %s", roomID)
	}

	room.mu.Lock()
	defer room.mu.Unlock()

	// 检查密码
	if room.Password != "" && room.Password != password {
		return nil, fmt.Errorf("房间密码错误")
	}

	// 检查玩家是否已在房间中
	if _, exists := room.Players[playerID]; exists {
		return room, nil // 玩家已在房间中，直接返回
	}

	// 检查房间状态
	if room.Status == pb.RoomStatus_PLAYING {
		return nil, fmt.Errorf("游戏进行中，无法加入")
	}

	// 检查房间人数限制
	if len(room.Players) >= rm.maxPlayers {
		return nil, fmt.Errorf("房间已满")
	}

	// 分配颜色
	color := pb.PlayerColor_WHITE // 第二个玩家默认白子
	if len(room.Players) == 0 {
		color = pb.PlayerColor_BLACK
	}

	// 加入房间
	playerInfo := &pb.PlayerInfo{
		PlayerId: playerID,
		Username: playerID, // 这里简化处理
		Color:    color,
		IsReady:  false,
		JoinTime: time.Now().UnixMilli(),
	}
	room.Players[playerID] = playerInfo

	log.Printf("玩家加入房间 - 玩家: %s, 房间: %s", playerID, roomID)
	return room, nil
}

// LeaveRoom 离开房间
func (rm *RoomManager) LeaveRoom(playerID, roomID string) error {
	rm.mu.RLock()
	room := rm.rooms[roomID]
	rm.mu.RUnlock()

	if room == nil {
		return fmt.Errorf("房间不存在: %s", roomID)
	}

	room.mu.Lock()
	defer room.mu.Unlock()

	// 检查玩家是否在房间中
	if _, exists := room.Players[playerID]; !exists {
		return fmt.Errorf("玩家不在房间中")
	}

	// 移除玩家
	delete(room.Players, playerID)

	// 如果是房主离开，转移房主权限或销毁房间
	if room.OwnerID == playerID {
		if len(room.Players) > 0 {
			// 转移房主权限给第一个玩家
			for newOwnerID := range room.Players {
				room.OwnerID = newOwnerID
				log.Printf("房主权限转移 - 新房主: %s, 房间: %s", newOwnerID, roomID)
				break
			}
		} else {
			// 房间为空，销毁房间
			rm.mu.Lock()
			delete(rm.rooms, roomID)
			rm.mu.Unlock()
			log.Printf("房间销毁 - 房间: %s", roomID)
		}
	}

	log.Printf("玩家离开房间 - 玩家: %s, 房间: %s", playerID, roomID)
	return nil
}

// DestroyRoom 销毁房间（仅房主可以）
func (rm *RoomManager) DestroyRoom(playerID, roomID string) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	room := rm.rooms[roomID]
	if room == nil {
		return fmt.Errorf("房间不存在: %s", roomID)
	}

	room.mu.Lock()
	defer room.mu.Unlock()

	// 检查权限
	if room.OwnerID != playerID {
		return fmt.Errorf("只有房主可以销毁房间")
	}

	// 销毁房间
	delete(rm.rooms, roomID)
	log.Printf("房间被房主销毁 - 房主: %s, 房间: %s", playerID, roomID)
	return nil
}

// GetRoom 获取房间信息
func (rm *RoomManager) GetRoom(roomID string) *Room {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	return rm.rooms[roomID]
}

// GetRoomList 获取房间列表
func (rm *RoomManager) GetRoomList(page, pageSize int32) ([]*Room, int32) {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	// 将房间转换为切片
	allRooms := make([]*Room, 0, len(rm.rooms))
	for _, room := range rm.rooms {
		allRooms = append(allRooms, room)
	}

	totalCount := int32(len(allRooms))

	// 分页处理
	start := (page - 1) * pageSize
	end := start + pageSize

	if start >= totalCount {
		return []*Room{}, totalCount
	}

	if end > totalCount {
		end = totalCount
	}

	return allRooms[start:end], totalCount
}

// StartGame 开始游戏
func (rm *RoomManager) StartGame(playerID, roomID string) error {
	room := rm.GetRoom(roomID)
	if room == nil {
		return fmt.Errorf("房间不存在: %s", roomID)
	}

	room.mu.Lock()
	defer room.mu.Unlock()

	// 检查权限
	if room.OwnerID != playerID {
		return fmt.Errorf("只有房主可以开始游戏")
	}

	// 检查房间状态
	if room.Status != pb.RoomStatus_WAITING {
		return fmt.Errorf("房间状态不允许开始游戏")
	}

	// 检查玩家数量
	if len(room.Players) != 2 {
		return fmt.Errorf("需要2名玩家才能开始游戏")
	}

	// 检查所有玩家是否准备就绪
	for _, player := range room.Players {
		if !player.IsReady {
			return fmt.Errorf("所有玩家必须准备就绪")
		}
	}

	// 开始游戏
	room.Status = pb.RoomStatus_PLAYING
	room.Game = game.NewGame()

	log.Printf("游戏开始 - 房间: %s", roomID)
	return nil
}

// PlacePiece 下棋
func (rm *RoomManager) PlacePiece(playerID, roomID string, x, y int32) (*game.Game, error) {
	room := rm.GetRoom(roomID)
	if room == nil {
		return nil, fmt.Errorf("房间不存在: %s", roomID)
	}

	room.mu.Lock()
	defer room.mu.Unlock()

	// 检查房间状态
	if room.Status != pb.RoomStatus_PLAYING {
		return nil, fmt.Errorf("游戏未开始")
	}

	// 检查玩家是否在房间中
	playerInfo, exists := room.Players[playerID]
	if !exists {
		return nil, fmt.Errorf("玩家不在房间中")
	}

	// 检查是否轮到该玩家
	if playerInfo.Color != room.Game.CurrentTurn {
		return nil, fmt.Errorf("不是您的回合")
	}

	// 下棋
	if err := room.Game.PlacePiece(playerID, x, y); err != nil {
		return nil, err
	}

	// 检查游戏是否结束
	if room.Game.IsGameOver() {
		room.Status = pb.RoomStatus_FINISHED
	}

	log.Printf("玩家下棋 - 玩家: %s, 房间: %s, 位置: (%d, %d)", playerID, roomID, x, y)
	return room.Game, nil
}

// startCleanup 启动房间清理协程
func (rm *RoomManager) startCleanup() {
	roomConfig := config.GetRoomConfig()
	ticker := time.NewTicker(time.Duration(roomConfig.AutoCleanupIntervalMin) * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		rm.cleanupRooms()
	}
}

// cleanupRooms 清理过期房间
func (rm *RoomManager) cleanupRooms() {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	gameConfig := config.GetGameConfig()
	timeout := time.Duration(gameConfig.Room.RoomTimeoutMin) * time.Minute
	now := time.Now().UnixMilli()

	for roomID, room := range rm.rooms {
		room.mu.RLock()
		roomAge := time.Duration(now-room.CreatedTime) * time.Millisecond
		isEmpty := len(room.Players) == 0
		isExpired := roomAge > timeout
		room.mu.RUnlock()

		if isEmpty || isExpired {
			delete(rm.rooms, roomID)
			log.Printf("清理房间 - 房间: %s, 原因: %s", roomID,
				func() string {
					if isEmpty {
						return "房间为空"
					}
					return "房间过期"
				}())
		}
	}
}

// GetRoomInfo 获取房间详细信息
func (r *Room) GetRoomInfo() *pb.RoomInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	players := make([]*pb.PlayerInfo, 0, len(r.Players))
	for _, player := range r.Players {
		players = append(players, player)
	}

	return &pb.RoomInfo{
		RoomId:      r.ID,
		RoomName:    r.Name,
		Status:      r.Status,
		Players:     players,
		GameState:   r.Game.GetGameState(),
		OwnerId:     r.OwnerID,
		CreatedTime: r.CreatedTime,
	}
}

// GetOtherPlayers 获取除指定玩家外的其他玩家ID列表
func (r *Room) GetOtherPlayers(excludePlayerID string) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	otherPlayers := make([]string, 0, len(r.Players)-1)
	for playerID := range r.Players {
		if playerID != excludePlayerID {
			otherPlayers = append(otherPlayers, playerID)
		}
	}
	return otherPlayers
}

// GetAllPlayers 获取房间内所有玩家ID列表
func (r *Room) GetAllPlayers() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	players := make([]string, 0, len(r.Players))
	for playerID := range r.Players {
		players = append(players, playerID)
	}
	return players
}

// ToggleReady 切换玩家准备状态
func (rm *RoomManager) ToggleReady(playerID, roomID string) (*Room, bool, error) {
	room := rm.GetRoom(roomID)
	if room == nil {
		return nil, false, fmt.Errorf("房间不存在: %s", roomID)
	}

	room.mu.Lock()
	defer room.mu.Unlock()

	// 检查玩家是否在房间中
	playerInfo, exists := room.Players[playerID]
	if !exists {
		return nil, false, fmt.Errorf("玩家不在房间中")
	}

	// 检查房间状态
	if room.Status != pb.RoomStatus_WAITING {
		return nil, false, fmt.Errorf("游戏已开始，无法切换准备状态")
	}

	// 切换准备状态
	playerInfo.IsReady = !playerInfo.IsReady
	newReadyState := playerInfo.IsReady

	log.Printf("玩家切换准备状态 - 玩家: %s, 房间: %s, 状态: %t", playerID, roomID, newReadyState)
	return room, newReadyState, nil
}
