package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"mua/gatesvr/config"

	"github.com/go-redis/redis/v8"
)

// RedisTokenCache Redis Token缓存管理器
type RedisTokenCache struct {
	client  *redis.Client
	prefix  string
	enabled bool
	mu      sync.RWMutex

	// 统计信息
	stats       CacheStats
	initialized bool
}

// CacheStats 缓存统计信息
type CacheStats struct {
	RedisHits     int64     `json:"redis_hits"`
	RedisMisses   int64     `json:"redis_misses"`
	RedisErrors   int64     `json:"redis_errors"`
	RedisWrites   int64     `json:"redis_writes"`
	RedisDeletes  int64     `json:"redis_deletes"`
	LastError     string    `json:"last_error"`
	LastErrorTime time.Time `json:"last_error_time"`
}

// Redis缓存管理器实例
var (
	redisTokenCache *RedisTokenCache
	redisOnce       sync.Once
)

// GetRedisTokenCache 获取Redis Token缓存管理器实例
func GetRedisTokenCache() *RedisTokenCache {
	redisOnce.Do(func() {
		redisTokenCache = NewRedisTokenCache()
	})
	return redisTokenCache
}

// NewRedisTokenCache 创建Redis Token缓存管理器
func NewRedisTokenCache() *RedisTokenCache {
	authCfg := config.GetAuthConfig()

	rtc := &RedisTokenCache{
		prefix:  authCfg.RedisKeyPrefix,
		enabled: authCfg.EnableRedisCache,
		stats:   CacheStats{},
	}

	if !authCfg.EnableRedisCache {
		log.Println("[Redis缓存] Redis缓存已禁用，使用内存缓存")
		return rtc
	}

	// 创建Redis客户端
	rtc.client = redis.NewClient(&redis.Options{
		Addr:         authCfg.RedisAddr,
		Password:     authCfg.RedisPassword,
		DB:           authCfg.RedisDB,
		PoolSize:     authCfg.RedisPoolSize,
		MinIdleConns: authCfg.RedisMinIdleConns,
		DialTimeout:  time.Duration(authCfg.RedisDialTimeout) * time.Second,
		ReadTimeout:  time.Duration(authCfg.RedisReadTimeout) * time.Second,
		WriteTimeout: time.Duration(authCfg.RedisWriteTimeout) * time.Second,
	})

	// 测试连接
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rtc.client.Ping(ctx).Err(); err != nil {
		log.Printf("[Redis缓存] 连接失败: %v, 降级到内存缓存", err)
		rtc.enabled = false
		rtc.recordError(err)
		return rtc
	}

	rtc.initialized = true
	log.Printf("[Redis缓存] 初始化成功 - 地址: %s, 数据库: %d, 连接池: %d",
		authCfg.RedisAddr, authCfg.RedisDB, authCfg.RedisPoolSize)

	return rtc
}

// recordError 记录错误信息
func (rtc *RedisTokenCache) recordError(err error) {
	rtc.mu.Lock()
	defer rtc.mu.Unlock()

	rtc.stats.RedisErrors++
	rtc.stats.LastError = err.Error()
	rtc.stats.LastErrorTime = time.Now()
}

// GetToken 从Redis获取Token信息
func (rtc *RedisTokenCache) GetToken(tokenString string) (*TokenClaims, bool) {
	if !rtc.enabled || rtc.client == nil {
		return nil, false
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	key := rtc.prefix + tokenString
	result, err := rtc.client.Get(ctx, key).Result()

	if err != nil {
		if err == redis.Nil {
			// 缓存未命中
			rtc.mu.Lock()
			rtc.stats.RedisMisses++
			rtc.mu.Unlock()
			return nil, false
		}

		// Redis错误
		rtc.recordError(err)
		log.Printf("[Redis缓存] 获取Token失败: %v", err)
		return nil, false
	}

	// 解析Token数据
	var claims TokenClaims
	if err := json.Unmarshal([]byte(result), &claims); err != nil {
		rtc.recordError(err)
		log.Printf("[Redis缓存] Token数据解析失败: %v", err)
		return nil, false
	}

	// 检查是否过期
	if time.Now().Unix() > claims.ExpireTime {
		// 异步删除过期Token
		go rtc.DeleteToken(tokenString)
		rtc.mu.Lock()
		rtc.stats.RedisMisses++
		rtc.mu.Unlock()
		return nil, false
	}

	// 缓存命中
	rtc.mu.Lock()
	rtc.stats.RedisHits++
	rtc.mu.Unlock()

	return &claims, true
}

// SetToken 向Redis存储Token信息
func (rtc *RedisTokenCache) SetToken(tokenString string, claims *TokenClaims) error {
	if !rtc.enabled || rtc.client == nil {
		return fmt.Errorf("Redis缓存未启用")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// 序列化Token数据
	data, err := json.Marshal(claims)
	if err != nil {
		rtc.recordError(err)
		return fmt.Errorf("Token数据序列化失败: %v", err)
	}

	key := rtc.prefix + tokenString
	expiration := time.Until(time.Unix(claims.ExpireTime, 0))

	// 设置过期时间，确保不会超过Token本身的过期时间
	if expiration <= 0 {
		return fmt.Errorf("Token已过期")
	}

	err = rtc.client.Set(ctx, key, data, expiration).Err()
	if err != nil {
		rtc.recordError(err)
		return fmt.Errorf("Redis存储失败: %v", err)
	}

	rtc.mu.Lock()
	rtc.stats.RedisWrites++
	rtc.mu.Unlock()

	return nil
}

// DeleteToken 从Redis删除Token
func (rtc *RedisTokenCache) DeleteToken(tokenString string) error {
	if !rtc.enabled || rtc.client == nil {
		return fmt.Errorf("Redis缓存未启用")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	key := rtc.prefix + tokenString
	err := rtc.client.Del(ctx, key).Err()

	if err != nil {
		rtc.recordError(err)
		return fmt.Errorf("Redis删除失败: %v", err)
	}

	rtc.mu.Lock()
	rtc.stats.RedisDeletes++
	rtc.mu.Unlock()

	return nil
}

// DeletePlayerTokens 删除玩家的所有Token
func (rtc *RedisTokenCache) DeletePlayerTokens(playerID string) error {
	if !rtc.enabled || rtc.client == nil {
		return fmt.Errorf("Redis缓存未启用")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 搜索该玩家的所有Token
	pattern := rtc.prefix + "*"
	iter := rtc.client.Scan(ctx, 0, pattern, 0).Iterator()

	var keysToDelete []string

	for iter.Next(ctx) {
		key := iter.Val()

		// 获取Token数据
		result, err := rtc.client.Get(ctx, key).Result()
		if err != nil {
			continue
		}

		var claims TokenClaims
		if err := json.Unmarshal([]byte(result), &claims); err != nil {
			continue
		}

		// 检查是否是该玩家的Token
		if claims.PlayerID == playerID {
			keysToDelete = append(keysToDelete, key)
		}
	}

	if err := iter.Err(); err != nil {
		rtc.recordError(err)
		return fmt.Errorf("搜索玩家Token失败: %v", err)
	}

	// 批量删除
	if len(keysToDelete) > 0 {
		err := rtc.client.Del(ctx, keysToDelete...).Err()
		if err != nil {
			rtc.recordError(err)
			return fmt.Errorf("批量删除Token失败: %v", err)
		}

		rtc.mu.Lock()
		rtc.stats.RedisDeletes += int64(len(keysToDelete))
		rtc.mu.Unlock()

		log.Printf("[Redis缓存] 删除玩家 %s 的 %d 个Token", playerID, len(keysToDelete))
	}

	return nil
}

// CleanExpiredTokens 清理过期Token
func (rtc *RedisTokenCache) CleanExpiredTokens() error {
	if !rtc.enabled || rtc.client == nil {
		return fmt.Errorf("Redis缓存未启用")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pattern := rtc.prefix + "*"
	iter := rtc.client.Scan(ctx, 0, pattern, 0).Iterator()

	var expiredKeys []string
	now := time.Now().Unix()

	for iter.Next(ctx) {
		key := iter.Val()

		// 获取Token数据
		result, err := rtc.client.Get(ctx, key).Result()
		if err != nil {
			if err == redis.Nil {
				// 键已不存在
				continue
			}
			log.Printf("[Redis缓存] 清理时获取Token失败: %v", err)
			continue
		}

		var claims TokenClaims
		if err := json.Unmarshal([]byte(result), &claims); err != nil {
			// 数据格式错误，标记删除
			expiredKeys = append(expiredKeys, key)
			continue
		}

		// 检查是否过期
		if now > claims.ExpireTime {
			expiredKeys = append(expiredKeys, key)
		}
	}

	if err := iter.Err(); err != nil {
		rtc.recordError(err)
		return fmt.Errorf("扫描过期Token失败: %v", err)
	}

	// 批量删除过期Token
	if len(expiredKeys) > 0 {
		err := rtc.client.Del(ctx, expiredKeys...).Err()
		if err != nil {
			rtc.recordError(err)
			return fmt.Errorf("删除过期Token失败: %v", err)
		}

		rtc.mu.Lock()
		rtc.stats.RedisDeletes += int64(len(expiredKeys))
		rtc.mu.Unlock()

		log.Printf("[Redis缓存] 清理了 %d 个过期Token", len(expiredKeys))
	}

	return nil
}

// GetStats 获取缓存统计信息
func (rtc *RedisTokenCache) GetStats() CacheStats {
	rtc.mu.RLock()
	defer rtc.mu.RUnlock()
	return rtc.stats
}

// GetConnectionInfo 获取连接信息
func (rtc *RedisTokenCache) GetConnectionInfo() map[string]interface{} {
	info := map[string]interface{}{
		"enabled":     rtc.enabled,
		"initialized": rtc.initialized,
		"prefix":      rtc.prefix,
	}

	if rtc.client != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		poolStats := rtc.client.PoolStats()
		info["pool_hits"] = poolStats.Hits
		info["pool_misses"] = poolStats.Misses
		info["pool_timeouts"] = poolStats.Timeouts
		info["pool_total_conns"] = poolStats.TotalConns
		info["pool_idle_conns"] = poolStats.IdleConns
		info["pool_stale_conns"] = poolStats.StaleConns

		// 测试连接状态
		if err := rtc.client.Ping(ctx).Err(); err != nil {
			info["connection_status"] = "error"
			info["connection_error"] = err.Error()
		} else {
			info["connection_status"] = "ok"
		}
	}

	return info
}

// IsEnabled 检查Redis缓存是否启用
func (rtc *RedisTokenCache) IsEnabled() bool {
	return rtc.enabled && rtc.client != nil
}

// Close 关闭Redis连接
func (rtc *RedisTokenCache) Close() error {
	if rtc.client != nil {
		return rtc.client.Close()
	}
	return nil
}

// RefreshConnection 刷新Redis连接
func (rtc *RedisTokenCache) RefreshConnection() error {
	if !rtc.enabled {
		return fmt.Errorf("Redis缓存未启用")
	}

	// 关闭现有连接
	if rtc.client != nil {
		rtc.client.Close()
	}

	// 重新创建连接
	authCfg := config.GetAuthConfig()
	rtc.client = redis.NewClient(&redis.Options{
		Addr:         authCfg.RedisAddr,
		Password:     authCfg.RedisPassword,
		DB:           authCfg.RedisDB,
		PoolSize:     authCfg.RedisPoolSize,
		MinIdleConns: authCfg.RedisMinIdleConns,
		DialTimeout:  time.Duration(authCfg.RedisDialTimeout) * time.Second,
		ReadTimeout:  time.Duration(authCfg.RedisReadTimeout) * time.Second,
		WriteTimeout: time.Duration(authCfg.RedisWriteTimeout) * time.Second,
	})

	// 测试连接
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rtc.client.Ping(ctx).Err(); err != nil {
		rtc.enabled = false
		rtc.recordError(err)
		return fmt.Errorf("Redis重连失败: %v", err)
	}

	rtc.initialized = true
	log.Println("[Redis缓存] 连接刷新成功")
	return nil
}
