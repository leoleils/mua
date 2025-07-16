#!/usr/bin/env python3
"""
调试心跳消息的结构
"""

import time
from common_pb2 import GameMessage, HeadMessage, MessageType

def create_heartbeat_message():
    """创建心跳消息"""
    heartbeat = GameMessage()
    heartbeat.msg_head.player_id = "test_player_123"
    heartbeat.msg_head.service_name = "gatesvr"
    heartbeat.msg_head.request_id = "initial_heartbeat"
    heartbeat.msg_head.timestamp = int(time.time() * 1000)
    heartbeat.msg_type = MessageType.HEARTBEAT
    
    return heartbeat

def debug_message():
    """调试消息内容"""
    msg = create_heartbeat_message()
    
    print("心跳消息内容:")
    print(f"  msg_head.player_id: '{msg.msg_head.player_id}'")
    print(f"  msg_head.service_name: '{msg.msg_head.service_name}'")
    print(f"  msg_head.request_id: '{msg.msg_head.request_id}'")
    print(f"  msg_head.timestamp: {msg.msg_head.timestamp}")
    print(f"  msg_type: {msg.msg_type} ({MessageType.HEARTBEAT})")
    print(f"  payload: '{msg.payload}'")
    
    # 序列化测试
    data = msg.SerializeToString()
    print(f"\n序列化结果:")
    print(f"  字节长度: {len(data)}")
    print(f"  前20字节: {data[:20]}")
    
    # 反序列化测试
    msg2 = GameMessage()
    msg2.ParseFromString(data)
    print(f"\n反序列化测试:")
    print(f"  player_id: '{msg2.msg_head.player_id}'")
    print(f"  msg_type: {msg2.msg_type}")
    print(f"  序列化/反序列化成功: {msg2.msg_head.player_id == msg.msg_head.player_id}")

if __name__ == "__main__":
    debug_message()
