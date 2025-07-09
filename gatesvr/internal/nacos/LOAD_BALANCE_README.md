# Nacos 负载均衡功能

## 概述

为 nacos 模块新增了完整的负载均衡功能，支持通过服务名获取 RPC 地址，并提供两种负载均衡策略：
- **轮询（Round Robin）**：按顺序依次选择服务实例
- **权重（Weighted）**：根据实例权重进行加权轮询选择

## 功能特性

### ✅ 负载均衡策略
- **轮询策略**：公平分配请求到各个实例
- **权重策略**：根据实例权重分配更多请求到高权重实例

### ✅ 健康检查
- 自动过滤不健康或未启用的实例
- 只向健康实例分发请求

### ✅ 智能缓存
- 负载均衡器自动缓存，避免重复创建
- 支持缓存清理，适应服务实例变更

### ✅ 容错处理
- 完善的错误处理和日志记录
- 服务实例不可用时的优雅降级

## API 接口

### 1. 便捷函数（推荐使用）

```go
// 轮询方式获取 RPC 地址
addr, err := nacos.GetRPCAddrRoundRobin("userservice")

// 权重方式获取 RPC 地址  
addr, err := nacos.GetRPCAddrWeighted("gameservice")

// 通用接口，支持动态选择策略
addr, err := nacos.GetRPCAddr("payservice", nacos.RoundRobin)
```

### 2. 支持分组的接口

```go
// 通过服务名+分组获取 RPC 地址（轮询）
addr, err := nacos.GetRPCAddrRoundRobinWithGroup("userservice", "prod")

// 通过服务名+分组获取 RPC 地址（权重）
addr, err := nacos.GetRPCAddrWeightedWithGroup("gameservice", "dev")

// 通用接口，支持分组和策略选择
addr, err := nacos.GetRPCAddrWithGroup("payservice", "test", nacos.Weighted)

// 分组为空时自动使用 DEFAULT_GROUP
addr, err := nacos.GetRPCAddrWithGroup("chatservice", "", nacos.RoundRobin)
```

### 3. 通过实例ID获取地址

```go
// 在指定服务和分组中查找实例ID
addr, err := nacos.GetRPCAddrByInstanceID("userservice", "prod", "user-001")

// 在默认分组中查找实例ID
addr, err := nacos.GetRPCAddrByInstanceIDDefault("gameservice", "game-002")

// 全局查找实例ID（会搜索多个已知服务）
addr, err := nacos.GetRPCAddrByInstanceIDGlobal("chat-003")
```

### 4. 自定义负载均衡器

```go
// 创建自定义负载均衡器
lb := nacos.NewLoadBalancer("myservice", nacos.Weighted)

// 创建支持分组的负载均衡器
lb := nacos.NewLoadBalancerWithGroup("myservice", "prod", nacos.RoundRobin)

// 获取地址
addr, err := lb.GetRPCAddr()
```

### 5. 扩展的实例信息接口

```go
// 获取指定服务和分组的所有实例地址
addrs := nacos.GetServiceAddrsWithGroup("userservice", "prod")

// 获取指定服务和分组的所有实例详细信息
instances, err := nacos.GetServiceInstancesWithGroup("gameservice", "dev")
```

### 6. 缓存管理

```go
// 清理指定服务的负载均衡器缓存
nacos.ClearLoadBalancerCache("userservice")

// 清理指定服务和分组的负载均衡器缓存
nacos.ClearLoadBalancerCacheWithGroup("userservice", "prod")
```

## 使用示例

### 基本用法

```go
// 在 RPC 调用前获取服务地址
serviceName := "userservice"
addr, err := nacos.GetRPCAddrRoundRobin(serviceName)
if err != nil {
    log.Printf("获取服务地址失败: %v", err)
    return
}

// 使用地址进行 gRPC 调用
conn, err := grpc.Dial(addr, grpc.WithInsecure())
// ...
```

### 在业务代码中使用

```go
func callUserService(userID int64) error {
    // 使用负载均衡获取服务地址
    addr, err := nacos.GetRPCAddrWeighted("userservice")
    if err != nil {
        return fmt.Errorf("获取用户服务地址失败: %v", err)
    }
    
    // 建立连接并调用
    return rpc.CallUserService(addr, userID)
}
```

### 批量服务调用

```go
services := []string{"userservice", "gameservice", "payservice"}

for _, serviceName := range services {
    addr, err := nacos.GetRPCAddrRoundRobin(serviceName)
    if err != nil {
        log.Printf("服务 %s 不可用: %v", serviceName, err)
        continue
    }
    
    // 调用服务
    go callService(serviceName, addr)
}
```

### 多环境分组调用

```go
// 根据环境选择不同分组
environment := "prod" // 或 "dev", "test"

// 调用生产环境的用户服务
addr, err := nacos.GetRPCAddrWeightedWithGroup("userservice", environment)
if err != nil {
    log.Printf("生产环境用户服务不可用: %v", err)
    return
}

// 建立连接并调用
conn, err := grpc.Dial(addr, grpc.WithInsecure())
// ...
```

### 通过实例ID精确调用

```go
// 场景：需要调用特定实例进行调试或特殊处理
instanceID := "userservice-node-001"

// 精确查找指定实例
addr, err := nacos.GetRPCAddrByInstanceIDGlobal(instanceID)
if err != nil {
    log.Printf("未找到指定实例 %s: %v", instanceID, err)
    return
}

log.Printf("找到目标实例: %s", addr)
// 直接调用该实例
err = callSpecificInstance(addr, data)
```

### 混合使用场景

```go
func callUserServiceSmart(userID int64, preferInstanceID string) error {
    var addr string
    var err error
    
    // 优先尝试指定实例
    if preferInstanceID != "" {
        addr, err = nacos.GetRPCAddrByInstanceIDDefault("userservice", preferInstanceID)
        if err == nil {
            log.Printf("使用指定实例: %s", addr)
            return rpc.CallUserService(addr, userID)
        }
        log.Printf("指定实例不可用，使用负载均衡: %v", err)
    }
    
    // 回退到负载均衡
    addr, err = nacos.GetRPCAddrWeighted("userservice")
    if err != nil {
        return fmt.Errorf("用户服务完全不可用: %v", err)
    }
    
    log.Printf("使用负载均衡选择实例: %s", addr)
    return rpc.CallUserService(addr, userID)
}
```

## 负载均衡算法详解

### 轮询算法（Round Robin）
- **原理**：按顺序依次选择每个健康实例
- **适用场景**：实例性能相近，需要公平分配负载
- **特点**：简单高效，分配均匀

### 权重算法（Weighted Round Robin）
- **原理**：根据实例权重进行加权轮询
- **适用场景**：实例性能差异较大，需要按能力分配负载
- **特点**：高权重实例获得更多请求，但保证所有实例都有机会

**权重算法示例：**
```
实例A (权重:3), 实例B (权重:1), 实例C (权重:2)
选择顺序：A -> A -> C -> A -> B -> C -> A -> A -> C -> B -> ...
```

## 性能优化

### 1. 缓存机制
- 负载均衡器实例自动缓存，避免重复创建
- 减少 nacos 查询频率，提升性能

### 2. 健康检查
- 只获取健康且启用的实例
- 避免向不可用实例发送请求

### 3. 容错处理
- 完善的错误处理，避免程序崩溃
- 详细的日志记录，便于问题排查

## 注意事项

### 1. 缓存更新
当服务实例发生变更时，建议清理缓存：
```go
nacos.ClearLoadBalancerCache("serviceName")
```

### 2. 并发安全
所有接口都是并发安全的，可以在多个 goroutine 中同时使用。

### 3. 错误处理
务必检查返回的错误，并做适当的降级处理：
```go
addr, err := nacos.GetRPCAddrRoundRobin("service")
if err != nil {
    // 降级逻辑，如使用备用服务或返回错误
    return handleServiceUnavailable(err)
}
```

## 迁移指南

### 从现有代码迁移

**原来的代码：**
```go
addrs := nacos.GetServiceAddrs("userservice")
if len(addrs) > 0 {
    addr := addrs[0] // 总是选择第一个
}
```

**新的代码：**
```go
addr, err := nacos.GetRPCAddrRoundRobin("userservice")
if err != nil {
    log.Printf("获取服务地址失败: %v", err)
    return
}
```

## 监控和日志

所有负载均衡操作都会记录详细日志：
```
[负载均衡-轮询] 选择服务 userservice 实例: 192.168.1.100:8080
[负载均衡-权重] 选择服务 gameservice 实例: 192.168.1.101:8081
```

可以通过日志监控负载均衡的工作情况和实例选择分布。 

## 最佳实践

### 1. 权重配置建议

- **高性能机器**: 权重设置为 2.0-3.0
- **标准机器**: 权重设置为 1.0  
- **低性能机器**: 权重设置为 0.5-0.8
- **新上线实例**: 初始权重 0.5，稳定后调整

### 2. 分组使用建议

- **环境隔离**: 使用不同分组区分 `prod`、`dev`、`test` 环境
- **版本管理**: 灰度发布时使用 `v1`、`v2` 等分组
- **地域分离**: 多机房部署时使用 `beijing`、`shanghai` 等分组
- **默认分组**: 简单场景下使用默认的 `DEFAULT_GROUP`

### 3. 实例ID使用建议

- **调试场景**: 开发时通过实例ID调用特定实例进行问题排查
- **灰度发布**: 将少量请求路由到特定的新版本实例
- **故障隔离**: 临时绕过有问题的实例
- **监控分析**: 对特定实例进行性能监控和分析

### 4. 策略选择指南

#### 轮询策略适合：
- 各实例性能相近
- 简单的负载分担需求
- 对响应时间要求不高

#### 权重策略适合：
- 实例性能差异较大
- 需要精确的流量控制
- 对响应时间敏感的业务

### 5. 错误处理建议

```go
func robustServiceCall(serviceName string) error {
    // 1. 首先尝试权重策略
    addr, err := nacos.GetRPCAddrWeighted(serviceName)
    if err != nil {
        log.Printf("权重策略失败，尝试轮询: %v", err)
        
        // 2. 回退到轮询策略
        addr, err = nacos.GetRPCAddrRoundRobin(serviceName)
        if err != nil {
            return fmt.Errorf("所有负载均衡策略都失败: %v", err)
        }
    }
    
    // 3. 调用服务，失败时清理缓存
    if err := callService(addr); err != nil {
        nacos.ClearLoadBalancerCache(serviceName)
        return err
    }
    
    return nil
}
```

### 6. 缓存管理建议

- **定期清理**: 在服务变更频繁时定期清理缓存
- **错误清理**: 服务调用失败时主动清理缓存
- **分组清理**: 按分组清理，避免影响其他环境
- **监控缓存**: 监控缓存命中率和实例变更情况 