package conn

import "time"

// ConnAdapter 定义了连接适配器的通用接口，屏蔽底层 TCP、WebSocket 等不同连接类型的差异，
// 便于上层统一处理消息收发、连接管理等逻辑。
type ConnAdapter interface {
	// ReadMessage 读取一条消息。
	// 返回消息类型（如 WebSocket 的 Text/Binary）、消息数据和错误信息。
	// 若连接已关闭或发生错误，err 不为 nil。
	ReadMessage() (msgType int, data []byte, err error)

	// WriteMessage 向连接写入一条消息。
	// msgType 指定消息类型，data 为消息内容。
	// 若写入失败，返回错误。
	WriteMessage(msgType int, data []byte) error

	// SetReadDeadline 设置下一次读操作的超时时间。
	// 超时后 ReadMessage 会返回超时错误。
	SetReadDeadline(t time.Time) error

	// Close 关闭连接，释放相关资源。
	// 多次调用应保证幂等。
	Close() error

	// RemoteIP 返回远端客户端的 IP 地址（字符串形式）。
	RemoteIP() string
}
