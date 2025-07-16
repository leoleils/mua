#!/usr/bin/env python3
"""
简单的连接测试脚本

测试TCP连接到gatesvr并发送简单的消息
"""

import sys
import logging
from tcp_client import TCPClient
from message_handler import MessageHandler

def test_connection():
    """测试连接功能"""
    
    # 设置日志
    logging.basicConfig(level=logging.INFO)
    
    print("开始测试连接...")
    
    # 创建客户端
    client = TCPClient("192.168.0.109", 6001)
    handler = MessageHandler("test_player", client)
    
    # 设置消息处理器
    client.set_message_handler(handler.handle_message)
    
    # 尝试连接
    if client.connect():
        print("✓ 连接成功")
        
        # 发送心跳
        try:
            client.send_heartbeat()
            print("✓ 心跳发送成功")
        except Exception as e:
            print(f"✗ 心跳发送失败: {e}")
        
        # 等待一段时间
        import time
        time.sleep(2)
        
        # 断开连接
        client.disconnect()
        print("✓ 连接断开")
        
    else:
        print("✗ 连接失败")
        print("请确保：")
        print("1. gatesvr正在运行")
        print("2. 监听端口6001")
        print("3. 地址192.168.0.109可访问")

if __name__ == "__main__":
    test_connection()
