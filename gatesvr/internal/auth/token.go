package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"mua/gatesvr/config"
)

// TokenClaims Token载荷结构
type TokenClaims struct {
	PlayerID    string   `json:"player_id"`   // 玩家ID
	Username    string   `json:"username"`    // 用户名
	Level       int      `json:"level"`       // 玩家等级
	VIP         int      `json:"vip"`         // VIP等级
	LoginTime   int64    `json:"login_time"`  // 登录时间戳
	ExpireTime  int64    `json:"expire_time"` // 过期时间戳
	Permissions []string `json:"permissions"` // 权限列表
	Platform    string   `json:"platform"`    // 登录平台
	DeviceID    string   `json:"device_id"`   // 设备ID
	SessionID   string   `json:"session_id"`  // 会话ID
}

// TokenInfo Token详细信息（包含原始Token）
type TokenInfo struct {
	Token    string       `json:"token"`
	Claims   *TokenClaims `json:"claims"`
	IsValid  bool         `json:"is_valid"`
	CreateAt time.Time    `json:"create_at"`
}

// TokenCache Token缓存结构
type TokenCache struct {
	cache sync.Map     // playerID -> *TokenInfo
	mu    sync.RWMutex // 保护清理操作

	// 性能统计
	hitCount   int64
	missCount  int64
	totalCount int64
}

// 全局Token缓存
var globalTokenCache = &TokenCache{}

// JWT Header
type JWTHeader struct {
	Alg string `json:"alg"`
	Typ string `json:"typ"`
}

// GenerateToken 生成JWT Token
func GenerateToken(claims *TokenClaims) (string, error) {
	cfg := config.GetAuthConfig()

	// 设置过期时间
	if claims.ExpireTime == 0 {
		claims.ExpireTime = time.Now().Add(time.Duration(cfg.TokenExpireHours) * time.Hour).Unix()
	}

	// 设置登录时间
	if claims.LoginTime == 0 {
		claims.LoginTime = time.Now().Unix()
	}

	// 1. 创建Header
	header := JWTHeader{
		Alg: "HS256",
		Typ: "JWT",
	}

	headerJSON, err := json.Marshal(header)
	if err != nil {
		return "", fmt.Errorf("序列化Header失败: %v", err)
	}
	headerEncoded := base64.RawURLEncoding.EncodeToString(headerJSON)

	// 2. 创建Payload
	payloadJSON, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("序列化Payload失败: %v", err)
	}
	payloadEncoded := base64.RawURLEncoding.EncodeToString(payloadJSON)

	// 3. 创建Signature
	message := headerEncoded + "." + payloadEncoded
	signature := createSignature(message, cfg.SecretKey)

	// 4. 组装Token
	token := message + "." + signature

	// 5. 缓存Token
	tokenInfo := &TokenInfo{
		Token:    token,
		Claims:   claims,
		IsValid:  true,
		CreateAt: time.Now(),
	}

	globalTokenCache.cache.Store(claims.PlayerID, tokenInfo)
	log.Printf("[Token生成] 玩家: %s, 过期时间: %s", claims.PlayerID, time.Unix(claims.ExpireTime, 0).Format("2006-01-02 15:04:05"))

	return token, nil
}

// VerifyToken 验证Token
func VerifyToken(token string) (*TokenClaims, error) {
	if token == "" {
		return nil, fmt.Errorf("Token为空")
	}

	// 分割Token
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("Token格式错误")
	}

	headerEncoded, payloadEncoded, signatureEncoded := parts[0], parts[1], parts[2]

	// 1. 验证签名
	message := headerEncoded + "." + payloadEncoded
	cfg := config.GetAuthConfig()
	expectedSignature := createSignature(message, cfg.SecretKey)

	if signatureEncoded != expectedSignature {
		return nil, fmt.Errorf("Token签名验证失败")
	}

	// 2. 解析Payload
	payloadJSON, err := base64.RawURLEncoding.DecodeString(payloadEncoded)
	if err != nil {
		return nil, fmt.Errorf("Token解码失败: %v", err)
	}

	var claims TokenClaims
	if err := json.Unmarshal(payloadJSON, &claims); err != nil {
		return nil, fmt.Errorf("Token反序列化失败: %v", err)
	}

	// 3. 验证过期时间
	now := time.Now().Unix()
	if claims.ExpireTime < now {
		return nil, fmt.Errorf("Token已过期")
	}

	return &claims, nil
}

// ValidatePlayerToken 验证玩家Token（带缓存优化）
func ValidatePlayerToken(playerID, token string) (*TokenClaims, error) {
	globalTokenCache.totalCount++

	// 1. 先检查缓存
	if cachedInfo, exists := globalTokenCache.cache.Load(playerID); exists {
		tokenInfo := cachedInfo.(*TokenInfo)

		// 验证缓存的Token是否匹配
		if tokenInfo.Token == token && tokenInfo.IsValid {
			// 检查是否过期
			if tokenInfo.Claims.ExpireTime >= time.Now().Unix() {
				globalTokenCache.hitCount++
				log.Printf("[Token验证] 缓存命中 - 玩家: %s", playerID)
				return tokenInfo.Claims, nil
			} else {
				// Token过期，从缓存中移除
				globalTokenCache.cache.Delete(playerID)
				log.Printf("[Token验证] Token过期 - 玩家: %s", playerID)
			}
		}
	}

	globalTokenCache.missCount++

	// 2. 缓存未命中，进行完整验证
	claims, err := VerifyToken(token)
	if err != nil {
		log.Printf("[Token验证] 验证失败 - 玩家: %s, 错误: %v", playerID, err)
		return nil, err
	}

	// 验证PlayerID是否匹配
	if claims.PlayerID != playerID {
		return nil, fmt.Errorf("Token中的玩家ID不匹配")
	}

	// 3. 更新缓存
	tokenInfo := &TokenInfo{
		Token:    token,
		Claims:   claims,
		IsValid:  true,
		CreateAt: time.Now(),
	}
	globalTokenCache.cache.Store(playerID, tokenInfo)

	log.Printf("[Token验证] 验证成功 - 玩家: %s, 平台: %s", playerID, claims.Platform)
	return claims, nil
}

// InvalidatePlayerToken 使玩家Token失效
func InvalidatePlayerToken(playerID string) {
	if cachedInfo, exists := globalTokenCache.cache.Load(playerID); exists {
		tokenInfo := cachedInfo.(*TokenInfo)
		tokenInfo.IsValid = false
		globalTokenCache.cache.Store(playerID, tokenInfo)
		log.Printf("[Token失效] 玩家: %s", playerID)
	}
}

// RefreshToken 刷新Token
func RefreshToken(playerID string) (string, error) {
	cachedInfo, exists := globalTokenCache.cache.Load(playerID)
	if !exists {
		return "", fmt.Errorf("未找到玩家Token缓存")
	}

	tokenInfo := cachedInfo.(*TokenInfo)
	claims := tokenInfo.Claims

	// 创建新的Claims，延长过期时间
	newClaims := *claims
	cfg := config.GetAuthConfig()
	newClaims.ExpireTime = time.Now().Add(time.Duration(cfg.TokenExpireHours) * time.Hour).Unix()

	// 生成新Token
	newToken, err := GenerateToken(&newClaims)
	if err != nil {
		return "", fmt.Errorf("生成新Token失败: %v", err)
	}

	log.Printf("[Token刷新] 玩家: %s, 新过期时间: %s", playerID, time.Unix(newClaims.ExpireTime, 0).Format("2006-01-02 15:04:05"))
	return newToken, nil
}

// createSignature 创建HMAC-SHA256签名
func createSignature(message, secret string) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(message))
	return base64.RawURLEncoding.EncodeToString(h.Sum(nil))
}

// GetCacheStats 获取缓存统计信息
func GetCacheStats() map[string]interface{} {
	totalCached := 0
	globalTokenCache.cache.Range(func(key, value interface{}) bool {
		totalCached++
		return true
	})

	var hitRate float64
	if globalTokenCache.totalCount > 0 {
		hitRate = float64(globalTokenCache.hitCount) / float64(globalTokenCache.totalCount) * 100
	}

	return map[string]interface{}{
		"total_requests": globalTokenCache.totalCount,
		"cache_hits":     globalTokenCache.hitCount,
		"cache_misses":   globalTokenCache.missCount,
		"hit_rate":       fmt.Sprintf("%.2f%%", hitRate),
		"cached_tokens":  totalCached,
	}
}

// CleanupExpiredTokens 清理过期Token
func CleanupExpiredTokens() {
	globalTokenCache.mu.Lock()
	defer globalTokenCache.mu.Unlock()

	now := time.Now().Unix()
	cleanedCount := 0

	globalTokenCache.cache.Range(func(key, value interface{}) bool {
		tokenInfo := value.(*TokenInfo)
		if tokenInfo.Claims.ExpireTime < now {
			globalTokenCache.cache.Delete(key)
			cleanedCount++
		}
		return true
	})

	if cleanedCount > 0 {
		log.Printf("[Token清理] 清理过期Token数量: %d", cleanedCount)
	}
}

// StartTokenCleanup 启动Token清理任务
func StartTokenCleanup() {
	cfg := config.GetAuthConfig()
	interval := time.Duration(cfg.CleanupIntervalMin) * time.Minute

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for range ticker.C {
			CleanupExpiredTokens()
		}
	}()

	log.Printf("[Token管理] 启动清理任务，间隔: %v", interval)
}

// GetPlayerTokenInfo 获取玩家Token信息
func GetPlayerTokenInfo(playerID string) (*TokenInfo, bool) {
	if cachedInfo, exists := globalTokenCache.cache.Load(playerID); exists {
		return cachedInfo.(*TokenInfo), true
	}
	return nil, false
}
