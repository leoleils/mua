#!/bin/bash

echo "=== 五子棋系统测试 ==="
echo ""

echo "1. 检查服务状态..."
ps aux | grep -E "(gatesvr|gomokusvr)" | grep -v grep
netstat -an | grep -E "(6001|50052)"
echo ""

echo "2. 测试简单连接..."
echo -e "test123\nlocalhost:6001\nhelp\nquit" | ./gomoku-client
