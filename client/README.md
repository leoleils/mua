# 五子棋客户端

基于 gatesvr 接口规范实现的完整五子棋客户端，支持完整的游戏流程和实时消息推送。

## 🚀 快速开始

### 环境要求

- Python 3.7+
- protobuf Python库

### 安装依赖

```bash
pip install -r requirements.txt
```

### 启动客户端

```bash
# 方式1：直接运行
python gomoku_client.py

# 方式2：作为可执行文件运行
./gomoku_client.py

# 方式3：使用原有的GUI客户端
python gomoku_gui.py
```

## 🎮 功能特性

### ✅ 已实现功能

1. **连接管理**
   - ✅ TCP连接到gatesvr (端口6001)
   - ✅ 自动心跳保活 (25秒间隔)
   - ✅ 首条消息验证 (5秒内发送)
   - ✅ 连接状态监控
   - ✅ 自动断线重连

2. **认证支持**
   - ✅ JWT Token认证
   - ✅ 认证失败处理
   - ✅ 异地登录检测

3. **房间管理**
   - ✅ 创建房间
   - ✅ 加入房间 (支持密码)
   - ✅ 离开房间
   - ✅ 销毁房间
   - ✅ 获取房间列表

4. **游戏功能**
   - ✅ 玩家准备/取消准备
   - ✅ 开始游戏
   - ✅ 下棋操作
   - ✅ 游戏状态显示
   - ✅ 胜负判断

5. **实时通知**
   - ✅ 对手下棋通知
   - ✅ 玩家加入/离开通知
   - ✅ 玩家准备状态通知
   - ✅ 游戏状态变化通知
   - ✅ 系统消息处理

6. **协议兼容**
   - ✅ 完全符合gatesvr接口规范
   - ✅ protobuf消息序列化
   - ✅ 正确的消息头格式
   - ✅ 服务消息类型支持

## 📋 使用说明

### 连接服务器

启动客户端后，输入玩家ID（可以留空自动生成）：

```
请输入玩家ID (直接回车使用自动生成): Alice
```

### 可用命令

| 命令 | 说明 | 示例 |
|------|------|------|
| `create <房间名> [密码]` | 创建房间 | `create MyRoom` |
| `join <房间ID> [密码]` | 加入房间 | `join room_001` |
| `list [页码] [页大小]` | 获取房间列表 | `list 1 5` |
| `ready` | 切换准备状态 | `ready` |
| `start` | 开始游戏（房主） | `start` |
| `place <x> <y>` | 下棋 | `place 7 7` |
| `leave` | 离开房间 | `leave` |
| `destroy` | 销毁房间（房主） | `destroy` |
| `status` | 查看状态 | `status` |
| `quit` | 退出程序 | `quit` |

### 游戏流程示例

1. **创建/加入房间**
   ```
   [Alice]> create TestRoom
   ✅ 房间创建成功！房间ID: room_12345
   ```

2. **等待其他玩家加入**
   ```
   🔔 玩家 Bob 加入了房间 room_12345
   ```

3. **准备游戏**
   ```
   [Alice]> ready
   ✅ 准备状态更新: 已准备
   ```

4. **开始游戏**
   ```
   [Alice]> start
   🎮 游戏开始！
   🎮 游戏状态:
      当前回合: BLACK
      结果: ONGOING
      总步数: 0
   ```

5. **下棋**
   ```
   [Alice]> place 7 7
   ✅ 下棋成功！
   📋 棋盘:
       0 1 2 3 4 5 6 7 8 9 A B C D E
    0  · · · · · · · · · · · · · · ·
    1  · · · · · · · · · · · · · · ·
    ...
    7  · · · · · · · ● · · · · · · ·
    ...
   ```

6. **接收对手下棋**
   ```
   🔔 对手下棋了！
   📋 棋盘:
       0 1 2 3 4 5 6 7 8 9 A B C D E
    0  · · · · · · · · · · · · · · ·
    ...
    7  · · · · · · · ● · · · · · · ·
    8  · · · · · · · ○ · · · · · · ·
    ...
   ```

## 🔧 技术架构

### 核心组件

1. **TCPClient**: TCP连接管理
   - 二进制协议支持
   - 自动心跳机制
   - 消息序列化/反序列化
   - 连接状态监控

2. **MessageHandler**: 消息处理器
   - 业务消息封装
   - 响应处理器管理
   - 通知处理器管理
   - 请求ID生成

3. **GomokuClient**: 五子棋客户端
   - 高级API封装
   - 游戏状态管理
   - UI交互处理

### 消息流程

```
用户输入 → GomokuClient → MessageHandler → TCPClient → gatesvr → gomokusvr
                                                        ↓
客户端显示 ← 通知处理 ← 推送解析 ← 消息接收 ← TCP连接 ← 推送通知
```

### protobuf消息格式

所有消息都遵循gatesvr接口规范：

```protobuf
// 请求消息
GameMessage {
  msg_head: {
    player_id: "Alice"
    service_name: "gomokusvr"
    request_id: "PlacePiece"
    timestamp: 1699123456789
    service_msg_type: SYNC
  }
  msg_type: SERVICE_MESSAGE
  payload: <PlacePieceRequest序列化>
}

// 响应消息
GameMessageResponse {
  head: { ... }
  code: 0
  data: <PlacePieceResponse序列化>
  response_timestamp: 1699123456790
}

// 推送通知
GameMessage {
  msg_head: {
    player_id: "Bob"
    service_name: "gomokusvr"
    request_id: "PiecePlacedNotification"
  }
  msg_type: CLIENT_MESSAGE
  payload: <PlacePieceResponse序列化>
}
```

## 🐛 调试指南

### 日志级别

修改日志级别查看详细信息：

```python
# 在代码中修改
logging.basicConfig(level=logging.DEBUG)
```

### 常见问题

1. **连接失败**
   - 检查gatesvr是否启动 (端口6001)
   - 检查网络连接
   - 查看服务器日志

2. **心跳超时**
   - 检查网络稳定性
   - 确认服务器心跳配置 (60秒超时)

3. **消息解析失败**
   - 检查protobuf版本兼容性
   - 确认.proto文件一致

4. **推送不到达**
   - 检查gomokusvr推送逻辑
   - 确认玩家路由信息
   - 查看gatesvr推送日志

### 测试工具

使用简单连接测试：

```bash
python simple_connect_test.py
```

使用心跳测试：

```bash
python test_heartbeat.py
```

## 📚 相关文档

- [gatesvr API文档](../gatesvr/TCP_WebSocket_接口使用说明.md)
- [gomokusvr API文档](../minigame/gomokusvr/API说明文档.md)
- [消息同步功能说明](../minigame/gomokusvr/消息同步功能说明.md)

## 🤝 贡献

欢迎提交Issue和Pull Request来改进客户端功能！ 