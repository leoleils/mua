package main

import (
	"fmt"
	"strings"

	"mua/client/pb"
)

// 棋盘显示相关

func (c *GomokuClient) displayBoard(gameState *pb.GameState) {
	if gameState == nil {
		fmt.Println("暂无棋盘数据")
		return
	}

	fmt.Println("\n   棋盘 (15x15)")
	fmt.Print("   ")
	
	// 打印列号
	for i := 0; i < 15; i++ {
		fmt.Printf("%2d", i)
	}
	fmt.Println()

	// 打印棋盘
	for y := 0; y < 15; y++ {
		fmt.Printf("%2d ", y)
		for x := 0; x < 15; x++ {
			pos := y*15 + x
			if pos < len(gameState.Board) {
				switch gameState.Board[pos] {
				case 0:
					fmt.Print(" ·") // 空位
				case 1:
					fmt.Print(" ●") // 黑子
				case 2:
					fmt.Print(" ○") // 白子
				default:
					fmt.Print(" ?")
				}
			} else {
				fmt.Print(" ·")
			}
		}
		fmt.Println()
	}
	
	// 显示游戏信息
	fmt.Printf("\n当前轮到: %s\n", getColorName(gameState.CurrentTurn))
	fmt.Printf("总步数: %d\n", gameState.TotalMoves)
	
	if gameState.Result != pb.GameResult_ONGOING {
		fmt.Printf("游戏结果: %s\n", getGameResultName(gameState.Result))
		if gameState.WinnerId != "" {
			fmt.Printf("获胜者: %s\n", gameState.WinnerId)
		}
	}
}

func getColorName(color pb.PlayerColor) string {
	switch color {
	case pb.PlayerColor_BLACK:
		return "黑子 ●"
	case pb.PlayerColor_WHITE:
		return "白子 ○"
	default:
		return "无"
	}
}

func getGameResultName(result pb.GameResult) string {
	switch result {
	case pb.GameResult_ONGOING:
		return "进行中"
	case pb.GameResult_BLACK_WIN:
		return "黑子获胜"
	case pb.GameResult_WHITE_WIN:
		return "白子获胜"
	case pb.GameResult_DRAW:
		return "平局"
	default:
		return "未知"
	}
}

func (c *GomokuClient) displayRoomInfo() {
	if c.currentRoom == nil {
		fmt.Println("当前未在任何房间中")
		return
	}

	room := c.currentRoom
	fmt.Printf("\n=== 房间信息 ===\n")
	fmt.Printf("房间ID: %s\n", room.RoomId)
	fmt.Printf("房间名: %s\n", room.RoomName)
	fmt.Printf("状态: %s\n", getRoomStatusName(room.Status))
	fmt.Printf("房主: %s\n", room.OwnerId)
	
	fmt.Printf("\n玩家列表:\n")
	for i, player := range room.Players {
		readyStatus := "未准备"
		if player.IsReady {
			readyStatus = "已准备"
		}
		
		ownerMark := ""
		if player.PlayerId == room.OwnerId {
			ownerMark = " [房主]"
		}
		
		fmt.Printf("  %d. %s%s - %s (%s)\n", 
			i+1, player.Username, ownerMark, getColorName(player.Color), readyStatus)
	}
	
	if room.GameState != nil {
		fmt.Println("\n=== 游戏状态 ===")
		c.displayBoard(room.GameState)
	}
}

func getRoomStatusName(status pb.RoomStatus) string {
	switch status {
	case pb.RoomStatus_WAITING:
		return "等待玩家"
	case pb.RoomStatus_PLAYING:
		return "游戏中"
	case pb.RoomStatus_FINISHED:
		return "游戏结束"
	default:
		return "未知"
	}
}

func (c *GomokuClient) displayRoomList(rooms []*pb.RoomInfo, totalCount int32) {
	fmt.Printf("\n=== 房间列表 (共%d个房间) ===\n", totalCount)
	
	if len(rooms) == 0 {
		fmt.Println("暂无房间")
		return
	}
	
	fmt.Println("序号 | 房间ID   | 房间名          | 状态     | 玩家数")
	fmt.Println("-----|----------|-----------------|----------|--------")
	
	for i, room := range rooms {
		fmt.Printf("%-4d | %-8s | %-15s | %-8s | %d/2\n",
			i+1,
			room.RoomId,
			truncateString(room.RoomName, 15),
			getRoomStatusName(room.Status),
			len(room.Players))
	}
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func (c *GomokuClient) showHelp() {
	fmt.Println(`
=== 五子棋客户端帮助 ===

连接命令:
  help, h         - 显示此帮助信息
  quit, q         - 退出程序
  status          - 显示连接状态

房间管理:
  rooms           - 查看房间列表
  create <name>   - 创建房间 (例: create 我的房间)
  join <roomId>   - 加入房间 (例: join abc123)
  leave           - 离开当前房间
  room            - 显示当前房间信息

游戏操作:
  start           - 开始游戏 (仅房主可用)
  ready           - 准备/取消准备
  move <x> <y>    - 下棋，坐标范围0-14 (例: move 7 7)
  board           - 显示棋盘
  surrender       - 认输

示例游戏流程:
  1. rooms              # 查看房间列表
  2. create 测试房间    # 创建房间
  3. 等待其他玩家: join <roomId>
  4. ready              # 准备
  5. start              # 房主开始游戏
  6. move 7 7           # 下棋
  7. board              # 查看棋盘

坐标说明: 棋盘左上角为(0,0)，右下角为(14,14)
`)
}

func (c *GomokuClient) showStatus() {
	fmt.Printf("\n=== 客户端状态 ===\n")
	fmt.Printf("玩家ID: %s\n", c.playerID)
	fmt.Printf("连接状态: %s\n", func() string {
		if c.connected {
			return "已连接"
		}
		return "未连接"
	}())
	
	if c.currentRoomID != "" {
		fmt.Printf("当前房间: %s\n", c.currentRoomID)
	} else {
		fmt.Printf("当前房间: 无\n")
	}
}

func (c *GomokuClient) showSuccess(message string) {
	fmt.Printf("✓ %s\n", message)
}

func (c *GomokuClient) showError(message string) {
	fmt.Printf("✗ %s\n", message)
}

func (c *GomokuClient) showInfo(message string) {
	fmt.Printf("ℹ %s\n", message)
}

func (c *GomokuClient) showNotification(message string) {
	fmt.Printf("📢 %s\n", message)
}

func (c *GomokuClient) validateMove(args []string) (int32, int32, error) {
	if len(args) != 2 {
		return 0, 0, fmt.Errorf("使用方法: move <x> <y>，坐标范围0-14")
	}
	
	x, err := parseCoordinate(args[0])
	if err != nil {
		return 0, 0, fmt.Errorf("无效的X坐标: %s", args[0])
	}
	
	y, err := parseCoordinate(args[1])
	if err != nil {
		return 0, 0, fmt.Errorf("无效的Y坐标: %s", args[1])
	}
	
	return x, y, nil
}

func parseCoordinate(s string) (int32, error) {
	var coord int
	if _, err := fmt.Sscanf(s, "%d", &coord); err != nil {
		return 0, err
	}
	
	if coord < 0 || coord > 14 {
		return 0, fmt.Errorf("坐标必须在0-14之间")
	}
	
	return int32(coord), nil
}

func (c *GomokuClient) validateRoomName(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("房间名不能为空")
	}
	if len(name) > 32 {
		return fmt.Errorf("房间名长度不能超过32个字符")
	}
	return nil
}
