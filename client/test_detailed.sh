#!/bin/bash

echo "=== 详细测试五子棋客户端 ==="

# 创建输入文件
cat > input.txt << ENDFILE
test_player
localhost:6001
status
rooms
quit
ENDFILE

echo "测试输入文件内容:"
cat input.txt
echo ""

echo "启动客户端并输入命令..."
./gomoku-client < input.txt

echo "清理输入文件..."
rm input.txt
