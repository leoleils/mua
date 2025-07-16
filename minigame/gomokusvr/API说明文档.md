# 五子棋服务 (GomokuSvr) API 说明文档

## 📋 目录

- [服务概述](#服务概述)
- [接口架构](#接口架构)
- [API 接口](#api-接口)
- [消息结构](#消息结构)
- [通知推送](#通知推送)
- [错误码](#错误码)
- [使用示例](#使用示例)
- [部署配置](#部署配置)

## 🎮 服务概述

五子棋服务 (GomokuSvr) 是一个基于 gRPC 的分布式游戏服务，提供完整的五子棋游戏功能，包括房间管理、游戏逻辑、实时消息推送等。

### 核心特性

- ✅ **房间管理**: 创建、加入、离开、销毁房间
- ✅ **游戏逻辑**: 15x15 棋盘，五子连珠获胜
- ✅ **实时通信**: 通过 gatesvr 实现玩家间消息同步
- ✅ **服务发现**: 基于 Nacos 的微服务架构
- ✅ **事件驱动**: Kafka 消息队列支持
- ✅ **负载均衡**: 支持多实例部署

### 技术栈

| 组件 | 技术 | 说明 |
|------|------|------|
| **RPC框架** | gRPC + Protocol Buffers | 高性能通信协议 |
| **服务发现** | Nacos | 微服务注册与发现 |
| **消息队列** | Kafka | 事件驱动架构 |
| **网关服务** | GateSvr | 客户端连接管理 |

## 🏗️ 接口架构

### 服务拓扑

```
客户端 → GateSvr (网关) → GomokuSvr (五子棋服务)
                          ↓
                       Nacos (服务发现)
                          ↓
                       Kafka (事件队列)
```

### gRPC 服务定义

```protobuf
// 通用RPC服务接口，与gatesvr集成
service CommonService {
  rpc SendMessage(GameMessage) returns (GameMessageResponse);
}
```

## 🔌 API 接口

### 统一调用方式

所有业务操作都通过 `SendMessage` 方法调用，通过 `method` 字段区分具体操作：

```protobuf
message GameMessage {
  MessageHead head = 1;        // 消息头
  MessageType msg_type = 2;    // 消息类型
  bytes payload = 3;           // 消息载荷
  int64 timestamp = 4;         // 时间戳
}

message MessageHead {
  string msg_id = 1;           // 消息ID
  string player_id = 2;        // 玩家ID
  string service_name = 3;     // 服务名 ("gomokusvr")
  string method = 4;           // 方法名
}
```

### 1. 房间管理

#### 1.1 创建房间 (`CreateRoom`)

**请求载荷** (`CreateRoomRequest`):
```protobuf
message CreateRoomRequest {
  string room_name = 1;  // 房间名称
  string password = 2;   // 房间密码（可选）
}
```

**响应载荷** (`CreateRoomResponse`):
```protobuf
message CreateRoomResponse {
  bool success = 1;      // 是否成功
  string room_id = 2;    // 房间ID
  string message = 3;    // 响应消息
}
```

#### 1.2 加入房间 (`JoinRoom`)

**请求载荷** (`JoinRoomRequest`):
```protobuf
message JoinRoomRequest {
  string room_id = 1;    // 房间ID
  string password = 2;   // 房间密码（如果有）
}
```

**响应载荷** (`JoinRoomResponse`):
```protobuf
message JoinRoomResponse {
  bool success = 1;      // 是否成功
  string message = 2;    // 响应消息
  RoomInfo room_info = 3; // 房间信息
}
```

#### 1.3 离开房间 (`LeaveRoom`)

**请求载荷** (`LeaveRoomRequest`):
```protobuf
message LeaveRoomRequest {
  string room_id = 1;    // 房间ID
}
```

**响应载荷** (`LeaveRoomResponse`):
```protobuf
message LeaveRoomResponse {
  bool success = 1;      // 是否成功
  string message = 2;    // 响应消息
}
```

#### 1.4 销毁房间 (`DestroyRoom`)

**请求载荷** (`DestroyRoomRequest`):
```protobuf
message DestroyRoomRequest {
  string room_id = 1;    // 房间ID
}
```

**响应载荷** (`DestroyRoomResponse`):
```protobuf
message DestroyRoomResponse {
  bool success = 1;      // 是否成功
  string message = 2;    // 响应消息
}
```

#### 1.5 获取房间列表 (`GetRoomList`)

**请求载荷** (`GetRoomListRequest`):
```protobuf
message GetRoomListRequest {
  int32 page = 1;        // 页码（从1开始）
  int32 page_size = 2;   // 每页大小
}
```

**响应载荷** (`GetRoomListResponse`):
```protobuf
message GetRoomListResponse {
  bool success = 1;                // 是否成功
  repeated RoomInfo rooms = 2;     // 房间列表
  int32 total_count = 3;           // 总数量
  string message = 4;              // 响应消息
}
```

### 2. 游戏操作

#### 2.1 玩家准备 (`PlayerReady`)

**请求载荷** (`PlayerReadyRequest`):
```protobuf
message PlayerReadyRequest {
  string room_id = 1;    // 房间ID
}
```

**响应载荷** (`PlayerReadyResponse`):
```protobuf
message PlayerReadyResponse {
  bool success = 1;      // 是否成功
  string message = 2;    // 响应消息
  bool is_ready = 3;     // 当前准备状态
  RoomInfo room_info = 4; // 更新后的房间信息
}
```

#### 2.2 开始游戏 (`StartGame`)

**权限要求**: 仅房主可以开始游戏  
**前置条件**: 房间内有2名玩家且都已准备就绪  

**请求载荷** (`StartGameRequest`):
```protobuf
message StartGameRequest {
  string room_id = 1;    // 房间ID
}
```

**响应载荷** (`StartGameResponse`):
```protobuf
message StartGameResponse {
  bool success = 1;      // 是否成功
  string message = 2;    // 响应消息
  GameState game_state = 3; // 游戏状态
}
```

**状态检查**:
- 检查玩家是否为房主
- 检查房间状态是否为 `WAITING`
- 检查房间是否有且仅有2名玩家
- 检查所有玩家是否已准备就绪

**通知行为**:
- 成功开始游戏后，向房间内所有玩家推送 `GameStartNotification` 通知
- 房间状态自动更新为 `PLAYING`
- 游戏状态重置，黑子先手

#### 2.3 下棋 (`PlacePiece`)

**请求载荷** (`PlacePieceRequest`):
```protobuf
message PlacePieceRequest {
  string room_id = 1;    // 房间ID
  int32 x = 2;           // X坐标（0-14）
  int32 y = 3;           // Y坐标（0-14）
}
```

**响应载荷** (`PlacePieceResponse`):
```protobuf
message PlacePieceResponse {
  bool success = 1;      // 是否成功
  string message = 2;    // 响应消息
  GameState game_state = 3; // 游戏状态
}
```

#### 2.4 获取游戏进度 (`GetGameProgress`)

**功能描述**: 获取指定房间的完整游戏进度信息，包括房间状态、游戏状态、棋盘状态和剩余时间等。

**权限要求**: 只有房间内的玩家可以查询该房间的游戏进度

**请求载荷** (`GetGameProgressRequest`):
```protobuf
message GetGameProgressRequest {
  string room_id = 1;    // 房间ID
}
```

**响应载荷** (`GetGameProgressResponse`):
```protobuf
message GetGameProgressResponse {
  bool success = 1;           // 是否成功
  string message = 2;         // 响应消息
  RoomInfo room_info = 3;     // 房间信息
  GameState game_state = 4;   // 游戏状态
  int32 remaining_time = 5;   // 剩余时间（秒）-1表示无限制
}
```

**状态检查**:
- 检查房间是否存在
- 检查玩家是否在房间中
- 返回完整的房间和游戏状态信息

**返回信息**:
- **房间信息**: 房间ID、名称、状态、玩家列表、房主等
- **游戏状态**: 棋盘状态、当前轮次、游戏结果、走棋历史等  
- **剩余时间**: 当前轮次剩余时间（当前实现为-1，表示无时间限制）

**使用场景**:
- 玩家重新连接时恢复游戏状态
- 观战模式获取当前游戏进度
- 前端界面刷新时同步最新状态
- 调试和监控游戏状态

## 📋 消息结构

### 核心数据结构

#### 房间信息 (`RoomInfo`)
```protobuf
message RoomInfo {
  string room_id = 1;              // 房间ID
  string room_name = 2;            // 房间名称
  RoomStatus status = 3;           // 房间状态
  repeated PlayerInfo players = 4; // 玩家列表
  GameState game_state = 5;        // 游戏状态
  string owner_id = 6;             // 房主ID
  int64 created_time = 7;          // 创建时间
}
```

#### 玩家信息 (`PlayerInfo`)
```protobuf
message PlayerInfo {
  string player_id = 1;    // 玩家ID
  string username = 2;     // 用户名
  PlayerColor color = 3;   // 棋子颜色
  bool is_ready = 4;       // 是否准备
  int64 join_time = 5;     // 加入时间
}
```

#### 游戏状态 (`GameState`)
```protobuf
message GameState {
  repeated int32 board = 1;           // 15x15棋盘（225个元素）
  PlayerColor current_turn = 2;       // 当前回合
  GameResult result = 3;              // 游戏结果
  string winner_id = 4;               // 获胜者ID
  int32 total_moves = 5;              // 总步数
  repeated Move move_history = 6;     // 走棋历史
}
```

#### 走棋记录 (`Move`)
```protobuf
message Move {
  string player_id = 1;      // 玩家ID
  PlayerColor color = 2;     // 棋子颜色
  int32 x = 3;               // X坐标
  int32 y = 4;               // Y坐标
  int32 move_number = 5;     // 步数
  int64 timestamp = 6;       // 时间戳
}
```

### 枚举类型

#### 房间状态 (`RoomStatus`)
```protobuf
enum RoomStatus {
  WAITING = 0;     // 等待玩家
  PLAYING = 1;     // 游戏中
  FINISHED = 2;    // 游戏结束
}
```

#### 棋子颜色 (`PlayerColor`)
```protobuf
enum PlayerColor {
  NONE = 0;        // 无颜色
  BLACK = 1;       // 黑子（先手）
  WHITE = 2;       // 白子（后手）
}
```

#### 游戏结果 (`GameResult`)
```protobuf
enum GameResult {
  ONGOING = 0;     // 进行中
  BLACK_WIN = 1;   // 黑子获胜
  WHITE_WIN = 2;   // 白子获胜
  DRAW = 3;        // 平局
}
```

## 🔔 通知推送

服务通过 gatesvr 向客户端推送实时通知，确保所有玩家能够及时收到游戏状态变化。

### 推送机制

- **协议**: gRPC `PushToClient` 接口
- **方式**: 异步推送，不等待客户端确认
- **目标**: 房间内相关玩家
- **格式**: protobuf 二进制序列化

### gRPC 推送协议格式

所有推送消息都通过 gatesvr 的 `PushToClient` gRPC 接口发送：

```protobuf
// gatesvr 推送请求
message PushRequest {
  string player_id = 1;           // 目标玩家ID
  string ip = 2;                  // 目标IP（可选）
  CallbackType cb_type = 3;       // 回调类型：PUSH
  common.GameMessage message = 4; // 推送的游戏消息
}

// 推送的游戏消息格式
message GameMessage {
  HeadMessage msg_head = 1;       // 消息头
  MessageType msg_type = 2;       // 消息类型：CLIENT_MESSAGE
  bytes payload = 3;              // 消息负载（具体通知内容）
  string msg_tap = 4;             // 消息标签
  int32 game_id = 5;              // 游戏ID
}

// 消息头格式
message HeadMessage {
  string player_id = 1;           // 目标玩家ID
  string service_name = 5;        // 服务名："gomokusvr"
  string request_id = 9;          // 请求ID（通知类型名称）
  int64 timestamp = 12;           // 时间戳
}
```

### 通知触发流程

1. **房主开始游戏** → 触发 `GameStartNotification` → 推送给房间内所有玩家
2. **玩家下棋** → 触发 `PiecePlacedNotification` → 推送给房间内其他玩家
3. **玩家加入房间** → 触发 `PlayerJoinedNotification` → 推送给房间内其他玩家
4. **玩家离开房间** → 触发 `PlayerLeftNotification` → 推送给房间内其他玩家
5. **玩家准备状态变化** → 触发 `PlayerReadyNotification` → 推送给房间内其他玩家

### 支持的通知类型

#### 1. 棋子放置通知 (`PiecePlacedNotification`)

**触发时机**: 玩家成功下棋后  
**推送对象**: 房间内除当前玩家外的所有其他玩家  

**完整推送协议**:
```protobuf
// gRPC PushRequest
{
  "player_id": "target_player_123",
  "cb_type": "PUSH",
  "message": {
    "msg_head": {
      "player_id": "target_player_123",
      "service_name": "gomokusvr",
      "request_id": "PiecePlacedNotification",
      "timestamp": 1699123456789
    },
    "msg_type": "CLIENT_MESSAGE",
    "payload": <PlacePieceResponse序列化后的bytes>
  }
}
```

**Payload 内容** (`PlacePieceResponse`):
```protobuf
message PlacePieceResponse {
  bool success = 1;           // 是否成功
  string message = 2;         // 响应消息
  GameState game_state = 3;   // 更新后的游戏状态
}
```

#### 2. 游戏开始通知 (`GameStartNotification`)

**触发时机**: 房主成功开始游戏时  
**推送对象**: 房间内所有玩家（包括房主）  

**完整推送协议**:
```protobuf
// gRPC PushRequest
{
  "player_id": "target_player_123",
  "cb_type": "PUSH", 
  "message": {
    "msg_head": {
      "player_id": "target_player_123",
      "service_name": "gomokusvr",
      "request_id": "GameStartNotification",
      "timestamp": 1699123456789
    },
    "msg_type": "CLIENT_MESSAGE",
    "payload": <GameStateNotify序列化后的bytes>
  }
}
```

**Payload 内容** (`GameStateNotify`):
```protobuf
message GameStateNotify {
  string room_id = 1;          // 房间ID
  GameState game_state = 2;    // 游戏状态
  string event_type = 3;       // 事件类型: "GAME_START"
  string event_message = 4;    // 事件消息: "游戏开始！"
}
```

#### 3. 游戏状态变化通知 (`GameStateChangedNotification`)

**触发时机**: 游戏状态发生变化时  
**推送对象**: 房间内所有玩家  

**完整推送协议**:
```protobuf
// gRPC PushRequest
{
  "player_id": "target_player_123",
  "cb_type": "PUSH",
  "message": {
    "msg_head": {
      "player_id": "target_player_123", 
      "service_name": "gomokusvr",
      "request_id": "GameStateChangedNotification",
      "timestamp": 1699123456789
    },
    "msg_type": "CLIENT_MESSAGE",
    "payload": <GameStateNotify序列化后的bytes>
  }
}
```

**Payload 内容** (`GameStateNotify`):
```protobuf
message GameStateNotify {
  string room_id = 1;          // 房间ID
  GameState game_state = 2;    // 游戏状态
  string event_type = 3;       // 事件类型: "GAME_STATE_CHANGED"
  string event_message = 4;    // 事件消息: "游戏状态已更新"
}
```

#### 4. 玩家事件通知

##### 玩家加入通知 (`PlayerJoinedNotification`)
**触发时机**: 新玩家加入房间  
**推送对象**: 房间内除新玩家外的其他玩家  

##### 玩家离开通知 (`PlayerLeftNotification`)
**触发时机**: 玩家离开房间  
**推送对象**: 房间内除离开玩家外的其他玩家  

##### 玩家准备通知 (`PlayerReadyNotification`)
**触发时机**: 玩家准备状态变化  
**推送对象**: 房间内除当前玩家外的其他玩家  

**完整推送协议**（以PlayerJoinedNotification为例）:
```protobuf
// gRPC PushRequest
{
  "player_id": "target_player_456",
  "cb_type": "PUSH",
  "message": {
    "msg_head": {
      "player_id": "target_player_456",
      "service_name": "gomokusvr", 
      "request_id": "PlayerJoinedNotification",
      "timestamp": 1699123456789
    },
    "msg_type": "CLIENT_MESSAGE",
    "payload": <PlayerEventNotify序列化后的bytes>
  }
}
```

**Payload 内容** (`PlayerEventNotify`):
```protobuf
message PlayerEventNotify {
  string room_id = 1;          // 房间ID
  string player_id = 2;        // 事件相关的玩家ID
  string event_type = 3;       // 事件类型：JOIN/LEAVE/READY
  PlayerInfo player_info = 4;  // 玩家信息（可选）
}
```

**各事件类型的 event_type 值**:
- `PlayerJoinedNotification`: `"JOIN"`
- `PlayerLeftNotification`: `"LEAVE"`  
- `PlayerReadyNotification`: `"READY"`

## ❌ 错误码

### 通用错误码

| 错误码 | 说明 | 解决方案 |
|--------|------|----------|
| 0 | 成功 | - |
| 400 | 请求参数错误 | 检查请求参数格式和内容 |
| 404 | 房间不存在 | 确认房间ID是否正确 |
| 403 | 权限不足 | 确认玩家是否有执行该操作的权限 |
| 409 | 状态冲突 | 检查游戏或房间状态是否允许该操作 |
| 500 | 服务器内部错误 | 联系技术支持 |

### 业务错误

| 场景 | 错误信息 | 说明 |
|------|----------|------|
| 房间已满 | "房间已满，无法加入" | 房间已有2名玩家 |
| 游戏进行中 | "游戏进行中，无法加入" | 房间状态为 PLAYING |
| 坐标越界 | "坐标超出范围" | x,y 坐标必须在 0-14 之间 |
| 位置已占用 | "位置已有棋子" | 该坐标已有棋子 |
| 游戏已结束 | "游戏已结束" | 无法在已结束的游戏中下棋 |
| 非当前回合 | "不是你的回合" | 当前不是该玩家的下棋回合 |

## 💡 使用示例

### 1. 创建房间示例

```protobuf
// 请求
{
  "head": {
    "msg_id": "msg_001",
    "player_id": "player_123",
    "service_name": "gomokusvr",
    "method": "CreateRoom"
  },
  "msg_type": "REQUEST",
  "payload": <CreateRoomRequest序列化后的bytes>,
  "timestamp": 1699123456789
}

// CreateRoomRequest 内容
{
  "room_name": "我的五子棋房间",
  "password": ""
}
```

### 2. 下棋示例

```protobuf
// 请求
{
  "head": {
    "msg_id": "msg_002", 
    "player_id": "player_123",
    "service_name": "gomokusvr",
    "method": "PlacePiece"
  },
  "msg_type": "REQUEST",
  "payload": <PlacePieceRequest序列化后的bytes>,
  "timestamp": 1699123456789
}

// PlacePieceRequest 内容
{
  "room_id": "room_001",
  "x": 7,
  "y": 7
}
```

### 3. 客户端接收通知示例

客户端会收到来自 gatesvr 的推送消息，包含完整的协议头和消息内容：

#### 3.1 棋子放置通知示例

```protobuf
// 完整的推送消息格式
{
  "msg_head": {
    "player_id": "player_456",              // 接收通知的玩家ID
    "service_name": "gomokusvr",            // 服务名
    "request_id": "PiecePlacedNotification", // 通知类型
    "timestamp": 1699123456789              // 时间戳
  },
  "msg_type": "CLIENT_MESSAGE",             // 消息类型
  "payload": <PlacePieceResponse序列化后的bytes>
}

// payload 反序列化后的内容 (PlacePieceResponse)
{
  "success": true,
  "message": "下棋成功",
  "game_state": {
    "board": [0, 0, 0, ..., 1, 0, ...],     // 棋盘状态
    "current_turn": "WHITE",                // 当前轮次
    "result": "ONGOING",                    // 游戏结果
    "total_moves": 3,                       // 总步数
    "move_history": [                       // 走棋历史
      {
        "player_id": "player_123",
        "color": "BLACK",
        "x": 7,
        "y": 7,
        "move_number": 1
      }
    ]
  }
}
```

#### 3.2 游戏开始通知示例

```protobuf
// 完整的推送消息格式
{
  "msg_head": {
    "player_id": "player_123",
    "service_name": "gomokusvr", 
    "request_id": "GameStartNotification",
    "timestamp": 1699123456789
  },
  "msg_type": "CLIENT_MESSAGE",
  "payload": <GameStateNotify序列化后的bytes>
}

// payload 反序列化后的内容 (GameStateNotify)
{
  "room_id": "room_001",
  "game_state": {
    "board": [0, 0, 0, ...],                // 初始空白棋盘
    "current_turn": "BLACK",                // 黑子先手
    "result": "ONGOING",                    // 游戏进行中
    "total_moves": 0                        // 初始步数为0
  },
  "event_type": "GAME_START",
  "event_message": "游戏开始！"
}
```

#### 3.3 玩家事件通知示例

```protobuf
// 玩家加入通知
{
  "msg_head": {
    "player_id": "player_456",
    "service_name": "gomokusvr",
    "request_id": "PlayerJoinedNotification",
    "timestamp": 1699123456789
  },
  "msg_type": "CLIENT_MESSAGE", 
  "payload": <PlayerEventNotify序列化后的bytes>
}

// payload 反序列化后的内容 (PlayerEventNotify)
{
  "room_id": "room_001",
  "player_id": "player_789",               // 新加入的玩家ID
  "event_type": "JOIN",
  "player_info": {                         // 新玩家信息
    "player_id": "player_789",
    "username": "新玩家",
    "color": "WHITE",
    "is_ready": false,
    "join_time": 1699123456789
  }
}
```

### 4. 获取游戏进度示例

```protobuf
// 请求
{
  "head": {
    "msg_id": "msg_003",
    "player_id": "player_123", 
    "service_name": "gomokusvr",
    "method": "GetGameProgress"
  },
  "msg_type": "REQUEST",
  "payload": <GetGameProgressRequest序列化后的bytes>,
  "timestamp": 1699123456789
}

// GetGameProgressRequest 内容
{
  "room_id": "room_001"
}

// 响应示例
{
  "success": true,
  "message": "获取游戏进度成功",
  "room_info": {
    "room_id": "room_001",
    "room_name": "我的五子棋房间",
    "status": "PLAYING",
    "players": [
      {
        "player_id": "player_123",
        "username": "玩家1",
        "color": "BLACK",
        "is_ready": true
      },
      {
        "player_id": "player_456", 
        "username": "玩家2",
        "color": "WHITE",
        "is_ready": true
      }
    ],
    "owner_id": "player_123"
  },
  "game_state": {
    "board": [0, 0, 0, ..., 1, 2, 0, ...], // 225个元素的棋盘数组
    "current_turn": "WHITE",
    "result": "ONGOING",
    "total_moves": 5,
    "move_history": [
      {
        "player_id": "player_123",
        "color": "BLACK", 
        "x": 7,
        "y": 7,
        "move_number": 1
      }
      // ... 更多走棋记录
    ]
  },
  "remaining_time": -1  // -1表示无时间限制
}
```

## ⚙️ 部署配置

### 服务端口

| 端口 | 协议 | 用途 |
|------|------|------|
| 50052 | gRPC | 对外服务端口 |

### 环境变量

| 变量名 | 说明 | 示例 |
|--------|------|------|
| `NACOS_USERNAME` | Nacos 用户名 | `nacos_user` |
| `NACOS_PASSWORD` | Nacos 密码 | `nacos_pass` |

### 配置文件

主配置文件：`config.yaml` 或 `config-local.yaml`

**关键配置项**：

```yaml
# 服务配置
service:
  name: "gomokusvr"
  port: 50052

# Nacos 配置
nacos:
  addr: "nacos.example.com"
  port: 8848
  enable_register: true

# 游戏配置
game:
  room:
    max_rooms: 1000
    max_players_per_room: 2
  board:
    size: 15
    win_condition: 5

# 通知配置  
notification:
  enabled: true
  workers: 3
  timeout_sec: 3
```

### 依赖服务

1. **GateSvr** (必需)
   - 负责客户端连接管理
   - 提供消息推送功能

2. **Nacos** (可选)
   - 服务注册与发现
   - 配置管理

3. **Kafka** (可选)
   - 事件驱动架构
   - 跨服务消息传递

### 启动命令

```bash
# 使用默认配置
./gomokusvr

# 使用指定配置文件
./gomokusvr -config config-local.yaml
```

### 健康检查

服务启动后可通过日志确认状态：

```
五子棋服务已启动，监听端口: 50052
已注册 gomoku.CommonService
已注册 common.CommonService (gatesvr兼容)
[通知管理器] 启动成功，工作协程数: 3
```

---

## 📞 技术支持

如有问题，请参考：
- [实现总结文档](./实现总结.md)
- [消息同步功能说明](./消息同步功能说明.md)
- [Kafka事件设计说明](./Kafka事件设计说明.md)

或联系开发团队获取支持。 