# GateServer TCP/WebSocket 接口使用说明

## 服务概述

GateServer (网关服务器) 提供 TCP 和 WebSocket 两种连接方式，作为客户端与后端游戏服务的网关。支持消息转发、认证管理、连接管理等功能。

## 1. 服务端口配置

| 协议类型 | 端口 | 用途 |
|---------|------|------|
| TCP | 6001 | TCP 二进制连接 |
| WebSocket | 6002 | WebSocket 连接 |
| gRPC | 50051 | 服务间 RPC 调用 |
| 监控 | 8082 | 健康检查和监控 |

## 2. 消息协议格式

### 2.1 协议基础

所有消息基于 protobuf 格式，使用 `GameMessage` 作为基础消息结构：

```protobuf
message GameMessage {
  HeadMessage msg_head = 1;     // 消息头
  MessageType msg_type = 2;     // 消息类型  
  bytes payload = 3;            // 消息负载（业务数据）
  string msg_tap = 4;           // 消息标签
  int32 game_id = 5;            // 游戏ID（可选）
}
```

### 2.2 消息头格式

```protobuf
message HeadMessage {
  string player_id = 1;                     // 玩家ID (必须)
  int32 client_type = 2;                    // 客户端类型
  string client_id = 3;                     // 客户端ID
  int64 role_id = 4;                        // 角色ID
  string service_name = 5;                  // 目标服务名
  string group = 6;                         // 服务分组
  string instance_id = 7;                   // 指定实例ID
  string token = 8;                         // 认证Token
  string request_id = 9;                    // 请求ID
  ServiceMessageType service_msg_type = 10; // 服务消息类型
  string load_balance_strategy = 11;        // 负载均衡策略
  int64 timestamp = 12;                     // 时间戳（毫秒）
}
```

### 2.3 消息类型枚举

```protobuf
enum MessageType {
  HEARTBEAT = 0;           // 心跳消息
  SERVICE_MESSAGE = 1;     // 服务消息（转发到后端）
  CLIENT_MESSAGE = 2;      // 客户端消息（网关处理）
  BROADCAST_MESSAGE = 3;   // 广播消息
}

enum ServiceMessageType {
  SYNC = 0;   // 同步等待回包
  ASYNC = 1;  // 异步不等待回包
}
```

## 3. TCP 连接协议

### 3.1 消息格式

TCP 使用自定义二进制协议：

```
[4字节长度头][消息体]
```

- **长度头**: 小端序整数，表示消息体长度
- **消息体**: protobuf 序列化的 `GameMessage`

### 3.2 TCP 客户端示例 (Python)

```python
import socket
import struct
from common_pb2 import GameMessage, HeadMessage, MessageType

class TCPClient:
    def __init__(self, host="127.0.0.1", port=6001):
        self.host = host
        self.port = port
        self.socket = None
        
    def connect(self):
        self.socket = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        self.socket.connect((self.host, self.port))
        
    def send_message(self, game_message):
        # 序列化消息
        data = game_message.SerializeToString()
        
        # 构造长度头（小端序）
        length = len(data)
        length_header = struct.pack('<I', length)
        
        # 发送长度头 + 消息体
        self.socket.send(length_header + data)
        
    def receive_message(self):
        # 读取长度头
        length_data = self.socket.recv(4)
        if not length_data:
            return None
            
        length = struct.unpack('<I', length_data)[0]
        
        # 读取消息体
        data = b''
        while len(data) < length:
            chunk = self.socket.recv(length - len(data))
            if not chunk:
                break
            data += chunk
            
        # 反序列化
        game_message = GameMessage()
        game_message.ParseFromString(data)
        return game_message
```

### 3.3 首条消息（连接验证）

客户端连接后**必须**首先发送心跳消息进行验证：

```python
def send_initial_heartbeat(self, player_id, token=""):
    heartbeat = GameMessage()
    heartbeat.msg_head.player_id = player_id
    heartbeat.msg_head.token = token  # 如果启用认证
    heartbeat.msg_type = MessageType.HEARTBEAT
    
    self.send_message(heartbeat)
```

## 4. WebSocket 连接协议

### 4.1 连接建立

```javascript
// JavaScript WebSocket 客户端示例
const ws = new WebSocket('ws://127.0.0.1:6002');

ws.onopen = function() {
    console.log('WebSocket 连接已建立');
    // 发送首条心跳消息
    sendHeartbeat();
};

ws.onmessage = function(event) {
    // event.data 是 ArrayBuffer 格式的 protobuf 数据
    const gameMessage = GameMessage.decode(new Uint8Array(event.data));
    handleMessage(gameMessage);
};
```

### 4.2 消息收发

WebSocket 使用二进制消息类型（Binary Frame）：

```javascript
function sendMessage(gameMessage) {
    // 序列化 protobuf 消息
    const data = GameMessage.encode(gameMessage).finish();
    
    // 发送二进制数据
    ws.send(data);
}

function sendHeartbeat(playerId, token = "") {
    const heartbeat = {
        msgHead: {
            playerId: playerId,
            token: token
        },
        msgType: 0  // HEARTBEAT
    };
    
    sendMessage(heartbeat);
}
```

## 5. 消息处理流程

### 5.1 连接建立流程

1. **客户端连接**: TCP连接到6001端口 或 WebSocket连接到6002端口
2. **首条验证**: 5秒内发送心跳消息（包含player_id）
3. **认证检查**: 如启用认证，验证Token
4. **会话建立**: 服务器记录玩家会话和路由信息
5. **正常通信**: 开始消息收发和定期心跳

### 5.2 心跳保活

- **客户端**: 每25秒发送一次心跳消息
- **服务器**: 60秒未收到消息则断开连接
- **消息格式**: `msg_type = HEARTBEAT`

```python
# 心跳消息示例
def create_heartbeat(player_id):
    return GameMessage(
        msg_head=HeadMessage(player_id=player_id),
        msg_type=MessageType.HEARTBEAT
    )
```

### 5.3 业务消息处理

#### 服务消息 (SERVICE_MESSAGE)
用于转发到后端服务：

```python
def create_service_message(player_id, service_name, payload_data):
    return GameMessage(
        msg_head=HeadMessage(
            player_id=player_id,
            service_name=service_name,  # 如: "gomokusvr"
            service_msg_type=ServiceMessageType.SYNC
        ),
        msg_type=MessageType.SERVICE_MESSAGE,
        payload=payload_data  # 业务数据
    )
```

#### 客户端消息 (CLIENT_MESSAGE)
由网关内部处理的消息：

```python
def create_client_message(player_id, payload_data):
    return GameMessage(
        msg_head=HeadMessage(player_id=player_id),
        msg_type=MessageType.CLIENT_MESSAGE,
        payload=payload_data
    )
```

### 5.4 服务器推送消息

#### 推送消息接收
服务器可以主动向客户端推送消息，客户端需要能够处理这些推送：

```python
def handle_push_message(self, game_message):
    """处理服务器推送的消息"""
    if game_message.msg_type == MessageType.SERVICE_MESSAGE:
        # 业务服务推送的消息
        if game_message.payload:
            try:
                # 解析业务数据
                response = GameMessageResponse()
                response.ParseFromString(game_message.payload)
                
                # 根据原始服务名称处理
                service_name = ""
                if game_message.msg_head:
                    service_name = game_message.msg_head.service_name
                
                self._handle_service_push(service_name, response)
                
            except Exception as e:
                self.logger.error(f"解析推送消息失败: {e}")
    
    elif game_message.msg_type == MessageType.CLIENT_MESSAGE:
        # 网关推送的客户端消息
        self._handle_gateway_push(game_message)

def _handle_service_push(self, service_name, response):
    """处理业务服务推送的消息"""
    if service_name == "gomokusvr":
        # 处理五子棋服务推送
        self._handle_gomoku_push(response)
    elif service_name == "chatservice":
        # 处理聊天服务推送
        self._handle_chat_push(response)
    # 其他服务...

def _handle_gateway_push(self, game_message):
    """处理网关推送的消息"""
    payload = game_message.payload.decode('utf-8')
    
    if payload.startswith("CONNECTION_AUTH_FAILED"):
        # 认证失败通知
        self.logger.error("连接认证失败，即将断开")
        self.disconnect()
    elif payload.startswith("MSG_AUTH_FAILED"):
        # 消息认证失败通知
        self.logger.warning("消息认证失败")
    # 其他网关消息...
```

#### 推送消息类型

| 推送类型 | 消息来源 | 用途 | msg_type |
|---------|----------|------|----------|
| 业务推送 | 后端服务 | 游戏状态变化、事件通知 | SERVICE_MESSAGE |
| 系统推送 | 网关 | 认证状态、连接管理 | CLIENT_MESSAGE |
| 广播推送 | 任意服务 | 全服通知、公告 | BROADCAST_MESSAGE |

#### 服务端推送接口 (gRPC)

后端服务可以通过 gRPC 接口向指定客户端推送消息：

```protobuf
service GateSvr {
  // 推送消息到客户端
  rpc PushToClient(PushRequest) returns (PushResponse);
}

message PushRequest {
  string player_id = 1;           // 目标玩家ID
  string ip = 2;                  // 目标IP（可选，优先使用player_id）
  CallbackType cb_type = 3;       // 回调类型
  common.GameMessage message = 4; // 要推送的消息
}

enum CallbackType {
  SYNC = 0;   // 同步推送，等待确认
  ASYNC = 1;  // 异步推送，不等待确认
  PUSH = 2;   // 单向推送
}
```

#### 推送消息示例

**Go 服务端推送代码**：
```go
// 推送五子棋游戏状态变化
func PushGameStateChange(playerID string, gameData []byte) error {
    // 构造游戏消息
    gameMsg := &commonpb.GameMessage{
        MsgHead: &commonpb.HeadMessage{
            PlayerId:    playerID,
            ServiceName: "gomokusvr",
        },
        MsgType: commonpb.MessageType_SERVICE_MESSAGE,
        Payload: gameData,
    }
    
    // 推送请求
    req := &pb.PushRequest{
        PlayerId: playerID,
        CbType:   pb.CallbackType_ASYNC,
        Message:  gameMsg,
    }
    
    // 调用网关推送接口
    return rpc.PushToClient(req)
}

// 推送系统通知
func PushSystemNotification(playerID string, message string) error {
    gameMsg := &commonpb.GameMessage{
        MsgHead: &commonpb.HeadMessage{
            PlayerId: playerID,
        },
        MsgType: commonpb.MessageType_CLIENT_MESSAGE,
        Payload: []byte(message),
    }
    
    req := &pb.PushRequest{
        PlayerId: playerID,
        CbType:   pb.CallbackType_PUSH,
        Message:  gameMsg,
    }
    
    return rpc.PushToClient(req)
}
```

**客户端完整接收示例**：
```python
class MessageHandler:
    def __init__(self, client):
        self.client = client
        self.logger = logging.getLogger(__name__)
        
    def handle_received_message(self, game_message):
        """处理接收到的消息（包括推送）"""
        try:
            msg_type = game_message.msg_type
            
            if msg_type == MessageType.HEARTBEAT:
                # 心跳响应，通常不需要处理
                pass
                
            elif msg_type == MessageType.SERVICE_MESSAGE:
                # 业务服务的响应或推送
                self._handle_service_message(game_message)
                
            elif msg_type == MessageType.CLIENT_MESSAGE:
                # 网关的响应或推送
                self._handle_client_message(game_message)
                
            elif msg_type == MessageType.BROADCAST_MESSAGE:
                # 广播消息
                self._handle_broadcast_message(game_message)
                
        except Exception as e:
            self.logger.error(f"处理消息失败: {e}")
    
    def _handle_service_message(self, game_message):
        """处理业务服务消息"""
        service_name = ""
        if game_message.msg_head:
            service_name = game_message.msg_head.service_name
            
        # 解析响应数据
        if game_message.payload:
            try:
                response = GameMessageResponse()
                response.ParseFromString(game_message.payload)
                
                if response.ret == 0:
                    # 成功响应
                    self._handle_service_success(service_name, response.data)
                else:
                    # 错误响应
                    self._handle_service_error(service_name, response.ret, response.reason)
                    
            except Exception as e:
                self.logger.error(f"解析服务消息失败: {e}")
    
    def _handle_service_success(self, service_name, data):
        """处理业务服务成功响应"""
        if service_name == "gomokusvr":
            # 处理五子棋服务响应/推送
            self._handle_gomoku_data(data)
        elif service_name == "chatservice":
            # 处理聊天服务响应/推送
            self._handle_chat_data(data)
        # 添加更多服务处理...
        
    def _handle_gomoku_data(self, data):
        """处理五子棋相关数据"""
        # 这里可以是游戏状态变化、对手下棋等推送
        print(f"收到五子棋推送数据: {data}")
        
    def _handle_client_message(self, game_message):
        """处理网关客户端消息"""
        payload = game_message.payload.decode('utf-8') if game_message.payload else ""
        
        if "AUTH_FAILED" in payload:
            self.logger.error(f"认证失败: {payload}")
            self.client.disconnect()
        elif "KICK" in payload:
            self.logger.warning(f"被踢下线: {payload}")
            self.client.disconnect()
        else:
                         self.logger.info(f"收到网关消息: {payload}")
```

#### 推送消息流程图

```
后端服务               GateServer               客户端
    |                      |                      |
    | PushToClient(gRPC)   |                      |
    |--------------------->|                      |
    |                      | 1.本地查找玩家连接    |
    |                      |                      |
    |                      | 2.发送GameMessage    |
    |                      |--------------------->|
    |                      |                      | 3.处理推送消息
    |                      |                      |
    | PushResponse         |                      |
    |<---------------------|                      |
    |                      |                      |
    
如果玩家在其他网关节点：
    |                      |                      |
    |                      | 4.查找玩家路由        |
    |                      |                      |
    |                      | 5.转发到目标网关      |
    |                      |--------------------->| 其他GateServer
    |                      |                      |
```

#### WebSocket推送示例

```javascript
// WebSocket客户端处理推送
ws.onmessage = function(event) {
    const data = new Uint8Array(event.data);
    const gameMessage = proto.common.GameMessage.deserializeBinary(data);
    
    // 区分推送和响应
    const msgType = gameMessage.getMsgType();
    
    if (msgType === proto.common.MessageType.SERVICE_MESSAGE) {
        // 业务服务推送
        const serviceName = gameMessage.getMsgHead().getServiceName();
        const payload = gameMessage.getPayload();
        
        // 解析响应数据
        const response = proto.common.GameMessageResponse.deserializeBinary(payload);
        
        if (serviceName === 'gomokusvr') {
            handleGomokuPush(response);
        } else if (serviceName === 'chatservice') {
            handleChatPush(response);
        }
        
    } else if (msgType === proto.common.MessageType.CLIENT_MESSAGE) {
        // 网关系统推送
        const message = new TextDecoder().decode(gameMessage.getPayload());
        handleSystemPush(message);
    }
};

function handleGomokuPush(response) {
    if (response.getRet() === 0) {
        // 游戏状态变化推送
        const gameData = response.getData();
        console.log('收到五子棋状态推送:', gameData);
        updateGameUI(gameData);
    }
}

function handleSystemPush(message) {
    if (message.includes('KICK')) {
        alert('您已被踢下线');
        ws.close();
    } else if (message.includes('AUTH_FAILED')) {
        alert('认证失败，请重新登录');
        ws.close();
    }
}
```

## 6. 认证机制

### 6.1 Token 生成

通过 gRPC 接口生成认证Token：

```protobuf
message GenerateAuthTokenRequest {
  string player_id = 1;           // 玩家ID（必须）
  string username = 2;            // 用户名
  int32 level = 3;                // 等级
  bool is_vip = 4;                // VIP状态
  repeated string permissions = 5; // 权限列表
  string platform = 6;            // 平台类型
  string device_id = 7;           // 设备ID
  int64 expire_hours = 8;         // 过期时间(小时)
}
```

### 6.2 Token 使用

在消息头中携带Token：

```python
def create_authenticated_message(player_id, token, service_name, payload):
    return GameMessage(
        msg_head=HeadMessage(
            player_id=player_id,
            token=token,
            service_name=service_name
        ),
        msg_type=MessageType.SERVICE_MESSAGE,
        payload=payload
    )
```

## 7. 错误处理

### 7.1 常见错误码

| 错误码 | 说明 |
|--------|------|
| 0 | 成功 |
| 1001 | 玩家ID无效 |
| 1002 | Token生成失败 |
| 2001 | 玩家不在线 |
| 3001 | 推送失败 |
| 3002 | 转发失败 |
| 5001 | 服务错误 |

### 7.2 错误响应格式

服务器返回错误时使用 `GameMessageResponse`:

```protobuf
message GameMessageResponse {
  HeadMessage msg_head = 1;       // 原消息头
  int32 ret = 2;                  // 返回码（0=成功）
  oneof payload {
    string reason = 3;            // 错误原因
    bytes data = 4;               // 响应数据
  }
  int64 response_timestamp = 5;   // 响应时间戳
}
```

## 8. 配置说明

### 8.1 连接配置

```yaml
connection:
  first_message_timeout_sec: 5     # 首条消息超时
  read_timeout_sec: 60             # 读取超时  
  write_timeout_sec: 10            # 写入超时
  max_message_size: 4194304        # 最大消息大小(4MB)
  heartbeat_interval_sec: 30       # 心跳间隔
```

### 8.2 认证配置

```yaml
auth:
  enabled: false                   # 是否启用认证
  secret_key: "your-secret-key"    # JWT密钥
  token_expire_hours: 24           # Token过期时间
  require_auth:                    # 需要认证的消息类型
    - "SERVICE_MESSAGE"
```

### 8.3 限流配置

```yaml
rate_limit:
  enabled: true                    # 启用限流
  global_rate: 500                 # 全局每秒请求数
  player_rate: 10                  # 单玩家每秒请求数
  service_rate: 100                # 单服务每秒请求数
```

## 9. 使用示例

### 9.1 简单连接示例

```python
#!/usr/bin/env python3
from tcp_client import TCPClient
from common_pb2 import *

def main():
    # 创建客户端
    client = TCPClient("127.0.0.1", 6001)
    
    # 连接并发送首条心跳
    if client.connect("player_123"):
        print("连接成功")
        
        # 发送业务消息到五子棋服务
        service_msg = GameMessage(
            msg_head=HeadMessage(
                player_id="player_123",
                service_name="gomokusvr",
                service_msg_type=ServiceMessageType.SYNC
            ),
            msg_type=MessageType.SERVICE_MESSAGE,
            payload=b"your_game_data"
        )
        
        client.send_message(service_msg)
        
        # 接收响应
        response = client.receive_message()
        print(f"收到响应: {response}")
        
    else:
        print("连接失败")

if __name__ == "__main__":
    main()
```

### 9.2 WebSocket示例

```html
<!DOCTYPE html>
<html>
<head>
    <title>WebSocket 客户端</title>
    <script src="protobuf.min.js"></script>
    <script src="common_pb.js"></script>
</head>
<body>
    <script>
        const ws = new WebSocket('ws://127.0.0.1:6002');
        
        ws.onopen = function() {
            console.log('WebSocket连接成功');
            
            // 发送首条心跳
            const heartbeat = new proto.common.GameMessage();
            const head = new proto.common.HeadMessage();
            head.setPlayerId('web_player_123');
            heartbeat.setMsgHead(head);
            heartbeat.setMsgType(proto.common.MessageType.HEARTBEAT);
            
            ws.send(heartbeat.serializeBinary());
        };
        
        ws.onmessage = function(event) {
            const data = new Uint8Array(event.data);
            const message = proto.common.GameMessage.deserializeBinary(data);
            console.log('收到消息:', message.toObject());
        };
        
        // 定期发送心跳
        setInterval(() => {
            const heartbeat = new proto.common.GameMessage();
            const head = new proto.common.HeadMessage();
            head.setPlayerId('web_player_123');
            heartbeat.setMsgHead(head);
            heartbeat.setMsgType(proto.common.MessageType.HEARTBEAT);
            
            ws.send(heartbeat.serializeBinary());
        }, 25000);
    </script>
</body>
</html>
```

## 10. 故障排查

### 10.1 连接问题

1. **连接超时**: 检查网络和端口
2. **首条消息超时**: 确保连接后5秒内发送心跳
3. **认证失败**: 检查Token是否有效
4. **异地登录**: 同一player_id会踢掉旧连接

### 10.2 消息问题

1. **消息丢失**: 检查消息序列化和网络状态
2. **格式错误**: 确保protobuf格式正确
3. **限流触发**: 降低消息发送频率
4. **服务不可用**: 检查后端服务状态

### 10.3 调试方法

1. **启用结构化日志**: `connection.enable_structured_log: true`
2. **查看服务器日志**: `gatesvr.log`
3. **检查监控接口**: `http://127.0.0.1:8082/health`
4. **使用调试工具**: 如 `client/debug_test.py`

## 版本信息

- **文档版本**: v1.0.0  
- **协议版本**: protobuf 3
- **更新日期**: 2024年12月

---

**注意事项**:
1. 生产环境请修改默认的JWT密钥
2. 建议启用认证和限流保护
3. 定期监控服务状态和性能指标
4. 遵循消息格式规范，确保兼容性 