package auth

import (
	"fmt"
	"log"
	"mua/gatesvr/config"
	"mua/gatesvr/internal/pb"
	"strings"
	"time"
)

// AuthResult 认证结果
type AuthResult struct {
	Success     bool         `json:"success"`
	PlayerID    string       `json:"player_id"`
	Claims      *TokenClaims `json:"claims"`
	Reason      string       `json:"reason"`
	ErrorCode   int          `json:"error_code"`
	NeedRefresh bool         `json:"need_refresh"`
}

// 认证错误代码
const (
	AuthErrCodeSuccess        = 0    // 成功
	AuthErrCodeTokenMissing   = 1001 // Token缺失
	AuthErrCodeTokenInvalid   = 1002 // Token无效
	AuthErrCodeTokenExpired   = 1003 // Token过期
	AuthErrCodePlayerMismatch = 1004 // 玩家ID不匹配
	AuthErrCodePermissionDeny = 1005 // 权限不足
	AuthErrCodeServiceBlocked = 1006 // 服务被禁用
	AuthErrCodeAuthDisabled   = 1007 // 认证已禁用
)

// MessageAuthenticator 消息认证器
type MessageAuthenticator struct {
	config config.AuthConfig
}

// NewMessageAuthenticator 创建消息认证器
func NewMessageAuthenticator() *MessageAuthenticator {
	return &MessageAuthenticator{
		config: config.GetAuthConfig(),
	}
}

// AuthenticateMessage 认证消息
func (ma *MessageAuthenticator) AuthenticateMessage(msg *pb.GameMessage) *AuthResult {
	// 1. 检查是否启用认证
	if !ma.config.Enabled {
		return &AuthResult{
			Success:   true,
			Reason:    "认证已禁用",
			ErrorCode: AuthErrCodeAuthDisabled,
		}
	}

	// 2. 检查消息头
	if msg.MsgHead == nil {
		return &AuthResult{
			Success:   false,
			Reason:    "消息头为空",
			ErrorCode: AuthErrCodeTokenMissing,
		}
	}

	playerID := msg.MsgHead.PlayerId
	token := msg.MsgHead.Token

	// 3. 检查是否需要认证
	if !ma.needAuthentication(msg) {
		return &AuthResult{
			Success:  true,
			PlayerID: playerID,
			Reason:   "消息类型无需认证",
		}
	}

	// 4. Token检查
	if token == "" {
		return &AuthResult{
			Success:   false,
			PlayerID:  playerID,
			Reason:    "Token缺失",
			ErrorCode: AuthErrCodeTokenMissing,
		}
	}

	if playerID == "" {
		return &AuthResult{
			Success:   false,
			Reason:    "玩家ID缺失",
			ErrorCode: AuthErrCodePlayerMismatch,
		}
	}

	// 5. 验证Token
	claims, err := ValidatePlayerToken(playerID, token)
	if err != nil {
		errorCode := AuthErrCodeTokenInvalid
		if strings.Contains(err.Error(), "过期") {
			errorCode = AuthErrCodeTokenExpired
		} else if strings.Contains(err.Error(), "不匹配") {
			errorCode = AuthErrCodePlayerMismatch
		}

		return &AuthResult{
			Success:   false,
			PlayerID:  playerID,
			Reason:    err.Error(),
			ErrorCode: errorCode,
		}
	}

	// 6. 权限检查
	if !ma.checkPermissions(msg, claims) {
		return &AuthResult{
			Success:   false,
			PlayerID:  playerID,
			Claims:    claims,
			Reason:    "权限不足",
			ErrorCode: AuthErrCodePermissionDeny,
		}
	}

	// 7. 检查是否需要刷新Token
	needRefresh := ma.needTokenRefresh(claims)

	return &AuthResult{
		Success:     true,
		PlayerID:    playerID,
		Claims:      claims,
		Reason:      "认证成功",
		ErrorCode:   AuthErrCodeSuccess,
		NeedRefresh: needRefresh,
	}
}

// needAuthentication 检查消息是否需要认证
func (ma *MessageAuthenticator) needAuthentication(msg *pb.GameMessage) bool {
	// 心跳消息不需要认证
	if msg.MsgType == pb.MessageType_HEARTBEAT {
		return false
	}

	// 检查消息类型是否在需要认证的列表中
	msgTypeStr := msg.MsgType.String()
	for _, authType := range ma.config.RequireAuth {
		if authType == msgTypeStr {
			// 检查服务是否在白名单中
			if msg.MsgHead != nil && msg.MsgHead.ServiceName != "" {
				return !ma.isServiceWhitelisted(msg.MsgHead.ServiceName)
			}
			return true
		}
	}

	return false
}

// isServiceWhitelisted 检查服务是否在白名单中
func (ma *MessageAuthenticator) isServiceWhitelisted(serviceName string) bool {
	for _, whiteService := range ma.config.WhitelistServices {
		if whiteService == serviceName {
			return true
		}
	}
	return false
}

// checkPermissions 检查权限
func (ma *MessageAuthenticator) checkPermissions(msg *pb.GameMessage, claims *TokenClaims) bool {
	// 基础权限检查：所有认证用户都有基本权限
	if len(claims.Permissions) == 0 {
		return true // 如果没有定义权限，默认允许
	}

	// 检查服务权限
	if msg.MsgHead != nil && msg.MsgHead.ServiceName != "" {
		serviceName := msg.MsgHead.ServiceName
		requiredPermission := "service:" + serviceName

		// 检查是否有该服务的权限
		for _, permission := range claims.Permissions {
			if permission == requiredPermission || permission == "service:*" || permission == "*" {
				return true
			}
		}

		// 没有找到对应权限
		log.Printf("[权限检查] 玩家[%s]无权访问服务[%s]", claims.PlayerID, serviceName)
		return false
	}

	return true
}

// needTokenRefresh 检查是否需要刷新Token
func (ma *MessageAuthenticator) needTokenRefresh(claims *TokenClaims) bool {
	if !ma.config.EnableAutoRefresh {
		return false
	}

	now := time.Now().Unix()
	expireTime := claims.ExpireTime
	refreshThreshold := int64(ma.config.RefreshThresholdMin * 60) // 转换为秒

	// 如果距离过期时间小于阈值，需要刷新
	return (expireTime - now) < refreshThreshold
}

// HandleAuthResult 处理认证结果
func (ma *MessageAuthenticator) HandleAuthResult(result *AuthResult, playerID string) *pb.GameMessage {
	if result.Success {
		// 认证成功，检查是否需要刷新Token
		if result.NeedRefresh {
			go ma.handleTokenRefresh(playerID)
		}
		return nil // 认证成功，返回nil继续处理消息
	}

	// 认证失败，构造错误响应
	errorMsg := &pb.GameMessage{
		MsgHead: &pb.HeadMessage{
			PlayerId: playerID,
		},
		MsgType: pb.MessageType_CLIENT_MESSAGE,
		Payload: []byte(fmt.Sprintf("AUTH_ERROR:%d:%s", result.ErrorCode, result.Reason)),
	}

	log.Printf("[认证失败] 玩家: %s, 错误: %s, 代码: %d", playerID, result.Reason, result.ErrorCode)
	return errorMsg
}

// handleTokenRefresh 处理Token刷新
func (ma *MessageAuthenticator) handleTokenRefresh(playerID string) {
	newToken, err := RefreshToken(playerID)
	if err != nil {
		log.Printf("[Token刷新失败] 玩家: %s, 错误: %v", playerID, err)
		return
	}

	// 构造Token刷新通知消息
	refreshMsg := &pb.GameMessage{
		MsgHead: &pb.HeadMessage{
			PlayerId: playerID,
			Token:    newToken,
		},
		MsgType: pb.MessageType_CLIENT_MESSAGE,
		Payload: []byte("TOKEN_REFRESHED"),
	}

	// 这里可以调用连接管理器发送Token刷新消息给客户端
	// conn.SendToPlayer(playerID, refreshMsg)

	log.Printf("[Token刷新成功] 玩家: %s", playerID)
	_ = refreshMsg // 暂时避免未使用变量警告
}

// GetAuthStats 获取认证统计信息
func (ma *MessageAuthenticator) GetAuthStats() map[string]interface{} {
	tokenStats := GetCacheStats()

	return map[string]interface{}{
		"auth_enabled":         ma.config.Enabled,
		"token_expire_hours":   ma.config.TokenExpireHours,
		"require_auth_types":   ma.config.RequireAuth,
		"whitelist_services":   ma.config.WhitelistServices,
		"auto_refresh_enabled": ma.config.EnableAutoRefresh,
		"token_stats":          tokenStats,
	}
}

// ValidateConnectionAuth 验证连接认证（用于连接建立时）
func (ma *MessageAuthenticator) ValidateConnectionAuth(playerID, token string) *AuthResult {
	// 连接时的认证检查
	if !ma.config.Enabled {
		return &AuthResult{
			Success:   true,
			PlayerID:  playerID,
			Reason:    "认证已禁用",
			ErrorCode: AuthErrCodeAuthDisabled,
		}
	}

	if token == "" {
		return &AuthResult{
			Success:   false,
			PlayerID:  playerID,
			Reason:    "连接Token缺失",
			ErrorCode: AuthErrCodeTokenMissing,
		}
	}

	claims, err := ValidatePlayerToken(playerID, token)
	if err != nil {
		errorCode := AuthErrCodeTokenInvalid
		if strings.Contains(err.Error(), "过期") {
			errorCode = AuthErrCodeTokenExpired
		}

		return &AuthResult{
			Success:   false,
			PlayerID:  playerID,
			Reason:    fmt.Sprintf("连接认证失败: %v", err),
			ErrorCode: errorCode,
		}
	}

	return &AuthResult{
		Success:  true,
		PlayerID: playerID,
		Claims:   claims,
		Reason:   "连接认证成功",
	}
}

// 全局认证器实例
var globalAuthenticator *MessageAuthenticator

// InitAuth 初始化认证系统
func InitAuth() {
	globalAuthenticator = NewMessageAuthenticator()

	// 启动Token清理任务
	StartTokenCleanup()

	log.Println("认证系统初始化完成")
}

// GetAuthenticator 获取全局认证器
func GetAuthenticator() *MessageAuthenticator {
	if globalAuthenticator == nil {
		globalAuthenticator = NewMessageAuthenticator()
	}
	return globalAuthenticator
}

// AuthenticateGameMessage 认证游戏消息（便捷函数）
func AuthenticateGameMessage(msg *pb.GameMessage) *AuthResult {
	return GetAuthenticator().AuthenticateMessage(msg)
}

// ValidateConnection 验证连接认证（便捷函数）
func ValidateConnection(playerID, token string) *AuthResult {
	return GetAuthenticator().ValidateConnectionAuth(playerID, token)
}
