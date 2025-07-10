package conn

import (
	"io"
	"net"
	"time"
)

type TCPConnAdapter struct {
	conn net.Conn
	ip   string
}

// NewTCPConnAdapter 创建TCP连接适配器
func NewTCPConnAdapter(conn net.Conn) *TCPConnAdapter {
	ip := conn.RemoteAddr().String()
	return &TCPConnAdapter{conn: conn, ip: ip}
}

// readFull 确保完整读取指定长度的数据
func (a *TCPConnAdapter) readFull(buf []byte) error {
	_, err := io.ReadFull(a.conn, buf)
	return err
}

func (a *TCPConnAdapter) ReadMessage() (int, []byte, error) {
	// 1. 读取消息长度（4字节）
	lenBuf := make([]byte, 4)
	if err := a.readFull(lenBuf); err != nil {
		return 0, nil, err
	}

	// 2. 解析消息长度（小端序）
	msgLen := int(lenBuf[0]) | int(lenBuf[1])<<8 | int(lenBuf[2])<<16 | int(lenBuf[3])<<24

	// 3. 验证消息长度的合理性
	if msgLen <= 0 {
		return 0, nil, io.ErrUnexpectedEOF
	}
	if msgLen > 10*1024 { // 10KB 限制
		return 0, nil, io.ErrShortBuffer
	}

	// 4. 读取完整的消息内容
	data := make([]byte, msgLen)
	if err := a.readFull(data); err != nil {
		return 0, nil, err
	}

	return 2, data, nil // 2 代表自定义的"二进制消息"
}

func (a *TCPConnAdapter) WriteMessage(msgType int, data []byte) error {
	// 构造长度头（小端序）
	length := len(data)
	lenBuf := []byte{
		byte(length),
		byte(length >> 8),
		byte(length >> 16),
		byte(length >> 24),
	}

	// 一次性写入长度头和数据
	fullData := append(lenBuf, data...)
	_, err := a.conn.Write(fullData)
	return err
}

func (a *TCPConnAdapter) SetReadDeadline(t time.Time) error {
	return a.conn.SetReadDeadline(t)
}

func (a *TCPConnAdapter) Close() error {
	return a.conn.Close()
}

func (a *TCPConnAdapter) RemoteIP() string {
	return a.ip
}
