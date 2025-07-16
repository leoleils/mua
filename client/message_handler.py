import time
import logging
from typing import Dict, Any, Callable, Optional
from common_pb2 import GameMessage, HeadMessage, MessageType, ServiceMessageType, GameMessageResponse
from gomoku_pb2 import (
    CreateRoomRequest, CreateRoomResponse,
    JoinRoomRequest, JoinRoomResponse, PlacePieceRequest, PlacePieceResponse,
    PlayerReadyRequest, PlayerReadyResponse, StartGameRequest, StartGameResponse,
    GetRoomListRequest, GetRoomListResponse, GameStateNotify, PlayerEventNotify,
    LeaveRoomRequest, LeaveRoomResponse, DestroyRoomRequest, DestroyRoomResponse
)

class MessageHandler:
    """消息处理器，负责游戏消息的创建和处理"""
    
    def __init__(self, player_id: str, tcp_client):
        self.player_id = player_id
        self.tcp_client = tcp_client
        self.logger = logging.getLogger(__name__)
        
        # 响应处理器：处理客户端请求的响应
        self.response_handlers: Dict[str, Callable] = {}
        
        # 通知处理器：处理服务器推送的通知
        self.notification_handlers: Dict[str, Callable] = {}
        
        # 系统消息处理器
        self.system_handlers: Dict[str, Callable] = {}
        
        # 请求计数器（用于生成唯一请求ID）
        self.request_counter = 0
        
        self.logger.info(f"🎮 消息处理器初始化完成，玩家ID: {player_id}")
        
    def set_response_handler(self, request_id: str, handler: Callable):
        """设置响应处理器"""
        self.response_handlers[request_id] = handler
        self.logger.debug(f"📝 设置响应处理器: {request_id}")
        
    def set_notification_handler(self, notification_type: str, handler: Callable):
        """设置通知处理器
        
        Args:
            notification_type: 通知类型（如：PiecePlacedNotification, PlayerJoinedNotification等）
            handler: 处理函数
        """
        self.notification_handlers[notification_type] = handler
        self.logger.debug(f"🔔 设置通知处理器: {notification_type}")
        
    def set_system_handler(self, system_type: str, handler: Callable):
        """设置系统消息处理器
        
        Args:
            system_type: 系统消息类型（如：AUTH_FAILED, KICKED等）
            handler: 处理函数
        """
        self.system_handlers[system_type] = handler
        self.logger.debug(f"⚙️ 设置系统处理器: {system_type}")
    
    def handle_message(self, message_data, is_push: bool = False):
        """处理消息的统一入口点（兼容旧接口）"""
        if is_push:
            # 推送消息，假设是GameMessage
            self._handle_notification(message_data)
        else:
            # 响应消息，假设是GameMessageResponse  
            self._handle_response(message_data)
    
    def handle_response(self, response: GameMessageResponse):
        """处理响应消息的公共接口"""
        self._handle_response(response)
    
    def handle_notification(self, notification: GameMessage):
        """处理推送通知的公共接口"""
        self._handle_notification(notification)
    
    def handle_system_message(self, message: str):
        """处理系统消息的公共接口"""
        self._handle_system_message(message)
    
    def _handle_response(self, response: GameMessageResponse):
        """处理响应消息"""
        request_id = response.msg_head.request_id if response.msg_head else "unknown"
        
        self.logger.info(f"📥 收到响应消息: ID={request_id}, 状态码={response.ret}")
        
        # 添加响应数据调试信息
        if response.ret == 0 and response.data:
            self.logger.debug(f"📊 响应数据长度: {len(response.data)} 字节")
        elif response.ret != 0 and response.reason:
            self.logger.debug(f"❌ 错误原因: {response.reason}")
        
        if request_id in self.response_handlers:
            handler = self.response_handlers.pop(request_id)  # 使用后移除
            try:
                handler(response)
                self.logger.debug(f"✅ 响应处理成功: {request_id}")
            except Exception as e:
                self.logger.error(f"❌ 响应处理失败 {request_id}: {e}")
        else:
            self.logger.warning(f"⚠️ 未找到响应处理器: {request_id}")
            # 记录响应内容用于调试
            if response.ret != 0:
                reason = response.reason if hasattr(response, 'reason') else "未知错误"
                self.logger.error(f"❌ 服务器返回错误 {request_id}: {response.ret} - {reason}")
            else:
                self.logger.info(f"🎯 成功响应但无处理器: {request_id}")
                # 显示可用的处理器
                available_handlers = list(self.response_handlers.keys())
                if available_handlers:
                    self.logger.debug(f"🔍 当前可用的响应处理器: {available_handlers}")
                else:
                    self.logger.debug("🔍 当前没有待处理的响应处理器")
    
    def _handle_notification(self, notification: GameMessage):
        """处理推送通知"""
        service_name = notification.msg_head.service_name
        request_id = notification.msg_head.request_id
        
        # 从request_id中提取操作类型
        # request_id格式通常为: operation_type_timestamp_sequence 或 simple_operation_name
        method = self._extract_method_from_request_id(request_id)
        
        self.logger.info(f"🔔 收到推送通知: 服务={service_name}, 方法={method}")
        
        if method in self.notification_handlers:
            handler = self.notification_handlers[method]
            try:
                # 解析推送的数据
                if notification.payload:
                    self._parse_and_handle_notification_payload(method, notification.payload, handler)
                else:
                    handler(notification)
                self.logger.debug(f"✅ 通知处理成功: {method}")
            except Exception as e:
                self.logger.error(f"❌ 通知处理失败 {method}: {e}")
        else:
            self.logger.info(f"📣 未处理的通知: {method}")
            # 尝试通用处理
            if 'default' in self.notification_handlers:
                try:
                    self.notification_handlers['default'](notification)
                except Exception as e:
                    self.logger.error(f"❌ 通用通知处理失败: {e}")
    
    def _extract_method_from_request_id(self, request_id: str) -> str:
        """从request_id中提取方法名"""
        if not request_id:
            return "unknown"
        
        # 操作类型映射表
        operation_map = {
            "create_room": "CreateRoom",
            "join_room": "JoinRoom", 
            "leave_room": "LeaveRoom",
            "destroy_room": "DestroyRoom",
            "player_ready": "PlayerReady",
            "start_game": "StartGame",
            "place_piece": "PlacePiece",
            "get_room_list": "GetRoomList"
        }
        
        # 尝试从request_id开头匹配操作类型
        for op_type, method_name in operation_map.items():
            if request_id.startswith(op_type):
                return method_name
        
        # 如果没有匹配到，可能是简单的方法名，直接返回
        return request_id
    
    def _parse_and_handle_notification_payload(self, method: str, payload: bytes, handler: Callable):
        """解析并处理通知的载荷数据"""
        try:
            # 根据方法名解析不同类型的数据
            
            # 游戏操作通知（使用对应的响应结构）
            if method == "CreateRoom":
                response = CreateRoomResponse()
                response.ParseFromString(payload)
                handler(response)
                
            elif method == "JoinRoom":
                response = JoinRoomResponse()
                response.ParseFromString(payload)
                handler(response)
                
            elif method == "LeaveRoom":
                # LeaveRoom没有特定的响应数据，使用通用响应
                try:
                    response = GameMessageResponse()
                    response.ParseFromString(payload)
                    handler(response)
                except:
                    handler("离开房间操作完成")
                
            elif method == "DestroyRoom":
                # DestroyRoom没有特定的响应数据，使用通用响应
                try:
                    response = GameMessageResponse()
                    response.ParseFromString(payload)
                    handler(response)
                except:
                    handler("房间销毁操作完成")
                
            elif method == "PlayerReady":
                response = PlayerReadyResponse()
                response.ParseFromString(payload)
                handler(response)
                
            elif method == "StartGame":
                response = StartGameResponse()
                response.ParseFromString(payload)
                handler(response)
                
            elif method == "PlacePiece":
                response = PlacePieceResponse()
                response.ParseFromString(payload)
                handler(response)
                
            # 事件通知
            elif method == "PiecePlacedNotification":
                response = PlacePieceResponse()
                response.ParseFromString(payload)
                handler(response)
                
            elif method == "GameStateChangedNotification":
                notify = GameStateNotify()
                notify.ParseFromString(payload)
                handler(notify)
                
            elif method == "PlayerJoinedNotification":
                notify = PlayerEventNotify()
                notify.ParseFromString(payload)
                handler(notify)
                
            elif method == "PlayerLeftNotification":
                notify = PlayerEventNotify()
                notify.ParseFromString(payload)
                handler(notify)
                
            elif method == "PlayerReadyNotification":
                notify = PlayerEventNotify()
                notify.ParseFromString(payload)
                handler(notify)
                
            elif method == "GameStartNotification":
                notify = GameStateNotify()
                notify.ParseFromString(payload)
                handler(notify)
                
            else:
                # 通用处理：尝试解析为字符串
                try:
                    text = payload.decode('utf-8')
                    handler(text)
                except:
                    # 如果不是文本，直接传递原始数据
                    handler(payload)
                    
        except Exception as e:
            self.logger.error(f"❌ 解析通知载荷失败 {method}: {e}")
            # 回退到原始处理
            handler(payload)
    
    def _handle_system_message(self, message: str):
        """处理系统消息"""
        self.logger.info(f"⚙️ 收到系统消息: {message}")
        
        if message in self.system_handlers:
            handler = self.system_handlers[message]
            try:
                handler(message)
                self.logger.debug(f"✅ 系统消息处理成功: {message}")
            except Exception as e:
                self.logger.error(f"❌ 系统消息处理失败 {message}: {e}")
        else:
            self.logger.warning(f"⚠️ 未处理的系统消息: {message}")
    
    def _generate_request_id(self, prefix: str) -> str:
        """生成唯一请求ID"""
        self.request_counter += 1
        timestamp = int(time.time() * 1000)
        return f"{prefix}_{timestamp}_{self.request_counter}"
    
    # ==================== 业务方法 ====================
    
    def create_room(self, room_name: str, password: str = "") -> str:
        """创建房间"""
        request_id = self._generate_request_id("create_room")
        
        # 创建请求
        create_req = CreateRoomRequest()
        create_req.room_name = room_name
        create_req.password = password
        
        # 发送游戏消息
        self._send_game_request(request_id, "CreateRoom", create_req)
        self.logger.info(f"📤 发送创建房间请求: {room_name}")
        return request_id
    
    def join_room(self, room_id: str, password: str = "") -> str:
        """加入房间"""
        request_id = self._generate_request_id("join_room")
        
        join_req = JoinRoomRequest()
        join_req.room_id = room_id
        join_req.password = password
        
        self._send_game_request(request_id, "JoinRoom", join_req)
        self.logger.info(f"📤 发送加入房间请求: {room_id}")
        return request_id
    
    def leave_room(self, room_id: str) -> str:
        """离开房间"""
        request_id = self._generate_request_id("leave_room")
        
        leave_req = LeaveRoomRequest()
        leave_req.room_id = room_id
        
        self._send_game_request(request_id, "LeaveRoom", leave_req)
        self.logger.info(f"📤 发送离开房间请求: {room_id}")
        return request_id
    
    def destroy_room(self, room_id: str) -> str:
        """销毁房间"""
        request_id = self._generate_request_id("destroy_room")
        
        destroy_req = DestroyRoomRequest()
        destroy_req.room_id = room_id
        
        self._send_game_request(request_id, "DestroyRoom", destroy_req)
        self.logger.info(f"📤 发送销毁房间请求: {room_id}")
        return request_id
    
    def player_ready(self, room_id: str) -> str:
        """玩家准备"""
        request_id = self._generate_request_id("player_ready")
        
        ready_req = PlayerReadyRequest()
        ready_req.room_id = room_id
        
        self._send_game_request(request_id, "PlayerReady", ready_req)
        self.logger.info(f"📤 发送玩家准备请求: {room_id}")
        return request_id
    
    def start_game(self, room_id: str) -> str:
        """开始游戏"""
        request_id = self._generate_request_id("start_game")
        
        start_req = StartGameRequest()
        start_req.room_id = room_id
        
        self._send_game_request(request_id, "StartGame", start_req)
        self.logger.info(f"📤 发送开始游戏请求: {room_id}")
        return request_id
    
    def place_piece(self, room_id: str, x: int, y: int) -> str:
        """下棋"""
        request_id = self._generate_request_id("place_piece")
        
        place_req = PlacePieceRequest()
        place_req.room_id = room_id
        place_req.x = x
        place_req.y = y
        
        self._send_game_request(request_id, "PlacePiece", place_req)
        self.logger.info(f"📤 发送下棋请求: {room_id} at ({x}, {y})")
        return request_id
    
    def get_room_list(self, page: int = 1, page_size: int = 10) -> str:
        """获取房间列表"""
        request_id = self._generate_request_id("get_room_list")
        
        list_req = GetRoomListRequest()
        list_req.page = page
        list_req.page_size = page_size
        
        self._send_game_request(request_id, "GetRoomList", list_req)
        self.logger.info(f"📤 发送获取房间列表请求: 页码={page}, 大小={page_size}")
        return request_id
    
    def _send_game_request(self, request_id: str, method: str, payload):
        """发送游戏请求
        
        Args:
            request_id: 请求ID（用于响应匹配）
            method: 方法名（如：CreateRoom, PlacePiece等）
            payload: 请求数据（protobuf对象）
        """
        # 创建游戏消息
        game_message = GameMessage()
        
        # 设置消息头（按照gatesvr接口规范）
        game_message.msg_head.player_id = self.player_id
        game_message.msg_head.service_name = "gomokusvr"  # 目标服务名
        game_message.msg_head.request_id = method         # 使用方法名作为request_id（服务器要求）
        game_message.msg_head.timestamp = int(time.time() * 1000)
        game_message.msg_head.service_msg_type = ServiceMessageType.SYNC  # 同步等待响应
        
        # 设置消息类型为服务消息
        game_message.msg_type = MessageType.SERVICE_MESSAGE
        
        # 序列化负载
        game_message.payload = payload.SerializeToString()
        
        # 记录请求ID以便后续匹配响应
        # 使用完整的request_id作为响应匹配的key
        handler_map = {
            "CreateRoom": self._get_response_handler("create_room"),
            "JoinRoom": self._get_response_handler("join_room"),
            "LeaveRoom": self._get_response_handler("leave_room"),
            "DestroyRoom": self._get_response_handler("destroy_room"),
            "PlayerReady": self._get_response_handler("player_ready"),
            "StartGame": self._get_response_handler("start_game"),
            "PlacePiece": self._get_response_handler("place_piece"),
            "GetRoomList": self._get_response_handler("get_room_list")
        }
        
        if method in handler_map:
            self.response_handlers[method] = handler_map[method]  # 使用方法名作为key
        else:
            self.response_handlers[method] = lambda resp: self._default_response_handler(method, resp)
        
        # 发送消息
        success = self.tcp_client.send_message(game_message)
        if success:
            self.logger.debug(f"✅ 消息发送成功: {method}")
        else:
            self.logger.error(f"❌ 消息发送失败: {method}")
            # 清理响应处理器
            self.response_handlers.pop(method, None)
    
    def _default_response_handler(self, request_id: str, response: GameMessageResponse):
        """默认响应处理器"""
        if response.ret == 0:
            self.logger.info(f"✅ 请求成功: {request_id}")
        else:
            reason = response.reason if hasattr(response, 'reason') else "未知错误"
            self.logger.error(f"❌ 请求失败: {request_id}, 错误码: {response.ret}, 原因: {reason}")
    
    def _get_response_handler(self, operation_type: str):
        """获取指定操作类型的响应处理器"""
        # 这个方法会被子类或使用者覆盖来提供具体的响应处理器
        # 默认返回一个通用处理器
        def generic_handler(response):
            if response.ret == 0:
                self.logger.info(f"✅ {operation_type} 操作成功")
            else:
                reason = response.reason if hasattr(response, 'reason') else "未知错误"
                self.logger.error(f"❌ {operation_type} 操作失败: {response.ret} - {reason}")
        
        return generic_handler
    
    # ==================== 工具方法 ====================
    
    def clear_handlers(self):
        """清理所有处理器"""
        self.response_handlers.clear()
        self.notification_handlers.clear()
        self.system_handlers.clear()
        self.logger.info("🧹 已清理所有消息处理器")
    
    def get_pending_requests(self) -> list:
        """获取待处理的请求列表"""
        return list(self.response_handlers.keys())
    
    def cancel_request(self, request_id: str) -> bool:
        """取消请求"""
        if request_id in self.response_handlers:
            del self.response_handlers[request_id]
            self.logger.info(f"🚫 已取消请求: {request_id}")
            return True
        return False
