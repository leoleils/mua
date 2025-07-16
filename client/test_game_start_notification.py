#!/usr/bin/env python3
"""
测试房主开始游戏通知功能
"""

import time
import threading
from gomoku_client import GomokuClient

def test_game_start_notification():
    """测试房主开始游戏通知功能"""
    
    print("🎮 测试房主开始游戏通知功能")
    print("=" * 50)
    
    # 创建两个客户端（房主和玩家）
    owner_client = GomokuClient()
    player_client = GomokuClient()
    
    try:
        # 连接到服务器
        print("\n1. 连接服务器...")
        owner_connected = owner_client.connect("owner_123", "")
        player_connected = player_client.connect("player_456", "")
        
        if not owner_connected or not player_connected:
            print("❌ 连接失败")
            return
        
        print("✅ 两个客户端都已连接")
        
        # 等待连接稳定
        time.sleep(1)
        
        # 房主创建房间
        print("\n2. 房主创建房间...")
        owner_client.create_room("测试房间", "")
        time.sleep(1)
        
        # 玩家加入房间
        print("\n3. 玩家加入房间...")
        player_client.join_room(owner_client.current_room_id, "")
        time.sleep(1)
        
        # 两个玩家都准备
        print("\n4. 玩家准备...")
        owner_client.player_ready()
        time.sleep(0.5)
        player_client.player_ready()
        time.sleep(1)
        
        # 房主开始游戏 - 这里会触发GameStartNotification
        print("\n5. 房主开始游戏...")
        print("   🎯 即将触发GameStartNotification通知")
        owner_client.start_game()
        
        # 等待通知处理
        time.sleep(2)
        
        print("\n✅ 测试完成！")
        print("📝 如果看到上面的'🎮 游戏开始通知!'消息，说明通知功能正常工作")
        
    except Exception as e:
        print(f"❌ 测试异常: {e}")
    finally:
        # 断开连接
        owner_client.disconnect()
        player_client.disconnect()

if __name__ == "__main__":
    test_game_start_notification() 