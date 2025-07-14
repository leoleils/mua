#!/bin/bash

echo "=== 五子棋客户端功能测试 ==="
echo ""

# 测试1：连接和基本命令
echo "测试1: 连接服务器并查看状态"
echo -e "test_player_1\nlocalhost:6001\nstatus\nquit" | ./gomoku-client
echo ""

# 测试2：房间管理
echo "测试2: 房间管理功能"
echo -e "test_player_2\nlocalhost:6001\nrooms\ncreate 测试房间\nroom\nquit" | ./gomoku-client
echo ""

echo "=== 测试完成 ==="
echo "如需进行完整的游戏测试，请手动运行以下命令："
echo "1. 启动第一个客户端: ./gomoku-client"
echo "2. 启动第二个客户端: ./gomoku-client"
echo "3. 在第一个客户端中创建房间: create 我的房间"
echo "4. 在第二个客户端中加入房间: join <房间ID>"
echo "5. 两个客户端都准备: ready"
echo "6. 房主开始游戏: start"
echo "7. 轮流下棋: move 7 7"
