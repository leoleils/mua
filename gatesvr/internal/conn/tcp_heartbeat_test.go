package conn

import (
	"log"
	"mua/gatesvr/internal/pb"
	"net"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"
)

// 请根据实际服务端监听端口修改
const testTCPAddr = "127.0.0.1:6001"

func TestTCPHeartbeatKeepAlive(t *testing.T) {
	conn, err := net.Dial("tcp", testTCPAddr)
	if err != nil {
		t.Fatalf("连接失败: %v", err)
	}
	defer conn.Close()

	// 1. 发送认证消息（首包）
	loginMsg := &pb.GameMessage{
		MsgHead: &pb.HeadMessage{
			PlayerId:   "test_player",
			Token:      "test_token",
			ClientType: 1, // 假设为android
		},
		MsgType: 0, // 认证
		Payload: []byte("login payload"),
	}
	data, _ := proto.Marshal(loginMsg)
	length := uint32(len(data))
	lenBuf := []byte{byte(length), byte(length >> 8), byte(length >> 16), byte(length >> 24)}
	_, err = conn.Write(append(lenBuf, data...))
	if err != nil {
		t.Fatalf("发送认证消息失败: %v", err)
	}

	// 新增：启动一个 goroutine 持续读取服务端消息
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := conn.Read(buf)
			if err != nil {
				log.Printf("连接被关闭或读取出错: %v", err)
				return
			}
			if n > 0 {
				log.Printf("收到服务端消息: %x", buf[:n])
			}
		}
	}()

	// 2. 发送心跳包，持续5分钟
	heartbeatMsg := &pb.GameMessage{
		MsgHead: &pb.HeadMessage{
			PlayerId: "test_player",
		},
		MsgType: 0, // 心跳
		Payload: []byte("heartbeat"),
	}
	heartbeatData, _ := proto.Marshal(heartbeatMsg)
	length = uint32(len(heartbeatData))
	lenBuf = []byte{byte(length), byte(length >> 8), byte(length >> 16), byte(length >> 24)}

	timeout := time.After(5 * time.Minute)
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			t.Log("心跳保持5分钟，测试通过")
			return
		case <-ticker.C:
			_, err := conn.Write(append(lenBuf, heartbeatData...))
			if err != nil {
				t.Fatalf("心跳发送失败: %v", err)
			}
			log.Println("心跳包已发送")
		}
	}
}
