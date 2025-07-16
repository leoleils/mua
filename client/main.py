#!/usr/bin/env python3
"""
五子棋客户端主程序

连接到gatesvr的TCP服务器，通过protobuf协议与gomokusvr进行五子棋游戏。

使用方法:
python main.py

功能特性:
- TCP连接到gatesvr (默认端口6001)
- protobuf消息协议
- 可视化五子棋界面
- 房间管理（创建、加入、列表）
- 实时游戏状态同步
- 玩家准备和游戏开始
- 五子棋下棋功能
"""

import sys
import os
import logging

# 添加当前目录到Python路径
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

from gomoku_gui import GomokuGUI

def setup_logging():
    """设置日志"""
    logging.basicConfig(
        level=logging.INFO,
        format='%(asctime)s - %(name)s - %(levelname)s - %(message)s',
        handlers=[
            logging.StreamHandler(sys.stdout),
            logging.FileHandler('gomoku_client.log')
        ]
    )

def main():
    """主函数"""
    print("五子棋客户端启动中...")
    print("=" * 50)
    print("连接信息:")
    print("- 默认服务器: 127.0.0.1:6001")
    print("- 协议: TCP + protobuf")
    print("- 游戏: 五子棋 (15x15)")
    print("=" * 50)
    
    # 设置日志
    setup_logging()
    
    try:
        # 启动GUI
        app = GomokuGUI()
        app.run()
    except KeyboardInterrupt:
        print("\n程序被用户中断")
    except Exception as e:
        print(f"程序运行错误: {e}")
        logging.exception("程序运行错误")
    finally:
        print("五子棋客户端已退出")

if __name__ == "__main__":
    main()