#!/usr/bin/env python3
"""
测试GUI优化功能
验证房主权限控制、状态管理等功能
"""

import tkinter as tk
from tkinter import messagebox
import time
import threading
from gomoku_gui import GomokuGUI

def test_gui_optimization():
    """测试GUI优化功能"""
    
    # 创建主窗口
    app = GomokuGUI()
    
    # 添加测试说明
    test_info = """
GUI优化测试说明：

1. 按钮状态控制：
   - 只有连接后，房间相关按钮才可用
   - 只有房主才能点击"开始游戏"按钮
   - 游戏开始后，"准备"按钮变为不可用
   
2. 状态显示：
   - 房主会显示"(房主)"标记
   - 普通玩家会显示相应提示
   - 游戏状态会实时更新
   
3. 错误处理：
   - 非房主点击"开始游戏"会提示权限不足
   - 游戏已开始时切换准备状态会提示不可操作
   
测试步骤：
1. 连接服务器
2. 创建房间（成为房主）
3. 观察按钮状态和提示信息
4. 测试开始游戏功能
"""
    
    # 在GUI中显示测试信息
    def show_test_info():
        messagebox.showinfo("测试说明", test_info)
    
    # 添加测试按钮
    test_frame = tk.Frame(app.root)
    test_frame.pack(side=tk.BOTTOM, fill=tk.X, padx=5, pady=5)
    
    tk.Button(test_frame, text="显示测试说明", command=show_test_info).pack(side=tk.LEFT, padx=5)
    
    # 添加状态监控
    def monitor_status():
        """监控状态变化"""
        status_info = f"""
当前状态：
- 连接状态: {'已连接' if app.tcp_client and app.tcp_client.is_connected() else '未连接'}
- 房间ID: {app.current_room_id or '无'}
- 是否房主: {'是' if app.is_owner else '否'}
- 准备状态: {'已准备' if app.is_ready else '未准备'}
- 游戏状态: {'已开始' if app.game_state else '未开始'}
"""
        print(status_info)
        app.root.after(5000, monitor_status)  # 每5秒监控一次
    
    tk.Button(test_frame, text="监控状态", command=monitor_status).pack(side=tk.LEFT, padx=5)
    
    # 显示初始测试信息
    app.log_message("🧪 GUI优化测试模式启动")
    app.log_message("📋 点击'显示测试说明'查看测试指导")
    app.log_message("🔍 点击'监控状态'在控制台查看详细状态")
    
    # 启动GUI
    app.root.mainloop()

if __name__ == "__main__":
    test_gui_optimization() 