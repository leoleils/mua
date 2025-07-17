#!/usr/bin/env python3
"""
测试坐标映射修复
验证GUI点击坐标与后端API坐标的映射关系是否正确
"""

import sys
import time
import logging
from tcp_client import TCPClient
from message_handler import MessageHandler

def test_coordinate_mapping():
    """测试坐标映射修复"""
    
    # 设置日志
    logging.basicConfig(
        level=logging.INFO,
        format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
    )
    
    print("🧪 测试坐标映射修复")
    print("=" * 60)
    print("📋 测试目标:")
    print("   1. 验证GUI点击坐标正确传递给后端API")
    print("   2. 验证下棋通知中的坐标能正确显示在GUI")
    print("   3. 确保点击位置和显示位置一致")
    print("=" * 60)
    
    # 创建消息处理器
    message_handler = MessageHandler()
    
    # 创建TCP客户端
    client = TCPClient()
    client.set_message_handler(message_handler)
    
    # 添加自定义下棋通知处理器
    def handle_coordinate_test(move_info):
        print("\n" + "="*50)
        print("🎯 收到下棋通知 - 坐标验证")
        print("="*50)
        
        if isinstance(move_info, dict):
            x = move_info.get('x', -1)
            y = move_info.get('y', -1)
            player_id = move_info.get('player_id', '未知')
            color = move_info.get('color', 0)
            
            print(f"📍 API坐标: x={x}, y={y}")
            print(f"🎮 玩家: {player_id}")
            print(f"🔢 颜色: {'⚫' if color == 1 else '⚪' if color == 2 else '❓'}")
            
            # 验证坐标范围
            if 0 <= x <= 14 and 0 <= y <= 14:
                print(f"✅ 坐标有效: ({x}, {y})")
                
                # 根据后端坐标系统计算一维数组索引
                board_index = x * 15 + y
                print(f"📋 棋盘索引: {board_index} (应该在0-224范围内)")
                
                # 验证界面映射
                gui_row = x  # 后端x对应GUI行
                gui_col = y  # 后端y对应GUI列
                print(f"🖼️ GUI界面位置: row={gui_row}, col={gui_col}")
                
            else:
                print(f"❌ 坐标无效: ({x}, {y})")
        else:
            print("⚠️ 收到旧格式的下棋通知")
            
        print("="*50)
        
    message_handler.set_system_handler("piece_placed_notification", handle_coordinate_test)
    
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
        
        print("\n" + "="*60)
        print("🎮 坐标映射测试说明")
        print("="*60)
        print("📋 可以使用以下命令测试坐标:")
        print("   move <x> <y>  - 测试下棋坐标 (x=行，y=列，范围0-14)")
        print("   例如: move 7 7  - 在中心位置下棋")
        print("   例如: move 0 0  - 在左上角下棋")
        print("   例如: move 14 14 - 在右下角下棋")
        print("   quit - 退出测试")
        print("="*60)
        print("🔍 观察日志中的坐标映射信息:")
        print("   - 发送请求时的坐标转换")
        print("   - 接收通知时的坐标验证")
        print("="*60)
        
        # 命令行交互
        while True:
            try:
                command = input("\n🎯 请输入命令: ").strip()
                if not command:
                    continue
                    
                parts = command.split()
                cmd = parts[0].lower()
                
                if cmd == "quit":
                    break
                elif cmd == "move":
                    if len(parts) != 3:
                        print("❌ 用法: move <x> <y>")
                        print("   x: 行坐标 (0-14)")
                        print("   y: 列坐标 (0-14)")
                        continue
                    
                    try:
                        x = int(parts[1])
                        y = int(parts[2])
                        
                        if not (0 <= x <= 14 and 0 <= y <= 14):
                            print("❌ 坐标必须在0-14范围内")
                            continue
                            
                        print(f"\n📤 测试下棋: API坐标(x={x}, y={y})")
                        print(f"🎯 这对应棋盘位置: 行{x}, 列{y}")
                        
                        # 发送下棋请求（room_id需要从之前的操作中获取）
                        current_room = input("🏠 请输入当前房间ID: ").strip()
                        if current_room:
                            message_handler.place_piece(current_room, x, y)
                        else:
                            print("⚠️ 需要先加入房间才能下棋")
                            
                    except ValueError:
                        print("❌ 坐标必须是数字")
                else:
                    print("❌ 未知命令，请使用 'move <x> <y>' 或 'quit'")
                    
            except KeyboardInterrupt:
                print("\n\n🛑 收到退出信号")
                break
                
    except Exception as e:
        print(f"❌ 测试过程中出错: {e}")
        return False
    finally:
        print("🔌 断开连接...")
        client.disconnect()
        
    print("✅ 坐标映射测试完成")
    return True

if __name__ == "__main__":
    test_coordinate_mapping() 