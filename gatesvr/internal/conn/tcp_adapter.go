package conn

import (
	"net"
	"time"
)

type TCPConnAdapter struct {
	conn net.Conn
	ip   string
}

func NewTCPConnAdapter(conn net.Conn) *TCPConnAdapter {
	ip := conn.RemoteAddr().String()
	return &TCPConnAdapter{conn: conn, ip: ip}
}

func (a *TCPConnAdapter) ReadMessage() (int, []byte, error) {
	lenBuf := make([]byte, 4)
	_, err := a.conn.Read(lenBuf)
	if err != nil {
		return 0, nil, err
	}
	msgLen := int(lenBuf[0]) | int(lenBuf[1])<<8 | int(lenBuf[2])<<16 | int(lenBuf[3])<<24
	if msgLen <= 0 || msgLen > 10*1024 {
		return 0, nil, err
	}
	data := make([]byte, msgLen)
	_, err = a.conn.Read(data)
	if err != nil {
		return 0, nil, err
	}
	return 2, data, nil // 2 代表自定义的"二进制消息"
}

func (a *TCPConnAdapter) WriteMessage(msgType int, data []byte) error {
	lenBuf := []byte{byte(len(data)), byte(len(data) >> 8), byte(len(data) >> 16), byte(len(data) >> 24)}
	_, err := a.conn.Write(append(lenBuf, data...))
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
