# Kafka事件设计说明

## 概述

五子棋服务(`gomokusvr`)集成了Kafka消息队列，用于实现分布式事件驱动架构，支持游戏事件发布和玩家状态监听。

## 配置架构

### 服务间Kafka配置对比

| 配置项 | gatesvr | gomokusvr |
|--------|---------|-----------|
| **主要用途** | 玩家连接状态管理 | 游戏事件发布 + 状态监听 |
| **生产主题** | `player_status_changed` | `game_events`, `room_events` |
| **消费主题** | 无 | `player_status_changed` |
| **消费者组** | `gatesvr-group-0` | `gomokusvr-group-0` |

## 事件类型设计

### 1. 游戏事件 (`game_events`)

**发布者**: gomokusvr  
**消费者**: 其他游戏服务、数据分析服务、排行榜服务

#### 事件类型

##### 1.1 游戏开始事件
```json
{
  "event_type": "GAME_STARTED",
  "event_id": "uuid",
  "timestamp": 1699123456789,
  "room_id": "room_001",
  "players": [
    {
      "player_id": "player1",
      "color": "BLACK",
      "name": "玩家1"
    },
    {
      "player_id": "player2", 
      "color": "WHITE",
      "name": "玩家2"
    }
  ],
  "game_config": {
    "board_size": 15,
    "win_condition": 5,
    "time_limit_sec": 1800
  }
}
```

##### 1.2 下棋事件
```json
{
  "event_type": "PIECE_PLACED",
  "event_id": "uuid",
  "timestamp": 1699123456789,
  "room_id": "room_001",
  "player_id": "player1",
  "move": {
    "x": 7,
    "y": 8,
    "color": "BLACK",
    "move_number": 15
  },
  "game_state": {
    "current_turn": "WHITE",
    "total_moves": 15,
    "status": "ONGOING"
  }
}
```

##### 1.3 游戏结束事件
```json
{
  "event_type": "GAME_ENDED",
  "event_id": "uuid", 
  "timestamp": 1699123456789,
  "room_id": "room_001",
  "result": {
    "status": "WIN",
    "winner_id": "player1",
    "winner_color": "BLACK",
    "total_moves": 89,
    "duration_sec": 1245,
    "end_reason": "FIVE_IN_ROW"
  },
  "final_board": "base64_encoded_board_state"
}
```

## 部署注意事项

### 1. Topic创建
```bash
# 创建game_events主题
kafka-topics.sh --create --topic game_events --partitions 3 --replication-factor 2

# 创建room_events主题  
kafka-topics.sh --create --topic room_events --partitions 2 --replication-factor 2
```

### 2. 证书文件
- kafka证书文件已复制到gomokusvr目录: `only-4096-ca-cert`
- 配置中使用相对路径: `./only-4096-ca-cert`

这个设计为五子棋服务提供了完整的事件驱动架构支持。
