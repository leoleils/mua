package handler

import (
	"context"
	"fmt"
	"log"
	"time"

	"mua/minigame/gomokusvr/internal/pb"

	"google.golang.org/protobuf/proto"
)

// GatewayHandler 实现gatesvr的common.CommonService接口
type GatewayHandler struct {
	pb.UnimplementedCommonServiceServer
	gameHandler *GameHandler
}

// NewGatewayHandler 创建网关处理器
func NewGatewayHandler(gameHandler *GameHandler) *GatewayHandler {
	return &GatewayHandler{
		gameHandler: gameHandler,
	}
}

// SendMessage 实现common.CommonService.SendMessage
func (g *GatewayHandler) SendMessage(ctx context.Context, req *pb.GameMessage) (*pb.GameMessageResponse, error) {
	log.Printf("收到gatesvr消息 - 玩家: %s, 方法: %s", req.Head.PlayerId, req.Head.Method)

	// 创建响应
	response := &pb.GameMessageResponse{
		Head:      req.Head,
		Code:      0,
		Timestamp: time.Now().UnixMilli(),
	}

	// 根据方法名分发处理
	var payload []byte
	var err error

	switch req.Head.Method {
	case "CreateRoom":
		payload, err = g.handleCreateRoom(req)
	case "JoinRoom":
		payload, err = g.handleJoinRoom(req)
	case "LeaveRoom":
		payload, err = g.handleLeaveRoom(req)
	case "DestroyRoom":
		payload, err = g.handleDestroyRoom(req)
	case "PlacePiece":
		payload, err = g.handlePlacePiece(req)
	case "GetRoomList":
		payload, err = g.handleGetRoomList(req)
	case "StartGame":
		payload, err = g.handleStartGame(req)
	case "PlayerReady":
		payload, err = g.handlePlayerReady(req)
	case "GetGameProgress":
		payload, err = g.handleGetGameProgress(req)
	default:
		response.Code = 400
		response.Message = fmt.Sprintf("未知的方法: %s", req.Head.Method)
		return response, nil
	}

	if err != nil {
		log.Printf("处理请求失败: %v", err)
		response.Code = 500
		response.Message = err.Error()
	} else {
		response.Payload = payload
	}

	log.Printf("返回gatesvr响应 - 玩家: %s, 状态: %d", req.Head.PlayerId, response.Code)
	return response, nil
}

func (g *GatewayHandler) handleCreateRoom(req *pb.GameMessage) ([]byte, error) {
	var createReq pb.CreateRoomRequest
	if err := proto.Unmarshal(req.Payload, &createReq); err != nil {
		return nil, fmt.Errorf("参数解析失败: %v", err)
	}

	room, err := g.gameHandler.roomManager.CreateRoom(req.Head.PlayerId, createReq.RoomName, createReq.Password)
	if err != nil {
		return nil, err
	}

	createResp := &pb.CreateRoomResponse{
		Success: true,
		RoomId:  room.ID,
		Message: "房间创建成功",
	}

	return proto.Marshal(createResp)
}

func (g *GatewayHandler) handleJoinRoom(req *pb.GameMessage) ([]byte, error) {
	var joinReq pb.JoinRoomRequest
	if err := proto.Unmarshal(req.Payload, &joinReq); err != nil {
		return nil, fmt.Errorf("参数解析失败: %v", err)
	}

	room, err := g.gameHandler.roomManager.JoinRoom(req.Head.PlayerId, joinReq.RoomId, joinReq.Password)
	if err != nil {
		return nil, err
	}

	joinResp := &pb.JoinRoomResponse{
		Success:  true,
		Message:  "加入房间成功",
		RoomInfo: room.GetRoomInfo(),
	}

	// 推送通知
	go g.gameHandler.notifyPlayerJoined(room, req.Head.PlayerId)

	return proto.Marshal(joinResp)
}

func (g *GatewayHandler) handleLeaveRoom(req *pb.GameMessage) ([]byte, error) {
	var leaveReq pb.LeaveRoomRequest
	if err := proto.Unmarshal(req.Payload, &leaveReq); err != nil {
		return nil, fmt.Errorf("参数解析失败: %v", err)
	}

	// 推送通知（离开前）
	if targetRoom := g.gameHandler.roomManager.GetRoom(leaveReq.RoomId); targetRoom != nil {
		go g.gameHandler.notifyPlayerLeft(targetRoom, req.Head.PlayerId)
	}

	err := g.gameHandler.roomManager.LeaveRoom(req.Head.PlayerId, leaveReq.RoomId)
	if err != nil {
		return nil, err
	}

	leaveResp := &pb.LeaveRoomResponse{
		Success: true,
		Message: "离开房间成功",
	}

	return proto.Marshal(leaveResp)
}

func (g *GatewayHandler) handleDestroyRoom(req *pb.GameMessage) ([]byte, error) {
	var destroyReq pb.DestroyRoomRequest
	if err := proto.Unmarshal(req.Payload, &destroyReq); err != nil {
		return nil, fmt.Errorf("参数解析失败: %v", err)
	}

	err := g.gameHandler.roomManager.DestroyRoom(req.Head.PlayerId, destroyReq.RoomId)
	if err != nil {
		return nil, err
	}

	destroyResp := &pb.DestroyRoomResponse{
		Success: true,
		Message: "房间销毁成功",
	}

	return proto.Marshal(destroyResp)
}

func (g *GatewayHandler) handlePlacePiece(req *pb.GameMessage) ([]byte, error) {
	var placeReq pb.PlacePieceRequest
	if err := proto.Unmarshal(req.Payload, &placeReq); err != nil {
		return nil, fmt.Errorf("参数解析失败: %v", err)
	}

	log.Printf("玩家 %s 请求在房间 %s 的位置 (%d,%d) 下棋", req.Head.PlayerId, placeReq.RoomId, placeReq.X, placeReq.Y)

	gameInstance, err := g.gameHandler.roomManager.PlacePiece(req.Head.PlayerId, placeReq.RoomId, placeReq.X, placeReq.Y)
	if err != nil {
		log.Printf("下棋失败: %v", err)
		return nil, err
	}

	// 转换为GameState
	gameState := gameInstance.GetGameState()

	placeResp := &pb.PlacePieceResponse{
		Success:   true,
		Message:   "下棋成功",
		GameState: gameState,
	}

	log.Printf("玩家 %s 下棋成功，游戏状态: %s", req.Head.PlayerId, gameState.Result.String())

	// 推送通知
	if room := g.gameHandler.roomManager.GetRoom(placeReq.RoomId); room != nil {
		log.Printf("开始推送下棋通知给房间 %s 的其他玩家", placeReq.RoomId)
		go g.gameHandler.notifyOtherPlayers(room, req.Head.PlayerId, placeResp)

		if gameState.Result != pb.GameResult_ONGOING {
			log.Printf("游戏结束，推送游戏结束通知给房间 %s 的所有玩家", placeReq.RoomId)
			go g.gameHandler.notifyGameOver(room, gameState)
		}
	} else {
		log.Printf("警告: 未找到房间 %s，无法推送通知", placeReq.RoomId)
	}

	return proto.Marshal(placeResp)
}

func (g *GatewayHandler) handleGetRoomList(req *pb.GameMessage) ([]byte, error) {
	var listReq pb.GetRoomListRequest
	if err := proto.Unmarshal(req.Payload, &listReq); err != nil {
		return nil, fmt.Errorf("参数解析失败: %v", err)
	}

	rooms, totalCount := g.gameHandler.roomManager.GetRoomList(listReq.Page, listReq.PageSize)

	var roomInfos []*pb.RoomInfo
	for _, room := range rooms {
		roomInfos = append(roomInfos, room.GetRoomInfo())
	}

	listResp := &pb.GetRoomListResponse{
		Success:    true,
		Rooms:      roomInfos,
		TotalCount: totalCount,
		Message:    "获取房间列表成功",
	}

	return proto.Marshal(listResp)
}

func (g *GatewayHandler) handleStartGame(req *pb.GameMessage) ([]byte, error) {
	var startReq pb.StartGameRequest
	if err := proto.Unmarshal(req.Payload, &startReq); err != nil {
		return nil, fmt.Errorf("参数解析失败: %v", err)
	}

	err := g.gameHandler.roomManager.StartGame(req.Head.PlayerId, startReq.RoomId)
	if err != nil {
		return nil, err
	}

	// 获取房间的游戏状态
	room := g.gameHandler.roomManager.GetRoom(startReq.RoomId)
	if room == nil {
		return nil, fmt.Errorf("房间不存在")
	}

	gameState := room.Game.GetGameState()

	startResp := &pb.StartGameResponse{
		Success:   true,
		Message:   "游戏开始",
		GameState: gameState,
	}

	// 推送通知给房间内所有玩家
	if room != nil {
		go g.gameHandler.notifyGameStarted(room, gameState)
	}

	return proto.Marshal(startResp)
}

func (g *GatewayHandler) handlePlayerReady(req *pb.GameMessage) ([]byte, error) {
	var readyReq pb.PlayerReadyRequest
	if err := proto.Unmarshal(req.Payload, &readyReq); err != nil {
		return nil, fmt.Errorf("参数解析失败: %v", err)
	}

	room, isReady, err := g.gameHandler.roomManager.ToggleReady(req.Head.PlayerId, readyReq.RoomId)
	if err != nil {
		return nil, err
	}

	readyResp := &pb.PlayerReadyResponse{
		Success:  true,
		Message:  fmt.Sprintf("准备状态已更新为: %t", isReady),
		IsReady:  isReady,
		RoomInfo: room.GetRoomInfo(),
	}

	// 推送通知给房间内其他玩家
	if room != nil {
		go g.gameHandler.notifyPlayerReady(room, req.Head.PlayerId, isReady)
	}

	return proto.Marshal(readyResp)
}

func (g *GatewayHandler) handleGetGameProgress(req *pb.GameMessage) ([]byte, error) {
	var progressReq pb.GetGameProgressRequest
	if err := proto.Unmarshal(req.Payload, &progressReq); err != nil {
		return nil, fmt.Errorf("参数解析失败: %v", err)
	}

	// 获取房间信息
	room := g.gameHandler.roomManager.GetRoom(progressReq.RoomId)
	if room == nil {
		return nil, fmt.Errorf("房间不存在: %s", progressReq.RoomId)
	}

	// 检查玩家是否在房间中
	if !room.HasPlayer(req.Head.PlayerId) {
		return nil, fmt.Errorf("玩家不在房间中")
	}

	// 获取游戏状态
	gameState := room.GetGameState()
	if gameState == nil {
		// 如果游戏还未开始，创建一个空的游戏状态
		gameState = &pb.GameState{
			Board:       make([]int32, 225), // 15x15=225个位置，全部为0
			CurrentTurn: pb.PlayerColor_BLACK,
			Result:      pb.GameResult_ONGOING,
			WinnerId:    "",
			TotalMoves:  0,
			MoveHistory: []*pb.Move{},
		}
	}

	// 计算剩余时间（简化实现，可以根据具体需求调整）
	remainingTime := int32(-1) // -1表示无限制时间

	progressResp := &pb.GetGameProgressResponse{
		Success:       true,
		Message:       "获取游戏进度成功",
		RoomInfo:      room.GetRoomInfo(),
		GameState:     gameState,
		RemainingTime: remainingTime,
	}

	log.Printf("玩家 %s 获取房间 %s 的游戏进度", req.Head.PlayerId, progressReq.RoomId)

	return proto.Marshal(progressResp)
}
