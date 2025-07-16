import time
import logging
from typing import Dict, Any, Callable, Optional
from common_pb2 import GameMessage, HeadMessage, MessageType, ServiceMessageType, GameMessageResponse
from gomoku_pb2 import (
    CreateRoomRequest, CreateRoomResponse,
    JoinRoomRequest, JoinRoomResponse, PlacePieceRequest, PlacePieceResponse,
    PlayerReadyRequest, PlayerReadyResponse, StartGameRequest, StartGameResponse,
    GetRoomListRequest, GetRoomListResponse, GameStateNotify, PlayerEventNotify,
    LeaveRoomRequest, LeaveRoomResponse, DestroyRoomRequest, DestroyRoomResponse,
    GetGameProgressRequest, GetGameProgressResponse
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
    
    def handle_room_event_direct(self, event: Dict[str, str]):
        """直接处理房间事件（用于protobuf消息）"""
        self.logger.info(f"🏠 直接处理房间事件: {event['room_id']} - {event['player_id']} {event['event_type']}")
        self._handle_room_event(event)
    
    def _handle_response(self, response: GameMessageResponse):
        """处理响应消息"""
        request_id = response.msg_head.request_id if response.msg_head else "unknown"
        
        self.logger.info(f"📥 收到响应消息: ID={request_id}, 状态码={response.ret}")
        
        # 添加响应数据调试信息
        if response.ret == 0 and response.data:
            self.logger.debug(f"📊 响应数据长度: {len(response.data)} 字节")
        elif response.ret != 0 and response.reason:
            self.logger.debug(f"❌ 错误原因: {response.reason}")
        
        handler = None
        handler_key = None
        
        # 首先尝试使用request_id查找处理器
        if request_id and request_id != "unknown" and request_id in self.response_handlers:
            handler = self.response_handlers.pop(request_id)
            handler_key = request_id
        else:
            # 如果request_id无效，尝试查找方法名处理器
            # 检查当前可用的处理器，找到匹配的方法名
            method_handlers = ["CreateRoom", "JoinRoom", "LeaveRoom", "DestroyRoom", 
                             "PlayerReady", "StartGame", "PlacePiece", "GetRoomList"]
            
            for method in method_handlers:
                if method in self.response_handlers:
                    handler = self.response_handlers.pop(method)
                    handler_key = method
                    self.logger.debug(f"🔄 使用方法名处理器: {method}")
                    break
        
        if handler:
            try:
                handler(response)
                self.logger.debug(f"✅ 响应处理成功: {handler_key}")
            except Exception as e:
                self.logger.error(f"❌ 响应处理失败 {handler_key}: {e}")
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
                
            elif method == "GetGameProgress":
                response = GetGameProgressResponse()
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
                # 通用处理：安全地尝试解析为字符串
                try:
                    # 首先尝试UTF-8解码
                    text = payload.decode('utf-8')
                    handler(text)
                except UnicodeDecodeError as e:
                    self.logger.warning(f"⚠️ 通知载荷包含非UTF-8数据 {method}: {e}")
                    # 使用错误处理模式解码
                    try:
                        text = payload.decode('utf-8', errors='replace')
                        handler(text)
                    except Exception:
                        # 如果仍然失败，传递原始数据
                        handler(payload)
                except Exception as e:
                    self.logger.debug(f"📦 通知载荷不是文本格式 {method}: {e}")
                    # 如果不是文本，直接传递原始数据
                    handler(payload)
                    
        except Exception as e:
            self.logger.error(f"❌ 解析通知载荷失败 {method}: {e}")
            # 提供更详细的错误诊断
            try:
                payload_size = len(payload) if payload else 0
                self.logger.debug(f"载荷诊断: method={method}, size={payload_size}")
                
                if "codec can't decode" in str(e) and payload:
                    hex_sample = payload[:20].hex() if len(payload) >= 20 else payload.hex()
                    self.logger.debug(f"载荷前20字节(hex): {hex_sample}")
                    
            except Exception as debug_e:
                self.logger.debug(f"载荷诊断失败: {debug_e}")
                
            # 回退到原始处理
            try:
                handler(payload)
            except Exception as handler_e:
                self.logger.error(f"❌ 回退处理也失败 {method}: {handler_e}")
    
    def _handle_system_message(self, message: str):
        """处理系统消息"""
        self.logger.info(f"⚙️ 收到系统消息: {message}")
        
        # 首先尝试直接匹配处理器
        if message in self.system_handlers:
            handler = self.system_handlers[message]
            try:
                handler(message)
                self.logger.debug(f"✅ 系统消息处理成功: {message}")
                return
            except Exception as e:
                self.logger.error(f"❌ 系统消息处理失败 {message}: {e}")
                return
        

        # 尝试解析房间事件格式: room_id\tplayer_id+event_type
        parsed_event = self._parse_room_event(message)
        if parsed_event:
            self._handle_room_event(parsed_event)
            return
        
        # 检查是否有通用处理器
        if 'default' in self.system_handlers:
            try:
                self.system_handlers['default'](message)
                self.logger.debug(f"✅ 通用系统消息处理成功: {message}")
                return
            except Exception as e:
                self.logger.error(f"❌ 通用系统消息处理失败 {message}: {e}")
        
        self.logger.warning(f"⚠️ 未处理的系统消息: {message}")
    
    def _parse_room_event(self, message: str) -> Optional[Dict[str, str]]:
        """解析房间事件消息格式
        
        支持多种格式：
        1. 标准格式：room_id\tplayer_id+event_type 或 room_id\nplayer_id+event_type
        2. 复杂格式：room_id + binary_data + event_type + text
        
        Returns:
            Dict包含: room_id, player_id, event_type, message_text
            如果解析失败返回None
        """
        try:
            # 移除首尾空白字符
            message = message.strip()
            if not message:
                return None
            
            # 首先尝试复杂格式解析，因为它更灵活
            complex_result = self._parse_complex_room_event(message)
            if complex_result:
                return complex_result
            
            # 如果复杂格式失败，尝试标准格式：支持制表符或换行符分隔
            if '\t' in message or '\n' in message:
                return self._parse_standard_room_event(message)
            
        except Exception as e:
            self.logger.debug(f"🔍 房间事件解析失败: {e}")
        
        return None
    
    def _parse_standard_room_event(self, message: str) -> Optional[Dict[str, str]]:
        """解析标准房间事件格式：room_id\tplayer_id+event_type 或 room_id\nplayer_id+event_type"""
        try:
            # 分割房间ID和玩家事件，支持制表符或换行符
            if '\t' in message:
                parts = message.split('\t')
            else:
                parts = message.split('\n')
            
            # 过滤空行和空白内容
            parts = [p.strip() for p in parts if p.strip()]
            
            if len(parts) < 2:
                return None
            
            room_id = parts[0].strip()
            player_event = parts[1].strip()
            
            # 解析玩家ID和事件类型
            event_types = ['START_GAME', 'PLACE_PIECE', 'UNREADY', 'READY', 'LEAVE', 'JOIN']
            
            player_id = ""
            event_type = ""
            
            # 首先尝试标准匹配
            for event in event_types:
                if event in player_event:
                    # 查找事件类型在字符串中的位置
                    event_pos = player_event.find(event)
                    if event_pos >= 0:
                        # 提取事件类型之前的部分作为player_id，并清理空白字符
                        player_id = player_event[:event_pos].strip()
                        event_type = event
                        break
            
            # 如果没有匹配到标准事件类型，尝试其他可能的分割方式
            if not event_type:
                import re
                match = re.match(r'(.+?)([A-Z_]+)', player_event)
                if match:
                    player_id = match.group(1)
                    event_type = match.group(2)
            
            if room_id and event_type:
                # 进一步清理player_id，移除可能的特殊字符
                clean_player_id = player_id or 'unknown'
                if clean_player_id != 'unknown':
                    # 移除换行符、制表符和其他空白字符
                    clean_player_id = clean_player_id.replace('\n', '').replace('\t', '').replace('\r', '').strip()
                    # 如果包含特殊字符，只保留字母数字和下划线
                    import re
                    clean_player_id = re.sub(r'[^a-zA-Z0-9_]', '', clean_player_id)
                
                return {
                    'room_id': room_id,
                    'player_id': clean_player_id,
                    'event_type': event_type,
                    'message_text': '',
                    'raw_message': message
                }
                
        except Exception as e:
            self.logger.debug(f"🔍 标准格式解析失败: {e}")
        
        return None
    
    def _parse_complex_room_event(self, message: str) -> Optional[Dict[str, str]]:
        """解析复杂房间事件格式：包含房间ID、事件类型和消息文本"""
        try:
            import re
            
            # 查找可能的事件类型
            event_types = ['GAME_START', 'START_GAME', 'PLACE_PIECE', 'PLAYER_JOIN', 'PLAYER_LEAVE', 'PLAYER_READY']
            
            event_type = None
            for event in event_types:
                if event in message:
                    event_type = event
                    break
            
            if not event_type:
                return None
            
            # 尝试提取房间ID和消息文本
            lines = message.split('\n')
            room_id = None
            message_text = ""
            
            # 更灵活的房间ID匹配
            for line in lines:
                line_clean = line.strip()
                if not line_clean:
                    continue
                
                # 检查是否看起来像房间ID（6-12位字符，包含字母和数字）
                if re.match(r'^[a-f0-9]{6,12}$', line_clean):
                    room_id = line_clean
                elif '"' in line:
                    # 提取引号中的文本
                    match = re.search(r'"([^"]+)"', line)
                    if match:
                        message_text = match.group(1)
                elif '游戏开始' in line:
                    # 提取中文消息
                    message_text = "游戏开始！"
                elif event_type in line:
                    # 这一行包含事件类型，尝试从原始行中提取房间ID
                    # 处理可能包含二进制字符的情况
                    room_matches = re.findall(r'([a-f0-9]{6,12})', line)
                    if room_matches and not room_id:
                        room_id = room_matches[0]
            
            # 如果没有找到明确的房间ID，尝试从消息开头提取
            if not room_id:
                # 查找消息开头的可能房间ID
                first_part = message.split('\n')[0] if message else ""
                # 尝试提取可能的房间ID格式（处理二进制字符干扰）
                matches = re.findall(r'([a-f0-9]{6,12})', first_part)
                if matches:
                    room_id = matches[0]
            
            # 如果仍然没有找到，尝试更宽松的匹配
            if not room_id:
                # 在整个消息中查找可能的房间ID
                matches = re.findall(r'([a-f0-9]{6,12})', message)
                if matches:
                    room_id = matches[0]
            
            if room_id and event_type:
                # 标准化事件类型
                if event_type == 'GAME_START':
                    event_type = 'START_GAME'
                    
                return {
                    'room_id': room_id,
                    'player_id': 'system',  # 复杂格式通常是系统事件
                    'event_type': event_type,
                    'message_text': message_text or "系统事件",
                    'raw_message': message
                }
                
        except Exception as e:
            self.logger.debug(f"🔍 复杂格式解析失败: {e}")
        
        return None
    
    def _handle_room_event(self, event: Dict[str, str]):
        """处理房间事件
        
        Args:
            event: 包含room_id, player_id, event_type的字典
        """
        room_id = event['room_id']
        player_id = event['player_id']
        event_type = event['event_type']
        
        self.logger.info(f"🏠 房间事件: {room_id} - {player_id} {event_type}")
        
        # 构建处理器键名
        handler_keys = [
            f"room_event_{event_type.lower()}",  # room_event_join
            f"room_{event_type.lower()}",        # room_join
            event_type.lower(),                  # join
            "room_event",                        # 通用房间事件
        ]
        
        # 尝试找到合适的处理器
        handler_found = False
        for key in handler_keys:
            if key in self.system_handlers:
                try:
                    self.system_handlers[key](event)
                    self.logger.debug(f"✅ 房间事件处理成功: {key}")
                    handler_found = True
                    break
                except Exception as e:
                    self.logger.error(f"❌ 房间事件处理失败 {key}: {e}")
        
        if not handler_found:
            # 记录未处理的房间事件
            self.logger.info(f"📣 未处理的房间事件: {event_type} in {room_id} by {player_id}")
            
            # 如果有通用房间事件处理器，使用它
            if 'default_room_event' in self.system_handlers:
                try:
                    self.system_handlers['default_room_event'](event)
                    self.logger.debug(f"✅ 通用房间事件处理成功")
                except Exception as e:
                    self.logger.error(f"❌ 通用房间事件处理失败: {e}")
    
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
    
    def get_game_progress(self, room_id: str) -> str:
        """获取游戏进度（包含完整房间信息和游戏状态）
        
        Args:
            room_id: 房间ID
            
        Returns:
            str: 请求ID（用于响应匹配）
        """
        request_id = self._generate_request_id("get_game_progress")
        
        progress_req = GetGameProgressRequest()
        progress_req.room_id = room_id
        
        self._send_game_request(request_id, "GetGameProgress", progress_req)
        self.logger.info(f"📤 发送获取游戏进度请求: 房间ID={room_id}")
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
            "GetRoomList": self._get_response_handler("get_room_list"),
            "GetGameProgress": self._get_response_handler("get_game_progress")  # 为将来使用做准备
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
    
    def handle_game_state_notify(self, game_state_event):
        """处理游戏状态通知（GameStateNotify）
        
        Args:
            game_state_event: GameStateNotify protobuf对象
        """
        room_id = game_state_event.room_id
        event_type = game_state_event.event_type
        event_message = game_state_event.event_message
        
        self.logger.info(f"🎮 处理游戏状态通知: 房间={room_id}, 事件={event_type}, 消息={event_message}")
        
        # 检查是否有棋盘状态更新
        if hasattr(game_state_event, 'game_state') and game_state_event.game_state:
            game_state = game_state_event.game_state
            self.logger.info(f"📋 游戏状态更新: 当前轮次={game_state.current_turn}, 总步数={game_state.total_moves}")
            
            # 处理最新的移动
            if game_state.move_history:
                latest_move = game_state.move_history[-1]
                self.logger.info(f"🔸 最新移动: 玩家={latest_move.player_id}, 位置=({latest_move.x}, {latest_move.y}), 颜色={latest_move.color}")
        
        # 根据事件类型进行处理
        if event_type == "MOVE" or "下棋" in event_message:
            self.logger.info(f"♟️ 玩家下棋事件: {event_message}")
        elif event_type == "WIN":
            self.logger.info(f"🏆 游戏获胜事件: {event_message}")
        elif event_type == "DRAW":
            self.logger.info(f"🤝 游戏平局事件: {event_message}")
        elif event_type == "START":
            self.logger.info(f"🚀 游戏开始事件: {event_message}")
        else:
            self.logger.info(f"📢 其他游戏事件: {event_type} - {event_message}")
        
        # 尝试调用注册的游戏状态处理器
        handler_keys = [
            f"game_state_{event_type.lower()}",  # game_state_move
            f"game_{event_type.lower()}",        # game_move  
            "game_state_notify",                 # 通用游戏状态通知
            "default"                            # 默认处理器
        ]
        
        handler_found = False
        for key in handler_keys:
            if key in self.system_handlers:
                try:
                    self.system_handlers[key](game_state_event)
                    self.logger.debug(f"✅ 游戏状态事件处理成功: {key}")
                    handler_found = True
                    break
                except Exception as e:
                    self.logger.error(f"❌ 游戏状态事件处理失败 {key}: {e}")
        
        if not handler_found:
            # 尝试调用通知处理器（为了兼容GUI的处理器）
            if "GameStateChangedNotification" in self.notification_handlers:
                try:
                    self.notification_handlers["GameStateChangedNotification"](game_state_event)
                    self.logger.debug(f"✅ 通过通知处理器处理游戏状态事件成功")
                    return
                except Exception as e:
                    self.logger.error(f"❌ 通知处理器处理游戏状态事件失败: {e}")
            
            self.logger.debug(f"📣 没有找到专门的游戏状态处理器，事件已记录: {event_type}")
    
    def handle_piece_placed_notification(self, piece_response):
        """处理下棋通知（PlacePieceResponse格式）
        
        Args:
            piece_response: PlacePieceResponse对象，包含游戏状态和下棋信息
        """
        self.logger.info(f"♟️ 处理下棋通知: 成功={piece_response.success}, 消息={piece_response.message}")
        
        # 提取下棋位置信息
        if piece_response.game_state and piece_response.game_state.move_history:
            last_move = piece_response.game_state.move_history[-1]
            self.logger.info(f"📍 对手下棋位置: 玩家={last_move.player_id}, 位置=({last_move.x}, {last_move.y}), 颜色={'⚫' if last_move.color == 1 else '⚪'}")
            
            # 构造详细的下棋通知信息
            move_info = {
                'success': piece_response.success,
                'message': piece_response.message,
                'player_id': last_move.player_id,
                'x': last_move.x,
                'y': last_move.y,
                'color': last_move.color,
                'move_number': last_move.move_number,
                'current_turn': piece_response.game_state.current_turn,
                'total_moves': piece_response.game_state.total_moves,
                'game_result': piece_response.game_state.result,
                'board': list(piece_response.game_state.board) if piece_response.game_state.board else [],
                'game_state': piece_response.game_state
            }
            
            # 尝试调用注册的下棋通知处理器
            handler_keys = [
                "piece_placed_notification",  # 下棋通知（新格式）
                "move_notification",          # 下棋通知（旧格式兼容）
                "piece_placed",              # 落子通知
                "game_move",                 # 游戏移动
                "default"                    # 默认处理器
            ]
            
            handler_found = False
            for key in handler_keys:
                if key in self.system_handlers:
                    try:
                        self.system_handlers[key](move_info)
                        self.logger.debug(f"✅ 下棋通知处理成功: {key}")
                        handler_found = True
                        break
                    except Exception as e:
                        self.logger.error(f"❌ 下棋通知处理失败 {key}: {e}")
            
            if not handler_found:
                self.logger.info(f"🎯 棋盘更新: 位置({last_move.x}, {last_move.y}) = {'⚫' if last_move.color == 1 else '⚪'}")
        else:
            self.logger.warning(f"⚠️ 下棋通知缺少游戏状态或走棋历史信息")

    def handle_move_notification(self, notification):
        """处理下棋成功通知（简单格式 - 向后兼容）
        
        Args:
            notification: 简单通知格式的字典，包含status和message
        """
        self.logger.info(f"♟️ 处理下棋成功通知: {notification['message']}")
        
        # 尝试调用注册的下棋通知处理器
        handler_keys = [
            "move_notification",      # 下棋通知
            "piece_placed",          # 落子通知
            "game_move",             # 游戏移动
            "default"                # 默认处理器
        ]
        
        handler_found = False
        for key in handler_keys:
            if key in self.system_handlers:
                try:
                    self.system_handlers[key](notification)
                    self.logger.debug(f"✅ 下棋通知处理成功: {key}")
                    handler_found = True
                    break
                except Exception as e:
                    self.logger.error(f"❌ 下棋通知处理失败 {key}: {e}")
        
        if not handler_found:
            self.logger.debug(f"📣 没有找到专门的下棋通知处理器，通知已记录")
    
    def handle_simple_notification(self, notification):
        """处理简单通知（非下棋通知）
        
        Args:
            notification: 简单通知格式的字典，包含status和message
        """
        self.logger.info(f"📢 处理简单通知: {notification['message']}")
        
        # 尝试调用注册的简单通知处理器
        if "simple_notification" in self.system_handlers:
            try:
                self.system_handlers["simple_notification"](notification)
                self.logger.debug(f"✅ 简单通知处理成功")
                return
            except Exception as e:
                self.logger.error(f"❌ 简单通知处理失败: {e}")
        
        # 如果没有专门的处理器，使用默认处理器
        if "default" in self.system_handlers:
            try:
                self.system_handlers["default"](notification)
                self.logger.debug(f"✅ 默认通知处理成功")
            except Exception as e:
                self.logger.error(f"❌ 默认通知处理失败: {e}")
        else:
            self.logger.debug(f"📣 没有找到简单通知处理器，通知已记录")
