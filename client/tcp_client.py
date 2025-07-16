import socket
import struct
import threading
import time
import logging
from typing import Optional, Callable
from common_pb2 import GameMessage, HeadMessage, MessageType, ServiceMessageType, GameMessageResponse

class TCPClient:
    """TCP客户端，负责与gatesvr的连接和消息收发"""
    
    def __init__(self, host: str = "127.0.0.1", port: int = 6001):
        self.host = host
        self.port = port
        self.socket: Optional[socket.socket] = None
        self.connected = False
        self.running = False
        self.receive_thread: Optional[threading.Thread] = None
        self.heartbeat_thread: Optional[threading.Thread] = None
        self.message_handler = None  # MessageHandler对象
        self.logger = logging.getLogger(__name__)
        
        # 心跳配置（按照gatesvr接口说明）
        self.heartbeat_interval = 25  # 25秒发送一次心跳
        self.first_message_timeout = 5  # 首条消息超时时间
        self.last_heartbeat_time = 0
        self.player_id = ""
        self.token = ""  # 认证Token
        
        # 消息接收统计
        self.last_message_time = time.time()
    
    def set_message_handler(self, handler):
        """设置消息处理器
        
        Args:
            handler: MessageHandler对象或回调函数（为了兼容性）
        """
        self.message_handler = handler
    
    def connect(self, player_id: str = "", token: str = "") -> bool:
        """连接到gatesvr
        
        Args:
            player_id: 玩家ID
            token: 认证Token（如果启用认证）
        """
        try:
            self.socket = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
            self.socket.settimeout(10)  # 10秒连接超时
            self.socket.connect((self.host, self.port))
            self.connected = True
            self.running = True
            self.player_id = player_id or f"player_{int(time.time() * 1000) % 100000}"
            self.token = token
            
            self.logger.info(f"TCP连接建立成功: {self.host}:{self.port}")
            
            # 根据 gatesvr 要求，连接后必须在5秒内发送首条心跳消息
            if not self._send_initial_heartbeat():
                self.logger.error("发送首条心跳失败")
                self.disconnect()
                return False
            
            # 启动接收线程
            self.receive_thread = threading.Thread(target=self._receive_loop, daemon=True)
            self.receive_thread.start()
            
            # 启动心跳线程
            self.heartbeat_thread = threading.Thread(target=self._heartbeat_loop, daemon=True)
            self.heartbeat_thread.start()
            
            self.logger.info(f"✅ 成功连接到GateServer，玩家ID: {self.player_id}")
            return True
            
        except Exception as e:
            self.logger.error(f"❌ 连接失败: {e}")
            self.connected = False
            return False
    
    def _send_initial_heartbeat(self) -> bool:
        """发送首条心跳消息（连接建立后5秒内必须发送）"""
        try:
            heartbeat = GameMessage()
            heartbeat.msg_head.player_id = self.player_id
            if self.token:
                heartbeat.msg_head.token = self.token
            heartbeat.msg_head.timestamp = int(time.time() * 1000)
            heartbeat.msg_type = MessageType.HEARTBEAT
            
            if self.send_message(heartbeat):
                self.last_heartbeat_time = time.time()
                self.logger.info(f"💓 发送首条心跳消息成功，玩家ID: {self.player_id}")
                return True
            return False
        except Exception as e:
            self.logger.error(f"发送首条心跳失败: {e}")
            return False
    
    def _heartbeat_loop(self):
        """心跳循环线程 - 每25秒发送一次心跳"""
        while self.running and self.connected:
            try:
                time.sleep(self.heartbeat_interval)
                if self.connected and self.running:
                    if self._send_heartbeat():
                        self.logger.debug("💓 心跳发送成功")
                    else:
                        self.logger.warning("⚠️ 心跳发送失败")
                        
            except Exception as e:
                self.logger.error(f"❌ 心跳线程异常: {e}")
                break
    
    def _send_heartbeat(self) -> bool:
        """发送心跳消息"""
        try:
            heartbeat = GameMessage()
            heartbeat.msg_head.player_id = self.player_id
            if self.token:
                heartbeat.msg_head.token = self.token
            heartbeat.msg_head.timestamp = int(time.time() * 1000)
            heartbeat.msg_type = MessageType.HEARTBEAT
            
            if self.send_message(heartbeat):
                self.last_heartbeat_time = time.time()
                return True
            return False
        except Exception as e:
            self.logger.error(f"发送心跳消息失败: {e}")
            return False
    
    def disconnect(self):
        """断开连接"""
        self.logger.info("�� 断开连接中...")
        self.running = False
        self.connected = False
        
        if self.socket:
            try:
                self.socket.close()
            except:
                pass
            self.socket = None
            
        # 等待线程结束
        if self.receive_thread and self.receive_thread.is_alive():
            self.receive_thread.join(timeout=2)
            
        if self.heartbeat_thread and self.heartbeat_thread.is_alive():
            self.heartbeat_thread.join(timeout=2)
            
        self.logger.info("✅ 连接已断开")
    
    def _handle_received_message(self, game_message: GameMessage):
        """处理接收到的消息"""
        try:
            msg_type = game_message.msg_type
            service_name = game_message.msg_head.service_name if game_message.msg_head else ""
            request_id = game_message.msg_head.request_id if game_message.msg_head else ""
            
            self.logger.debug(f"📥 收到消息详情: type={msg_type}, service={service_name}, request_id={request_id}")
            
            if msg_type == MessageType.HEARTBEAT:
                # 心跳响应，通常不需要特殊处理
                self.logger.debug("💓 收到心跳响应")
                
            elif msg_type == MessageType.SERVICE_MESSAGE:
                self.logger.debug(f"📥 收到推送消息: {msg_type}, 服务: {service_name}, 方法: {request_id}")
                
                # 检查是否是响应消息还是推送通知
                if game_message.payload:
                    try:
                        # 尝试解析为响应消息
                        response = GameMessageResponse()
                        response.ParseFromString(game_message.payload)
                        self.logger.debug(f"🎯 这是一个响应消息: ret={response.ret}")
                        if hasattr(self.message_handler, 'handle_response'):
                            self.message_handler.handle_response(response)
                        elif callable(self.message_handler):
                            # 兼容旧的回调函数接口
                            self.message_handler(response, False)
                        else:
                            self.logger.warning("无法处理响应：message_handler不可调用")
                    except Exception as e:
                        self.logger.debug(f"📢 这是一个推送通知: {e}")
                        # 不是响应消息，作为推送通知处理
                        if hasattr(self.message_handler, 'handle_notification'):
                            self.message_handler.handle_notification(game_message)
                        elif callable(self.message_handler):
                            # 兼容旧的回调函数接口
                            self.message_handler(game_message, True)
                        else:
                            self.logger.warning("无法处理推送通知：message_handler不可调用")
                else:
                    self.logger.debug("📢 这是一个无payload的推送通知")
                    if hasattr(self.message_handler, 'handle_notification'):
                        self.message_handler.handle_notification(game_message)
                    elif callable(self.message_handler):
                        # 兼容旧的回调函数接口
                        self.message_handler(game_message, True)
                    else:
                        self.logger.warning("无法处理推送通知：message_handler不可调用")
                    
            elif msg_type == MessageType.CLIENT_MESSAGE:
                self.logger.debug(f"📥 收到客户端消息")
                # 网关内部消息 - 使用消息头中的request_id来确定通知类型
                notification_type = request_id if request_id else ""
                self.logger.debug(f"🔍 CLIENT_MESSAGE 通知类型: {notification_type}")
                
                if game_message.payload:
                    # 添加调试信息，显示消息的前几个字节
                    hex_sample = game_message.payload[:20].hex() if len(game_message.payload) >= 20 else game_message.payload.hex()
                    self.logger.debug(f"🔍 CLIENT_MESSAGE payload前20字节(hex): {hex_sample}")
                    
                    # 根据通知类型进行具体处理
                    if notification_type == "PiecePlacedNotification":
                        try:
                            from gomoku_pb2 import PlacePieceResponse
                            piece_response = PlacePieceResponse()
                            piece_response.ParseFromString(game_message.payload)
                            
                            self.logger.info(f"♟️ 收到下棋通知: 成功={piece_response.success}, 消息={piece_response.message}")
                            
                            # 提取下棋位置信息
                            if piece_response.game_state and piece_response.game_state.move_history:
                                last_move = piece_response.game_state.move_history[-1]
                                self.logger.info(f"📍 最新下棋位置: 玩家={last_move.player_id}, 位置=({last_move.x}, {last_move.y}), 颜色={last_move.color}")
                            
                            if hasattr(self.message_handler, 'handle_piece_placed_notification'):
                                self.message_handler.handle_piece_placed_notification(piece_response)
                            
                            return  # 成功处理下棋通知
                            
                        except Exception as e:
                            self.logger.error(f"❌ 解析下棋通知失败: {e}")
                    
                    elif notification_type == "GameStartNotification":
                        try:
                            from gomoku_pb2 import GameStateNotify
                            game_state_event = GameStateNotify()
                            game_state_event.ParseFromString(game_message.payload)
                            
                            self.logger.info(f"🎮 收到GameStateNotify: room={game_state_event.room_id}, event={game_state_event.event_type}, message={game_state_event.event_message}")
                            
                            if hasattr(self.message_handler, 'handle_game_state_notify'):
                                self.message_handler.handle_game_state_notify(game_state_event)
                            
                            return  # 成功处理游戏开始通知
                            
                        except Exception as e:
                            self.logger.error(f"❌ 解析游戏开始通知失败: {e}")
                    
                    elif notification_type in ["PlayerJoinedNotification", "PlayerLeftNotification", "PlayerReadyNotification"]:
                        try:
                            from gomoku_pb2 import PlayerEventNotify
                            player_event = PlayerEventNotify()
                            player_event.ParseFromString(game_message.payload)
                            
                            self.logger.info(f"⚙️ 收到PlayerEventNotify: room={player_event.room_id}, player={player_event.player_id}, event={player_event.event_type}")
                            
                            if hasattr(self.message_handler, 'handle_room_event_direct'):
                                # 构建事件字典并直接处理
                                event = {
                                    'room_id': player_event.room_id,
                                    'player_id': player_event.player_id,
                                    'event_type': player_event.event_type,
                                    'message_text': '',
                                    'raw_message': game_message.payload
                                }
                                self.message_handler.handle_room_event_direct(event)
                            
                            return  # 成功处理玩家事件通知
                            
                        except Exception as e:
                            self.logger.error(f"❌ 解析玩家事件通知失败: {e}")
                    
                    # 如果没有匹配的通知类型，尝试通用处理
                    self.logger.debug(f"🔍 未知通知类型 {notification_type}，尝试通用解析")
                    
                    # 首先尝试作为protobuf数据处理
                    try:
                        # 检查是否为PlayerEventNotify
                        from gomoku_pb2 import PlayerEventNotify
                        player_event = PlayerEventNotify()
                        player_event.ParseFromString(game_message.payload)
                        
                        self.logger.info(f"⚙️ 收到PlayerEventNotify: room={player_event.room_id}, player={player_event.player_id}, event={player_event.event_type}")
                        
                        if hasattr(self.message_handler, 'handle_room_event_direct'):
                            # 构建事件字典并直接处理
                            event = {
                                'room_id': player_event.room_id,
                                'player_id': player_event.player_id,
                                'event_type': player_event.event_type,
                                'message_text': '',
                                'raw_message': game_message.payload
                            }
                            self.message_handler.handle_room_event_direct(event)
                        
                        return  # 成功处理protobuf消息，直接返回
                        
                    except Exception as protobuf_error:
                        self.logger.debug(f"🔍 不是PlayerEventNotify，尝试GameStateNotify: {protobuf_error}")
                    
                    # 尝试解析为GameStateNotify (游戏状态通知)
                    try:
                        from gomoku_pb2 import GameStateNotify
                        game_state_event = GameStateNotify()
                        game_state_event.ParseFromString(game_message.payload)
                        
                        self.logger.info(f"🎮 收到GameStateNotify: room={game_state_event.room_id}, event={game_state_event.event_type}, message={game_state_event.event_message}")
                        
                        if hasattr(self.message_handler, 'handle_game_state_notify'):
                            self.message_handler.handle_game_state_notify(game_state_event)
                        
                        return  # 成功处理GameStateNotify消息，直接返回
                        
                    except Exception as game_state_error:
                        self.logger.debug(f"🔍 不是GameStateNotify，尝试简单通知格式: {game_state_error}")
                    
                    # 尝试解析为简单的通知消息格式 (status_code + message_text + extra_data)
                    try:
                        notification = self._parse_simple_notification(game_message.payload)
                        if notification:
                            self.logger.info(f"💬 收到简单通知: 状态={notification['status']}, 消息='{notification['message']}'")
                            
                            # 处理下棋成功通知
                            if "下棋" in notification['message'] and "成功" in notification['message']:
                                if hasattr(self.message_handler, 'handle_move_notification'):
                                    self.message_handler.handle_move_notification(notification)
                                else:
                                    self.logger.info(f"♟️ 处理下棋成功通知: {notification['message']}")
                            else:
                                # 其他类型的通知
                                if hasattr(self.message_handler, 'handle_simple_notification'):
                                    self.message_handler.handle_simple_notification(notification)
                                else:
                                    self.logger.info(f"📢 系统通知: {notification['message']}")
                            
                            return  # 成功处理简单通知，直接返回
                            
                    except Exception as simple_error:
                        self.logger.debug(f"🔍 不是简单通知格式，尝试文本处理: {simple_error}")
                    
                    # 如果都不是protobuf，尝试作为文本消息处理
                    message = ""
                    try:
                        message = game_message.payload.decode('utf-8')
                    except UnicodeDecodeError as e:
                        self.logger.warning(f"⚠️ 客户端消息包含非UTF-8数据，使用错误处理: {e}")
                        message = game_message.payload.decode('utf-8', errors='replace')
                    except Exception as e:
                        self.logger.error(f"❌ 解码客户端消息失败: {e}")
                        message = str(game_message.payload)
                        
                    self.logger.info(f"⚙️ 收到系统消息: {message}")
                    
                    if hasattr(self.message_handler, 'handle_system_message'):
                        self.message_handler.handle_system_message(message)
                    elif callable(self.message_handler):
                        # 兼容旧的回调函数接口
                        self.message_handler(message, True)
                    else:
                        self.logger.warning("无法处理系统消息：message_handler不可调用")
                    
            elif msg_type == MessageType.BROADCAST_MESSAGE:
                self.logger.debug(f"📥 收到广播消息")
                # 广播消息
                if hasattr(self.message_handler, 'handle_notification'):
                    self.message_handler.handle_notification(game_message)
                elif callable(self.message_handler):
                    # 兼容旧的回调函数接口
                    self.message_handler(game_message, True)
                else:
                    self.logger.warning("无法处理广播消息：message_handler不可调用")
                
            else:
                self.logger.warning(f"⚠️ 未知消息类型: {msg_type}")
                
        except Exception as e:
            self.logger.error(f"❌ 消息处理失败: {e}")
            # 添加更详细的错误信息和诊断
            try:
                msg_type = game_message.msg_type if game_message else "unknown"
                service_name = game_message.msg_head.service_name if game_message and game_message.msg_head else "unknown"
                request_id = game_message.msg_head.request_id if game_message and game_message.msg_head else "unknown"
                payload_size = len(game_message.payload) if game_message and game_message.payload else 0
                
                self.logger.debug(f"消息诊断: type={msg_type}, service={service_name}, request_id={request_id}, payload_size={payload_size}")
                
                # 如果是编码错误，提供更多调试信息
                if "codec can't decode" in str(e):
                    self.logger.debug(f"编码错误详情: 可能是protobuf消息被错误地当作文本处理")
                    if game_message and game_message.payload:
                        hex_sample = game_message.payload[:20].hex() if len(game_message.payload) >= 20 else game_message.payload.hex()
                        self.logger.debug(f"Payload前20字节(hex): {hex_sample}")
                        
            except Exception as debug_e:
                self.logger.debug(f"消息诊断失败: {debug_e}")
    
    def send_message(self, game_message: GameMessage) -> bool:
        """发送游戏消息
        
        Args:
            game_message: 要发送的游戏消息
            
        Returns:
            bool: 发送是否成功
        """
        if not self.connected or not self.socket:
            self.logger.error("❌ 未连接到服务器，无法发送消息")
            return False
            
        try:
            # 序列化消息
            data = game_message.SerializeToString()
            
            # 检查消息大小（gatesvr限制4MB）
            if len(data) > 4 * 1024 * 1024:
                self.logger.error(f"❌ 消息太大: {len(data)} bytes")
                return False
            
            # 发送消息长度（4字节，小端序）
            length_header = struct.pack('<I', len(data))
            self.socket.sendall(length_header)
            
            # 发送消息内容
            self.socket.sendall(data)
            
            # 记录发送的消息类型
            msg_type_name = MessageType.Name(game_message.msg_type)
            service_name = game_message.msg_head.service_name
            request_id = game_message.msg_head.request_id
            
            self.logger.debug(f"📤 发送消息: {msg_type_name}, 服务: {service_name}, 方法: {request_id}")
            return True
            
        except Exception as e:
            self.logger.error(f"❌ 发送消息失败: {e}")
            return False
    
    def _receive_loop(self):
        """接收消息循环"""
        self.logger.info("🎧 消息接收线程启动")
        
        while self.running and self.connected:
            try:
                # 接收消息长度（4字节）
                length_data = self._recv_exact(4)
                if not length_data:
                    break
                    
                length = struct.unpack('<I', length_data)[0]
                
                # 检查消息长度合理性
                if length > 4 * 1024 * 1024:  # 4MB限制
                    self.logger.error(f"❌ 收到异常大小的消息: {length} bytes")
                    break
                
                # 接收消息内容
                message_data = self._recv_exact(length)
                if not message_data:
                    break
                
                # 更新最后消息时间
                self.last_message_time = time.time()
                
                # 解析并处理消息
                self._parse_and_handle_message(message_data)
                
            except Exception as e:
                if self.running:
                    self.logger.error(f"❌ 接收消息失败: {e}")
                break
        
        self.connected = False
        self.logger.info("🎧 消息接收线程退出")
    
    def _parse_and_handle_message(self, message_data: bytes):
        """解析并处理收到的消息"""
        try:
            # 解析为GameMessage
            game_message = GameMessage()
            game_message.ParseFromString(message_data)
            
            # 使用新的消息处理逻辑
            self._handle_received_message(game_message)
                    
        except Exception as e:
            self.logger.error(f"❌ 消息解析失败: {e}")
            # 打印十六进制数据用于调试
            hex_data = message_data[:100].hex() if len(message_data) > 100 else message_data.hex()
            self.logger.debug(f"原始数据(前100字节): {hex_data}")
    
    def _parse_simple_notification(self, payload: bytes) -> dict:
        """解析简单的通知消息格式
        
        格式: field1(status) + field2(message_text) + field3(extra_data)
        
        Returns:
            dict: 包含status, message, extra_data的字典，解析失败返回None
        """
        try:
            offset = 0
            fields = {}
            
            # 解析字段1: 状态码 (varint)
            if offset >= len(payload):
                return None
                
            tag = payload[offset]
            offset += 1
            
            if (tag >> 3) != 1 or (tag & 0x7) != 0:  # 期望field1是varint
                return None
                
            status = payload[offset]
            offset += 1
            fields['status'] = status
            
            # 解析字段2: 消息文本 (length-delimited)
            if offset >= len(payload):
                return None
                
            tag = payload[offset]
            offset += 1
            
            if (tag >> 3) != 2 or (tag & 0x7) != 2:  # 期望field2是length-delimited
                return None
                
            msg_length = payload[offset]
            offset += 1
            
            if offset + msg_length > len(payload):
                return None
                
            message_bytes = payload[offset:offset + msg_length]
            offset += msg_length
            
            try:
                message = message_bytes.decode('utf-8')
                fields['message'] = message
            except UnicodeDecodeError:
                return None
            
            # 解析字段3: 额外数据 (可选)
            extra_data = None
            if offset < len(payload):
                tag = payload[offset]
                if (tag >> 3) == 3 and (tag & 0x7) == 2:  # field3是length-delimited
                    offset += 1
                    # 这里可能是varint长度，暂时简单处理
                    # 由于数据可能被截断，我们不强制要求解析成功
                    extra_data = payload[offset:]
            
            fields['extra_data'] = extra_data
            return fields
            
        except Exception as e:
            self.logger.debug(f"简单通知解析失败: {e}")
            return None
    
    def _handle_system_message(self, game_message: GameMessage):
        """处理系统消息（CLIENT_MESSAGE类型）"""
        try:
            # 安全地解码payload，处理可能的编码错误
            payload = ""
            if game_message.payload:
                try:
                    payload = game_message.payload.decode('utf-8')
                except UnicodeDecodeError as e:
                    self.logger.warning(f"⚠️ 系统消息包含非UTF-8数据，使用错误处理: {e}")
                    payload = game_message.payload.decode('utf-8', errors='replace')
                except Exception as e:
                    self.logger.error(f"❌ 解码系统消息失败: {e}")
                    payload = str(game_message.payload)
            
            if "AUTH_FAILED" in payload:
                self.logger.error(f"🔐 认证失败: {payload}")
                if self.message_handler:
                    # 通知上层处理认证失败
                    self.message_handler.handle_system_message("AUTH_FAILED")
                self.disconnect()
                
            elif "KICK" in payload:
                self.logger.warning(f"👋 被踢下线: {payload}")
                if self.message_handler:
                    self.message_handler.handle_system_message("KICKED")
                self.disconnect()
                
            elif "CONNECTION_AUTH_FAILED" in payload:
                self.logger.error(f"🔐 连接认证失败: {payload}")
                self.disconnect()
                
            else:
                self.logger.info(f"📢 系统消息: {payload}")
                
        except Exception as e:
            self.logger.error(f"❌ 处理系统消息失败: {e}")
    
    def _recv_exact(self, n: int) -> bytes:
        """精确接收n个字节"""
        data = b''
        while len(data) < n and self.running:
            try:
                packet = self.socket.recv(n - len(data))
                if not packet:
                    return b''
                data += packet
            except socket.timeout:
                if not self.running:
                    break
                continue
            except Exception:
                return b''
        return data
    
    def is_connected(self) -> bool:
        """检查连接状态"""
        return self.connected and self.running
    
    def get_connection_info(self) -> dict:
        """获取连接信息"""
        return {
            "connected": self.connected,
            "player_id": self.player_id,
            "last_heartbeat": self.last_heartbeat_time,
            "last_message": self.last_message_time,
            "host": self.host,
            "port": self.port
        }
