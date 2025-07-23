# GateServer (网关服务器)

GateServer 是一个高性能的游戏网关服务器，提供 TCP 和 WebSocket 连接支持，负责客户端连接管理、消息转发、认证管理等功能。

## 🎯 核心功能

- **双协议支持**: TCP (6001) + WebSocket (6002) 客户端连接
- **消息转发**: 支持同步/异步转发到后端微服务
- **消息推送**: 服务端主动推送到客户端 (SYNC/ASYNC/PUSH)
- **认证管理**: JWT Token 认证和权限管理 (Redis缓存优化)
- **连接管理**: 心跳保活、异地登录检测、连接监控
- **负载均衡**: 支持轮询和权重负载均衡
- **限流保护**: 全局、服务、玩家三级限流
- **服务发现**: 基于 Nacos 的服务注册发现
- **事件驱动**: Kafka 事件推送和状态同步
- **监控告警**: 健康检查和性能监控

## 🚀 快速开始

### 启动服务
```bash
# 开发环境
./gatesvr

# 指定配置文件
./gatesvr -config config-local.yaml
```

### 连接测试
```bash
# TCP连接测试
telnet 127.0.0.1 6001

# 客户端测试
cd ../client
python main.py
```

## 📋 功能特性

- ✅ **双协议支持**: TCP (6001) + WebSocket (6002)
- ✅ **消息转发**: 支持同步/异步转发到后端服务
- ✅ **消息推送**: 支持服务端主动推送到客户端 (SYNC/ASYNC/PUSH)
- ✅ **认证管理**: JWT Token 认证和权限管理
- ✅ **连接管理**: 心跳保活、异地登录检测、连接监控
- ✅ **负载均衡**: 支持轮询和权重负载均衡
- ✅ **限流保护**: 全局、服务、玩家三级限流
- ✅ **服务发现**: 基于 Nacos 的服务注册发现
- ✅ **消息队列**: Kafka 事件推送和状态同步
- ✅ **监控告警**: 健康检查和性能监控

## 📖 文档索引

### 📚 接口文档
- **[TCP/WebSocket 接口使用说明](TCP_WebSocket_接口使用说明.md)** - 详细的接口文档和使用示例
- **[TCP/WebSocket 快速参考](TCP_WebSocket_快速参考.md)** - 关键信息快速查询
- **[GateServer API 文档](GateServer_API_Documentation.md)** - gRPC API 接口说明
- **[RPC 接口总结](RPC_Interface_Summary.md)** - RPC 接口快速总结

### 🔧 配置和部署
- **[Token生成RPC接口文档](Token生成RPC接口文档.md)** - JWT Token 生成和管理
- **[config.yaml](config.yaml)** - 生产环境配置文件
- **[config-local.yaml](config-local.yaml)** - 开发环境配置文件

### 📋 内部文档
- **[认证功能说明](内部文档-认证功能说明.md)** - 认证机制详细说明
- **[连接处理优化说明](内部文档-连接处理优化说明.md)** - 连接处理优化方案
- **[限流功能说明](内部文档-限流功能说明.md)** - 限流机制实现细节

## 🏗️ 架构概览

```
客户端 (TCP/WS) 
    ↓
GateServer (网关)
    ↓
Backend Services (后端服务)
    ↓
Database/Cache (数据存储)
```

### 核心组件

| 组件 | 说明 | 文件位置 |
|------|------|----------|
| **连接管理** | TCP/WebSocket连接处理 | `internal/conn/` |
| **消息转发** | 后端服务消息转发 | `internal/forwarder/` |
| **消息推送** | 服务端主动推送到客户端 | `internal/app/app.go` (PushToClient) |
| **认证管理** | JWT Token认证 | `internal/auth/` |
| **路由管理** | 玩家路由和会话管理 | `internal/route/`, `internal/session/` |
| **服务发现** | Nacos服务注册发现 | `internal/nacos/` |
| **消息队列** | Kafka事件处理 | `internal/kafka/` |

## 🔌 端口配置

| 端口 | 协议 | 用途 |
|------|------|------|
| 6001 | TCP | 客户端TCP连接 |
| 6002 | WebSocket | 客户端WS连接 |
| 50051 | gRPC | 服务间RPC调用 |
| 8082 | HTTP | 健康检查和监控 |

## 🛠️ 开发指南

### 环境要求
- Go 1.19+
- Protocol Buffers 3.0+
- Nacos 服务注册中心
- Redis (可选，用于Token缓存)
- Kafka (可选，用于事件推送)

### 编译和运行
```bash
# 编译
go build -o gatesvr cmd/main.go

# 运行
./gatesvr

# 指定配置
./gatesvr -config config-local.yaml
```

### protobuf生成
```bash
# 生成Go代码
protoc --go_out=. --go-grpc_out=. proto/*.proto

# 生成Python代码（客户端）
protoc --python_out=../client proto/*.proto
```

## 🧪 测试和调试

### 客户端测试
```bash
cd ../client

# GUI测试客户端
python main.py

# 简单连接测试
python simple_connect_test.py

# 心跳测试
python test_heartbeat.py
```

### 监控检查
```bash
# 健康检查
curl http://127.0.0.1:8082/health

# 服务状态
curl http://127.0.0.1:8082/status
```

### 日志调试
```bash
# 查看实时日志
tail -f gatesvr.log

# 启用结构化日志
# 在config.yaml中设置: connection.enable_structured_log: true
```

## 📊 性能和监控

### 性能指标
- **并发连接数**: 支持千级并发连接
- **消息吞吐量**: 万级QPS消息处理
- **延迟**: 毫秒级消息转发延迟
- **可用性**: 99.9%+ 服务可用性

### 监控指标
- 连接数统计
- 消息处理QPS  
- 服务响应时间
- 错误率统计
- 资源使用情况

## 🔧 配置说明

### 核心配置项
```yaml
# 连接配置
connection:
  first_message_timeout_sec: 5
  read_timeout_sec: 60
  heartbeat_interval_sec: 30

# 认证配置  
auth:
  enabled: false
  secret_key: "your-secret-key"
  token_expire_hours: 24

# 限流配置
rate_limit:
  enabled: true
  global_rate: 500
  player_rate: 10
```

## 🚀 部署指南

### Docker部署

```bash
# 构建镜像
docker build -t gatesvr:latest .

# 运行容器
docker run -d \
  --name gatesvr \
  -p 6001:6001 \
  -p 6002:6002 \
  -p 50051:50051 \
  -p 8082:8082 \
  -v ./config.yaml:/app/config.yaml \
  gatesvr:latest

# 环境变量配置
docker run -d \
  --name gatesvr \
  -e GATESVR_CONFIG=config-local.yaml \
  -v ./config-local.yaml:/app/config-local.yaml \
  gatesvr:latest
```

### Kubernetes部署

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: gatesvr
spec:
  replicas: 3
  selector:
    matchLabels:
      app: gatesvr
  template:
    metadata:
      labels:
        app: gatesvr
    spec:
      containers:
      - name: gatesvr
        image: gatesvr:latest
        ports:
        - containerPort: 6001
        - containerPort: 6002
        - containerPort: 50051
        - containerPort: 8082
        env:
        - name: GATESVR_CONFIG
          value: "config.yaml"
        livenessProbe:
          httpGet:
            path: /health
            port: 8082
          initialDelaySeconds: 30
          periodSeconds: 30
---
apiVersion: v1
kind: Service
metadata:
  name: gatesvr-service
spec:
  selector:
    app: gatesvr
  ports:
  - name: tcp
    port: 6001
    targetPort: 6001
  - name: websocket
    port: 6002
    targetPort: 6002
  - name: grpc
    port: 50051
    targetPort: 50051
  - name: monitor
    port: 8082
    targetPort: 8082
  type: ClusterIP
```

### 环境配置

#### 开发环境 (config-local.yaml)
```yaml
# 本地开发配置，禁用认证和限流
nacos:
  addr: localhost
  port: 8848
auth:
  enabled: false
rate_limit:
  enabled: false
```

#### 生产环境 (config.yaml)
```yaml
# 生产环境配置，启用所有功能
nacos:
  addr: mse-3cb9ef30-p.nacos-ans.mse.aliyuncs.com
kafka:
  brokers:
    - alikafka-serverless-cn-0mm42ujh802-1000.alikafka.aliyuncs.com:9093
auth:
  enabled: true
  secret_key: "production-secret-key"
rate_limit:
  enabled: true
```

## ⚙️ 配置详解

### 核心配置项

#### 连接配置
```yaml
connection:
  first_message_timeout_sec: 5     # 首条消息超时(秒)
  read_timeout_sec: 60             # 读取超时(秒)
  write_timeout_sec: 10            # 写入超时(秒)
  enable_structured_log: true      # 启用结构化日志
  max_message_size: 4194304        # 最大消息大小(4MB)
  heartbeat_interval_sec: 30       # 心跳间隔(秒)
  max_connections: 100             # 最大连接数
  max_idle_time_sec: 300           # 最大空闲时间(秒)
  cleanup_interval_sec: 60         # 清理间隔(秒)
```

#### 认证配置
```yaml
auth:
  enabled: true                           # 是否启用认证
  secret_key: "your-jwt-secret-key"      # JWT密钥
  token_expire_hours: 24                 # Token过期时间(小时)
  cleanup_interval_min: 10               # 清理间隔(分钟)
  require_auth:                          # 需要认证的消息类型
    - "SERVICE_MESSAGE"
  whitelist_services:                    # 免认证服务
    - "health"
    - "version"
  max_tokens_per_player: 3               # 单玩家最大Token数
  enable_auto_refresh: true              # 自动刷新
  refresh_threshold_min: 60              # 刷新阈值(分钟)
  
  # Redis缓存配置
  enable_redis_cache: true               # 启用Redis缓存
  redis_addr: "localhost:6379"           # Redis地址
  redis_password: ""                     # Redis密码
  redis_db: 0                           # 数据库索引
  redis_key_prefix: "mua:token:"         # 键前缀
```

#### 限流配置
```yaml
rate_limit:
  enabled: true                    # 启用限流
  global_rate: 500                 # 全局每秒请求数
  global_capacity: 1000            # 全局令牌桶容量
  service_rate: 100                # 单服务每秒请求数
  service_capacity: 200            # 单服务令牌桶容量
  player_rate: 10                  # 单玩家每秒请求数
  player_capacity: 20              # 单玩家令牌桶容量
```

## 📡 接口文档

### gRPC API接口

#### 1. KickPlayer - 踢下线
```protobuf
rpc KickPlayer(KickPlayerRequest) returns (KickPlayerResponse);

message KickPlayerRequest {
  string player_id = 1;  // 玩家ID
  string reason = 2;     // 踢下线原因
}

message KickPlayerResponse {
  int32 ret = 1;         // 返回码
  string reason = 2;     // 返回信息
}
```

#### 2. PushToClient - 推送消息
```protobuf
rpc PushToClient(PushRequest) returns (PushResponse);

message PushRequest {
  string player_id = 1;           // 目标玩家ID
  string ip = 2;                  // 目标IP（可选）
  CallbackType cb_type = 3;       // 回调类型
  common.GameMessage message = 4; // 要推送的消息
}

message PushResponse {
  int32 ret = 1;         // 返回码
  string reason = 2;     // 返回信息
}
```

#### 3. ForwardMessage - 消息转发
```protobuf
rpc ForwardMessage(ForwardMessageRequest) returns (ForwardMessageResponse);

message ForwardMessageRequest {
  common.GameMessage message = 1;  // 要转发的消息
  string target_gatesvr_id = 2;    // 目标网关ID
}

message ForwardMessageResponse {
  int32 ret = 1;         // 返回码
  string reason = 2;     // 返回信息
}
```

#### 4. GenerateAuthToken - 生成Token
```protobuf
rpc GenerateAuthToken(GenerateAuthTokenRequest) returns (GenerateAuthTokenResponse);

message GenerateAuthTokenRequest {
  string player_id = 1;           // 玩家ID
  string username = 2;            // 用户名
  int32 level = 3;                // 等级
  bool is_vip = 4;                // VIP状态
  repeated string permissions = 5; // 权限列表
  string platform = 6;            // 平台类型
  string device_id = 7;           // 设备ID
}

message GenerateAuthTokenResponse {
  int32 ret = 1;         // 返回码
  string reason = 2;     // 返回信息
  string token = 3;      // 生成的Token
}
```

### HTTP监控接口

#### 健康检查
```bash
GET /health
# 响应: {"status": "ok", "timestamp": "2024-01-01T00:00:00Z"}
```

#### 服务状态
```bash
GET /status
# 响应: 包含连接数、内存使用、CPU使用等详细信息
```

#### 认证缓存统计
```bash
GET /auth/cache/stats
# 响应: 内存缓存和Redis缓存的命中率统计
```

### TCP/WebSocket客户端接口

#### 连接建立
```python
# TCP连接
socket.connect(('127.0.0.1', 6001))

# WebSocket连接
ws = new WebSocket('ws://127.0.0.1:6002')
```

#### 心跳消息
```protobuf
message GameMessage {
  msg_type = HEARTBEAT  # 心跳类型
  msg_head.player_id = "player123"  # 玩家ID
}
```

#### 业务消息
```protobuf
message GameMessage {
  msg_type = SERVICE_MESSAGE  # 服务消息
  msg_head.service_name = "gomokusvr"  # 目标服务
  msg_head.service_msg_type = SYNC  # 同步/异步
  payload = "业务数据"  # 业务负载
}
```

## 📊 监控和运维

### 性能监控
- **连接监控**: 实时连接数、连接建立/断开速率
- **消息监控**: 消息处理QPS、平均响应时间
- **缓存监控**: Token缓存命中率、Redis连接状态
- **资源监控**: CPU、内存、网络使用情况

### 日志管理
```bash
# 结构化日志
{"level":"info","time":"2024-01-01T00:00:00Z","msg":"连接建立","player_id":"player123"}

# 错误日志
{"level":"error","time":"2024-01-01T00:00:00Z","msg":"认证失败","player_id":"player123","error":"token_expired"}
```

### 告警配置
- 连接数异常告警
- 错误率过高告警
- 响应时间过长告警
- Redis连接失败告警

## 🚨 常见问题

### 连接问题
1. **端口被占用**: 检查端口6001/6002是否被占用
2. **连接超时**: 检查防火墙和网络配置
3. **认证失败**: 检查Token配置和有效性

### 性能问题
1. **连接数过多**: 调整系统ulimit限制
2. **内存占用高**: 检查连接泄漏和垃圾回收
3. **CPU使用率高**: 分析热点代码和优化算法

### 配置问题
1. **Nacos连接失败**: 检查Nacos地址和网络
2. **Redis缓存问题**: 检查Redis连接和配置
3. **Kafka事件处理**: 检查Kafka配置和权限

### 更多问题
参见各个详细文档的故障排查章节。

## 📝 开发计划

- [ ] 支持HTTP/2连接
- [ ] 增加连接池管理
- [ ] 完善监控告警
- [ ] 性能优化和压力测试
- [ ] 集群部署方案

## 📞 联系方式

- **开发团队**: MUA Game Team
- **技术支持**: 参见项目文档
- **Bug反馈**: 提交Issue

---

**版本**: v1.0.0  
**更新**: 2024年12月  
**许可**: 内部项目
