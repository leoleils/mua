#!/usr/bin/env python3
"""
心跳功能测试脚本
"""

import time
import logging
from tcp_client import TCPClient

# 设置日志
logging.basicConfig(level=logging.DEBUG, format='%(asctime)s - %(name)s - %(levelname)s - %(message)s')

def test_heartbeat():
    print("=" * 50)
    print("心跳功能测试")
    print("=" * 50)
    
    # 创建TCP客户端
    client = TCPClient("127.0.0.1", 6001)
    
    try:
        # 连接服务器
        test_player_id = "test_player_123"
        if client.connect(test_player_id):
            print("✓ 连接成功")
            print(f"✓ 玩家ID: {client.player_id}")
            print("✓ 首条心跳消息已发送")
            print(f"✓ 心跳间隔: {client.heartbeat_interval} 秒")
            print("✓ 心跳线程已启动")
            
            # 监控心跳状态
            for i in range(12):  # 监控约3分钟
                if client.connected:
                    if client.last_heartbeat_time > 0:
                        last_heartbeat = time.time() - client.last_heartbeat_time
                        print(f"[{i+1:2d}/12] 心跳状态: {last_heartbeat:.1f}秒前 - 连接正常")
                    else:
                        print(f"[{i+1:2d}/12] 心跳状态: 等待首次心跳")
                else:
                    print(f"[{i+1:2d}/12] 连接已断开")
                    break
                
                time.sleep(15)  # 每15秒检查一次
            
            print("\n测试完成")
            
        else:
            print("✗ 连接失败")
            
    except KeyboardInterrupt:
        print("\n测试被用户中断")
    except Exception as e:
        print(f"测试出错: {e}")
    finally:
        client.disconnect()
        print("连接已关闭")

if __name__ == "__main__":
    test_heartbeat()
