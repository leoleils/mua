package conn

import "time"

type ConnAdapter interface {
	ReadMessage() (msgType int, data []byte, err error)
	WriteMessage(msgType int, data []byte) error
	SetReadDeadline(t time.Time) error
	Close() error
	RemoteIP() string
}
