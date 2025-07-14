package main

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"time"

	"mua/client/pb"
	"google.golang.org/protobuf/proto"
)

func sendMessage(conn net.Conn, msg *pb.GameMessage) error {
	data, err := proto.Marshal(msg)
	if err != nil {
		return err
	}

	// 发送长度头
	length := uint32(len(data))
	lenBuf := make([]byte, 4)
	binary.LittleEndian.PutUint32(lenBuf, length)

	if _, err := conn.Write(lenBuf); err != nil {
		return err
	}

	if _, err := conn.Write(data); err != nil {
		return err
	}

	return nil
}

func receiveMessage(conn net.Conn) (*pb.GameMessage, error) {
	// 读取消息长度
	lenBuf := make([]byte, 4)
	if _, err := io.ReadFull(conn, lenBuf); err != nil {
		return nil, err
	}

	length := binary.LittleEndian.Uint32(lenBuf)
	
	// 读取消息体
	data := make([]byte, length)
	if _, err := io.ReadFull(conn, data); err != nil {
		return nil, err
	}

	// 反序列化消息
	var msg pb.GameMessage
	if err := proto.Unmarshal(data, &msg); err != nil {
		return nil, err
	}

	return &msg, nil
}

func main() {
	fmt.Println("=== 简单测试客户端 ===")

	// 连接到服务器
	if err != nil {
		log.Fatalf("连接失败: %v", err)
	}

	fmt.Println("已连接到服务器")

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
		log.Fatalf("发送心跳失败: %v", err)
	}

	fmt.Println("已发送心跳")

	// 接收心跳响应
	if _, err := receiveMessage(conn); err != nil {
		log.Fatalf("接收心跳响应失败: %v", err)
	}

	fmt.Println("收到心跳响应，认证成功")

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
		log.Fatalf("发送获取房间列表请求失败: %v", err)
	}

	fmt.Println("已发送获取房间列表请求")

	// 接收响应
	response, err := receiveMessage(conn)
	if err != nil {
		log.Fatalf("接收响应失败: %v", err)
	}

	fmt.Printf("收到响应: 类型=%v, 负载大小=%d字节\n", response.MsgType, len(response.Payload))

	// 如果是服务消息响应，解析它
	if response.MsgType == pb.MessageType_SERVICE_MESSAGE {
		var serviceResp pb.GameMessageResponse
		if err := proto.Unmarshal(response.Payload, &serviceResp); err == nil {
			fmt.Printf("服务响应: Ret=%d\n", serviceResp.Ret)
			if serviceResp.Payload != nil {
				switch p := serviceResp.Payload.(type) {
				case *pb.GameMessageResponse_Data:
					fmt.Printf("响应数据大小: %d字节\n", len(p.Data))
					// 尝试解析房间列表
					var roomList map[string]interface{}
					if err := json.Unmarshal(p.Data, &roomList); err == nil {
						fmt.Printf("房间列表: %v\n", roomList)
					}
				case *pb.GameMessageResponse_Reason:
					fmt.Printf("错误原因: %s\n", p.Reason)
				}
			}
		}
	}

	fmt.Println("测试完成")
}
