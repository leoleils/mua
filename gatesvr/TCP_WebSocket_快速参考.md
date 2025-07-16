# GateServer TCP/WebSocket 快速参考

## 🚀 快速开始

### 连接信息
```
TCP端口:     6001
WebSocket端口: 6002  
默认地址:    127.0.0.1
```

### 首条消息（必须）
```python
# 连接后5秒内必须发送心跳消息
heartbeat = GameMessage(
    msg_head=HeadMessage(player_id="your_player_id"),
    msg_type=MessageType.HEARTBEAT
)
```

## 📨 消息格式速查

### 基础消息结构
```protobuf
GameMessage {
  HeadMessage msg_head = 1;  // 消息头（包含player_id）
  MessageType msg_type = 2;  // 0=心跳, 1=服务消息, 2=客户端消息
  bytes payload = 3;         // 业务数据
}
```

### 消息类型
| 类型 | 值 | 用途 |
|-----|---|------|
| HEARTBEAT | 0 | 心跳保活 |
| SERVICE_MESSAGE | 1 | 转发到后端服务 |
| CLIENT_MESSAGE | 2 | 网关内部处理 |
| BROADCAST_MESSAGE | 3 | 广播消息 |

## 🔧 协议实现

### TCP协议格式
```
[4字节长度][protobuf消息体]
长度: 小端序整数
```

### TCP发送函数
```python
def send_tcp_message(socket, game_message):
    data = game_message.SerializeToString()
    length = len(data)
    header = struct.pack('<I', length)  # 小端序
    socket.send(header + data)
```

### TCP接收函数  
```python
def receive_tcp_message(socket):
    # 读长度头
    length_data = socket.recv(4)
    length = struct.unpack('<I', length_data)[0]
    
    # 读消息体
    data = socket.recv(length)
    
    # 解析protobuf
    message = GameMessage()
    message.ParseFromString(data)
    return message
```

### WebSocket实现
```javascript
// 发送
const data = GameMessage.encode(message).finish();
websocket.send(data);

// 接收
websocket.onmessage = function(event) {
    const message = GameMessage.decode(new Uint8Array(event.data));
};
```

## 💡 常用消息模板

### 心跳消息
```python
GameMessage(
    msg_head=HeadMessage(player_id="player_123"),
    msg_type=MessageType.HEARTBEAT
)
```

### 服务消息（同步）
```python
GameMessage(
    msg_head=HeadMessage(
        player_id="player_123",
        service_name="gomokusvr",
        service_msg_type=ServiceMessageType.SYNC
    ),
    msg_type=MessageType.SERVICE_MESSAGE,
    payload=your_business_data
)
```

### 带认证的消息
```python
GameMessage(
    msg_head=HeadMessage(
        player_id="player_123",
        token="your_jwt_token",
        service_name="gomokusvr"
    ),
    msg_type=MessageType.SERVICE_MESSAGE,
    payload=your_business_data
)
```

### 📡 推送消息处理

#### 客户端接收推送
```python
def handle_push_message(game_message):
    if game_message.msg_type == MessageType.SERVICE_MESSAGE:
        # 业务服务推送（游戏状态变化等）
        service_name = game_message.msg_head.service_name
        response = GameMessageResponse()
        response.ParseFromString(game_message.payload)
        handle_service_push(service_name, response)
        
    elif game_message.msg_type == MessageType.CLIENT_MESSAGE:
        # 网关系统推送（认证失败、踢下线等）
        payload = game_message.payload.decode('utf-8')
        handle_gateway_push(payload)
```

#### 服务端推送接口
```go
// gRPC推送接口
rpc PushToClient(PushRequest) returns (PushResponse);

// 推送请求
message PushRequest {
  string player_id = 1;           // 目标玩家ID  
  CallbackType cb_type = 3;       // SYNC/ASYNC/PUSH
  common.GameMessage message = 4; // 推送消息内容
}
```

#### 推送类型
| 类型 | 用途 | 等待确认 |
|------|------|----------|
| SYNC | 同步推送 | ✅ |
| ASYNC | 异步推送 | ❌ |
| PUSH | 单向推送 | ❌ |

## ⚡ 关键配置

### 超时设置
```yaml
connection:
  first_message_timeout_sec: 5   # 首条消息超时
  read_timeout_sec: 60          # 读超时
  heartbeat_interval_sec: 30    # 心跳间隔
```

### 建议的客户端心跳频率
```python
heartbeat_interval = 25  # 秒（比服务器60秒超时小）
```

## 🛠️ 错误码速查

| 错误码 | 说明 | 处理方法 |
|--------|------|----------|
| 0 | 成功 | - |
| 1001 | 玩家ID无效 | 检查player_id |
| 1002 | Token生成失败 | 检查Token参数 |
| 2001 | 玩家不在线 | 重新连接 |
| 3001 | 推送失败 | 重试或检查连接 |
| 5001 | 服务错误 | 检查后端服务 |

## 🔍 调试技巧

### 检查连接状态
```bash
# 测试TCP端口
telnet 127.0.0.1 6001

# 测试WebSocket端口  
telnet 127.0.0.1 6002
```

### 启用详细日志
```yaml
connection:
  enable_structured_log: true
```

### 监控接口
```bash
curl http://127.0.0.1:8082/health
```

## 📝 最佳实践

1. **连接建立**: 连接后立即发送心跳消息
2. **心跳频率**: 每25秒发送一次心跳
3. **错误处理**: 捕获所有连接和消息错误
4. **重连机制**: 实现自动重连逻辑
5. **消息队列**: 对于重要消息实现重发机制
6. **线程安全**: 多线程环境下注意消息收发的同步

## 🚨 常见问题

**Q: 连接后立即断开？**
A: 检查是否在5秒内发送了首条心跳消息

**Q: 消息发送失败？**  
A: 检查protobuf格式和网络连接

**Q: 认证失败？**
A: 检查Token是否有效，确认认证配置

**Q: 异地登录被踢？**
A: 同一player_id只能有一个连接，新连接会踢掉旧连接

**Q: 消息接收不到？**
A: 检查消息类型和service_name是否正确

**Q: 推送消息丢失？**
A: 检查玩家是否在线，确认player_id正确

**Q: 推送消息处理失败？**
A: 检查payload格式，确保正确解析GameMessageResponse

---
📚 **详细文档**: 参见 `TCP_WebSocket_接口使用说明.md`
🔧 **示例代码**: 参见 `client/` 目录下的Python示例 