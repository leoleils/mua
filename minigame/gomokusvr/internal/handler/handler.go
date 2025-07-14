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

	if len(otherPlayers) > 0 {
		notification.NotifyPiecePlaced(otherPlayers, placeResp)
		log.Printf("推送下棋通知给玩家: %v", otherPlayers)
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
