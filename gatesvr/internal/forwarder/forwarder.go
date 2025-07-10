package forwarder

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"mua/gatesvr/config"
	"mua/gatesvr/internal/nacos"
	"mua/gatesvr/internal/pb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
)

// 连接信息
type connInfo struct {
	conn     *grpc.ClientConn
	client   pb.CommonServiceClient
	lastUsed time.Time
}

// 令牌桶
type tokenBucket struct {
	tokens   int64     // 当前令牌数
	capacity int64     // 桶容量
	rate     int64     // 令牌生成速率（每秒）
	lastTime time.Time // 上次更新时间
	mu       sync.Mutex
}

// 创建令牌桶
func newTokenBucket(capacity, rate int64) *tokenBucket {
	return &tokenBucket{
		tokens:   capacity,
		capacity: capacity,
		rate:     rate,
		lastTime: time.Now(),
	}
}

// 尝试获取令牌
func (tb *tokenBucket) tryAcquire(count int64) bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	now := time.Now()
	// 计算需要添加的令牌数
	elapsed := now.Sub(tb.lastTime).Seconds()
	tokensToAdd := int64(elapsed * float64(tb.rate))

	// 更新令牌数，不超过容量
	tb.tokens += tokensToAdd
	if tb.tokens > tb.capacity {
		tb.tokens = tb.capacity
	}
	tb.lastTime = now

	// 尝试消费令牌
	if tb.tokens >= count {
		tb.tokens -= count
		return true
	}
	return false
}

// 获取当前令牌数
func (tb *tokenBucket) getTokens() int64 {
	tb.mu.Lock()
	defer tb.mu.Unlock()
	return tb.tokens
}

// 限流器
type rateLimiter struct {
	globalBucket  *tokenBucket            // 全局限流
	serviceBucket map[string]*tokenBucket // 按服务限流
	playerBucket  map[string]*tokenBucket // 按玩家限流
	mu            sync.RWMutex            // 保护 map

	// 配置
	globalRate      int64 // 全局速率（每秒）
	globalCapacity  int64 // 全局容量
	serviceRate     int64 // 服务速率（每秒）
	serviceCapacity int64 // 服务容量
	playerRate      int64 // 玩家速率（每秒）
	playerCapacity  int64 // 玩家容量

	// 清理相关
	cleanupInterval time.Duration
	bucketIdleTime  time.Duration
}

// 创建限流器
func newRateLimiter() *rateLimiter {
	cfg := config.GetRateLimitConfig()

	rl := &rateLimiter{
		globalBucket:  newTokenBucket(cfg.GlobalCapacity, cfg.GlobalRate),
		serviceBucket: make(map[string]*tokenBucket),
		playerBucket:  make(map[string]*tokenBucket),

		globalRate:      cfg.GlobalRate,
		globalCapacity:  cfg.GlobalCapacity,
		serviceRate:     cfg.ServiceRate,
		serviceCapacity: cfg.ServiceCapacity,
		playerRate:      cfg.PlayerRate,
		playerCapacity:  cfg.PlayerCapacity,

		cleanupInterval: time.Duration(cfg.CleanupIntervalMin) * time.Minute,
		bucketIdleTime:  time.Duration(cfg.BucketIdleTimeMin) * time.Minute,
	}

	// 如果启用限流，启动清理协程
	if cfg.Enabled {
		go rl.startCleanup()
		log.Printf("[限流器] 启动成功 - 全局: %d/%d, 服务: %d/%d, 玩家: %d/%d",
			cfg.GlobalRate, cfg.GlobalCapacity,
			cfg.ServiceRate, cfg.ServiceCapacity,
			cfg.PlayerRate, cfg.PlayerCapacity)
	} else {
		log.Println("[限流器] 已禁用")
	}

	return rl
}

// 启动清理协程
func (rl *rateLimiter) startCleanup() {
	ticker := time.NewTicker(rl.cleanupInterval)
	defer ticker.Stop()

	for range ticker.C {
		rl.cleanupIdleBuckets()
	}
}

// 清理空闲的令牌桶
func (rl *rateLimiter) cleanupIdleBuckets() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()

	// 清理服务令牌桶
	for service, bucket := range rl.serviceBucket {
		if now.Sub(bucket.lastTime) > rl.bucketIdleTime {
			delete(rl.serviceBucket, service)
			log.Printf("[限流清理] 清理服务令牌桶: %s", service)
		}
	}

	// 清理玩家令牌桶
	for player, bucket := range rl.playerBucket {
		if now.Sub(bucket.lastTime) > rl.bucketIdleTime {
			delete(rl.playerBucket, player)
			log.Printf("[限流清理] 清理玩家令牌桶: %s", player)
		}
	}
}

// 获取服务令牌桶
func (rl *rateLimiter) getServiceBucket(service string) *tokenBucket {
	rl.mu.RLock()
	bucket, exists := rl.serviceBucket[service]
	rl.mu.RUnlock()

	if exists {
		return bucket
	}

	// 创建新的服务令牌桶
	rl.mu.Lock()
	defer rl.mu.Unlock()

	// 双重检查
	if bucket, exists := rl.serviceBucket[service]; exists {
		return bucket
	}

	bucket = newTokenBucket(rl.serviceCapacity, rl.serviceRate)
	rl.serviceBucket[service] = bucket
	log.Printf("[限流] 创建服务令牌桶: %s, 容量: %d, 速率: %d/s", service, rl.serviceCapacity, rl.serviceRate)
	return bucket
}

// 获取玩家令牌桶
func (rl *rateLimiter) getPlayerBucket(playerID string) *tokenBucket {
	rl.mu.RLock()
	bucket, exists := rl.playerBucket[playerID]
	rl.mu.RUnlock()

	if exists {
		return bucket
	}

	// 创建新的玩家令牌桶
	rl.mu.Lock()
	defer rl.mu.Unlock()

	// 双重检查
	if bucket, exists := rl.playerBucket[playerID]; exists {
		return bucket
	}

	bucket = newTokenBucket(rl.playerCapacity, rl.playerRate)
	rl.playerBucket[playerID] = bucket
	log.Printf("[限流] 创建玩家令牌桶: %s, 容量: %d, 速率: %d/s", playerID, rl.playerCapacity, rl.playerRate)
	return bucket
}

// 检查是否允许请求
func (rl *rateLimiter) allowRequest(serviceName, playerID string) (bool, string) {
	cfg := config.GetRateLimitConfig()

	// 如果限流未启用，直接允许
	if !cfg.Enabled {
		return true, ""
	}

	// 1. 全局限流检查
	if !rl.globalBucket.tryAcquire(1) {
		log.Printf("[限流拒绝] 全局限流，当前令牌: %d", rl.globalBucket.getTokens())
		return false, "全局请求过于频繁，请稍后重试"
	}

	// 2. 服务限流检查
	serviceBucket := rl.getServiceBucket(serviceName)
	if !serviceBucket.tryAcquire(1) {
		log.Printf("[限流拒绝] 服务限流 %s，当前令牌: %d", serviceName, serviceBucket.getTokens())
		return false, fmt.Sprintf("服务 %s 请求过于频繁，请稍后重试", serviceName)
	}

	// 3. 玩家限流检查
	if playerID != "" {
		playerBucket := rl.getPlayerBucket(playerID)
		if !playerBucket.tryAcquire(1) {
			log.Printf("[限流拒绝] 玩家限流 %s，当前令牌: %d", playerID, playerBucket.getTokens())
			return false, "请求过于频繁，请稍后重试"
		}
	}

	return true, ""
}

// 获取限流状态
func (rl *rateLimiter) getStatus() map[string]interface{} {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	return map[string]interface{}{
		"global_tokens":   rl.globalBucket.getTokens(),
		"global_capacity": rl.globalCapacity,
		"service_buckets": len(rl.serviceBucket),
		"player_buckets":  len(rl.playerBucket),
	}
}

// MessageForwarder 消息转发器
type MessageForwarder struct {
	connections map[string]*connInfo // 连接池
	mu          sync.RWMutex         // 读写锁保护连接池
	rateLimiter *rateLimiter         // 限流器

	// 性能优化配置
	maxIdleTime     time.Duration // 最大空闲时间
	cleanupInterval time.Duration // 清理间隔
	maxConnections  int           // 最大连接数
}

// NewMessageForwarder 创建消息转发器
func NewMessageForwarder() *MessageForwarder {
	mf := &MessageForwarder{
		connections:     make(map[string]*connInfo),
		rateLimiter:     newRateLimiter(),
		maxIdleTime:     5 * time.Minute, // 5分钟空闲超时
		cleanupInterval: 1 * time.Minute, // 1分钟清理一次
		maxConnections:  100,             // 最大100个连接
	}

	// 启动连接池清理协程
	go mf.startCleanup()

	return mf
}

// startCleanup 启动连接池清理协程
func (f *MessageForwarder) startCleanup() {
	ticker := time.NewTicker(f.cleanupInterval)
	defer ticker.Stop()

	for range ticker.C {
		f.cleanupIdleConnections()
	}
}

// cleanupIdleConnections 清理空闲连接
func (f *MessageForwarder) cleanupIdleConnections() {
	f.mu.Lock()
	defer f.mu.Unlock()

	now := time.Now()
	for addr, info := range f.connections {
		if now.Sub(info.lastUsed) > f.maxIdleTime {
			info.conn.Close()
			delete(f.connections, addr)
			log.Printf("[连接池清理] 清理空闲连接: %s", addr)
		}
	}
}

// ForwardMessage 转发服务消息
func (f *MessageForwarder) ForwardMessage(msg *pb.GameMessage) (*pb.GameMessageResponse, error) {
	head := msg.MsgHead
	if head == nil {
		return nil, fmt.Errorf("消息头为空")
	}

	// 检查是否为服务消息
	if msg.MsgType != pb.MessageType_SERVICE_MESSAGE {
		return nil, fmt.Errorf("不是服务消息类型: %v", msg.MsgType)
	}

	serviceName := head.ServiceName
	if serviceName == "" {
		return nil, fmt.Errorf("服务名不能为空")
	}

	// 限流检查
	playerID := head.PlayerId
	if allowed, reason := f.rateLimiter.allowRequest(serviceName, playerID); !allowed {
		log.Printf("[限流] 拒绝请求 - 服务: %s, 玩家: %s, 原因: %s", serviceName, playerID, reason)
		return &pb.GameMessageResponse{
			MsgHead:           head,
			Ret:               429, // HTTP 429 Too Many Requests
			Payload:           &pb.GameMessageResponse_Reason{Reason: reason},
			ResponseTimestamp: time.Now().UnixMilli(),
		}, nil
	}

	// 获取目标服务地址
	targetAddr, err := f.getTargetServiceAddr(head)
	if err != nil {
		return nil, fmt.Errorf("获取目标服务地址失败: %v", err)
	}

	// 根据消息类型选择处理方式
	switch head.ServiceMsgType {
	case pb.ServiceMessageType_SYNC:
		return f.forwardSync(targetAddr, msg)
	case pb.ServiceMessageType_ASYNC:
		return f.forwardAsync(targetAddr, msg)
	default:
		return nil, fmt.Errorf("未知的服务消息类型: %v", head.ServiceMsgType)
	}
}

// getTargetServiceAddr 获取目标服务地址
func (f *MessageForwarder) getTargetServiceAddr(head *pb.HeadMessage) (string, error) {
	serviceName := head.ServiceName
	groupName := head.Group
	instanceID := head.InstanceId

	// 如果指定了实例ID，直接通过实例ID获取地址
	if instanceID != "" {
		log.Printf("[消息转发] 使用指定实例ID: %s", instanceID)

		if groupName == "" {
			groupName = "DEFAULT_GROUP"
		}

		addr, err := nacos.GetRPCAddrByInstanceID(serviceName, groupName, instanceID)
		if err != nil {
			return "", fmt.Errorf("通过实例ID获取地址失败: %v", err)
		}
		return addr, nil
	}

	// 获取服务配置
	serviceConfig := config.GetServiceConfig(serviceName)

	// 使用配置中的分组（如果消息头没有指定）
	if groupName == "" {
		groupName = serviceConfig.Group
	}

	// 根据负载均衡策略选择实例
	strategy := f.getLoadBalanceStrategy(head, serviceConfig)

	var addr string
	var err error

	switch strategy {
	case "round_robin":
		if groupName == "DEFAULT_GROUP" {
			addr, err = nacos.GetRPCAddrRoundRobin(serviceName)
		} else {
			addr, err = nacos.GetRPCAddrRoundRobinWithGroup(serviceName, groupName)
		}
	case "weighted":
		if groupName == "DEFAULT_GROUP" {
			addr, err = nacos.GetRPCAddrWeighted(serviceName)
		} else {
			addr, err = nacos.GetRPCAddrWeightedWithGroup(serviceName, groupName)
		}
	default:
		return "", fmt.Errorf("不支持的负载均衡策略: %s", strategy)
	}

	if err != nil {
		return "", fmt.Errorf("负载均衡获取地址失败: %v", err)
	}

	log.Printf("[消息转发] 服务=%s@%s, 策略=%s, 地址=%s", serviceName, groupName, strategy, addr)
	return addr, nil
}

// getLoadBalanceStrategy 获取负载均衡策略
func (f *MessageForwarder) getLoadBalanceStrategy(head *pb.HeadMessage, serviceConfig config.ServiceConfig) string {
	// 优先使用消息头中指定的策略
	if head.LoadBalanceStrategy != "" {
		return head.LoadBalanceStrategy
	}

	// 使用服务配置中的策略
	return serviceConfig.LoadBalance
}

// forwardSync 同步转发（等待回包）- 优化版本
func (f *MessageForwarder) forwardSync(targetAddr string, msg *pb.GameMessage) (*pb.GameMessageResponse, error) {
	log.Printf("[同步转发] 目标地址: %s, 玩家: %s", targetAddr, msg.MsgHead.PlayerId)

	// 获取连接和客户端
	client, err := f.getClient(targetAddr)
	if err != nil {
		return nil, fmt.Errorf("获取连接失败: %v", err)
	}

	// 设置超时
	serviceConfig := config.GetServiceConfig(msg.MsgHead.ServiceName)
	timeout := time.Duration(serviceConfig.TimeoutMs) * time.Millisecond

	// 添加最大超时限制，防止过长等待
	if timeout > 30*time.Second {
		timeout = 30 * time.Second
		log.Printf("[同步转发] 超时时间限制为30s, 服务: %s", msg.MsgHead.ServiceName)
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// 设置请求时间戳
	msg.MsgHead.Timestamp = time.Now().UnixMilli()

	// 发送请求
	resp, err := client.SendMessage(ctx, msg)
	if err != nil {
		// 如果是连接错误，清理该连接
		if isConnectionError(err) {
			f.removeConnection(targetAddr)
		}
		return nil, fmt.Errorf("RPC调用失败: %v", err)
	}

	// 设置响应时间戳
	resp.ResponseTimestamp = time.Now().UnixMilli()

	elapsed := resp.ResponseTimestamp - msg.MsgHead.Timestamp
	log.Printf("[同步转发成功] 玩家: %s, 耗时: %dms", msg.MsgHead.PlayerId, elapsed)

	// 记录慢查询
	if elapsed > 1000 { // 超过1秒
		log.Printf("[慢查询警告] 服务: %s, 耗时: %dms", msg.MsgHead.ServiceName, elapsed)
	}

	return resp, nil
}

// forwardAsync 异步转发（不等待回包）
func (f *MessageForwarder) forwardAsync(targetAddr string, msg *pb.GameMessage) (*pb.GameMessageResponse, error) {
	log.Printf("[异步转发] 目标地址: %s, 玩家: %s", targetAddr, msg.MsgHead.PlayerId)

	// 异步执行转发
	go func() {
		client, err := f.getClient(targetAddr)
		if err != nil {
			log.Printf("[异步转发失败] 获取连接失败: %v", err)
			return
		}

		// 异步请求使用较短的超时时间
		cfg := config.GetConfig()
		timeout := time.Duration(cfg.ServiceForwarding.AsyncTimeoutMs) * time.Millisecond
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		msg.MsgHead.Timestamp = time.Now().UnixMilli()

		_, err = client.SendMessage(ctx, msg)
		if err != nil {
			log.Printf("[异步转发失败] RPC调用失败: %v", err)
			// 如果是连接错误，清理该连接
			if isConnectionError(err) {
				f.removeConnection(targetAddr)
			}
		} else {
			log.Printf("[异步转发成功] 玩家: %s", msg.MsgHead.PlayerId)
		}
	}()

	// 立即返回成功响应
	return &pb.GameMessageResponse{
		MsgHead:           msg.MsgHead,
		Ret:               0,
		Payload:           &pb.GameMessageResponse_Reason{Reason: "异步处理中"},
		ResponseTimestamp: time.Now().UnixMilli(),
	}, nil
}

// getClient 获取或创建客户端（优化版本）
func (f *MessageForwarder) getClient(targetAddr string) (pb.CommonServiceClient, error) {
	// 先尝试读锁获取现有连接
	f.mu.RLock()
	if info, exists := f.connections[targetAddr]; exists {
		// 检查连接状态
		if info.conn.GetState() == connectivity.Ready || info.conn.GetState() == connectivity.Idle {
			info.lastUsed = time.Now() // 更新使用时间
			f.mu.RUnlock()
			return info.client, nil
		}
	}
	f.mu.RUnlock()

	// 需要创建新连接，使用写锁
	f.mu.Lock()
	defer f.mu.Unlock()

	// 双重检查，可能在等待写锁期间其他协程已经创建了连接
	if info, exists := f.connections[targetAddr]; exists {
		if info.conn.GetState() == connectivity.Ready || info.conn.GetState() == connectivity.Idle {
			info.lastUsed = time.Now()
			return info.client, nil
		}
		// 连接状态不正常，清理旧连接
		info.conn.Close()
		delete(f.connections, targetAddr)
	}

	// 检查连接数限制
	if len(f.connections) >= f.maxConnections {
		return nil, fmt.Errorf("连接池已满，当前连接数: %d", len(f.connections))
	}

	// 创建新连接
	conn, err := grpc.Dial(targetAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(4*1024*1024)), // 4MB
	)
	if err != nil {
		return nil, fmt.Errorf("创建gRPC连接失败: %v", err)
	}

	// 创建客户端并缓存
	client := pb.NewCommonServiceClient(conn)
	f.connections[targetAddr] = &connInfo{
		conn:     conn,
		client:   client,
		lastUsed: time.Now(),
	}

	log.Printf("[连接池] 创建新连接: %s, 当前连接数: %d", targetAddr, len(f.connections))
	return client, nil
}

// removeConnection 移除连接
func (f *MessageForwarder) removeConnection(targetAddr string) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if info, exists := f.connections[targetAddr]; exists {
		info.conn.Close()
		delete(f.connections, targetAddr)
		log.Printf("[连接池] 移除异常连接: %s", targetAddr)
	}
}

// isConnectionError 判断是否为连接错误
func isConnectionError(err error) bool {
	// 简单的错误类型判断，可以根据需要扩展
	errStr := err.Error()
	return len(errStr) > 0 && (
	// 可以添加更多连接错误的判断条件
	false) // 暂时返回false，避免过度清理
}

// Close 关闭转发器，清理所有连接
func (f *MessageForwarder) Close() {
	f.mu.Lock()
	defer f.mu.Unlock()

	for addr, info := range f.connections {
		info.conn.Close()
		log.Printf("[连接池] 关闭连接: %s", addr)
	}
	f.connections = make(map[string]*connInfo)
}

// 全局转发器实例
var globalForwarder *MessageForwarder

// Init 初始化全局转发器
func Init() {
	globalForwarder = NewMessageForwarder()
	log.Println("消息转发器初始化完成")
}

// GetForwarder 获取全局转发器
func GetForwarder() *MessageForwarder {
	if globalForwarder == nil {
		globalForwarder = NewMessageForwarder()
	}
	return globalForwarder
}

// ForwardServiceMessage 转发服务消息（便捷函数）
func ForwardServiceMessage(msg *pb.GameMessage) (*pb.GameMessageResponse, error) {
	return GetForwarder().ForwardMessage(msg)
}

// GetRateLimiterStatus 获取限流器状态（用于监控）
func (f *MessageForwarder) GetRateLimiterStatus() map[string]interface{} {
	return f.rateLimiter.getStatus()
}
