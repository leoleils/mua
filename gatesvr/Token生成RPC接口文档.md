# Token生成RPC接口文档

## 概述

该RPC接口提供了一个安全的方式供外部系统生成鉴权Token，用于客户端连接网关服务器时的身份验证。

## 接口定义

### 服务名
`GateSvr`

### 方法名
`GenerateAuthToken`

### 请求消息 (GenerateAuthTokenRequest)

| 字段名 | 类型 | 是否必填 | 说明 |
|--------|------|----------|------|
| player_id | string | 是 | 玩家唯一标识符 |
| username | string | 否 | 用户名 |
| level | int32 | 否 | 玩家等级 |
| is_vip | bool | 否 | 是否为VIP用户 |
| permissions | repeated string | 否 | 权限列表，如["login", "game", "chat"] |
| platform | string | 否 | 客户端平台：android/ios/web等 |
| device_id | string | 否 | 设备唯一标识符 |
| expire_hours | int64 | 否 | Token过期时间（小时），默认使用配置值 |

### 响应消息 (GenerateAuthTokenResponse)

| 字段名 | 类型 | 说明 |
|--------|------|------|
| success | bool | Token生成是否成功 |
| token | string | 生成的JWT Token |
| expire_time | int64 | Token过期时间戳（毫秒） |
| error_message | string | 错误信息（失败时） |
| error_code | int32 | 错误码（失败时） |

## 错误码说明

| 错误码 | 说明 |
|--------|------|
| 1001 | 玩家ID不能为空 |
| 1002 | Token生成失败 |
| 1003 | Token验证失败 |

## 使用流程

1. **用户登录验证**：外部系统（如用户服务）验证用户身份
2. **调用Token生成**：验证成功后，调用此RPC接口生成Token
3. **返回Token给客户端**：将生成的Token返回给客户端
4. **客户端连接网关**：客户端使用Token连接到网关服务器

## 示例代码

### Go客户端示例

```go
package main

import (
    "context"
    "fmt"
    "log"
    "time"

    "mua/gatesvr/internal/pb"
    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials/insecure"
)

func main() {
    // 连接到gRPC服务器
    conn, err := grpc.Dial("localhost:50051", grpc.WithTransportCredentials(insecure.NewCredentials()))
    if err != nil {
        log.Fatalf("连接失败: %v", err)
    }
    defer conn.Close()

    // 创建客户端
    client := pb.NewGateSvrClient(conn)

    // 创建Token生成请求
    req := &pb.GenerateAuthTokenRequest{
        PlayerId:     "player_12345",
        Username:     "TestUser",
        Level:        10,
        IsVip:        true,
        Permissions:  []string{"login", "game", "chat"},
        Platform:     "android",
        DeviceId:     "device_abcdef",
        ExpireHours:  24, // 24小时过期
    }

    // 调用RPC接口
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    resp, err := client.GenerateAuthToken(ctx, req)
    if err != nil {
        log.Fatalf("RPC调用失败: %v", err)
    }

    // 处理结果
    if resp.Success {
        fmt.Printf("Token生成成功!\n")
        fmt.Printf("Token: %s\n", resp.Token)
        fmt.Printf("过期时间: %s\n", time.UnixMilli(resp.ExpireTime).Format("2006-01-02 15:04:05"))
    } else {
        fmt.Printf("Token生成失败!\n")
        fmt.Printf("错误码: %d\n", resp.ErrorCode)
        fmt.Printf("错误信息: %s\n", resp.ErrorMessage)
    }
}
```

### Python客户端示例

```python
import grpc
import time
from datetime import datetime

# 导入生成的protobuf模块
import gatesvr_pb2
import gatesvr_pb2_grpc

def generate_token():
    # 创建gRPC连接
    with grpc.insecure_channel('localhost:50051') as channel:
        stub = gatesvr_pb2_grpc.GateSvrStub(channel)
        
        # 创建请求
        request = gatesvr_pb2.GenerateAuthTokenRequest(
            player_id="player_12345",
            username="TestUser",
            level=10,
            is_vip=True,
            permissions=["login", "game", "chat"],
            platform="ios",
            device_id="device_abcdef",
            expire_hours=24
        )
        
        # 调用RPC
        try:
            response = stub.GenerateAuthToken(request, timeout=5)
            
            if response.success:
                print("Token生成成功!")
                print(f"Token: {response.token}")
                expire_dt = datetime.fromtimestamp(response.expire_time / 1000)
                print(f"过期时间: {expire_dt.strftime('%Y-%m-%d %H:%M:%S')}")
            else:
                print("Token生成失败!")
                print(f"错误码: {response.error_code}")
                print(f"错误信息: {response.error_message}")
                
        except grpc.RpcError as e:
            print(f"RPC调用失败: {e}")

if __name__ == "__main__":
    generate_token()
```

## 配置说明

Token生成功能依赖以下配置项：

### config.yaml 配置
```yaml
auth:
  enabled: true
  secret_key: "your-secret-key-here"  # JWT签名密钥
  token_expire_hours: 24              # 默认Token过期时间（小时）
  enable_token_cache: true            # 是否启用Token缓存
  redis:
    enabled: false                    # 是否启用Redis缓存
    addr: "localhost:6379"
    password: ""
    db: 0
    key_prefix: "gatesvr:token:"
```

## 安全考虑

1. **密钥安全**：JWT签名密钥必须保密，定期轮换
2. **网络安全**：生产环境建议使用TLS加密gRPC通信
3. **权限控制**：只有可信的外部系统可以调用此接口
4. **过期时间**：合理设置Token过期时间，避免过长的有效期
5. **日志记录**：记录Token生成的关键操作用于审计

## 性能优化

1. **缓存机制**：系统支持内存+Redis双重缓存，提高Token验证性能
2. **连接池**：客户端应使用gRPC连接池避免频繁建立连接
3. **批量处理**：如需批量生成Token，建议并行调用

## 监控指标

系统会记录以下关键指标：
- Token生成成功/失败次数
- Token生成耗时
- 各错误类型统计
- 缓存命中率

## 故障排查

### 常见问题

1. **连接超时**
   - 检查网关服务是否正常运行
   - 确认网络连通性
   - 验证gRPC端口(50051)是否开放

2. **Token生成失败**
   - 检查player_id是否为空
   - 确认auth配置是否正确
   - 查看服务端日志获取详细错误信息

3. **Token无法使用**
   - 检查Token是否已过期
   - 确认客户端连接时使用的Token格式正确
   - 验证网关服务的密钥配置

## 版本历史

- v1.0: 初始版本，支持基本Token生成功能
- 当前版本支持的功能：
  - JWT Token生成
  - 自定义过期时间
  - VIP状态处理
  - 权限列表支持
  - 平台和设备ID记录 