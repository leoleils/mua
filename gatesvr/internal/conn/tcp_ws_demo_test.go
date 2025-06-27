package conn

import (
	_"log"
	"net"
	"testing"
	"time"
	_"bytes"
	"encoding/binary"
	"github.com/gorilla/websocket"
	"google.golang.org/protobuf/proto"
	"mua/gatesvr/internal/pb"
)

func TestTCPDemo(t *testing.T) {
	addr := "127.0.0.1:6001"
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("TCP连接失败: %v", err)
	}
	defer conn.Close()

	msg := &pb.GameMessage{
		PlayerId:  "demo-tcp-player",
		MsgType:   1,
		Payload:   []byte("hello tcp"),
		ClientType: 3,
	}
	data, err := proto.Marshal(msg)
	if err != nil {
		t.Fatalf("protobuf序列化失败: %v", err)
	}
	// 发送长度前缀+数据
	lenBuf := make([]byte, 4)
	binary.LittleEndian.PutUint32(lenBuf, uint32(len(data)))
	if _, err := conn.Write(append(lenBuf, data...)); err != nil {
		t.Fatalf("TCP发送失败: %v", err)
	}
	// 简单等待服务器响应
	buf := make([]byte, 1024)
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, err := conn.Read(buf)
	if err != nil {
		t.Logf("TCP无响应或已关闭: %v", err)
	} else {
		t.Logf("TCP收到响应: %x", buf[:n])
	}
}

func TestWSDemo(t *testing.T) {
	url := "ws://127.0.0.1:6002/ws"
	c, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("WebSocket连接失败: %v", err)
	}
	defer c.Close()

	msg := &pb.GameMessage{
		PlayerId:  "demo-ws-player",
		MsgType:   1,
		Payload:   []byte("hello ws"),
		ClientType: 5,
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