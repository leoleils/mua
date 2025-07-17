#!/usr/bin/env python3
"""
测试下棋通知功能
验证PiecePlacedNotification是否能正确解析并显示下棋位置信息
"""

import sys
import time
import logging
from tcp_client import TCPClient
from message_handler import MessageHandler

def test_piece_notification():
    """测试下棋通知功能"""
    
    # 设置日志
    logging.basicConfig(
        level=logging.INFO,
        format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
    )
    
    print("🧪 测试下棋通知功能")
    print("=" * 50)
    
    # 创建消息处理器
    message_handler = MessageHandler()
    
    # 创建TCP客户端
    client = TCPClient()
    client.set_message_handler(message_handler)
    
    # 添加自定义下棋通知处理器
    def handle_piece_notification(move_info):
        print("\n🎯 收到下棋通知处理器回调！")
        print(f"📍 位置: ({move_info.get('x', '?')}, {move_info.get('y', '?')})")
        print(f"🎮 玩家: {move_info.get('player_id', '未知')}")
        print(f"🔢 步数: {move_info.get('move_number', '?')}")
        print(f"⚫⚪ 颜色: {'⚫' if move_info.get('color') == 1 else '⚪' if move_info.get('color') == 2 else '❓'}")
        print(f"🎯 轮次: {'⚫' if move_info.get('current_turn') == 1 else '⚪' if move_info.get('current_turn') == 2 else '❓'}")
        print(f"📊 总步数: {move_info.get('total_moves', '?')}")
        
    message_handler.set_system_handler("piece_placed_notification", handle_piece_notification)
    
    # 连接服务器
    try:
        print("🔌 连接服务器...")
        if not client.connect("127.0.0.1", 8080):
            print("❌ 连接失败")
            return False
            
        print("✅ 连接成功")
        
        # 获取用户认证信息
        username = input("👤 请输入用户名: ").strip()
        password = input("🔑 请输入密码: ").strip()
        
        # 认证
        print("🔐 进行认证...")
        auth_result = message_handler.authenticate(username, password)
        print(f"🔐 认证结果: {auth_result}")
        
        # 等待认证响应
        time.sleep(1)
        
        # 获取房间信息
        room_id = input("🏠 请输入房间ID (或输入'create'创建新房间): ").strip()
        
        if room_id.lower() == 'create':
            room_name = input("🏠 请输入房间名称: ").strip()
            print(f"🏗️ 创建房间: {room_name}")
            message_handler.create_room(room_name, "")
            time.sleep(1)
        else:
            print(f"🚪 加入房间: {room_id}")
            message_handler.join_room(room_id, "")
            time.sleep(1)
        
        print("\n⏳ 等待其他玩家加入和准备...")
        print("💡 在另一个客户端中加入同一房间并准备，然后开始游戏")
        print("🎮 当有人下棋时，将会显示详细的下棋通知信息")
        print("🛑 按 Ctrl+C 退出测试")
        
        # 持续监听消息
        while True:
            try:
                time.sleep(1)
            except KeyboardInterrupt:
                print("\n\n🛑 收到退出信号")
                break
                
    except Exception as e:
        print(f"❌ 测试过程中出错: {e}")
        return False
    finally:
        print("🔌 断开连接...")
        client.disconnect()
        
    print("✅ 测试完成")
    return True

if __name__ == "__main__":
    test_piece_notification() 