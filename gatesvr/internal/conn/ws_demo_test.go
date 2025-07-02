package conn

import (
	"mua/gatesvr/internal/pb"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"google.golang.org/protobuf/proto"
)

func TestWSDemo(t *testing.T) {
	url := "ws://127.0.0.1:6002/ws"
	c, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("WebSocket连接失败: %v", err)
	}
	defer c.Close()

	msg := &pb.GameMessage{
		MsgHead: &pb.HeadMessage{
			PlayerId:   "demo-ws-player",
			ClientType: 5,
		},
		MsgType: 1,
		Payload: []byte("hello ws"),
	}
	data, err := proto.Marshal(msg)
	if err != nil {
		t.Fatalf("protobuf序列化失败: %v", err)
	}
	if err := c.WriteMessage(websocket.BinaryMessage, data); err != nil {
		t.Fatalf("WebSocket发送失败: %v", err)
	}
	c.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, resp, err := c.ReadMessage()
	if err != nil {
		t.Logf("WebSocket无响应或已关闭: %v", err)
	} else {
		t.Logf("WebSocket收到响应: %x", resp)
	}
}
