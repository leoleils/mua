import tkinter as tk
from tkinter import ttk, messagebox, simpledialog
import logging
import threading
import time
from typing import Optional, List, Dict
from tcp_client import TCPClient
from message_handler import MessageHandler
from gomoku_pb2 import (
    GameState, RoomInfo, PlayerInfo, PlayerColor, 
    GameResult, GameStateNotify, PlayerEventNotify, GetRoomListResponse, RoomStatus
)

class RoomListWindow:
    """房间列表显示窗口"""
    
    def __init__(self, parent, room_list_data: GetRoomListResponse, join_callback):
        self.parent = parent
        self.room_list_data = room_list_data
        self.join_callback = join_callback
        
        # 创建弹窗
        self.window = tk.Toplevel(parent)
        self.window.title("房间列表")
        self.window.geometry("600x400")
        self.window.resizable(True, True)
        
        # 使窗口居中
        self.window.transient(parent)
        self.window.grab_set()
        
        self.setup_ui()
        
    def setup_ui(self):
        """设置UI"""
        # 标题
        title_frame = ttk.Frame(self.window)
        title_frame.pack(fill=tk.X, padx=10, pady=5)
        
        title_label = ttk.Label(title_frame, text="📋 房间列表", font=("Arial", 14, "bold"))
        title_label.pack(side=tk.LEFT)
        
        # 房间数量信息
        count_info = f"总共 {len(self.room_list_data.rooms)} 个房间"
        count_label = ttk.Label(title_frame, text=count_info, foreground="gray")
        count_label.pack(side=tk.RIGHT)
        
        # 房间列表
        list_frame = ttk.Frame(self.window)
        list_frame.pack(fill=tk.BOTH, expand=True, padx=10, pady=5)
        
        # 创建Treeview用于显示房间列表
        columns = ("room_id", "room_name", "players", "status", "password")
        self.tree = ttk.Treeview(list_frame, columns=columns, show="headings", height=15)
        
        # 设置列标题
        self.tree.heading("room_id", text="房间ID")
        self.tree.heading("room_name", text="房间名称")
        self.tree.heading("players", text="玩家数")
        self.tree.heading("status", text="状态")
        self.tree.heading("password", text="密码")
        
        # 设置列宽
        self.tree.column("room_id", width=100)
        self.tree.column("room_name", width=200)
        self.tree.column("players", width=80)
        self.tree.column("status", width=100)
        self.tree.column("password", width=80)
        
        # 添加滚动条
        scrollbar = ttk.Scrollbar(list_frame, orient=tk.VERTICAL, command=self.tree.yview)
        self.tree.configure(yscrollcommand=scrollbar.set)
        
        # 布局
        self.tree.pack(side=tk.LEFT, fill=tk.BOTH, expand=True)
        scrollbar.pack(side=tk.RIGHT, fill=tk.Y)
        
        # 填充数据
        self.populate_room_list()
        
        # 按钮区域
        button_frame = ttk.Frame(self.window)
        button_frame.pack(fill=tk.X, padx=10, pady=5)
        
        # 加入房间按钮
        ttk.Button(button_frame, text="加入选中房间", command=self.join_selected_room).pack(side=tk.LEFT, padx=5)
        
        # 刷新按钮
        ttk.Button(button_frame, text="刷新列表", command=self.refresh_room_list).pack(side=tk.LEFT, padx=5)
        
        # 关闭按钮
        ttk.Button(button_frame, text="关闭", command=self.window.destroy).pack(side=tk.RIGHT, padx=5)
        
        # 状态栏
        status_frame = ttk.Frame(self.window)
        status_frame.pack(fill=tk.X, padx=10, pady=5)
        
        self.status_label = ttk.Label(status_frame, text="💡 双击房间可直接加入", foreground="blue")
        self.status_label.pack(anchor=tk.W)
        
        # 绑定双击事件
        self.tree.bind("<Double-1>", self.on_double_click)
        
    def populate_room_list(self):
        """填充房间列表数据"""
        # 清空现有数据
        for item in self.tree.get_children():
            self.tree.delete(item)
        
        if not self.room_list_data.rooms:
            # 没有房间时显示提示
            self.tree.insert("", "end", values=("", "暂无房间", "", "", ""))
            return
        
        # 添加房间数据
        for room in self.room_list_data.rooms:
            # 状态
            if hasattr(room, 'status'):
                if room.status == RoomStatus.PLAYING:  # PLAYING = 1
                    status = "🎮 游戏中"
                elif room.status == RoomStatus.FINISHED:  # FINISHED = 2
                    status = "🏁 已结束"
                else:  # WAITING = 0 or default
                    status = "⏳ 等待中"
            else:
                # 兼容旧版本
                status = "🎮 游戏中" if getattr(room, 'is_gaming', False) else "⏳ 等待中"
            
            # 密码状态
            has_password = bool(getattr(room, 'password', '') and room.password.strip())
            password_status = "🔒 有密码" if has_password else "🔓 无密码"
            
            # 玩家数
            player_count = f"{len(room.players)}/2"
            
            # 插入数据
            item_id = self.tree.insert("", "end", values=(
                room.room_id,
                room.room_name,
                player_count,
                status,
                password_status
            ))
            
            # 根据状态设置不同的标签（颜色）
            if "游戏中" in status:
                self.tree.set(item_id, "status", status)
            elif "等待中" in status:
                self.tree.set(item_id, "status", status)
    
    def get_selected_room_id(self):
        """获取选中的房间ID"""
        selection = self.tree.selection()
        if not selection:
            return None
        
        item = selection[0]
        room_id = self.tree.item(item, "values")[0]
        return room_id if room_id else None
    
    def join_selected_room(self):
        """加入选中的房间"""
        room_id = self.get_selected_room_id()
        if not room_id:
            messagebox.showwarning("提示", "请先选择一个房间")
            return
        
        if room_id == "暂无房间":
            return
        
        # 检查是否需要密码
        selection = self.tree.selection()[0]
        values = self.tree.item(selection, "values")
        password_status = values[4]
        
        password = ""
        if "有密码" in password_status:
            password = simpledialog.askstring("房间密码", f"请输入房间 {room_id} 的密码:", show="*")
            if password is None:  # 用户取消
                return
        
        # 调用加入房间回调
        self.join_callback(room_id, password)
        self.status_label.config(text=f"正在加入房间 {room_id}...", foreground="orange")
    
    def on_double_click(self, event):
        """双击房间时直接加入"""
        self.join_selected_room()
    
    def refresh_room_list(self):
        """刷新房间列表"""
        self.status_label.config(text="正在刷新房间列表...", foreground="orange")
        # 这里可以触发重新获取房间列表
        # 由于需要异步操作，这里只是显示提示
        messagebox.showinfo("提示", "请点击主界面的'房间列表'按钮重新获取")

class GomokuGUI:
    """五子棋GUI界面"""
    
    def __init__(self):
        self.root = tk.Tk()
        self.root.title("五子棋客户端")
        self.root.geometry("1000x700")
        
        # 网络组件
        self.tcp_client: Optional[TCPClient] = None
        self.message_handler: Optional[MessageHandler] = None
        self.player_id = ""
        self.current_room: Optional[RoomInfo] = None
        self.current_room_id = ""
        self.is_ready = False
        self.game_state: Optional[GameState] = None
        self.is_owner = False  # 是否是房主
        
        # GUI组件
        self.board_canvas: Optional[tk.Canvas] = None
        self.board_size = 15
        self.cell_size = 30
        self.board_data = [[0 for _ in range(15)] for _ in range(15)]
        
        # 日志设置
        logging.basicConfig(level=logging.INFO)
        self.logger = logging.getLogger(__name__)
        
        self.setup_ui()
    
    def setup_ui(self):
        """设置UI界面"""
        # 主框架
        main_frame = ttk.Frame(self.root)
        main_frame.pack(fill=tk.BOTH, expand=True, padx=5, pady=5)
        
        # 左侧面板
        left_frame = ttk.Frame(main_frame)
        left_frame.pack(side=tk.LEFT, fill=tk.Y, padx=(0, 5))
        
        # 右侧面板（棋盘）
        right_frame = ttk.Frame(main_frame)
        right_frame.pack(side=tk.RIGHT, fill=tk.BOTH, expand=True)
        
        # 设置左侧面板
        self.setup_left_panel(left_frame)
        
        # 设置右侧棋盘
        self.setup_board(right_frame)
    
    def setup_left_panel(self, parent):
        """设置左侧控制面板"""
        # 连接面板
        conn_frame = ttk.LabelFrame(parent, text="连接设置")
        conn_frame.pack(fill=tk.X, pady=(0, 5))
        
        ttk.Label(conn_frame, text="玩家ID:").pack(anchor=tk.W)
        self.player_id_entry = ttk.Entry(conn_frame, width=20)
        self.player_id_entry.pack(fill=tk.X, pady=(0, 5))
        self.player_id_entry.insert(0, f"player_{int(time.time()) % 10000}")
        
        ttk.Label(conn_frame, text="服务器地址:").pack(anchor=tk.W)
        self.server_entry = ttk.Entry(conn_frame, width=20)
        self.server_entry.pack(fill=tk.X, pady=(0, 5))
        self.server_entry.insert(0, "127.0.0.1:6001")
        
        self.connect_btn = ttk.Button(conn_frame, text="连接", command=self.connect_server)
        self.connect_btn.pack(fill=tk.X)
        
        # 房间面板
        room_frame = ttk.LabelFrame(parent, text="房间管理")
        room_frame.pack(fill=tk.X, pady=(5, 0))
        
        # 第一行：创建和加入
        row1_frame = ttk.Frame(room_frame)
        row1_frame.pack(fill=tk.X, pady=1)
        ttk.Button(row1_frame, text="创建房间", command=self.create_room).pack(side=tk.LEFT, fill=tk.X, expand=True, padx=(0, 2))
        ttk.Button(row1_frame, text="加入房间", command=self.join_room).pack(side=tk.LEFT, fill=tk.X, expand=True, padx=(2, 0))
        
        # 第二行：房间列表
        ttk.Button(room_frame, text="房间列表", command=self.get_room_list).pack(fill=tk.X, pady=1)
        
        # 第三行：离开和销毁
        row3_frame = ttk.Frame(room_frame)
        row3_frame.pack(fill=tk.X, pady=1)
        ttk.Button(row3_frame, text="离开房间", command=self.leave_room).pack(side=tk.LEFT, fill=tk.X, expand=True, padx=(0, 2))
        ttk.Button(row3_frame, text="销毁房间", command=self.destroy_room).pack(side=tk.LEFT, fill=tk.X, expand=True, padx=(2, 0))
        
        # 分隔线
        ttk.Separator(room_frame, orient='horizontal').pack(fill=tk.X, pady=5)
        
        # 游戏控制
        self.ready_btn = ttk.Button(room_frame, text="准备", command=self.toggle_ready, state=tk.DISABLED)
        self.ready_btn.pack(fill=tk.X, pady=1)
        
        self.start_btn = ttk.Button(room_frame, text="开始游戏", command=self.start_game, state=tk.DISABLED)
        self.start_btn.pack(fill=tk.X, pady=1)
        
        # 状态显示
        status_display_frame = ttk.LabelFrame(parent, text="当前状态")
        status_display_frame.pack(fill=tk.X, pady=(5, 0))
        
        # 当前房间信息
        self.room_info_label = ttk.Label(status_display_frame, text="房间: 未加入", foreground="gray")
        self.room_info_label.pack(anchor=tk.W, pady=2)
        
        # 准备状态
        self.ready_info_label = ttk.Label(status_display_frame, text="状态: 未准备", foreground="gray")
        self.ready_info_label.pack(anchor=tk.W, pady=2)
        
        # 游戏状态
        self.game_info_label = ttk.Label(status_display_frame, text="游戏: 未开始", foreground="gray")
        self.game_info_label.pack(anchor=tk.W, pady=2)
        
        # 刷新状态按钮
        ttk.Button(status_display_frame, text="刷新状态", command=self.refresh_status_display).pack(fill=tk.X, pady=2)
        
        # 日志面板
        log_frame = ttk.LabelFrame(parent, text="消息日志")
        log_frame.pack(fill=tk.BOTH, expand=True, pady=(5, 0))
        
        self.status_text = tk.Text(log_frame, height=15, width=30, wrap=tk.WORD)
        status_scrollbar = ttk.Scrollbar(log_frame, orient=tk.VERTICAL, command=self.status_text.yview)
        self.status_text.configure(yscrollcommand=status_scrollbar.set)
        
        self.status_text.pack(side=tk.LEFT, fill=tk.BOTH, expand=True)
        status_scrollbar.pack(side=tk.RIGHT, fill=tk.Y)
        
        # 清空日志按钮
        ttk.Button(log_frame, text="清空日志", command=self.clear_log).pack(fill=tk.X, pady=2)
    
    def refresh_status_display(self):
        """刷新状态显示"""
        # 更新房间信息
        if self.current_room_id:
            room_text = f"房间: {self.current_room_id}"
            if self.is_owner:
                room_text += " (房主)"
            self.room_info_label.config(text=room_text, foreground="green")
        else:
            self.room_info_label.config(text="房间: 未加入", foreground="gray")
        
        # 更新准备状态
        if self.is_ready:
            self.ready_info_label.config(text="状态: 已准备", foreground="blue")
            self.ready_btn.config(text="取消准备")
        else:
            self.ready_info_label.config(text="状态: 未准备", foreground="gray")
            self.ready_btn.config(text="准备")
        
        # 更新游戏状态
        if self.game_state:
            if self.game_state.result == 0:  # GameResult.ONGOING
                game_text = f"游戏: 进行中"
                if hasattr(self.game_state, 'current_player_id'):
                    game_text += f" (轮到: {self.game_state.current_player_id})"
                self.game_info_label.config(text=game_text, foreground="orange")
            else:
                if self.game_state.winner_id:
                    game_text = f"游戏: 结束 (获胜: {self.game_state.winner_id})"
                else:
                    game_text = "游戏: 结束 (平局)"
                self.game_info_label.config(text=game_text, foreground="red")
        else:
            self.game_info_label.config(text="游戏: 未开始", foreground="gray")
            
        # 更新按钮状态
        self._update_button_states()
    
    def _update_button_states(self):
        """更新按钮状态"""
        # 检查是否连接
        connected = self.tcp_client and self.tcp_client.is_connected()
        
        # 检查是否在房间中
        in_room = bool(self.current_room_id)
        
        # 检查游戏是否已开始
        game_started = self.game_state is not None and self.game_state.result == 0
        
        # 检查游戏是否已结束
        game_ended = self.game_state is not None and self.game_state.result != 0
        
        # 准备按钮：连接且在房间中且游戏未开始
        if connected and in_room and not game_started and not game_ended:
            self.ready_btn.config(state=tk.NORMAL)
        else:
            self.ready_btn.config(state=tk.DISABLED)
            
        # 开始游戏按钮：连接且在房间中且是房主且游戏未开始
        if connected and in_room and self.is_owner and not game_started and not game_ended:
            self.start_btn.config(state=tk.NORMAL)
        else:
            self.start_btn.config(state=tk.DISABLED)
    
    def clear_log(self):
        """清空日志"""
        self.status_text.delete(1.0, tk.END)
        self.log_message("📝 日志已清空")
    
    def setup_board(self, parent):
        """设置五子棋棋盘"""
        board_frame = ttk.LabelFrame(parent, text="五子棋棋盘")
        board_frame.pack(fill=tk.BOTH, expand=True)
        
        # 创建棋盘画布
        canvas_size = self.board_size * self.cell_size + 40
        self.board_canvas = tk.Canvas(
            board_frame, 
            width=canvas_size, 
            height=canvas_size,
            bg='#DEB887'
        )
        self.board_canvas.pack(expand=True)
        
        # 绑定点击事件
        self.board_canvas.bind("<Button-1>", self.on_board_click)
        
        # 绘制棋盘
        self.draw_board()
    
    def draw_board(self):
        """绘制棋盘"""
        self.board_canvas.delete("all")
        
        offset = 20
        
        # 绘制网格线
        for i in range(self.board_size):
            x = offset + i * self.cell_size
            y = offset + i * self.cell_size
            
            # 竖线
            self.board_canvas.create_line(
                x, offset, x, offset + (self.board_size - 1) * self.cell_size,
                fill='black', width=1
            )
            
            # 横线
            self.board_canvas.create_line(
                offset, y, offset + (self.board_size - 1) * self.cell_size, y,
                fill='black', width=1
            )
        
        # 绘制星位
        star_positions = [(3, 3), (3, 11), (11, 3), (11, 11), (7, 7)]
        for row, col in star_positions:
            x = offset + col * self.cell_size
            y = offset + row * self.cell_size
            self.board_canvas.create_oval(x-3, y-3, x+3, y+3, fill='black')
        
        # 绘制棋子
        self.draw_pieces()
    
    def draw_pieces(self):
        """绘制棋子"""
        if not self.game_state:
            return
            
        offset = 20
        
        for i, piece in enumerate(self.game_state.board):
            if piece != 0:
                row = i // self.board_size
                col = i % self.board_size
                
                x = offset + col * self.cell_size
                y = offset + row * self.cell_size
                
                color = 'black' if piece == 1 else 'white'
                outline = 'black' if piece == 2 else 'gray'
                
                self.board_canvas.create_oval(
                    x-12, y-12, x+12, y+12,
                    fill=color, outline=outline, width=2
                )
    
    def on_board_click(self, event):
        """棋盘点击事件"""
        if not self.tcp_client or not self.tcp_client.connected:
            self.log_message("未连接到服务器")
            return
            
        offset = 20
        col = round((event.x - offset) / self.cell_size)
        row = round((event.y - offset) / self.cell_size)
        
        if 0 <= row < self.board_size and 0 <= col < self.board_size:
            self.place_piece(row, col)
    
    def connect_server(self):
        """连接或断开服务器"""
        if self.tcp_client and self.tcp_client.connected:
            # 当前已连接，执行断开操作
            self.disconnect_server()
            return
        
        # 执行连接操作
        if self.tcp_client and self.tcp_client.connected:
            messagebox.showinfo("提示", "已连接到服务器")
            return
        
        server_addr = self.server_entry.get().strip()
        if ':' in server_addr:
            host, port = server_addr.split(':')
            port = int(port)
        else:
            host = server_addr
            port = 6001
        
        self.player_id = self.player_id_entry.get().strip()
        if not self.player_id:
            messagebox.showerror("错误", "请输入玩家ID")
            return
        
        # 创建TCP客户端
        self.tcp_client = TCPClient(host, port)
        self.message_handler = MessageHandler(self.player_id, self.tcp_client)
        
        # 设置消息处理器
        self.tcp_client.set_message_handler(self.message_handler)
        
        # 设置响应处理器
        self._setup_response_handlers()
        
        # 设置通知处理器
        self._setup_notification_handlers()
        
        # 设置系统消息处理器
        self._setup_system_handlers()
        
        # 连接
        try:
            if self.tcp_client.connect(self.player_id):
                self.connect_btn.config(text="断开")
                self.log_message(f"✅ 已连接到服务器 {host}:{port}")
                self.log_message(f"🎮 玩家ID: {self.player_id}")
                self.log_message("💓 心跳功能已启用 (25秒间隔)")
                self.log_message("✅ 首条心跳消息已发送")
                
                # 重置状态
                self.current_room_id = ""
                self.current_room = None
                self.is_ready = False
                self.game_state = None
                self.is_owner = False
                
                # 更新按钮状态（通过状态管理逻辑控制）
                self.refresh_status_display()
                
                # 启动心跳状态监控
                self.monitor_connection_status()
            else:
                messagebox.showerror("错误", "连接服务器失败")
                self.tcp_client = None
                self.message_handler = None
        except Exception as e:
            messagebox.showerror("连接错误", f"连接失败: {str(e)}")
            self.tcp_client = None
            self.message_handler = None
    
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
            "get_room_list": self._handle_get_room_list_response
        }
        
        return handler_map.get(operation_type, self._default_response_handler)
    
    def _default_response_handler(self, response):
        """默认响应处理器"""
        if response.ret == 0:
            self.log_message("✅ 操作成功")
        else:
            reason = response.reason if hasattr(response, 'reason') else "未知错误"
            self.log_message(f"❌ 操作失败: {response.ret} - {reason}")
    
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
        self.message_handler.set_notification_handler("PlayerReadyNotification", self._handle_player_ready_state_notification)
        
        # 游戏开始通知
        self.message_handler.set_notification_handler("GameStartNotification", self._handle_game_start_notification)
        
        # 默认通知处理器
        self.message_handler.set_notification_handler("default", self._handle_default_notification)
    
    def _setup_system_handlers(self):
        """设置系统消息处理器"""
        self.message_handler.set_system_handler("AUTH_FAILED", self._handle_auth_failed)
        self.message_handler.set_system_handler("KICKED", self._handle_kicked)
    
    def setup_notification_handlers(self):
        """设置通知处理器（旧方法，保持兼容性）"""
        self._setup_notification_handlers()
    
    def monitor_connection_status(self):
        """监控连接状态"""
        if self.tcp_client and self.tcp_client.connected:
            # 显示心跳状态
            if self.tcp_client.last_heartbeat_time > 0:
                last_heartbeat = time.time() - self.tcp_client.last_heartbeat_time
                if last_heartbeat < 30:  # 30秒内有心跳
                    status = f"💓 心跳正常 ({int(last_heartbeat)}秒前)"
                else:
                    status = f"⚠️ 心跳异常 ({int(last_heartbeat)}秒前)"
            else:
                status = "⏳ 等待首次心跳"
            
            # 更新状态（每5秒更新一次）
            self.root.after(5000, self.monitor_connection_status)
    
    # ==================== 响应处理器 ====================
    
    def _handle_create_room_response(self, response):
        """处理创建房间响应"""
        if response.ret == 0:
            # 解析响应数据
            from gomoku_pb2 import CreateRoomResponse
            create_resp = CreateRoomResponse()
            create_resp.ParseFromString(response.data)
            self.current_room_id = create_resp.room_id
            self.is_owner = True  # 创建房间的玩家是房主
            self.log_message(f"✅ 房间创建成功！房间ID: {create_resp.room_id}")
            self.log_message("👑 您是房主，可以开始游戏")
            self.refresh_status_display()
        else:
            self.log_message(f"❌ 房间创建失败: {response.reason}")
    
    def _handle_join_room_response(self, response):
        """处理加入房间响应"""
        if response.ret == 0:
            from gomoku_pb2 import JoinRoomResponse
            join_resp = JoinRoomResponse()
            join_resp.ParseFromString(response.data)
            self.current_room_id = join_resp.room_info.room_id
            self.current_room = join_resp.room_info
            
            # 检查是否是房主
            self.is_owner = (join_resp.room_info.owner_id == self.player_id)
            
            self.log_message(f"✅ 成功加入房间: {self.current_room_id}")
            self.log_message(f"🏠 房间名: {join_resp.room_info.room_name}")
            self.log_message(f"👥 玩家数: {len(join_resp.room_info.players)}/2")
            
            if self.is_owner:
                self.log_message("👑 您是房主，可以开始游戏")
            else:
                self.log_message("👥 您是普通玩家，等待房主开始游戏")
            
            self.refresh_status_display()
        else:
            self.log_message(f"❌ 加入房间失败: {response.reason}")
    
    def _handle_leave_room_response(self, response):
        """处理离开房间响应"""
        if response.ret == 0:
            self.current_room_id = ""
            self.current_room = None
            self.is_ready = False
            self.game_state = None
            self.is_owner = False
            self.log_message("✅ 已离开房间")
            self.refresh_status_display()
        else:
            self.log_message(f"❌ 离开房间失败: {response.reason}")
    
    def _handle_destroy_room_response(self, response):
        """处理销毁房间响应"""
        if response.ret == 0:
            self.current_room_id = ""
            self.current_room = None
            self.is_ready = False
            self.game_state = None
            self.log_message("✅ 房间已销毁")
            self.refresh_status_display()
        else:
            self.log_message(f"❌ 销毁房间失败: {response.reason}")
    
    def _handle_player_ready_response(self, response):
        """处理玩家准备响应"""
        if response.ret == 0:
            from gomoku_pb2 import PlayerReadyResponse
            ready_resp = PlayerReadyResponse()
            ready_resp.ParseFromString(response.data)
            self.is_ready = ready_resp.is_ready
            status = "已准备" if self.is_ready else "取消准备"
            self.log_message(f"✅ {status}")
            self.refresh_status_display()
        else:
            self.log_message(f"❌ 准备操作失败: {response.reason}")
    
    def _handle_start_game_response(self, response):
        """处理开始游戏响应"""
        if response.ret == 0:
            from gomoku_pb2 import StartGameResponse
            start_resp = StartGameResponse()
            start_resp.ParseFromString(response.data)
            self.game_state = start_resp.game_state
            self.log_message("✅ 游戏开始！")
            self.update_board_display()
            self.refresh_status_display()
        else:
            self.log_message(f"❌ 开始游戏失败: {response.reason}")
    
    def _handle_place_piece_response(self, response):
        """处理下棋响应"""
        if response.ret == 0:
            from gomoku_pb2 import PlacePieceResponse
            place_resp = PlacePieceResponse()
            place_resp.ParseFromString(response.data)
            self.game_state = place_resp.game_state
            self.log_message("✅ 下棋成功！")
            self.update_board_display()
            self.refresh_status_display()
        else:
            self.log_message(f"❌ 下棋失败: {response.reason}")
    
    def _handle_get_room_list_response(self, response):
        """处理获取房间列表响应"""
        if response.ret == 0:
            list_resp = GetRoomListResponse()
            list_resp.ParseFromString(response.data)
            
            # 在日志中显示房间列表信息
            self.log_message(f"📋 房间列表获取成功，共{len(list_resp.rooms)}个房间")
            
            # 显示房间列表窗口
            self.show_room_list_window(list_resp)
        else:
            self.log_message(f"❌ 获取房间列表失败: {response.reason}")
    
    def show_room_list_window(self, room_list_data: GetRoomListResponse):
        """显示房间列表窗口"""
        try:
            # 创建房间列表窗口
            room_list_window = RoomListWindow(
                self.root, 
                room_list_data, 
                self.join_room_from_list
            )
        except Exception as e:
            self.log_message(f"❌ 显示房间列表失败: {e}")
            messagebox.showerror("错误", f"显示房间列表失败: {e}")
    
    def join_room_from_list(self, room_id: str, password: str = ""):
        """从房间列表加入房间"""
        self.log_message(f"📤 从列表加入房间: {room_id}")
        if not self.tcp_client or not self.tcp_client.connected:
            messagebox.showerror("错误", "未连接到服务器")
            return
        
        request_id = self.message_handler.join_room(room_id, password)
        self.log_message(f"📤 加入房间请求: {room_id}")
    
    # ==================== 游戏操作通知处理器 ====================
    
    def _handle_create_room_notification(self, data):
        """处理创建房间操作的推送通知"""
        if hasattr(data, 'room_id'):
            self.log_message("🏠 房间创建完成！")
            self.log_message(f"   📍 房间ID: {data.room_id}")
            if hasattr(data, 'room_name') and data.room_name:
                self.log_message(f"   🏷️ 房间名: {data.room_name}")
            self.log_message(f"   ✨ 房间已准备就绪，等待玩家加入")
        else:
            self.log_message("🏠 房间创建操作完成的推送通知")
    
    def _handle_join_room_notification(self, data):
        """处理加入房间操作的推送通知"""
        if hasattr(data, 'room_info'):
            room_info = data.room_info
            self.log_message("🚪 房间加入完成！")
            self.log_message(f"   📍 房间ID: {room_info.room_id}")
            self.log_message(f"   🏷️ 房间名: {room_info.room_name}")
            self.log_message(f"   👥 当前玩家: {len(room_info.players)}/2")
            
            if room_info.players:
                self.log_message("   🎮 房间内玩家:")
                for player in room_info.players:
                    ready_status = "✅已准备" if player.is_ready else "⏳未准备"
                    self.log_message(f"      • {player.player_id} ({ready_status})")
            
            status = "🎮游戏中" if room_info.is_gaming else "⏳等待中"
            self.log_message(f"   🎯 状态: {status}")
        else:
            self.log_message("🚪 房间加入操作完成的推送通知")
    
    def _handle_leave_room_notification(self, data):
        """处理离开房间操作的推送通知"""
        if isinstance(data, str):
            self.log_message(f"🚶 {data}")
        else:
            self.log_message("🚶 房间离开操作完成的推送通知")
    
    def _handle_destroy_room_notification(self, data):
        """处理销毁房间操作的推送通知"""
        if isinstance(data, str):
            self.log_message(f"🏚️ {data}")
        else:
            self.log_message("🏚️ 房间销毁操作完成的推送通知")
    
    def _handle_player_ready_operation_notification(self, data):
        """处理玩家准备操作的推送通知"""
        if hasattr(data, 'is_ready'):
            status = "已准备" if data.is_ready else "取消准备"
            self.log_message("⏳ 准备状态更新完成！")
            self.log_message(f"   🎮 当前状态: {status}")
        else:
            self.log_message("⏳ 玩家准备操作完成的推送通知")
    
    def _handle_start_game_notification(self, data):
        """处理开始游戏操作的推送通知"""
        if hasattr(data, 'game_state'):
            self.log_message("🎮 游戏开始操作完成！")
            self.log_message("   🎯 游戏已正式开始")
            if hasattr(data.game_state, 'current_player_id'):
                self.log_message(f"   🎮 当前轮到: {data.game_state.current_player_id}")
        else:
            self.log_message("🎮 游戏开始操作完成的推送通知")
    
    def _handle_place_piece_operation_notification(self, data):
        """处理下棋操作的推送通知"""
        if hasattr(data, 'game_state'):
            self.log_message("♟️ 下棋操作完成！")
            if hasattr(data.game_state, 'last_move') and data.game_state.last_move:
                move = data.game_state.last_move
                self.log_message(f"   📍 落子位置: ({move.x}, {move.y})")
                self.log_message(f"   🎮 玩家: {move.player_id}")
            
            # 检查游戏结果
            if hasattr(data.game_state, 'result') and data.game_state.result != 0:
                if data.game_state.winner_id:
                    self.log_message(f"   🏆 游戏结束，获胜者: {data.game_state.winner_id}")
                else:
                    self.log_message("   🤝 游戏结束，平局")
        else:
            self.log_message("♟️ 下棋操作完成的推送通知")
    
    # ==================== 事件通知处理器 ====================
    
    def _handle_piece_placed_notification(self, notification):
        """处理棋子放置通知"""
        place_resp = notification
        self.game_state = place_resp.game_state
        
        self.log_message("🔔 对手下棋了！")
        if hasattr(place_resp.game_state, 'last_move') and place_resp.game_state.last_move:
            move = place_resp.game_state.last_move
            self.log_message(f"   📍 落子位置: ({move.x}, {move.y})")
            self.log_message(f"   🎮 对手: {move.player_id}")
        
        self.update_board_display()
        self.refresh_status_display()
    
    def _handle_game_state_changed_notification(self, notification):
        """处理游戏状态变化通知"""
        game_notify = notification
        self.game_state = game_notify.game_state
        
        self.log_message("🔔 游戏状态变化!")
        self.log_message(f"   📝 事件类型: {game_notify.event_type}")
        if game_notify.event_message:
            self.log_message(f"   💬 事件消息: {game_notify.event_message}")
        
        self.update_board_display()
        self.refresh_status_display()
    
    def _handle_player_joined_notification(self, notification):
        """处理玩家加入通知"""
        player_notify = notification
        
        self.log_message("🔔 新玩家加入房间!")
        self.log_message(f"   🎮 玩家ID: {player_notify.player_id}")
        self.log_message(f"   🏠 房间ID: {player_notify.room_id}")
        self.log_message(f"   📝 事件类型: {player_notify.event_type}")
        
        if hasattr(player_notify, 'player_info') and player_notify.player_info:
            player_info = player_notify.player_info
            ready_status = "✅已准备" if player_info.is_ready else "⏳未准备"
            self.log_message(f"   🎯 准备状态: {ready_status}")
        
        self.refresh_status_display()
    
    def _handle_player_left_notification(self, notification):
        """处理玩家离开通知"""
        player_notify = notification
        
        self.log_message("🔔 玩家离开房间!")
        self.log_message(f"   🎮 玩家ID: {player_notify.player_id}")
        self.log_message(f"   🏠 房间ID: {player_notify.room_id}")
        self.log_message(f"   📝 事件类型: {player_notify.event_type}")
        self.log_message(f"   👋 该玩家已离开游戏")
        
        self.refresh_status_display()
    
    def _handle_player_ready_state_notification(self, notification):
        """处理玩家准备状态通知"""
        player_notify = notification
        
        self.log_message("🔔 玩家准备状态变化!")
        self.log_message(f"   🎮 玩家ID: {player_notify.player_id}")
        self.log_message(f"   🏠 房间ID: {player_notify.room_id}")
        
        if player_notify.event_type == "READY":
            self.log_message("   ✅ 玩家已准备")
        elif player_notify.event_type == "UNREADY":
            self.log_message("   ⏳ 玩家取消准备")
        else:
            self.log_message(f"   📝 准备状态: {player_notify.event_type}")
        
        if hasattr(player_notify, 'player_info') and player_notify.player_info:
            player_info = player_notify.player_info
            ready_status = "✅已准备" if player_info.is_ready else "⏳未准备"
            self.log_message(f"   🎯 当前状态: {ready_status}")
        
        self.refresh_status_display()
    
    def _handle_game_start_notification(self, notification):
        """处理游戏开始通知"""
        game_notify = notification  # 已经在message_handler中解析为GameStateNotify
        self.game_state = game_notify.game_state
        
        self.log_message("🎮 游戏开始通知!")
        self.log_message(f"   🏠 房间ID: {game_notify.room_id}")
        self.log_message(f"   📝 事件类型: {game_notify.event_type}")
        if game_notify.event_message:
            self.log_message(f"   💬 事件消息: {game_notify.event_message}")
        
        # 显示当前游戏状态
        if self.game_state:
            current_turn = self._get_player_color_name(self.game_state.current_turn)
            self.log_message(f"   🎯 当前轮到: {current_turn}")
            self.log_message(f"   🔢 总步数: {self.game_state.total_moves}")
            result_name = self._get_game_result_name(self.game_state.result)
            self.log_message(f"   📊 游戏状态: {result_name}")
            self.log_message("   🎮 游戏已正式开始，可以开始下棋了！")
        
        # 更新UI状态
        self.refresh_status_display()
        
        # 如果有游戏状态，重新绘制棋盘
        if self.game_state and len(self.game_state.board) == 225:
            self.draw_board()
    
    def _handle_default_notification(self, notification):
        """处理默认通知"""
        method = notification.msg_head.request_id if hasattr(notification, 'msg_head') else "unknown"
        self.log_message(f"🔔 收到未处理的通知: {method}")
    
    # ==================== 系统消息处理器 ====================
    
    def _handle_auth_failed(self, message):
        """处理认证失败"""
        self.log_message("🔐 认证失败，连接将被断开")
        messagebox.showerror("认证失败", "连接认证失败，请重新连接")
    
    def _handle_kicked(self, message):
        """处理被踢下线"""
        self.log_message("👋 您已被踢下线")
        messagebox.showwarning("被踢下线", "您已被踢下线")
    
    # ==================== 旧的处理器方法（保持兼容性） ====================
    
    def handle_player_ready_notification(self, notification):
        """处理玩家准备通知（旧方法）"""
        self._handle_player_ready_state_notification(notification)
    
    def handle_game_state_notification(self, notification):
        """处理游戏状态通知（旧方法）"""
        self._handle_game_state_changed_notification(notification)
    
    def handle_player_event_notification(self, notification):
        """处理玩家事件通知（旧方法）"""
        self._handle_player_joined_notification(notification)
    
    def update_board_display(self):
        """更新棋盘显示"""
        if self.game_state:
            self.draw_pieces()
        else:
            self.log_message("⚠️ 游戏状态为空，无法更新棋盘")
    
    def create_room(self):
        """创建房间"""
        if not self.tcp_client or not self.tcp_client.connected:
            messagebox.showerror("错误", "未连接到服务器")
            return
        
        room_name = simpledialog.askstring("创建房间", "请输入房间名称:")
        if room_name:
            password = simpledialog.askstring("创建房间", "请输入房间密码（可选）:", show='*')
            request_id = self.message_handler.create_room(room_name, password or "")
            self.log_message(f"📤 创建房间请求: {room_name}")
    
    def join_room(self):
        """加入房间"""
        if not self.tcp_client or not self.tcp_client.connected:
            messagebox.showerror("错误", "未连接到服务器")
            return
        
        room_id = simpledialog.askstring("加入房间", "请输入房间ID:")
        if room_id:
            password = simpledialog.askstring("加入房间", "请输入房间密码（如果有）:", show='*')
            request_id = self.message_handler.join_room(room_id, password or "")
            self.log_message(f"📤 加入房间请求: {room_id}")
    
    def leave_room(self):
        """离开房间"""
        if not self.tcp_client or not self.tcp_client.connected:
            messagebox.showerror("错误", "未连接到服务器")
            return
        
        if not self.current_room_id:
            messagebox.showwarning("警告", "当前不在房间中")
            return
        
        if messagebox.askyesno("离开房间", f"确定要离开房间 {self.current_room_id} 吗？"):
            request_id = self.message_handler.leave_room(self.current_room_id)
            self.log_message(f"📤 离开房间请求: {self.current_room_id}")
    
    def destroy_room(self):
        """销毁房间"""
        if not self.tcp_client or not self.tcp_client.connected:
            messagebox.showerror("错误", "未连接到服务器")
            return
        
        if not self.current_room_id:
            messagebox.showwarning("警告", "当前不在房间中")
            return
        
        if messagebox.askyesno("销毁房间", f"确定要销毁房间 {self.current_room_id} 吗？"):
            request_id = self.message_handler.destroy_room(self.current_room_id)
            self.log_message(f"📤 销毁房间请求: {self.current_room_id}")
    
    def get_room_list(self):
        """获取房间列表"""
        if not self.tcp_client or not self.tcp_client.connected:
            messagebox.showerror("错误", "未连接到服务器")
            return
        
        request_id = self.message_handler.get_room_list()
        self.log_message("📤 获取房间列表...")
    
    def toggle_ready(self):
        """切换准备状态"""
        if not self.tcp_client or not self.tcp_client.connected:
            messagebox.showerror("错误", "未连接到服务器")
            return
        
        if not self.current_room_id:
            messagebox.showwarning("警告", "当前不在房间中")
            return
        
        # 检查游戏是否已开始
        if self.game_state is not None and self.game_state.result == 0:
            messagebox.showwarning("游戏状态", "游戏已开始，无法切换准备状态")
            self.log_message("⚠️ 游戏已开始，无法切换准备状态")
            return
        
        request_id = self.message_handler.player_ready(self.current_room_id)
        status = "取消准备" if self.is_ready else "准备"
        self.log_message(f"📤 {status}状态请求...")
    
    def start_game(self):
        """开始游戏"""
        if not self.tcp_client or not self.tcp_client.connected:
            messagebox.showerror("错误", "未连接到服务器")
            return
        
        if not self.current_room_id:
            messagebox.showwarning("警告", "当前不在房间中")
            return
        
        # 检查是否是房主
        if not self.is_owner:
            messagebox.showwarning("权限不足", "只有房主可以开始游戏")
            self.log_message("⚠️ 只有房主可以开始游戏")
            return
        
        # 检查游戏是否已开始
        if self.game_state is not None and self.game_state.result == 0:
            messagebox.showwarning("游戏状态", "游戏已经开始")
            self.log_message("⚠️ 游戏已经开始")
            return
        
        request_id = self.message_handler.start_game(self.current_room_id)
        self.log_message(f"📤 开始游戏请求: {self.current_room_id}")
    
    def place_piece(self, row: int, col: int):
        """下棋"""
        if not self.tcp_client or not self.tcp_client.connected:
            messagebox.showerror("错误", "未连接到服务器")
            return
        
        if not self.current_room_id:
            messagebox.showwarning("警告", "当前不在房间中")
            return
        
        if not self.game_state:
            messagebox.showwarning("警告", "游戏尚未开始")
            return
        
        request_id = self.message_handler.place_piece(self.current_room_id, col, row)
        self.log_message(f"📤 下棋请求: ({row}, {col})")
    
    def get_status(self):
        """获取客户端状态"""
        status = {
            "connected": self.tcp_client.is_connected() if self.tcp_client else False,
            "player_id": self.player_id,
            "current_room": self.current_room_id,
            "is_ready": self.is_ready,
            "in_game": self.game_state is not None
        }
        
        if self.tcp_client:
            status.update({
                "host": self.tcp_client.host,
                "port": self.tcp_client.port,
                "last_heartbeat": self.tcp_client.last_heartbeat_time if hasattr(self.tcp_client, 'last_heartbeat_time') else 0,
                "last_message": self.tcp_client.last_message_time if hasattr(self.tcp_client, 'last_message_time') else 0
            })
        
        return status
    
    def disconnect_server(self):
        """断开服务器连接"""
        if self.tcp_client:
            self.tcp_client.disconnect()
            self.tcp_client = None
            self.message_handler = None
            self.current_room_id = ""
            self.current_room = None
            self.is_ready = False
            self.game_state = None
            self.is_owner = False
            
            self.connect_btn.config(text="连接")
            self.ready_btn.config(state=tk.DISABLED)
            self.start_btn.config(state=tk.DISABLED)
            self.log_message("�� 已断开与服务器的连接")
    
    def log_message(self, message: str):
        """记录消息到状态面板"""
        timestamp = time.strftime("%H:%M:%S")
        log_text = f"[{timestamp}] {message}\n"
        
        # 确保在主线程中更新GUI
        if threading.current_thread() == threading.main_thread():
            self.status_text.insert(tk.END, log_text)
            self.status_text.see(tk.END)
            
            # 限制文本长度
            if int(self.status_text.index(tk.END).split('.')[0]) > 100:
                self.status_text.delete('1.0', '20.0')
        else:
            # 从其他线程调用时，使用after方法
            self.root.after(0, lambda: self.log_message(message))
    
    def run(self):
        """运行GUI"""
        self.root.mainloop()
        
        # 清理
        if self.tcp_client:
            self.tcp_client.disconnect()

if __name__ == "__main__":
    app = GomokuGUI()
    app.run()