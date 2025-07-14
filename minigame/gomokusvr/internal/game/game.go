package game

import (
	"fmt"
	"time"

	"mua/minigame/gomokusvr/internal/pb"
)

const (
	BoardSize = 15 // 棋盘大小 15x15
	WinCount  = 5  // 连成5子获胜
)

// Game 五子棋游戏
type Game struct {
	Board       [BoardSize][BoardSize]int32 `json:"board"`        // 棋盘 0=空 1=黑子 2=白子
	CurrentTurn pb.PlayerColor              `json:"current_turn"` // 当前轮次
	Result      pb.GameResult               `json:"result"`       // 游戏结果
	WinnerID    string                      `json:"winner_id"`    // 获胜者ID
	TotalMoves  int32                       `json:"total_moves"`  // 总步数
	MoveHistory []*pb.Move                  `json:"move_history"` // 走棋历史
}

// NewGame 创建新游戏
func NewGame() *Game {
	return &Game{
		Board:       [BoardSize][BoardSize]int32{},
		CurrentTurn: pb.PlayerColor_BLACK, // 黑子先手
		Result:      pb.GameResult_ONGOING,
		WinnerID:    "",
		TotalMoves:  0,
		MoveHistory: make([]*pb.Move, 0),
	}
}

// PlacePiece 下棋
func (g *Game) PlacePiece(playerID string, x, y int32) error {
	// 检查坐标有效性
	if x < 0 || x >= BoardSize || y < 0 || y >= BoardSize {
		return fmt.Errorf("坐标超出范围: (%d, %d)", x, y)
	}

	// 检查位置是否为空
	if g.Board[x][y] != 0 {
		return fmt.Errorf("位置 (%d, %d) 已有棋子", x, y)
	}

	// 检查游戏是否已结束
	if g.Result != pb.GameResult_ONGOING {
		return fmt.Errorf("游戏已结束")
	}

	// 下棋
	pieceColor := int32(g.CurrentTurn)
	g.Board[x][y] = pieceColor
	g.TotalMoves++

	// 记录走棋历史
	move := &pb.Move{
		PlayerId:   playerID,
		Color:      g.CurrentTurn,
		X:          x,
		Y:          y,
		MoveNumber: g.TotalMoves,
		Timestamp:  time.Now().UnixMilli(),
	}
	g.MoveHistory = append(g.MoveHistory, move)

	// 检查胜负
	if g.checkWin(x, y, pieceColor) {
		if g.CurrentTurn == pb.PlayerColor_BLACK {
			g.Result = pb.GameResult_BLACK_WIN
		} else {
			g.Result = pb.GameResult_WHITE_WIN
		}
		g.WinnerID = playerID
	} else if g.TotalMoves >= BoardSize*BoardSize {
		// 棋盘下满，平局
		g.Result = pb.GameResult_DRAW
	} else {
		// 切换回合
		if g.CurrentTurn == pb.PlayerColor_BLACK {
			g.CurrentTurn = pb.PlayerColor_WHITE
		} else {
			g.CurrentTurn = pb.PlayerColor_BLACK
		}
	}

	return nil
}

// checkWin 检查是否获胜
func (g *Game) checkWin(x, y int32, color int32) bool {
	// 四个方向：水平、垂直、左斜、右斜
	directions := [][2]int32{
		{0, 1},  // 水平
		{1, 0},  // 垂直
		{1, 1},  // 左斜
		{1, -1}, // 右斜
	}

	for _, dir := range directions {
		count := 1 // 包含当前下的棋子
		dx, dy := dir[0], dir[1]

		// 正方向计算
		nx, ny := x+dx, y+dy
		for nx >= 0 && nx < BoardSize && ny >= 0 && ny < BoardSize && g.Board[nx][ny] == color {
			count++
			nx += dx
			ny += dy
		}

		// 反方向计算
		nx, ny = x-dx, y-dy
		for nx >= 0 && nx < BoardSize && ny >= 0 && ny < BoardSize && g.Board[nx][ny] == color {
			count++
			nx -= dx
			ny -= dy
		}

		// 检查是否连成5子
		if count >= WinCount {
			return true
		}
	}

	return false
}

// GetGameState 获取游戏状态
func (g *Game) GetGameState() *pb.GameState {
	// 将二维棋盘转换为一维数组
	board := make([]int32, BoardSize*BoardSize)
	for i := 0; i < BoardSize; i++ {
		for j := 0; j < BoardSize; j++ {
			board[i*BoardSize+j] = g.Board[i][j]
		}
	}

	return &pb.GameState{
		Board:       board,
		CurrentTurn: g.CurrentTurn,
		Result:      g.Result,
		WinnerId:    g.WinnerID,
		TotalMoves:  g.TotalMoves,
		MoveHistory: g.MoveHistory,
	}
}

// SetGameState 设置游戏状态（用于恢复游戏）
func (g *Game) SetGameState(state *pb.GameState) {
	// 将一维数组转换为二维棋盘
	for i := 0; i < BoardSize; i++ {
		for j := 0; j < BoardSize; j++ {
			if i*BoardSize+j < len(state.Board) {
				g.Board[i][j] = state.Board[i*BoardSize+j]
			}
		}
	}

	g.CurrentTurn = state.CurrentTurn
	g.Result = state.Result
	g.WinnerID = state.WinnerId
	g.TotalMoves = state.TotalMoves
	g.MoveHistory = state.MoveHistory
}

// IsValidMove 检查是否是有效的走棋
func (g *Game) IsValidMove(x, y int32) bool {
	if x < 0 || x >= BoardSize || y < 0 || y >= BoardSize {
		return false
	}
	if g.Board[x][y] != 0 {
		return false
	}
	if g.Result != pb.GameResult_ONGOING {
		return false
	}
	return true
}

// GetLastMove 获取最后一步棋
func (g *Game) GetLastMove() *pb.Move {
	if len(g.MoveHistory) == 0 {
		return nil
	}
	return g.MoveHistory[len(g.MoveHistory)-1]
}

// IsGameOver 游戏是否结束
func (g *Game) IsGameOver() bool {
	return g.Result != pb.GameResult_ONGOING
}

// GetPieceAt 获取指定位置的棋子
func (g *Game) GetPieceAt(x, y int32) int32 {
	if x < 0 || x >= BoardSize || y < 0 || y >= BoardSize {
		return 0
	}
	return g.Board[x][y]
}
