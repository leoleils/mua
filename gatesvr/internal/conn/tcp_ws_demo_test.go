package conn

import (
	_ "bytes"
	"encoding/binary"
	_ "log"
	"mua/gatesvr/internal/pb"
	"net"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"
)

func TestTCPDemo(t *testing.T) {
	addr := "127.0.0.1:6001"
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("TCP连接失败: %v", err)
	}
	defer conn.Close()

	msg := &pb.GameMessage{
		MsgHead: &pb.HeadMessage{
			PlayerId:   "demo-tcp-player",
			ClientType: 3,
		},
		MsgType: 1,
		Payload: []byte("hello tcp"),
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

func TestTCPLoginAndHeartbeat(t *testing.T) {
	addr := "127.0.0.1:6001"
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("TCP连接失败: %v", err)
	}
	defer conn.Close()

	playerID := "demo-tcp-player"
	clientType := 3
	// 1. 发送登录（认证）消息 MsgType=100
	loginMsg := &pb.GameMessage{
		MsgHead: &pb.HeadMessage{
			PlayerId:   playerID,
			ClientType: int32(clientType),
		},
		MsgType: 100,
		Payload: []byte("login-payload"),
	}
	loginData, err := proto.Marshal(loginMsg)
	if err != nil {
		t.Fatalf("protobuf序列化失败: %v", err)
	}
	lenBuf := make([]byte, 4)
	binary.LittleEndian.PutUint32(lenBuf, uint32(len(loginData)))
	if _, err := conn.Write(append(lenBuf, loginData...)); err != nil {
		t.Fatalf("TCP发送登录失败: %v", err)
	}
	// 2. 等待令牌响应 MsgType=101
	buf := make([]byte, 1024)
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, err := conn.Read(buf)
	if err != nil {
		t.Fatalf("TCP未收到认证响应: %v", err)
	}
	var resp pb.GameMessage
	if err := proto.Unmarshal(buf[4:n], &resp); err != nil {
		t.Fatalf("认证响应反序列化失败: %v", err)
	}
	if resp.MsgType != 101 {
		t.Fatalf("认证响应MsgType错误: %d", resp.MsgType)
	}
	t.Logf("收到令牌: %s", string(resp.Payload))
	// 3. 持续发送心跳 MsgType=0，10分钟
	ticker := time.NewTicker(30 * time.Second)
	end := time.After(10 * time.Minute)
	for {
		select {
		case <-ticker.C:
			heartbeat := &pb.GameMessage{
				MsgHead: &pb.HeadMessage{
					PlayerId:   playerID,
					ClientType: int32(clientType),
				},
				MsgType: 0,
			}
			hbData, _ := proto.Marshal(heartbeat)
			lenBuf := make([]byte, 4)
			binary.LittleEndian.PutUint32(lenBuf, uint32(len(hbData)))
			if _, err := conn.Write(append(lenBuf, hbData...)); err != nil {
				t.Fatalf("心跳发送失败: %v", err)
			}
			t.Logf("心跳已发送")
		case <-end:
			t.Logf("心跳测试结束")
			return
		}
	}
}
