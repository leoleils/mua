# GateServer RPC API 文档

## 服务概述
GateServer 是游戏网关服务器，提供连接管理、消息处理、认证管理功能。

## 服务地址
- gRPC端口: 50051
- TCP接入端口: 6001  
- WebSocket接入端口: 6002
- 监控端口: 8082

## RPC接口列表

### 1. KickPlayer - 踢下线
- 功能：踢指定玩家下线
- 请求：KickPlayerRequest
- 响应：KickPlayerResponse

### 2. PushToClient - 推送消息  
- 功能：向指定玩家推送消息
- 请求：PushRequest
- 响应：PushResponse

### 3. ForwardMessage - 消息转发
- 功能：转发消息到其他网关节点
- 请求：ForwardMessageRequest  
- 响应：ForwardMessageResponse

### 4. GenerateAuthToken - 生成Token
- 功能：生成JWT认证Token
- 请求：GenerateAuthTokenRequest
- 响应：GenerateAuthTokenResponse

## 错误码
- 0: 成功
- 1001: 玩家ID无效
- 1002: Token生成失败
- 2001: 玩家不在线
- 3001: 推送失败
- 3002: 转发失败

版本: v1.0.0

