package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"
)

// IP白名单管理
type IPWhitelist struct {
	whitelist map[string]bool
	mu        sync.RWMutex
}

var (
	ipWhitelist = &IPWhitelist{
		whitelist: make(map[string]bool),
	}
	// 默认允许的IP段
	defaultAllowedIPs = []string{
		"127.0.0.1",     // 本地回环
		"192.168.",      // 内网
		"10.",           // 内网
		"172.16.",       // 内网
		"172.17.",       // 内网
		"172.18.",       // 内网
		"172.19.",       // 内网
		"172.20.",       // 内网
		"172.21.",       // 内网
		"172.22.",       // 内网
		"172.23.",       // 内网
		"172.24.",       // 内网
		"172.25.",       // 内网
		"172.26.",       // 内网
		"172.27.",       // 内网
		"172.28.",       // 内网
		"172.29.",       // 内网
		"172.30.",       // 内网
		"172.31.",       // 内网
	}
)

// 初始化IP白名单
func InitIPWhitelist() {
	ipWhitelist.mu.Lock()
	defer ipWhitelist.mu.Unlock()
	
	// 添加默认允许的IP
	for _, ip := range defaultAllowedIPs {
		ipWhitelist.whitelist[ip] = true
	}
	log.Printf("[认证] IP白名单初始化完成，默认允许内网IP")
}

// 添加IP到白名单
func AddToWhitelist(ip string) {
	ipWhitelist.mu.Lock()
	defer ipWhitelist.mu.Unlock()
	ipWhitelist.whitelist[ip] = true
	log.Printf("[认证] 添加IP到白名单: %s", ip)
}

// 从白名单移除IP
func RemoveFromWhitelist(ip string) {
	ipWhitelist.mu.Lock()
	defer ipWhitelist.mu.Unlock()
	delete(ipWhitelist.whitelist, ip)
	log.Printf("[认证] 从白名单移除IP: %s", ip)
}

// 检查IP是否在白名单中
func IsIPAllowed(ip string) bool {
	ipWhitelist.mu.RLock()
	defer ipWhitelist.mu.RUnlock()
	
	// 检查精确匹配
	if ipWhitelist.whitelist[ip] {
		return true
	}
	
	// 检查IP段匹配
	for allowedIP := range ipWhitelist.whitelist {
		if strings.HasSuffix(allowedIP, ".") && strings.HasPrefix(ip, allowedIP) {
			return true
		}
	}
	
	return false
}

// 会话令牌管理
type TokenManager struct {
	secretKey []byte
	tokens    map[string]*TokenInfo
	mu        sync.RWMutex
}

type TokenInfo struct {
	PlayerID  string
	IP        string
	ExpiresAt time.Time
	CreatedAt time.Time
}

var (
	tokenManager = &TokenManager{
		secretKey: []byte("your-secret-key-change-this-in-production"),
		tokens:    make(map[string]*TokenInfo),
	}
)

// 初始化令牌管理器
func InitTokenManager(secretKey string) {
	if secretKey != "" {
		tokenManager.secretKey = []byte(secretKey)
	}
	
	// 启动令牌清理协程
	go tokenManager.cleanupExpiredTokens()
	log.Printf("[认证] 令牌管理器初始化完成")
}

// 生成会话令牌
func GenerateToken(playerID, ip string, duration time.Duration) string {
	tokenManager.mu.Lock()
	defer tokenManager.mu.Unlock()
	
	// 生成唯一令牌
	timestamp := time.Now().Unix()
	data := fmt.Sprintf("%s:%s:%d", playerID, ip, timestamp)
	
	// 使用HMAC-SHA256签名
	h := hmac.New(sha256.New, tokenManager.secretKey)
	h.Write([]byte(data))
	signature := hex.EncodeToString(h.Sum(nil))
	
	token := fmt.Sprintf("%s.%s", data, signature)
	
	// 存储令牌信息
	tokenManager.tokens[token] = &TokenInfo{
		PlayerID:  playerID,
		IP:        ip,
		ExpiresAt: time.Now().Add(duration),
		CreatedAt: time.Now(),
	}
	
	log.Printf("[认证] 为玩家[%s]生成令牌，IP: %s", playerID, ip)
	return token
}

// 从payload生成令牌（payload包含IP信息）
func GenerateTokenFromPayload(playerID string, payload []byte, duration time.Duration) string {
	// 假设payload包含IP信息，这里简化处理
	ip := string(payload)
	if ip == "" {
		ip = "unknown"
	}
	return GenerateToken(playerID, ip, duration)
}

// 验证会话令牌
func ValidateToken(token, playerID, ip string) bool {
	tokenManager.mu.RLock()
	defer tokenManager.mu.RUnlock()
	
	tokenInfo, exists := tokenManager.tokens[token]
	if !exists {
		log.Printf("[认证] 令牌不存在: %s", token)
		return false
	}
	
	// 检查是否过期
	if time.Now().After(tokenInfo.ExpiresAt) {
		log.Printf("[认证] 令牌已过期: %s", token)
		return false
	}
	
	// 检查玩家ID和IP是否匹配
	if tokenInfo.PlayerID != playerID || tokenInfo.IP != ip {
		log.Printf("[认证] 令牌信息不匹配: token_player=%s, player=%s, token_ip=%s, ip=%s", 
			tokenInfo.PlayerID, playerID, tokenInfo.IP, ip)
		return false
	}
	
	return true
}

// 撤销令牌
func RevokeToken(token string) {
	tokenManager.mu.Lock()
	defer tokenManager.mu.Unlock()
	
	if tokenInfo, exists := tokenManager.tokens[token]; exists {
		log.Printf("[认证] 撤销玩家[%s]的令牌", tokenInfo.PlayerID)
		delete(tokenManager.tokens, token)
	}
}

// 清理过期令牌
func (tm *TokenManager) cleanupExpiredTokens() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	
	for range ticker.C {
		tm.mu.Lock()
		now := time.Now()
		count := 0
		for token, info := range tm.tokens {
			if now.After(info.ExpiresAt) {
				delete(tm.tokens, token)
				count++
			}
		}
		tm.mu.Unlock()
		
		if count > 0 {
			log.Printf("[认证] 清理了 %d 个过期令牌", count)
		}
	}
}

// 获取令牌统计信息
func GetTokenStats() map[string]interface{} {
	tokenManager.mu.RLock()
	defer tokenManager.mu.RUnlock()
	
	now := time.Now()
	activeCount := 0
	expiredCount := 0
	
	for _, info := range tokenManager.tokens {
		if now.After(info.ExpiresAt) {
			expiredCount++
		} else {
			activeCount++
		}
	}
	
	return map[string]interface{}{
		"total_tokens":   len(tokenManager.tokens),
		"active_tokens":  activeCount,
		"expired_tokens": expiredCount,
	}
}