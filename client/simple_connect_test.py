#!/usr/bin/env python3
"""
简单连接测试
"""

import socket
import struct
import time
from common_pb2 import GameMessage, MessageType

def test_simple_connect():
    print("=" * 50)
    print("简单连接测试")
    print("=" * 50)
    
    try:
        # 创建socket连接
        sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        sock.settimeout(10)
        
        print("连接到服务器...")
        sock.connect(("127.0.0.1", 6001))
        print("✓ TCP连接成功")
        
        # 创建心跳消息
        heartbeat = GameMessage()
        heartbeat.msg_head.player_id = "test_player_123"
        heartbeat.msg_head.service_name = "gatesvr"
        heartbeat.msg_head.request_id = "initial_heartbeat"
        heartbeat.msg_head.timestamp = int(time.time() * 1000)
        heartbeat.msg_type = MessageType.HEARTBEAT
        
        # 序列化消息
        data = heartbeat.SerializeToString()
        print(f"✓ 消息序列化成功，长度: {len(data)} 字节")
        
        # 发送消息长度（小端序，与gatesvr一致）
        length = struct.pack('<I', len(data))
        sock.sendall(length)
        print("✓ 发送消息长度成功")
        
        # 发送消息内容
        sock.sendall(data)
        print("✓ 发送心跳消息成功")
        
        # 尝试接收响应
        print("等待服务器响应...")
        sock.settimeout(5)
        
        try:
            response_length_data = sock.recv(4)
            if response_length_data:
                response_length = struct.unpack('<I', response_length_data)[0]
                print(f"✓ 收到响应长度: {response_length}")
                
                response_data = sock.recv(response_length)
                if response_data:
                    print(f"✓ 收到响应数据: {len(response_data)} 字节")
                    
                    # 尝试解析响应
                    response_msg = GameMessage()
                    response_msg.ParseFromString(response_data)
                    print(f"✓ 响应消息类型: {response_msg.msg_type}")
                    
            else:
                print("✗ 没有收到响应")
                
        except socket.timeout:
            print("✗ 等待响应超时")
        except Exception as e:
            print(f"✗ 接收响应时出错: {e}")
        
        # 保持连接一段时间
        print("保持连接30秒...")
        time.sleep(30)
        
    except Exception as e:
        print(f"✗ 连接测试失败: {e}")
        import traceback
        traceback.print_exc()
    finally:
        sock.close()
        print("连接已关闭")

if __name__ == "__main__":
    test_simple_connect()
