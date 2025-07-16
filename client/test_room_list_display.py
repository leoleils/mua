#!/usr/bin/env python3
"""
测试房间列表显示功能
"""

import tkinter as tk
from gomoku_gui import GomokuGUI
from gomoku_pb2 import GetRoomListResponse, RoomInfo, PlayerInfo, RoomStatus

def create_test_room_data():
    """创建测试房间数据"""
    
    # 创建测试房间列表响应
    room_list_response = GetRoomListResponse()
    room_list_response.success = True
    room_list_response.total_count = 3
    room_list_response.message = "获取房间列表成功"
    
    # 房间1：等待中，无密码
    room1 = RoomInfo()
    room1.room_id = "room_001"
    room1.room_name = "新手房间"
    room1.status = RoomStatus.WAITING
    room1.owner_id = "player_001"
    room1.created_time = 1640995200000
    
    # 添加玩家
    player1 = PlayerInfo()
    player1.player_id = "player_001"
    player1.username = "新手玩家"
    player1.is_ready = True
    room1.players.append(player1)
    
    # 房间2：游戏中，有密码
    room2 = RoomInfo()
    room2.room_id = "room_002"
    room2.room_name = "高手对决"
    room2.status = RoomStatus.PLAYING
    room2.owner_id = "player_002"
    room2.created_time = 1640995300000
    room2.password = "123456"  # 设置密码
    
    # 添加玩家
    player2 = PlayerInfo()
    player2.player_id = "player_002"
    player2.username = "高手A"
    player2.is_ready = True
    room2.players.append(player2)
    
    player3 = PlayerInfo()
    player3.player_id = "player_003"
    player3.username = "高手B"
    player3.is_ready = True
    room2.players.append(player3)
    
    # 房间3：等待中，有密码，只有一个玩家
    room3 = RoomInfo()
    room3.room_id = "room_003"
    room3.room_name = "私人房间"
    room3.status = RoomStatus.WAITING
    room3.owner_id = "player_004"
    room3.created_time = 1640995400000
    room3.password = "private"
    
    player4 = PlayerInfo()
    player4.player_id = "player_004"
    player4.username = "房主"
    player4.is_ready = False
    room3.players.append(player4)
    
    # 添加房间到响应
    room_list_response.rooms.append(room1)
    room_list_response.rooms.append(room2)
    room_list_response.rooms.append(room3)
    
    return room_list_response

def test_room_list_display():
    """测试房间列表显示"""
    
    # 创建GUI应用
    app = GomokuGUI()
    
    # 创建测试数据
    test_data = create_test_room_data()
    
    # 添加测试按钮
    test_frame = tk.Frame(app.root)
    test_frame.pack(side=tk.BOTTOM, fill=tk.X, padx=5, pady=5)
    
    def show_test_room_list():
        """显示测试房间列表"""
        app.show_room_list_window(test_data)
    
    def simulate_join_room(room_id, password=""):
        """模拟加入房间"""
        print(f"模拟加入房间: {room_id}, 密码: {password or '无'}")
        app.log_message(f"🎯 模拟加入房间: {room_id}")
        if password:
            app.log_message(f"🔐 使用密码: {password}")
    
    # 替换加入房间回调为模拟函数
    app.join_room_from_list = simulate_join_room
    
    tk.Button(test_frame, text="显示测试房间列表", command=show_test_room_list).pack(side=tk.LEFT, padx=5)
    
    # 添加说明
    app.log_message("🧪 房间列表显示测试模式")
    app.log_message("📋 点击'显示测试房间列表'查看房间列表界面")
    app.log_message("💡 测试数据包含3个房间：新手房间、高手对决、私人房间")
    app.log_message("🔐 测试密码功能和加入房间功能")
    
    # 启动GUI
    app.root.mainloop()

if __name__ == "__main__":
    test_room_list_display() 