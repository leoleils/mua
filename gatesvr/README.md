# GateServer (网关服务器)

GateServer 是一个高性能的游戏网关服务器，提供 TCP 和 WebSocket 连接支持，负责客户端连接管理、消息转发、认证管理等功能。

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

## 🚨 常见问题

### 连接问题
1. **端口被占用**: 检查端口6001/6002是否被占用
2. **连接超时**: 检查防火墙和网络配置
3. **认证失败**: 检查Token配置和有效性

### 性能问题
1. **连接数过多**: 调整系统ulimit限制
2. **内存占用高**: 检查连接泄漏和垃圾回收
3. **CPU使用率高**: 分析热点代码和优化算法

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
