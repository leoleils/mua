package conn

import (
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

type WSConnAdapter struct {
	conn *websocket.Conn
	ip   string
}

// NewWSConnAdapter 创建WebSocket连接适配器
func NewWSConnAdapter(conn *websocket.Conn, r *http.Request) *WSConnAdapter {
	ip := r.RemoteAddr
	return &WSConnAdapter{conn: conn, ip: ip}
}

func (a *WSConnAdapter) ReadMessage() (int, []byte, error) {
	return a.conn.ReadMessage()
}

func (a *WSConnAdapter) WriteMessage(msgType int, data []byte) error {
	return a.conn.WriteMessage(msgType, data)
}

func (a *WSConnAdapter) SetReadDeadline(t time.Time) error {
	return a.conn.SetReadDeadline(t)
}

func (a *WSConnAdapter) Close() error {
	return a.conn.Close()
}

func (a *WSConnAdapter) RemoteIP() string {
	return a.ip
}
