package auth

import (
	"encoding/json"
	"fmt"
	"log"
	"time"
)

// TokenGenerator Token生成器工具
type TokenGenerator struct {
	secretKey string
}

// NewTokenGenerator 创建Token生成器
func NewTokenGenerator(secretKey string) *TokenGenerator {
	if secretKey == "" {
		secretKey = "mua-gatesvr-jwt-secret-key-2024-production"
	}
	return &TokenGenerator{secretKey: secretKey}
}

// GenerateTestToken 生成测试Token
func (tg *TokenGenerator) GenerateTestToken(playerID, username string) (string, error) {
	claims := &TokenClaims{
		PlayerID:    playerID,
		Username:    username,
		Level:       1,
		VIP:         0,
		LoginTime:   time.Now().Unix(),
		ExpireTime:  time.Now().Add(24 * time.Hour).Unix(),
		Permissions: []string{"service:*"}, // 给予所有服务权限
		Platform:    "test",
		DeviceID:    "test-device",
		SessionID:   fmt.Sprintf("session_%d", time.Now().UnixNano()),
	}

	return GenerateToken(claims)
}

// GeneratePlayerToken 生成玩家Token（带权限控制）
func (tg *TokenGenerator) GeneratePlayerToken(playerID, username string, level, vip int, permissions []string, expireHours int) (string, error) {
	if expireHours <= 0 {
		expireHours = 24
	}

	claims := &TokenClaims{
		PlayerID:    playerID,
		Username:    username,
		Level:       level,
		VIP:         vip,
		LoginTime:   time.Now().Unix(),
		ExpireTime:  time.Now().Add(time.Duration(expireHours) * time.Hour).Unix(),
		Permissions: permissions,
		Platform:    "game",
		DeviceID:    fmt.Sprintf("device_%s", playerID),
		SessionID:   fmt.Sprintf("session_%s_%d", playerID, time.Now().UnixNano()),
	}

	return GenerateToken(claims)
}

// ParseTokenClaims 解析Token载荷
func (tg *TokenGenerator) ParseTokenClaims(token string) (*TokenClaims, error) {
	return VerifyToken(token)
}

// CreateDefaultPermissions 创建默认权限
func CreateDefaultPermissions(playerLevel, vipLevel int) []string {
	permissions := []string{
		"service:userservice", // 用户服务
		"service:gameservice", // 游戏服务
	}

	// VIP用户额外权限
	if vipLevel > 0 {
		permissions = append(permissions, "service:vipservice")
	}

	// 高级玩家额外权限
	if playerLevel >= 50 {
		permissions = append(permissions, "service:advancedservice")
	}

	// 管理员权限
	if playerLevel >= 100 {
		permissions = append(permissions, "service:*")
	}

	return permissions
}

// TokenInfo Token信息展示
type TokenDisplay struct {
	Token     string       `json:"token"`
	Claims    *TokenClaims `json:"claims"`
	IsValid   bool         `json:"is_valid"`
	ExpiresIn string       `json:"expires_in"`
	CreatedAt string       `json:"created_at"`
}

// DisplayToken 展示Token信息
func (tg *TokenGenerator) DisplayToken(token string) (*TokenDisplay, error) {
	claims, err := VerifyToken(token)
	if err != nil {
		return &TokenDisplay{
			Token:   token,
			IsValid: false,
		}, err
	}

	now := time.Now().Unix()
	expiresIn := time.Unix(claims.ExpireTime, 0).Sub(time.Now())
	createdAt := time.Unix(claims.LoginTime, 0)

	return &TokenDisplay{
		Token:     token,
		Claims:    claims,
		IsValid:   claims.ExpireTime > now,
		ExpiresIn: expiresIn.String(),
		CreatedAt: createdAt.Format("2006-01-02 15:04:05"),
	}, nil
}

// BatchGenerateTokens 批量生成Token
func (tg *TokenGenerator) BatchGenerateTokens(playerIDs []string) map[string]string {
	tokens := make(map[string]string)

	for _, playerID := range playerIDs {
		token, err := tg.GenerateTestToken(playerID, "test_"+playerID)
		if err != nil {
			log.Printf("生成Token失败 - 玩家: %s, 错误: %v", playerID, err)
			continue
		}
		tokens[playerID] = token
	}

	return tokens
}

// ExportTokens 导出Token到JSON
func (tg *TokenGenerator) ExportTokens(tokens map[string]string, filename string) error {
	tokenInfos := make(map[string]*TokenDisplay)

	for playerID, token := range tokens {
		display, err := tg.DisplayToken(token)
		if err != nil {
			log.Printf("解析Token失败 - 玩家: %s, 错误: %v", playerID, err)
			continue
		}
		tokenInfos[playerID] = display
	}

	jsonData, err := json.MarshalIndent(tokenInfos, "", "  ")
	if err != nil {
		return fmt.Errorf("JSON序列化失败: %v", err)
	}

	log.Printf("Token导出成功，共 %d 个Token", len(tokenInfos))
	log.Printf("JSON数据:\n%s", string(jsonData))

	return nil
}

// ValidateTokenBatch 批量验证Token
func (tg *TokenGenerator) ValidateTokenBatch(tokens map[string]string) map[string]bool {
	results := make(map[string]bool)

	for playerID, token := range tokens {
		_, err := ValidatePlayerToken(playerID, token)
		results[playerID] = (err == nil)

		if err != nil {
			log.Printf("Token验证失败 - 玩家: %s, 错误: %v", playerID, err)
		} else {
			log.Printf("Token验证成功 - 玩家: %s", playerID)
		}
	}

	return results
}

// CreateAdminToken 创建管理员Token
func (tg *TokenGenerator) CreateAdminToken(adminID string) (string, error) {
	claims := &TokenClaims{
		PlayerID:    adminID,
		Username:    "admin_" + adminID,
		Level:       999,
		VIP:         10,
		LoginTime:   time.Now().Unix(),
		ExpireTime:  time.Now().Add(7 * 24 * time.Hour).Unix(), // 7天有效期
		Permissions: []string{"*"},                             // 所有权限
		Platform:    "admin",
		DeviceID:    "admin-console",
		SessionID:   fmt.Sprintf("admin_session_%d", time.Now().UnixNano()),
	}

	return GenerateToken(claims)
}

// CreateGuestToken 创建访客Token（权限受限）
func (tg *TokenGenerator) CreateGuestToken(guestID string) (string, error) {
	claims := &TokenClaims{
		PlayerID:    guestID,
		Username:    "guest_" + guestID,
		Level:       0,
		VIP:         0,
		LoginTime:   time.Now().Unix(),
		ExpireTime:  time.Now().Add(2 * time.Hour).Unix(),          // 2小时有效期
		Permissions: []string{"service:health", "service:version"}, // 仅基础权限
		Platform:    "guest",
		DeviceID:    "guest-device",
		SessionID:   fmt.Sprintf("guest_session_%d", time.Now().UnixNano()),
	}

	return GenerateToken(claims)
}

// 便捷函数

// QuickGenerateToken 快速生成测试Token
func QuickGenerateToken(playerID string) (string, error) {
	generator := NewTokenGenerator("")
	return generator.GenerateTestToken(playerID, "test_"+playerID)
}

// QuickValidateToken 快速验证Token
func QuickValidateToken(playerID, token string) bool {
	_, err := ValidatePlayerToken(playerID, token)
	return err == nil
}

// PrintTokenInfo 打印Token信息（用于调试）
func PrintTokenInfo(token string) {
	generator := NewTokenGenerator("")
	display, err := generator.DisplayToken(token)
	if err != nil {
		log.Printf("Token解析失败: %v", err)
		return
	}

	log.Printf("=== Token信息 ===")
	log.Printf("Token: %s", display.Token[:50]+"...")
	log.Printf("有效性: %v", display.IsValid)
	log.Printf("剩余时间: %s", display.ExpiresIn)
	log.Printf("创建时间: %s", display.CreatedAt)
	if display.Claims != nil {
		log.Printf("玩家ID: %s", display.Claims.PlayerID)
		log.Printf("用户名: %s", display.Claims.Username)
		log.Printf("等级: %d", display.Claims.Level)
		log.Printf("VIP: %d", display.Claims.VIP)
		log.Printf("权限: %v", display.Claims.Permissions)
		log.Printf("平台: %s", display.Claims.Platform)
	}
	log.Printf("===============")
}
