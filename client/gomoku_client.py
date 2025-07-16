#!/usr/bin/env python3
"""
五子棋客户端测试程序
根据 gatesvr 接口说明实现的完整客户端
"""

import logging
import time
import threading
from typing import Optional, Dict, Any
from tcp_client import TCPClient
from message_handler import MessageHandler
from gomoku_pb2 import (
    CreateRoomResponse, JoinRoomResponse, PlacePieceResponse,
    PlayerReadyResponse, StartGameResponse, GetRoomListResponse,
    GameStateNotify, PlayerEventNotify, LeaveRoomResponse, DestroyRoomResponse,
    GameState, PlayerColor, GameResult
)

class GomokuClient:
    """五子棋客户端"""
    
    def __init__(self, host: str = "127.0.0.1", port: int = 6001):
        self.host = host
        self.port = port
        self.tcp_client: Optional[TCPClient] = None
        self.message_handler: Optional[MessageHandler] = None
        self.player_id = ""
        self.current_room_id = ""
        self.is_ready = False
        self.game_state: Optional[GameState] = None
        self.logger = logging.getLogger(__name__)
        
        # 设置日志格式
        logging.basicConfig(
            level=logging.INFO,
            format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
        )
        
    def connect(self, player_id: str = "", token: str = "") -> bool:
        """连接到服务器"""
        try:
            self.player_id = player_id or f"player_{int(time.time() * 1000) % 100000}"
            
            # 创建TCP客户端
            self.tcp_client = TCPClient(self.host, self.port)
            
            # 创建消息处理器
            self.message_handler = MessageHandler(self.player_id, self.tcp_client)
            
            # 设置消息处理器
            self.tcp_client.set_message_handler(self.message_handler)
            
            # 设置响应处理器
            self._setup_response_handlers()
            
            # 设置通知处理器
            self._setup_notification_handlers()
            
            # 设置系统消息处理器
            self._setup_system_handlers()
            
            # 连接到服务器
            if self.tcp_client.connect(self.player_id, token):
                self.logger.info(f"🎉 连接成功！玩家ID: {self.player_id}")
                return True
            else:
                self.logger.error("❌ 连接失败")
                return False
                
        except Exception as e:
            self.logger.error(f"❌ 连接异常: {e}")
            return False
    
    def disconnect(self):
        """断开连接"""
        if self.tcp_client:
            self.tcp_client.disconnect()
        self.logger.info("👋 已断开连接")
    
    def _setup_response_handlers(self):
        """设置响应处理器"""
        # 创建房间响应
        self.message_handler.set_response_handler("CreateRoom", self._handle_create_room_response)
        
        # 加入房间响应
        self.message_handler.set_response_handler("JoinRoom", self._handle_join_room_response)
        
        # 离开房间响应
        self.message_handler.set_response_handler("LeaveRoom", self._handle_leave_room_response)
        
        # 销毁房间响应
        self.message_handler.set_response_handler("DestroyRoom", self._handle_destroy_room_response)
        
        # 玩家准备响应
        self.message_handler.set_response_handler("PlayerReady", self._handle_player_ready_response)
        
        # 开始游戏响应
        self.message_handler.set_response_handler("StartGame", self._handle_start_game_response)
        
        # 下棋响应
        self.message_handler.set_response_handler("PlacePiece", self._handle_place_piece_response)
        
        # 获取房间列表响应
        self.message_handler.set_response_handler("GetRoomList", self._handle_get_room_list_response)
        
        # 获取游戏进度响应
        self.message_handler.set_response_handler("GetGameProgress", self._handle_get_game_progress_response)
        
        # 覆盖消息处理器的响应处理器获取方法
        self.message_handler._get_response_handler = self._get_response_handler
    
    def _get_response_handler(self, operation_type: str):
        """获取指定操作类型的响应处理器"""
        handler_map = {
            "create_room": self._handle_create_room_response,
            "join_room": self._handle_join_room_response,
            "leave_room": self._handle_leave_room_response,
            "destroy_room": self._handle_destroy_room_response,
            "player_ready": self._handle_player_ready_response,
            "start_game": self._handle_start_game_response,
            "place_piece": self._handle_place_piece_response,
            "get_room_list": self._handle_get_room_list_response,
            "get_game_progress": self._handle_get_game_progress_response
        }
        
        return handler_map.get(operation_type, self._default_response_handler)
    
    def _default_response_handler(self, response):
        """默认响应处理器"""
        if response.ret == 0:
            print("✅ 操作成功")
        else:
            reason = response.reason if hasattr(response, 'reason') else "未知错误"
            print(f"❌ 操作失败: {response.ret} - {reason}")
    
    def _setup_notification_handlers(self):
        """设置通知处理器"""
        # 游戏操作通知 - 当执行这些操作时，服务器会发送推送通知
        self.message_handler.set_notification_handler("CreateRoom", self._handle_create_room_notification)
        self.message_handler.set_notification_handler("JoinRoom", self._handle_join_room_notification) 
        self.message_handler.set_notification_handler("LeaveRoom", self._handle_leave_room_notification)
        self.message_handler.set_notification_handler("DestroyRoom", self._handle_destroy_room_notification)
        self.message_handler.set_notification_handler("PlayerReady", self._handle_player_ready_operation_notification)
        self.message_handler.set_notification_handler("StartGame", self._handle_start_game_notification)
        self.message_handler.set_notification_handler("PlacePiece", self._handle_place_piece_operation_notification)
        
        # 棋子放置通知
        self.message_handler.set_notification_handler("PiecePlacedNotification", self._handle_piece_placed_notification)
        
        # 游戏状态变化通知
        self.message_handler.set_notification_handler("GameStateChangedNotification", self._handle_game_state_changed_notification)
        
        # 玩家加入通知
        self.message_handler.set_notification_handler("PlayerJoinedNotification", self._handle_player_joined_notification)
        
        # 玩家离开通知
        self.message_handler.set_notification_handler("PlayerLeftNotification", self._handle_player_left_notification)
        
        # 玩家准备状态通知
        self.message_handler.set_notification_handler("PlayerReadyNotification", self._handle_player_ready_notification)
        
        # 游戏开始通知
        self.message_handler.set_notification_handler("GameStartNotification", self._handle_game_start_notification)
        
        # 默认通知处理器
        self.message_handler.set_notification_handler("default", self._handle_default_notification)
    
    def _setup_system_handlers(self):
        """设置系统消息处理器"""
        self.message_handler.set_system_handler("AUTH_FAILED", self._handle_auth_failed)
        self.message_handler.set_system_handler("KICKED", self._handle_kicked)
        
        # 房间事件处理器
        self.message_handler.set_system_handler("room_event", self._handle_room_event)
        self.message_handler.set_system_handler("default_room_event", self._handle_room_event)
        
        # 特定房间事件处理器
        self.message_handler.set_system_handler("room_event_join", self._handle_player_join_event)
        self.message_handler.set_system_handler("room_event_leave", self._handle_player_leave_event)
        self.message_handler.set_system_handler("room_event_ready", self._handle_player_ready_event)
        self.message_handler.set_system_handler("room_event_unready", self._handle_player_unready_event)
        self.message_handler.set_system_handler("room_event_start_game", self._handle_game_start_event)
        
        # 下棋通知处理器
        self.message_handler.set_system_handler("move_notification", self._handle_move_notification)
        self.message_handler.set_system_handler("piece_placed", self._handle_move_notification)
    
    # ==================== 响应处理器 ====================
    
    def _handle_create_room_response(self, response):
        """处理创建房间响应"""
        if response.ret == 0:
            # 解析响应数据
            create_resp = CreateRoomResponse()
            create_resp.ParseFromString(response.data)
            self.current_room_id = create_resp.room_id
            print(f"✅ 房间创建成功！房间ID: {create_resp.room_id}")
        else:
            print(f"❌ 房间创建失败: {response.reason}")
    
    def _handle_join_room_response(self, response):
        """处理加入房间响应"""
        if response.ret == 0:
            join_resp = JoinRoomResponse()
            join_resp.ParseFromString(response.data)
            self.current_room_id = join_resp.room_info.room_id
            print(f"✅ 成功加入房间: {self.current_room_id}")
            self._print_room_info(join_resp.room_info)
        else:
            print(f"❌ 加入房间失败: {response.reason}")
    
    def _handle_leave_room_response(self, response):
        """处理离开房间响应"""
        if response.ret == 0:
            self.current_room_id = ""
            self.is_ready = False
            self.game_state = None
            print("✅ 已离开房间")
        else:
            print(f"❌ 离开房间失败: {response.reason}")
    
    def _handle_destroy_room_response(self, response):
        """处理销毁房间响应"""
        if response.ret == 0:
            self.current_room_id = ""
            self.is_ready = False
            self.game_state = None
            print("✅ 房间已销毁")
        else:
            print(f"❌ 销毁房间失败: {response.reason}")
    
    def _handle_player_ready_response(self, response):
        """处理玩家准备响应"""
        if response.ret == 0:
            ready_resp = PlayerReadyResponse()
            ready_resp.ParseFromString(response.data)
            self.is_ready = ready_resp.is_ready
            status = "已准备" if self.is_ready else "取消准备"
            print(f"✅ {status}")
        else:
            print(f"❌ 准备操作失败: {response.reason}")
    
    def _handle_start_game_response(self, response):
        """处理开始游戏响应"""
        if response.ret == 0:
            start_resp = StartGameResponse()
            start_resp.ParseFromString(response.data)
            self.game_state = start_resp.game_state
            print("✅ 游戏开始！")
            self._print_game_state(self.game_state)
        else:
            print(f"❌ 开始游戏失败: {response.reason}")
    
    def _handle_place_piece_response(self, response):
        """处理下棋响应"""
        if response.ret == 0:
            place_resp = PlacePieceResponse()
            place_resp.ParseFromString(response.data)
            self.game_state = place_resp.game_state
            print("✅ 下棋成功！")
            self._print_game_state(self.game_state)
        else:
            print(f"❌ 下棋失败: {response.reason}")
    
    def _handle_get_room_list_response(self, response):
        """处理获取房间列表响应"""
        if response.ret == 0:
            list_resp = GetRoomListResponse()
            list_resp.ParseFromString(response.data)
            print(f"📋 房间列表 (第{list_resp.page_number}页，共{list_resp.total_pages}页):")
            
            if not list_resp.rooms:
                print("   暂无房间")
            else:
                for room in list_resp.rooms:
                    status = "游戏中" if room.is_gaming else "等待中"
                    lock = "🔒" if room.has_password else "🔓"
                    print(f"   {lock} [{room.room_id}] {room.room_name} ({len(room.players)}/2) - {status}")
        else:
            print(f"❌ 获取房间列表失败: {response.reason}")
    
    def _handle_get_game_progress_response(self, response):
        """处理获取游戏进度响应"""
        if response.ret == 0:
            from gomoku_pb2 import GetGameProgressResponse
            progress_resp = GetGameProgressResponse()
            progress_resp.ParseFromString(response.data)
            
            print("📊 游戏进度信息:")
            print(f"   ✅ {progress_resp.message}")
            
            # 显示房间信息
            if progress_resp.room_info:
                room = progress_resp.room_info
                print(f"   🏠 房间: [{room.room_id}] {room.room_name}")
                status_name = "等待中" if room.status == 0 else "游戏中" if room.status == 1 else "已结束"
                print(f"   📊 状态: {status_name}")
                print(f"   👥 玩家数: {len(room.players)}/2")
                if room.owner_id:
                    print(f"   👑 房主: {room.owner_id}")
                
                # 显示玩家信息
                for player in room.players:
                    ready_status = "✅" if player.is_ready else "⏳"
                    color_name = "黑子" if player.color == 1 else "白子" if player.color == 2 else "无"
                    print(f"      {ready_status} {player.player_id} ({color_name})")
            
            # 显示游戏状态
            if progress_resp.game_state:
                self.game_state = progress_resp.game_state
                game_state = progress_resp.game_state
                
                result_name = self._get_game_result_name(game_state.result)
                print(f"   🎮 游戏状态: {result_name}")
                print(f"   🔢 总步数: {game_state.total_moves}")
                
                if game_state.current_turn:
                    current_color = self._get_player_color_name(game_state.current_turn)
                    print(f"   🎯 当前轮次: {current_color}")
                
                if game_state.winner_id:
                    print(f"   🏆 获胜者: {game_state.winner_id}")
                    
                # 显示走棋历史（最近3步）
                if game_state.move_history:
                    print(f"   📋 最近走棋:")
                    recent_moves = game_state.move_history[-3:]  # 最近3步
                    for move in recent_moves:
                        color_name = "黑子" if move.color == 1 else "白子"
                        print(f"      {move.move_number}. {move.player_id} {color_name} ({move.x}, {move.y})")
                
                # 显示棋盘状态
                if len(game_state.board) == 225:
                    print("   📋 当前棋盘:")
                    self._print_board(game_state.board)
            
            # 显示剩余时间
            if progress_resp.remaining_time != -1:
                print(f"   ⏰ 剩余时间: {progress_resp.remaining_time}秒")
            else:
                print(f"   ⏰ 剩余时间: 无限制")
                
        else:
            print(f"❌ 获取游戏进度失败: {response.reason}")
    
    # ==================== 通知处理器 ====================
    
    def _handle_piece_placed_notification(self, notification):
        """处理棋子放置通知（包含完整PlacePieceResponse数据）"""
        try:
            place_resp = notification  # 已经在message_handler中解析
            
            # 更新游戏状态
            if hasattr(place_resp, 'game_state') and place_resp.game_state:
                self.game_state = place_resp.game_state
                
                # 显示下棋信息
                print("🔔 收到对手下棋通知！")
                
                # 显示操作结果
                if hasattr(place_resp, 'success') and place_resp.success:
                    if hasattr(place_resp, 'message') and place_resp.message:
                        print(f"   ✅ {place_resp.message}")
                else:
                    print("   ❌ 下棋操作失败")
                
                # 显示走棋历史信息
                if hasattr(self.game_state, 'move_history') and self.game_state.move_history:
                    total_moves = len(self.game_state.move_history)
                    latest_move = self.game_state.move_history[-1]
                    
                    print(f"   📍 落子位置: ({latest_move.x}, {latest_move.y})")
                    print(f"   🎮 玩家: {latest_move.player_id}")
                    print(f"   🔢 第 {latest_move.move_number} 步 (总步数: {total_moves})")
                    
                    # 显示棋子颜色
                    color_name = "黑子" if latest_move.color == 1 else "白子" if latest_move.color == 2 else "未知"
                    print(f"   ⚫ 棋子颜色: {color_name}")
                
                # 显示当前轮次信息
                if hasattr(self.game_state, 'current_turn'):
                    current_color = "黑子" if self.game_state.current_turn == 1 else "白子" if self.game_state.current_turn == 2 else "未知"
                    print(f"   🎯 下一轮次: {current_color}")
                
                # 检查游戏结果
                if hasattr(self.game_state, 'result') and self.game_state.result != 0:  # ONGOING = 0
                    if self.game_state.result == 1:  # BLACK_WIN
                        print("🏆 游戏结束：黑子获胜！")
                    elif self.game_state.result == 2:  # WHITE_WIN
                        print("🏆 游戏结束：白子获胜！")
                    elif self.game_state.result == 3:  # DRAW
                        print("🤝 游戏结束：平局！")
                    
                    if hasattr(self.game_state, 'winner_id') and self.game_state.winner_id:
                        print(f"   🎉 获胜者: {self.game_state.winner_id}")
                
                print("=" * 50)
                # 显示完整游戏状态
                self._print_game_state(self.game_state)
                print("✅ 棋盘状态已同步更新")
                
            else:
                print("⚠️ 下棋通知中没有游戏状态数据")
                
        except Exception as e:
            print(f"❌ 处理下棋通知失败: {e}")
            # 尝试基本的状态更新
            if hasattr(notification, 'game_state'):
                self.game_state = notification.game_state
                self._print_game_state(self.game_state)
    
    def _handle_game_state_changed_notification(self, notification):
        """处理游戏状态变化通知"""
        game_notify = notification  # 已经在message_handler中解析
        self.game_state = game_notify.game_state
        
        print(f"🔔 游戏状态变化!")
        print(f"   📝 事件类型: {game_notify.event_type}")
        if game_notify.event_message:
            print(f"   💬 事件消息: {game_notify.event_message}")
        
        self._print_game_state(self.game_state)
    
    def _handle_player_joined_notification(self, notification):
        """处理玩家加入通知"""
        player_notify = notification  # 已经在message_handler中解析
        
        print(f"🔔 新玩家加入房间!")
        print(f"   🎮 玩家ID: {player_notify.player_id}")
        print(f"   🏠 房间ID: {player_notify.room_id}")
        print(f"   📝 事件类型: {player_notify.event_type}")
        
        # 如果有玩家信息，显示详细信息
        if hasattr(player_notify, 'player_info') and player_notify.player_info:
            player_info = player_notify.player_info
            ready_status = "✅已准备" if player_info.is_ready else "⏳未准备"
            print(f"   🎯 准备状态: {ready_status}")
    
    def _handle_player_left_notification(self, notification):
        """处理玩家离开通知"""
        player_notify = notification  # 已经在message_handler中解析
        
        print(f"🔔 玩家离开房间!")
        print(f"   🎮 玩家ID: {player_notify.player_id}")
        print(f"   🏠 房间ID: {player_notify.room_id}")
        print(f"   📝 事件类型: {player_notify.event_type}")
        print(f"   👋 该玩家已离开游戏")
    
    def _handle_player_ready_notification(self, notification):
        """处理玩家准备状态通知"""
        player_notify = notification  # 已经在message_handler中解析
        
        print(f"🔔 玩家准备状态变化!")
        print(f"   🎮 玩家ID: {player_notify.player_id}")
        print(f"   🏠 房间ID: {player_notify.room_id}")
        
        if player_notify.event_type == "READY":
            print(f"   ✅ 玩家已准备")
        elif player_notify.event_type == "UNREADY":
            print(f"   ⏳ 玩家取消准备")
        else:
            print(f"   📝 准备状态: {player_notify.event_type}")
        
        # 如果有玩家信息，显示详细信息
        if hasattr(player_notify, 'player_info') and player_notify.player_info:
            player_info = player_notify.player_info
            ready_status = "✅已准备" if player_info.is_ready else "⏳未准备"
            print(f"   🎯 当前状态: {ready_status}")
    
    def _handle_game_start_notification(self, notification):
        """处理游戏开始通知"""
        game_notify = notification  # 已经在message_handler中解析为GameStateNotify
        self.game_state = game_notify.game_state
        
        print(f"🎮 游戏开始通知!")
        print(f"   🏠 房间ID: {game_notify.room_id}")
        print(f"   📝 事件类型: {game_notify.event_type}")
        if game_notify.event_message:
            print(f"   💬 事件消息: {game_notify.event_message}")
        
        # 显示当前游戏状态
        if self.game_state:
            print(f"   🎯 当前轮到: {self._get_player_color_name(self.game_state.current_turn)}")
            print(f"   🔢 总步数: {self.game_state.total_moves}")
            print(f"   📊 游戏状态: {self._get_game_result_name(self.game_state.result)}")
            print("   🎮 游戏已正式开始，可以开始下棋了！")
        
        # 如果是15x15棋盘，显示空白棋盘
        if self.game_state and len(self.game_state.board) == 225:
            print("   📋 当前棋盘状态:")
            self._print_board(self.game_state.board)
    
    def _handle_default_notification(self, notification):
        """处理默认通知"""
        method = notification.msg_head.request_id if hasattr(notification, 'msg_head') else "unknown"
        print(f"🔔 收到未处理的通知: {method}")
    
    # ==================== 游戏操作通知处理器 ====================
    
    def _handle_create_room_notification(self, data):
        """处理创建房间操作的推送通知"""
        if hasattr(data, 'room_id'):
            # 解析到了CreateRoomResponse数据
            print(f"🏠 房间创建完成！")
            print(f"   📍 房间ID: {data.room_id}")
            if hasattr(data, 'room_name') and data.room_name:
                print(f"   🏷️ 房间名: {data.room_name}")
            print(f"   ✨ 房间已准备就绪，等待玩家加入")
        else:
            # 简单通知
            print("🏠 房间创建操作完成的推送通知")
    
    def _handle_join_room_notification(self, data):
        """处理加入房间操作的推送通知"""
        if hasattr(data, 'room_info'):
            # 解析到了JoinRoomResponse数据
            room_info = data.room_info
            print(f"🚪 房间加入完成！")
            print(f"   📍 房间ID: {room_info.room_id}")
            print(f"   🏷️ 房间名: {room_info.room_name}")
            print(f"   👥 当前玩家: {len(room_info.players)}/2")
            
            # 显示房间内玩家
            if room_info.players:
                print("   🎮 房间内玩家:")
                for player in room_info.players:
                    ready_status = "✅已准备" if player.is_ready else "⏳未准备"
                    print(f"      • {player.player_id} ({ready_status})")
            
            # 显示游戏状态
            status = "🎮游戏中" if room_info.is_gaming else "⏳等待中"
            print(f"   🎯 状态: {status}")
        else:
            print("🚪 房间加入操作完成的推送通知")
    
    def _handle_leave_room_notification(self, data):
        """处理离开房间操作的推送通知"""
        if isinstance(data, str):
            print(f"🚶 {data}")
        else:
            print("🚶 房间离开操作完成的推送通知")
    
    def _handle_destroy_room_notification(self, data):
        """处理销毁房间操作的推送通知"""
        if isinstance(data, str):
            print(f"🏚️ {data}")
        else:
            print("🏚️ 房间销毁操作完成的推送通知")
    
    def _handle_player_ready_operation_notification(self, data):
        """处理玩家准备操作的推送通知"""
        if hasattr(data, 'is_ready'):
            # 解析到了PlayerReadyResponse数据
            status = "已准备" if data.is_ready else "取消准备"
            print(f"⏳ 准备状态更新完成！")
            print(f"   🎮 当前状态: {status}")
        else:
            print("⏳ 玩家准备操作完成的推送通知")
    
    def _handle_start_game_notification(self, data):
        """处理开始游戏操作的推送通知"""
        if hasattr(data, 'game_state'):
            # 解析到了StartGameResponse数据
            print("🎮 游戏开始操作完成！")
            print("   🎯 游戏已正式开始")
            if hasattr(data.game_state, 'current_player_id'):
                print(f"   🎮 当前轮到: {data.game_state.current_player_id}")
        else:
            print("🎮 游戏开始操作完成的推送通知")
    
    def _handle_place_piece_operation_notification(self, data):
        """处理下棋操作的推送通知"""
        if hasattr(data, 'game_state'):
            # 解析到了PlacePieceResponse数据
            print("♟️ 下棋操作完成！")
            if hasattr(data.game_state, 'last_move') and data.game_state.last_move:
                move = data.game_state.last_move
                print(f"   📍 落子位置: ({move.x}, {move.y})")
                print(f"   🎮 玩家: {move.player_id}")
            
            # 检查游戏结果
            if hasattr(data.game_state, 'result') and data.game_state.result != 0:  # GameResult.ONGOING
                if data.game_state.winner_id:
                    print(f"   🏆 游戏结束，获胜者: {data.game_state.winner_id}")
                else:
                    print("   🤝 游戏结束，平局")
        else:
            print("♟️ 下棋操作完成的推送通知")
    
    # ==================== 系统消息处理器 ====================
    
    def _handle_auth_failed(self, message):
        """处理认证失败"""
        print("🔐 认证失败，连接将被断开")
    
    def _handle_kicked(self, message):
        """处理被踢下线"""
        print("👋 您已被踢下线")
    
    # ==================== 业务方法 ====================
    
    def create_room(self, room_name: str, password: str = ""):
        """创建房间"""
        if not self.tcp_client or not self.tcp_client.is_connected():
            print("❌ 未连接到服务器")
            return
        self.message_handler.create_room(room_name, password)
    
    def join_room(self, room_id: str, password: str = ""):
        """加入房间"""
        if not self.tcp_client or not self.tcp_client.is_connected():
            print("❌ 未连接到服务器")
            return
        self.message_handler.join_room(room_id, password)
    
    def leave_room(self):
        """离开房间"""
        if not self.current_room_id:
            print("❌ 当前不在房间中")
            return
        self.message_handler.leave_room(self.current_room_id)
    
    def destroy_room(self):
        """销毁房间"""
        if not self.current_room_id:
            print("❌ 当前不在房间中")
            return
        self.message_handler.destroy_room(self.current_room_id)
    
    def player_ready(self):
        """切换准备状态"""
        if not self.current_room_id:
            print("❌ 当前不在房间中")
            return
        self.message_handler.player_ready(self.current_room_id)
    
    def start_game(self):
        """开始游戏"""
        if not self.current_room_id:
            print("❌ 当前不在房间中")
            return
        self.message_handler.start_game(self.current_room_id)
    
    def place_piece(self, x: int, y: int):
        """下棋"""
        if not self.current_room_id:
            print("❌ 当前不在房间中")
            return
        if not self.game_state:
            print("❌ 游戏尚未开始")
            return
        self.message_handler.place_piece(self.current_room_id, x, y)
    
    def get_room_list(self, page: int = 1, page_size: int = 10):
        """获取房间列表"""
        if not self.tcp_client or not self.tcp_client.is_connected():
            print("❌ 未连接到服务器")
            return
        self.message_handler.get_room_list(page, page_size)
    
    def get_game_progress(self, room_id: str = None):
        """获取游戏进度"""
        if not self.tcp_client or not self.tcp_client.is_connected():
            print("❌ 未连接到服务器")
            return
        
        # 如果没有指定房间ID，使用当前房间
        target_room_id = room_id or self.current_room_id
        if not target_room_id:
            print("❌ 请指定房间ID或先加入房间")
            return
            
        self.message_handler.get_game_progress(target_room_id)
    
    # ==================== 工具方法 ====================
    
    def _print_room_info(self, room_info):
        """打印房间信息"""
        print(f"🏠 房间信息:")
        print(f"   ID: {room_info.room_id}")
        print(f"   名称: {room_info.room_name}")
        print(f"   状态: {room_info.status}")
        print(f"   玩家数: {len(room_info.players)}")
        for player in room_info.players:
            color_name = PlayerColor.Name(player.color)
            ready_status = "✅" if player.is_ready else "❌"
            print(f"     - {player.player_id} ({color_name}) {ready_status}")
    
    def _print_game_state(self, game_state):
        """打印游戏状态"""
        if not game_state:
            return
            
        print(f"🎮 游戏状态:")
        print(f"   当前回合: {PlayerColor.Name(game_state.current_turn)}")
        print(f"   结果: {GameResult.Name(game_state.result)}")
        print(f"   总步数: {game_state.total_moves}")
        
        if game_state.result != GameResult.ONGOING:
            if game_state.winner_id:
                print(f"   🏆 获胜者: {game_state.winner_id}")
            else:
                print(f"   🤝 平局")
        
        # 打印棋盘（简化版）
        print("📋 棋盘:")
        self._print_board(game_state.board)
    
    def _print_board(self, board_data):
        """打印棋盘"""
        # 棋盘数据是15x15展平为225个元素
        if len(board_data) != 225:
            print("   ⚠️ 棋盘数据异常")
            return
        
        print("    0 1 2 3 4 5 6 7 8 9 A B C D E")
        for y in range(15):
            row = f" {y:2X} "
            for x in range(15):
                idx = y * 15 + x
                piece = board_data[idx]
                if piece == 0:
                    row += "· "
                elif piece == 1:
                    row += "● "  # 黑子
                else:
                    row += "○ "  # 白子
            print(row)
    
    def get_status(self) -> dict:
        """获取客户端状态"""
        status = {
            "connected": self.tcp_client.is_connected() if self.tcp_client else False,
            "player_id": self.player_id,
            "current_room": self.current_room_id,
            "is_ready": self.is_ready,
            "in_game": self.game_state is not None
        }
        
        if self.tcp_client:
            status.update(self.tcp_client.get_connection_info())
            
        return status

    # ==================== 房间事件处理器 ====================
    
    def _handle_room_event(self, event):
        """通用房间事件处理器
        
        Args:
            event: 包含room_id, player_id, event_type的字典
        """
        try:
            room_id = event['room_id']
            player_id = event['player_id']
            event_type = event['event_type']
            
            print(f"🏠 房间事件: {player_id} {event_type} (房间:{room_id})")
            
        except Exception as e:
            print(f"❌ 房间事件处理失败: {e}")
    
    def _handle_player_join_event(self, event):
        """处理玩家加入房间事件"""
        try:
            room_id = event['room_id']
            player_id = event['player_id']
            
            print(f"👥 玩家 {player_id} 加入了房间 {room_id}")
            
            # 如果是当前房间，提示刷新状态
            if hasattr(self, 'current_room_id') and self.current_room_id == room_id:
                print("💡 提示: 输入 'status' 查看最新房间状态")
                
        except Exception as e:
            print(f"❌ 玩家加入事件处理失败: {e}")
    
    def _handle_player_leave_event(self, event):
        """处理玩家离开房间事件"""
        try:
            room_id = event['room_id']
            player_id = event['player_id']
            
            print(f"👋 玩家 {player_id} 离开了房间 {room_id}")
            
            # 如果是当前房间，提示刷新状态
            if hasattr(self, 'current_room_id') and self.current_room_id == room_id:
                print("💡 提示: 输入 'status' 查看最新房间状态")
                
        except Exception as e:
            print(f"❌ 玩家离开事件处理失败: {e}")
    
    def _handle_player_ready_event(self, event):
        """处理玩家准备事件"""
        try:
            room_id = event['room_id']
            player_id = event['player_id']
            
            print(f"✅ 玩家 {player_id} 已准备就绪")
            
            # 如果是当前房间，提示相关信息
            if hasattr(self, 'current_room_id') and self.current_room_id == room_id:
                print("💡 如果所有玩家都准备好了，房主可以开始游戏")
                
        except Exception as e:
            print(f"❌ 玩家准备事件处理失败: {e}")
    
    def _handle_player_unready_event(self, event):
        """处理玩家取消准备事件"""
        try:
            room_id = event['room_id']
            player_id = event['player_id']
            
            print(f"⏸️ 玩家 {player_id} 取消准备")
            
        except Exception as e:
            print(f"❌ 玩家取消准备事件处理失败: {e}")
    
    def _handle_game_start_event(self, event):
        """处理游戏开始事件"""
        try:
            room_id = event['room_id']
            player_id = event['player_id']
            message_text = event.get('message_text', '')
            
            # 显示游戏开始消息
            if message_text:
                print(f"🎮 {message_text}")
            else:
                print(f"🎮 房主 {player_id} 开始了游戏！")
            
            # 如果是当前房间，更新游戏状态
            if hasattr(self, 'current_room_id') and self.current_room_id == room_id:
                print("🎯 游戏开始！进入游戏状态...")
                
                # 初始化游戏状态
                self._initialize_game_state()
                
                print("✨ 游戏已开始，您可以使用 'place x y' 命令落子")
                print("💡 输入 'status' 查看当前游戏状态")
                
        except Exception as e:
            print(f"❌ 游戏开始事件处理失败: {e}")
    
    def _handle_move_notification(self, notification):
        """处理简单下棋通知（备选方案，当PiecePlacedNotification不可用时使用）"""
        try:
            message = notification.get('message', '下棋成功')
            print(f"♟️ 收到简单下棋通知: {message}")
            print("ℹ️ 注意：这是简化版通知，可能缺少详细信息")
            
            # 简单通知不包含完整游戏状态，我们需要等待GameStateNotify或主动刷新
            print("⏳ 等待游戏状态更新...")
            
            # 延迟一小段时间后检查游戏状态
            import time
            time.sleep(0.5)
            self._check_and_refresh_game_state()
                
        except Exception as e:
            print(f"❌ 简单下棋通知处理失败: {e}")
    
    def _check_and_refresh_game_state(self):
        """检查并刷新游戏状态"""
        try:
            if self.game_state and hasattr(self.game_state, 'move_history'):
                # 如果已经有完整的游戏状态，显示状态信息
                print("✅ 棋盘状态已更新")
                self._show_game_status()
            else:
                # 如果没有完整状态，尝试重新加入房间以获取最新状态
                print("🔄 游戏状态不完整，尝试刷新...")
                if hasattr(self, 'current_room_id') and self.current_room_id:
                    self._refresh_room_state()
                else:
                    print("⚠️ 无法刷新：当前房间ID未知")
                    
        except Exception as e:
            print(f"❌ 检查游戏状态失败: {e}")
    
    def _refresh_room_state(self):
        """刷新房间状态（重新加入房间以获取最新信息）"""
        try:
            if not self.tcp_client or not self.tcp_client.connected:
                print("❌ 无法刷新：未连接到服务器")
                return
                
            print(f"🔄 正在刷新房间状态: {self.current_room_id}")
            
            # 重新加入房间以获取最新的房间信息和游戏状态
            self.message_handler.set_response_handler("JoinRoom", self._handle_refresh_join_response)
            request_id = self.message_handler.join_room(self.current_room_id, "")
            
        except Exception as e:
            print(f"❌ 刷新房间状态失败: {e}")
    
    def _handle_refresh_join_response(self, response):
        """处理刷新加入房间的响应"""
        try:
            if response.ret == 0:
                from gomoku_pb2 import JoinRoomResponse
                join_resp = JoinRoomResponse()
                join_resp.ParseFromString(response.data)
                
                # 更新房间信息
                self.room_info = join_resp.room_info
                self.current_room_id = join_resp.room_info.room_id
                
                # 更新游戏状态
                if join_resp.room_info.game_state:
                    self.game_state = join_resp.room_info.game_state
                    print("✅ 游戏状态已刷新")
                    
                    # 显示最新的走棋记录
                    if hasattr(self.game_state, 'move_history') and self.game_state.move_history:
                        total_moves = len(self.game_state.move_history)
                        latest_move = self.game_state.move_history[-1]
                        print(f"📋 总步数: {total_moves}, 最新一步: ({latest_move.x}, {latest_move.y}) by {latest_move.player_id}")
                    
                    # 显示游戏状态
                    self._show_game_status()
                else:
                    print("⚠️ 房间信息中没有游戏状态")
                    
            else:
                print(f"❌ 刷新房间状态失败: {response.reason}")
                
        except Exception as e:
            print(f"❌ 处理刷新响应失败: {e}")
    
    def _initialize_game_state(self):
        """初始化游戏状态"""
        try:
            from gomoku_pb2 import GameState, GameResult
            
            # 创建一个基本的游戏状态
            self.game_state = GameState()
            self.game_state.result = GameResult.ONGOING  # 游戏进行中
            
            # 初始化15x15的空棋盘
            self.game_state.board.extend([0] * 225)  # 15*15 = 225
            
            # 设置初始的游戏信息
            self.game_state.total_moves = 0
            
            print("🎲 游戏状态已初始化")
            
        except Exception as e:
            print(f"❌ 初始化游戏状态失败: {e}")
            # 如果初始化失败，至少设置一个标记表示游戏已开始
            self.game_state = True


def main():
    """主程序"""
    print("🎮 五子棋客户端测试程序")
    print("基于 gatesvr 接口实现")
    print("=" * 50)
    
    client = GomokuClient()
    
    # 连接到服务器
    player_id = input("请输入玩家ID (直接回车使用自动生成): ").strip()
    if not client.connect(player_id):
        print("连接失败，程序退出")
        return
    
    print("\n可用命令:")
    print("  create <房间名> [密码]  - 创建房间")
    print("  join <房间ID> [密码]   - 加入房间") 
    print("  list [页码] [页大小]   - 获取房间列表")
    print("  ready                 - 切换准备状态")
    print("  start                 - 开始游戏")
    print("  place <x> <y>         - 下棋 (x,y: 0-14)")
    print("  progress [房间ID]     - 获取游戏进度")
    print("  leave                 - 离开房间")
    print("  destroy               - 销毁房间")
    print("  status                - 查看状态")
    print("  quit                  - 退出程序")
    print()
    
    try:
        while True:
            try:
                command = input(f"[{client.player_id}]> ").strip()
                if not command:
                    continue
                
                parts = command.split()
                cmd = parts[0].lower()
                
                if cmd == "quit":
                    break
                elif cmd == "create":
                    if len(parts) < 2:
                        print("用法: create <房间名> [密码]")
                        continue
                    room_name = parts[1]
                    password = parts[2] if len(parts) > 2 else ""
                    client.create_room(room_name, password)
                    
                elif cmd == "join":
                    if len(parts) < 2:
                        print("用法: join <房间ID> [密码]")
                        continue
                    room_id = parts[1]
                    password = parts[2] if len(parts) > 2 else ""
                    client.join_room(room_id, password)
                    
                elif cmd == "list":
                    page = int(parts[1]) if len(parts) > 1 else 1
                    page_size = int(parts[2]) if len(parts) > 2 else 10
                    client.get_room_list(page, page_size)
                    
                elif cmd == "ready":
                    client.player_ready()
                    
                elif cmd == "start":
                    client.start_game()
                    
                elif cmd == "place":
                    if len(parts) < 3:
                        print("用法: place <x> <y>")
                        continue
                    try:
                        x = int(parts[1])
                        y = int(parts[2])
                        client.place_piece(x, y)
                    except ValueError:
                        print("坐标必须是数字")
                        
                elif cmd == "progress":
                    room_id = parts[1] if len(parts) > 1 else None
                    client.get_game_progress(room_id)
                        
                elif cmd == "leave":
                    client.leave_room()
                    
                elif cmd == "destroy":
                    client.destroy_room()
                    
                elif cmd == "status":
                    status = client.get_status()
                    print("📊 客户端状态:")
                    for key, value in status.items():
                        print(f"   {key}: {value}")
                        
                else:
                    print(f"未知命令: {cmd}")
                    
            except KeyboardInterrupt:
                break
            except Exception as e:
                print(f"命令执行错误: {e}")
                
    except KeyboardInterrupt:
        pass
    finally:
        print("\n正在断开连接...")
        client.disconnect()
        print("👋 再见！")


if __name__ == "__main__":
    main() 