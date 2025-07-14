package main

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"os"
	"strconv"

	"strings"
	"sync"
	"time"

	"mua/client/pb"

	"google.golang.org/protobuf/proto"
)

type GomokuClient struct {
	conn      net.Conn
	playerID  string
	connected bool
	mu        sync.RWMutex
	done      chan struct{}

	// 当前房间信息
	currentRoomID string
	currentRoom   *pb.RoomInfo

	// 消息处理
	msgHandlers map[string]func([]byte) error
}

func NewGomokuClient(playerID string) *GomokuClient {
	if playerID == "" {
		playerID = fmt.Sprintf("player_%d", rand.Intn(10000))
	}

	client := &GomokuClient{
		playerID:    playerID,
		done:        make(chan struct{}),
		msgHandlers: make(map[string]func([]byte) error),
	}

	// 注册消息处理器
	client.registerMessageHandlers()

	return client
}

func (c *GomokuClient) registerMessageHandlers() {
	c.msgHandlers["CreateRoom"] = c.handleCreateRoomResponse
	c.msgHandlers["JoinRoom"] = c.handleJoinRoomResponse
	c.msgHandlers["LeaveRoom"] = c.handleLeaveRoomResponse
	c.msgHandlers["PlacePiece"] = c.handlePlacePieceResponse
	c.msgHandlers["GetRoomList"] = c.handleGetRoomListResponse
	c.msgHandlers["StartGame"] = c.handleStartGameResponse
	c.msgHandlers["PlayerReady"] = c.handlePlayerReadyResponse
}

func (c *GomokuClient) Connect(addr string) error {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return fmt.Errorf("连接失败: %v", err)
	}

	c.conn = conn
	c.connected = true

	// 发送首条心跳消息
	if err := c.sendHeartbeat(); err != nil {
		c.conn.Close()
		return fmt.Errorf("认证失败: %v", err)
	}

	fmt.Printf("已连接到服务器: %s，玩家ID: %s\n", addr, c.playerID)

	// 启动消息接收和心跳协程
	go c.receiveMessages()
	go c.heartbeatLoop()

	return nil
}

func (c *GomokuClient) sendHeartbeat() error {
	heartbeat := &pb.GameMessage{
		MsgHead: &pb.HeadMessage{
			PlayerId:    c.playerID,
			ServiceName: "gomokusvr",
			Group:       "DEFAULT_GROUP",
			Timestamp:   time.Now().UnixMilli(),
		},
		MsgType: pb.MessageType_HEARTBEAT,
		Payload: []byte{},
	}

	return c.sendMessage(heartbeat)
}

func (c *GomokuClient) sendMessage(msg *pb.GameMessage) error {
	if !c.connected {
		return fmt.Errorf("客户端未连接")
	}

	data, err := proto.Marshal(msg)
	if err != nil {
		return fmt.Errorf("序列化消息失败: %v", err)
	}

	// 发送长度头（4字节小端序）
	length := uint32(len(data))
	lenBuf := make([]byte, 4)
	binary.LittleEndian.PutUint32(lenBuf, length)

	if _, err := c.conn.Write(lenBuf); err != nil {
		return fmt.Errorf("发送长度头失败: %v", err)
	}

	if _, err := c.conn.Write(data); err != nil {
		return fmt.Errorf("发送消息体失败: %v", err)
	}

	return nil
}

func (c *GomokuClient) receiveMessage() (*pb.GameMessage, error) {
	// 读取消息长度
	lenBuf := make([]byte, 4)
	if _, err := io.ReadFull(c.conn, lenBuf); err != nil {
		return nil, fmt.Errorf("读取长度头失败: %v", err)
	}

	length := binary.LittleEndian.Uint32(lenBuf)
	if length == 0 {
		return nil, fmt.Errorf("无效的消息长度")
	}

	// 读取消息体
	data := make([]byte, length)
	if _, err := io.ReadFull(c.conn, data); err != nil {
		return nil, fmt.Errorf("读取消息体失败: %v", err)
	}

	// 反序列化消息
	var msg pb.GameMessage
	if err := proto.Unmarshal(data, &msg); err != nil {
		return nil, fmt.Errorf("反序列化消息失败: %v", err)
	}

	return &msg, nil
}

func (c *GomokuClient) receiveMessages() {
	defer func() {
		c.mu.Lock()
		c.connected = false
		c.mu.Unlock()
		close(c.done)
	}()

	for c.connected {
		msg, err := c.receiveMessage()
		if err != nil {
			if c.connected {
				log.Printf("接收消息失败: %v", err)
			}
			break
		}

		c.handleMessage(msg)
	}
}

func (c *GomokuClient) handleMessage(msg *pb.GameMessage) {
	switch msg.MsgType {
	case pb.MessageType_HEARTBEAT:
		// 心跳回应
		return
	case pb.MessageType_SERVICE_MESSAGE:
		// 服务消息响应
		c.handleServiceResponse(msg)
	case pb.MessageType_NOTIFY:
		// 通知消息
		c.handleNotifyMessage(msg)
	default:
		log.Printf("未知消息类型: %v", msg.MsgType)
	}
}

func (c *GomokuClient) handleServiceResponse(msg *pb.GameMessage) {
	var response pb.GameMessageResponse
	if err := proto.Unmarshal(msg.Payload, &response); err != nil {
		log.Printf("解析服务响应失败: %v", err)
		return
	}

	// 根据请求ID处理不同的响应
	requestId := ""
	if response.MsgHead != nil {
		requestId = response.MsgHead.RequestId
	}

	if handler, exists := c.msgHandlers[requestId]; exists {
		if response.GetData() != nil {
			handler(response.GetData())
		}
	}

	if response.Ret == 0 {
		c.showSuccess("操作成功")
	} else {
		c.showError(fmt.Sprintf("操作失败: %s", response.GetReason()))
	}
}

func (c *GomokuClient) handleNotifyMessage(msg *pb.GameMessage) {
	// 处理游戏状态通知和玩家事件通知
	var gameStateNotify pb.GameStateNotify
	if err := proto.Unmarshal(msg.Payload, &gameStateNotify); err == nil {
		c.handleGameStateNotify(&gameStateNotify)
		return
	}

	var playerEventNotify pb.PlayerEventNotify
	if err := proto.Unmarshal(msg.Payload, &playerEventNotify); err == nil {
		c.handlePlayerEventNotify(&playerEventNotify)
		return
	}

	c.showNotification("收到服务器通知")
}

func (c *GomokuClient) heartbeatLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if c.connected {
				if err := c.sendHeartbeat(); err != nil {
					log.Printf("发送心跳失败: %v", err)
					return
				}
			}
		case <-c.done:
			return
		}
	}
}

func (c *GomokuClient) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.connected {
		c.connected = false
		c.conn.Close()
	}
}

func (c *GomokuClient) sendServiceMessage(method string, payload []byte) error {
	msg := &pb.GameMessage{
		MsgHead: &pb.HeadMessage{
			PlayerId:       c.playerID,
			ServiceName:    "gomokusvr",
			Group:          "DEFAULT_GROUP",
			RequestId:      method,
			Timestamp:      time.Now().UnixMilli(),
			ServiceMsgType: pb.ServiceMessageType_SYNC,
		},
		MsgType: pb.MessageType_SERVICE_MESSAGE,
		Payload: payload,
	}

	fmt.Printf("发送服务消息: %s, 负载大小: %d字节\n", method, len(payload))
	return c.sendMessage(msg)
}

// 游戏功能实现

func (c *GomokuClient) createRoom(roomName string) error {
	if err := c.validateRoomName(roomName); err != nil {
		return err
	}

	req := &pb.CreateRoomRequest{
		RoomName: roomName,
	}

	data, err := proto.Marshal(req)
	if err != nil {
		return fmt.Errorf("序列化创建房间请求失败: %v", err)
	}

	return c.sendServiceMessage("CreateRoom", data)
}

func (c *GomokuClient) joinRoom(roomID string) error {
	if roomID == "" {
		return fmt.Errorf("房间ID不能为空")
	}

	req := &pb.JoinRoomRequest{
		RoomId: roomID,
	}

	data, err := proto.Marshal(req)
	if err != nil {
		return fmt.Errorf("序列化加入房间请求失败: %v", err)
	}

	return c.sendServiceMessage("JoinRoom", data)
}

func (c *GomokuClient) leaveRoom() error {
	if c.currentRoomID == "" {
		return fmt.Errorf("当前未在任何房间中")
	}

	req := &pb.LeaveRoomRequest{
		RoomId: c.currentRoomID,
	}

	data, err := proto.Marshal(req)
	if err != nil {
		return fmt.Errorf("序列化离开房间请求失败: %v", err)
	}

	return c.sendServiceMessage("LeaveRoom", data)
}

func (c *GomokuClient) getRoomList() error {
	req := &pb.GetRoomListRequest{
		Page:     1,
		PageSize: 10,
	}

	data, err := proto.Marshal(req)
	if err != nil {
		return fmt.Errorf("序列化房间列表请求失败: %v", err)
	}

	return c.sendServiceMessage("GetRoomList", data)
}

func (c *GomokuClient) startGame() error {
	if c.currentRoomID == "" {
		return fmt.Errorf("当前未在任何房间中")
	}

	req := &pb.StartGameRequest{
		RoomId: c.currentRoomID,
	}

	data, err := proto.Marshal(req)
	if err != nil {
		return fmt.Errorf("序列化开始游戏请求失败: %v", err)
	}

	return c.sendServiceMessage("StartGame", data)
}

func (c *GomokuClient) placePiece(x, y int32) error {
	if c.currentRoomID == "" {
		return fmt.Errorf("当前未在任何房间中")
	}

	req := &pb.PlacePieceRequest{
		RoomId: c.currentRoomID,
		X:      x,
		Y:      y,
	}

	data, err := proto.Marshal(req)
	if err != nil {
		return fmt.Errorf("序列化下棋请求失败: %v", err)
	}

	return c.sendServiceMessage("PlacePiece", data)
}

// 消息处理器实现

func (c *GomokuClient) handleCreateRoomResponse(data []byte) error {
	var response pb.CreateRoomResponse
	if err := proto.Unmarshal(data, &response); err != nil {
		return err
	}

	if response.Success {
		c.currentRoomID = response.RoomId
		c.showSuccess(fmt.Sprintf("房间创建成功，房间ID: %s", response.RoomId))
	} else {
		c.showError(fmt.Sprintf("房间创建失败: %s", response.Message))
	}

	return nil
}

func (c *GomokuClient) handleJoinRoomResponse(data []byte) error {
	var response pb.JoinRoomResponse
	if err := proto.Unmarshal(data, &response); err != nil {
		return err
	}

	if response.Success {
		c.currentRoomID = response.RoomInfo.RoomId
		c.currentRoom = response.RoomInfo
		c.showSuccess("成功加入房间")
		c.displayRoomInfo()
	} else {
		c.showError(fmt.Sprintf("加入房间失败: %s", response.Message))
	}

	return nil
}

func (c *GomokuClient) handleLeaveRoomResponse(data []byte) error {
	var response pb.LeaveRoomResponse
	if err := proto.Unmarshal(data, &response); err != nil {
		return err
	}

	if response.Success {
		c.currentRoomID = ""
		c.currentRoom = nil
		c.showSuccess("成功离开房间")
	} else {
		c.showError(fmt.Sprintf("离开房间失败: %s", response.Message))
	}

	return nil
}

func (c *GomokuClient) handlePlacePieceResponse(data []byte) error {
	var response pb.PlacePieceResponse
	if err := proto.Unmarshal(data, &response); err != nil {
		return err
	}

	if response.Success {
		c.showSuccess("下棋成功")
		if response.GameState != nil {
			if c.currentRoom != nil {
				c.currentRoom.GameState = response.GameState
			}
			c.displayBoard(response.GameState)
		}
	} else {
		c.showError(fmt.Sprintf("下棋失败: %s", response.Message))
	}

	return nil
}

func (c *GomokuClient) handleGetRoomListResponse(data []byte) error {
	var response pb.GetRoomListResponse
	if err := proto.Unmarshal(data, &response); err != nil {
		return err
	}

	if response.Success {
		c.displayRoomList(response.Rooms, response.TotalCount)
	} else {
		c.showError(fmt.Sprintf("获取房间列表失败: %s", response.Message))
	}

	return nil
}

func (c *GomokuClient) handleStartGameResponse(data []byte) error {
	var response pb.StartGameResponse
	if err := proto.Unmarshal(data, &response); err != nil {
		return err
	}

	if response.Success {
		c.showSuccess("游戏开始!")
		if response.GameState != nil {
			if c.currentRoom != nil {
				c.currentRoom.GameState = response.GameState
				c.currentRoom.Status = pb.RoomStatus_PLAYING
			}
			c.displayBoard(response.GameState)
		}
	} else {
		c.showError(fmt.Sprintf("开始游戏失败: %s", response.Message))
	}

	return nil
}

func (c *GomokuClient) handleGameStateNotify(notify *pb.GameStateNotify) {
	c.showNotification(fmt.Sprintf("游戏状态更新: %s", notify.EventMessage))

	if notify.GameState != nil && c.currentRoom != nil {
		c.currentRoom.GameState = notify.GameState
		c.displayBoard(notify.GameState)
	}
}

func (c *GomokuClient) handlePlayerEventNotify(notify *pb.PlayerEventNotify) {
	c.showNotification(fmt.Sprintf("玩家事件: %s", notify.EventMessage))
}

// 交互式命令处理

func (c *GomokuClient) runInteractiveMode() {
	scanner := bufio.NewScanner(os.Stdin)
	c.runInteractiveModeWithScanner(scanner)
}

func (c *GomokuClient) runInteractiveModeWithScanner(scanner *bufio.Scanner) {
	c.showHelp()

	for {
		fmt.Print("五子棋> ")
		fmt.Printf("[DEBUG] 等待输入...\n")
		if !scanner.Scan() {
			fmt.Printf("[DEBUG] scanner.Scan() 返回false，退出循环\n")
			if err := scanner.Err(); err != nil {
				fmt.Printf("[DEBUG] scanner 错误: %v\n", err)
			}
			break
		}

		line := strings.TrimSpace(scanner.Text())
		fmt.Printf("[DEBUG] 读取到输入: '%s'\n", line)
		if line == "" {
			fmt.Printf("[DEBUG] 空行，继续\n")
			continue
		}

		parts := strings.Fields(line)
		if len(parts) == 0 {
			fmt.Printf("[DEBUG] 无有效字段，继续\n")
			continue
		}

		cmd := parts[0]
		args := parts[1:]
		fmt.Printf("[DEBUG] 解析命令: cmd='%s', args=%v\n", cmd, args)

		if err := c.handleCommand(cmd, args); err != nil {
			if err.Error() == "exit" {
				fmt.Printf("[DEBUG] 收到退出命令\n")
				break
			}
			c.showError(err.Error())
		}
	}
	fmt.Printf("[DEBUG] 退出交互模式\n")
}

func (c *GomokuClient) handleCommand(cmd string, args []string) error {
	fmt.Printf("处理命令: %s\n", cmd)
	switch cmd {
	case "help", "h":
		c.showHelp()
	case "quit", "q":
		return fmt.Errorf("exit")
	case "status":
		c.showStatus()
	case "rooms":
		fmt.Println("执行rooms命令...")
		return c.getRoomList()
	case "create":
		if len(args) < 1 {
			return fmt.Errorf("使用方法: create <房间名>")
		}
		fmt.Printf("执行create命令，房间名: %s\n", strings.Join(args, " "))
		return c.createRoom(strings.Join(args, " "))
	case "join":
		if len(args) < 1 {
			return fmt.Errorf("使用方法: join <房间ID>")
		}
		return c.joinRoom(args[0])
	case "leave":
		return c.leaveRoom()
	case "start":
		return c.startGame()
	case "ready":
		return c.playerReady()
	case "move":
		if len(args) < 2 {
			return fmt.Errorf("使用方法: move <x> <y>")
		}
		x, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("无效的x坐标: %s", args[0])
		}
		y, err := strconv.Atoi(args[1])
		if err != nil {
			return fmt.Errorf("无效的y坐标: %s", args[1])
		}
		if err := c.validateMove(int32(x), int32(y)); err != nil {
			return err
		}
		return c.placePiece(int32(x), int32(y))
	case "room":
		c.displayRoomInfo()
	case "board":
		if c.currentRoom != nil && c.currentRoom.GameState != nil {
			c.displayBoard(c.currentRoom.GameState)
		} else {
			c.showInfo("当前没有游戏进行中")
		}
	default:
		return fmt.Errorf("未知命令: %s，输入 help 查看帮助", cmd)
	}

	return nil
}

func main() {
	fmt.Println("=== 五子棋客户端 ===")

	// 获取玩家ID
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Print("请输入玩家ID (直接回车使用随机ID): ")
	scanner.Scan()
	playerID := strings.TrimSpace(scanner.Text())

	// 创建客户端
	client := NewGomokuClient(playerID)
	defer client.Close()

	// 连接到服务器
	fmt.Print("请输入服务器地址 (默认 localhost:6001): ")
	scanner.Scan()
	addr := strings.TrimSpace(scanner.Text())
	if addr == "" {
		addr = "localhost:6001"
	}

	if err := client.Connect(addr); err != nil {
		log.Fatalf("连接失败: %v", err)
	}

	// 进入交互模式，重用同一个scanner
	client.runInteractiveModeWithScanner(scanner)

	fmt.Println("客户端退出")
}

// 缺失的方法
func (c *GomokuClient) showSuccess(msg string) {
	fmt.Printf("✅ %s\n", msg)
}

func (c *GomokuClient) showError(msg string) {
	fmt.Printf("❌ %s\n", msg)
}

func (c *GomokuClient) showNotification(msg string) {
	fmt.Printf("🔔 %s\n", msg)
}

func (c *GomokuClient) validateRoomName(name string) error {
	if name == "" {
		return fmt.Errorf("房间名不能为空")
	}
	if len(name) > 32 {
		return fmt.Errorf("房间名不能超过32个字符")
	}
	return nil
}

func (c *GomokuClient) displayRoomInfo() {
	if c.currentRoom == nil {
		fmt.Println("当前未在任何房间中")
		return
	}

	fmt.Printf("房间信息:\n")
	fmt.Printf("  房间ID: %s\n", c.currentRoom.RoomId)
	fmt.Printf("  房间名: %s\n", c.currentRoom.RoomName)
	fmt.Printf("  状态: %s\n", c.getRoomStatusName(c.currentRoom.Status))
	fmt.Printf("  玩家数: %d\n", len(c.currentRoom.Players))
}

func (c *GomokuClient) getRoomStatusName(status pb.RoomStatus) string {
	switch status {
	case pb.RoomStatus_WAITING:
		return "等待玩家"
	case pb.RoomStatus_PLAYING:
		return "游戏中"
	case pb.RoomStatus_FINISHED:
		return "已结束"
	default:
		return "未知"
	}
}

func (c *GomokuClient) displayBoard(gameState *pb.GameState) {
	if gameState == nil || len(gameState.Board) == 0 {
		fmt.Println("棋盘为空")
		return
	}

	fmt.Println("棋盘状态:")
	fmt.Print("   ")
	for i := 0; i < 15; i++ {
		fmt.Printf("%2d ", i)
	}
	fmt.Println()

	for i := 0; i < 15; i++ {
		fmt.Printf("%2d ", i)
		for j := 0; j < 15; j++ {
			pos := i*15 + j
			if pos < len(gameState.Board) {
				switch gameState.Board[pos] {
				case 0:
					fmt.Print(" · ")
				case 1:
					fmt.Print(" ● ")
				case 2:
					fmt.Print(" ○ ")
				default:
					fmt.Print(" ? ")
				}
			} else {
				fmt.Print(" · ")
			}
		}
		fmt.Println()
	}
}

func (c *GomokuClient) displayRoomList(rooms []*pb.RoomInfo, totalCount int32) {
	fmt.Printf("房间列表 (总数: %d):\n", totalCount)
	if len(rooms) == 0 {
		fmt.Println("暂无房间")
		return
	}

	for i, room := range rooms {
		fmt.Printf("%d. [%s] %s - %s (%d/2 玩家)\n",
			i+1, room.RoomId, room.RoomName,
			c.getRoomStatusName(room.Status),
			len(room.Players))
	}
}

func (c *GomokuClient) showHelp() {
	fmt.Println("可用命令:")
	fmt.Println("  rooms        - 查看房间列表")
	fmt.Println("  create <名称> - 创建房间")
	fmt.Println("  join <房间ID> - 加入房间")
	fmt.Println("  leave        - 离开房间")
	fmt.Println("  ready        - 切换准备状态")
	fmt.Println("  start        - 开始游戏")
	fmt.Println("  move <x> <y> - 下棋")
	fmt.Println("  board        - 显示棋盘")
	fmt.Println("  status       - 显示状态")
	fmt.Println("  help         - 显示帮助")
	fmt.Println("  quit         - 退出")
}

func (c *GomokuClient) showStatus() {
	fmt.Printf("连接状态: %s\n", map[bool]string{true: "已连接", false: "未连接"}[c.connected])
	fmt.Printf("玩家ID: %s\n", c.playerID)
	if c.currentRoomID != "" {
		fmt.Printf("当前房间: %s\n", c.currentRoomID)
	} else {
		fmt.Printf("当前房间: 无\n")
	}
}

func (c *GomokuClient) validateMove(x, y int32) error {
	if x < 0 || x >= 15 || y < 0 || y >= 15 {
		return fmt.Errorf("坐标超出范围 (0-14)")
	}
	return nil
}

func (c *GomokuClient) showInfo(msg string) {
	fmt.Printf("ℹ️  %s\n", msg)
}

// playerReady 发送准备请求
func (c *GomokuClient) playerReady() error {
	if c.currentRoomID == "" {
		return fmt.Errorf("当前未在任何房间中")
	}

	req := &pb.PlayerReadyRequest{
		RoomId: c.currentRoomID,
	}

	data, err := proto.Marshal(req)
	if err != nil {
		return fmt.Errorf("序列化准备请求失败: %v", err)
	}

	return c.sendServiceMessage("PlayerReady", data)
}

// handlePlayerReadyResponse 处理准备响应
func (c *GomokuClient) handlePlayerReadyResponse(data []byte) error {
	var response pb.PlayerReadyResponse
	if err := proto.Unmarshal(data, &response); err != nil {
		return err
	}

	if response.Success {
		readyStatus := "未准备"
		if response.IsReady {
			readyStatus = "已准备"
		}
		c.showSuccess(fmt.Sprintf("准备状态更新成功: %s", readyStatus))

		// 更新当前房间信息
		if response.RoomInfo != nil {
			c.currentRoom = response.RoomInfo
			c.displayRoomInfo()
		}
	} else {
		c.showError(fmt.Sprintf("准备状态更新失败: %s", response.Message))
	}

	return nil
}
