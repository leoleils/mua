package handler

import (
	"log"

	"mua/minigame/gomokusvr/internal/notification"
	"mua/minigame/gomokusvr/internal/pb"
	"mua/minigame/gomokusvr/internal/room"

	"google.golang.org/protobuf/proto"
)

// GameHandler 游戏消息处理器
type GameHandler struct {
	roomManager *room.RoomManager
}

// NewGameHandler 创建新的游戏处理器
func NewGameHandler() *GameHandler {
	return &GameHandler{
		roomManager: room.GetRoomManager(),
	}
}

// notifyOtherPlayers 通知房间内其他玩家
func (h *GameHandler) notifyOtherPlayers(room *room.Room, currentPlayerID string, placeResp *pb.PlacePieceResponse) {
	allPlayers := room.GetAllPlayers()
	otherPlayers := make([]string, 0, len(allPlayers)-1)
	for _, playerID := range allPlayers {
		if playerID != currentPlayerID {
			otherPlayers = append(otherPlayers, playerID)
		}
	}

	log.Printf("房间 %s 中玩家 %s 下棋，需要通知的其他玩家: %v (共%d人)",
		room.ID, currentPlayerID, otherPlayers, len(otherPlayers))

	if len(otherPlayers) > 0 {
		log.Printf("开始推送棋子放置通知 (PiecePlacedNotification) 给玩家: %v", otherPlayers)
		notification.NotifyPiecePlaced(otherPlayers, placeResp)
		log.Printf("✅ 已完成推送下棋通知给玩家: %v", otherPlayers)
	} else {
		log.Printf("⚠️  房间 %s 中没有其他玩家需要接收通知", room.ID)
	}
}

// notifyGameOver 通知游戏结束
func (h *GameHandler) notifyGameOver(room *room.Room, gameState *pb.GameState) {
	allPlayers := room.GetAllPlayers()
	if len(allPlayers) > 0 {
		notification.NotifyGameStateChanged(allPlayers, gameState)
		log.Printf("推送游戏结束通知给玩家: %v", allPlayers)
	}
}

// notifyPlayerJoined 通知玩家加入
func (h *GameHandler) notifyPlayerJoined(room *room.Room, joinedPlayerID string) {
	allPlayers := room.GetAllPlayers()
	otherPlayers := make([]string, 0, len(allPlayers)-1)
	for _, playerID := range allPlayers {
		if playerID != joinedPlayerID {
			otherPlayers = append(otherPlayers, playerID)
		}
	}

	if len(otherPlayers) > 0 {
		notification.NotifyPlayerJoined(otherPlayers, room.ID, joinedPlayerID)
		log.Printf("推送玩家加入通知给玩家: %v, 加入者: %s", otherPlayers, joinedPlayerID)
	}
}

// notifyPlayerLeft 通知玩家离开
func (h *GameHandler) notifyPlayerLeft(room *room.Room, leftPlayerID string) {
	allPlayers := room.GetAllPlayers()
	otherPlayers := make([]string, 0, len(allPlayers)-1)
	for _, playerID := range allPlayers {
		if playerID != leftPlayerID {
			otherPlayers = append(otherPlayers, playerID)
		}
	}

	if len(otherPlayers) > 0 {
		notification.NotifyPlayerLeft(otherPlayers, room.ID, leftPlayerID)
		log.Printf("推送玩家离开通知给玩家: %v, 离开者: %s", otherPlayers, leftPlayerID)
	}
}

// notifyPlayerReady 通知玩家准备状态变化
func (h *GameHandler) notifyPlayerReady(room *room.Room, readyPlayerID string, isReady bool) {
	allPlayers := room.GetAllPlayers()
	otherPlayers := make([]string, 0, len(allPlayers)-1)
	for _, playerID := range allPlayers {
		if playerID != readyPlayerID {
			otherPlayers = append(otherPlayers, playerID)
		}
	}

	// 获取准备玩家的信息
	var playerInfo *pb.PlayerInfo
	roomInfo := room.GetRoomInfo()
	for _, player := range roomInfo.Players {
		if player.PlayerId == readyPlayerID {
			playerInfo = player
			break
		}
	}

	if len(otherPlayers) > 0 && playerInfo != nil {
		notification.NotifyPlayerReady(otherPlayers, room.ID, readyPlayerID, playerInfo)
		log.Printf("推送玩家准备状态通知给玩家: %v, 准备者: %s, 状态: %t", otherPlayers, readyPlayerID, isReady)
	}
}

// structToBytes 结构体转字节
func structToBytes(data proto.Message) []byte {
	bytes, _ := proto.Marshal(data)
	return bytes
}

// notifyGameStarted 通知游戏开始
func (h *GameHandler) notifyGameStarted(room *room.Room, gameState *pb.GameState) {
	// 获取房间内所有玩家
	allPlayers := room.GetAllPlayers()

	log.Printf("推送游戏开始通知给房间 %s 的所有玩家: %v", room.ID, allPlayers)

	// 构造游戏开始通知
	notify := &pb.GameStateNotify{
		RoomId:       room.ID,
		GameState:    gameState,
		EventType:    "GAME_START",
		EventMessage: "游戏开始！",
	}

	// 推送通知给所有玩家
	notification.PushGameNotificationToPlayers(allPlayers, notify, "GameStartNotification")

	log.Printf("✅ 已完成推送游戏开始通知给房间 %s 的所有玩家: %v", room.ID, allPlayers)
}
