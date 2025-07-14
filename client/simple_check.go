package main

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"time"

	"mua/client/pb"
	"google.golang.org/protobuf/proto"
)

func main() {
	// 连接到gatesvr
	conn, err := net.Dial("tcp", "localhost:6001")
	if err != nil {
		fmt.Printf("❌ 连接失败: %v\n", err)
		return
	}
	defer conn.Close()
	fmt.Println("✅ 已连接到gatesvr")

	// 发送心跳认证
	heartbeat := &pb.GameMessage{
		MsgHead: &pb.HeadMessage{
			PlayerId:    "test_player",
			ServiceName: "gomokusvr",
			Group:       "DEFAULT_GROUP",
			Timestamp:   time.Now().UnixMilli(),
		},
		MsgType: pb.MessageType_HEARTBEAT,
		Payload: []byte{},
	}

	if err := sendMessage(conn, heartbeat); err != nil {
		fmt.Printf("❌ 发送心跳失败: %v\n", err)
		return
	}
	fmt.Println("✅ 已发送心跳")

	// 接收心跳响应
	if _, err := receiveMessage(conn); err != nil {
		fmt.Printf("❌ 接收心跳响应失败: %v\n", err)
		return
	}
	fmt.Println("✅ 收到心跳响应，认证成功")

	// 发送获取房间列表请求
	listReq := map[string]interface{}{
		"page": 1,
		"page_size": 10,
	}
	payload, _ := json.Marshal(listReq)

	serviceMsg := &pb.GameMessage{
		MsgHead: &pb.HeadMessage{
			PlayerId:       "test_player",
			ServiceName:    "gomokusvr",
			Group:          "DEFAULT_GROUP",
			RequestId:      "GetRoomList",
			Timestamp:      time.Now().UnixMilli(),
			ServiceMsgType: pb.ServiceMessageType_SYNC,
		},
		MsgType: pb.MessageType_SERVICE_MESSAGE,
		Payload: payload,
	}

	if err := sendMessage(conn, serviceMsg); err != nil {
		fmt.Printf("❌ 发送获取房间列表请求失败: %v\n", err)
		return
	}
	fmt.Println("✅ 已发送获取房间列表请求")

	// 接收响应
	response, err := receiveMessage(conn)
	if err != nil {
		fmt.Printf("❌ 接收响应失败: %v\n", err)
		return
	}

	fmt.Printf("✅ 收到响应: 类型=%v, 负载大小=%d字节\n", response.MsgType, len(response.Payload))

	// 解析服务响应
	if response.MsgType == pb.MessageType_SERVICE_MESSAGE {
		var serviceResp pb.GameMessageResponse
		if err := proto.Unmarshal(response.Payload, &serviceResp); err == nil {
			fmt.Printf("✅ 服务响应解析成功: Ret=%d\n", serviceResp.Ret)
			if serviceResp.Ret == 0 {
				fmt.Println("🎉 测试成功！gatesvr → gomokusvr 通信正常")
			} else {
				fmt.Printf("⚠️  服务返回错误: %d\n", serviceResp.Ret)
			}
		} else {
			fmt.Printf("❌ 解析服务响应失败: %v\n", err)
		}
	}

	fmt.Println("测试完成")
}

func sendMessage(conn net.Conn, msg *pb.GameMessage) error {
	data, err := proto.Marshal(msg)
	if err != nil {
		return err
	}

	length := uint32(len(data))
	lenBuf := make([]byte, 4)
	binary.LittleEndian.PutUint32(lenBuf, length)

	if _, err := conn.Write(lenBuf); err != nil {
		return err
	}
	_, err := conn.Write(data)
	return err
}

func receiveMessage(conn net.Conn) (*pb.GameMessage, error) {
	lenBuf := make([]byte, 4)
	if _, err := io.ReadFull(conn, lenBuf); err != nil {
		return nil, err
	}

	length := binary.LittleEndian.Uint32(lenBuf)
	data := make([]byte, length)
	if _, err := io.ReadFull(conn, data); err != nil {
		return nil, err
	}

	var msg pb.GameMessage
	if err := proto.Unmarshal(data, &msg); err != nil {
		return nil, err
	}
	return &msg, nil
}
