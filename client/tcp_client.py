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
                # 网关内部消息
                if game_message.payload:
                    message = game_message.payload.decode('utf-8')
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
            self.logger.debug(f"消息详情: {game_message}")
    
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
    
    def _handle_system_message(self, game_message: GameMessage):
        """处理系统消息（CLIENT_MESSAGE类型）"""
        try:
            payload = game_message.payload.decode('utf-8') if game_message.payload else ""
            
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
