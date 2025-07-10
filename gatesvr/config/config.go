package config

import (
	"io/ioutil"
	"log"
	"os"
	"sync"
	"time"

	"gopkg.in/yaml.v2"
)

type NacosConfig struct {
	Addr     string `yaml:"addr"`
	Port     uint64 `yaml:"port"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	Service  string `yaml:"service"`
	Group    string `yaml:"group"`
}

type KafkaConfig struct {
	Brokers  []string `yaml:"brokers"`
	CaCert   string   `yaml:"caCert"`
	Topic    string   `yaml:"topic"`
	Username string   `yaml:"username"`
	Password string   `yaml:"password"`
	GroupID  string   `yaml:"groupId"`
}

// 服务特定配置
type ServiceConfig struct {
	Group       string `yaml:"group"`
	LoadBalance string `yaml:"load_balance"`
	TimeoutMs   int    `yaml:"timeout_ms"`
}

// 服务转发配置
type ServiceForwardingConfig struct {
	DefaultLoadBalance string                   `yaml:"default_load_balance"`
	RequestTimeoutMs   int                      `yaml:"request_timeout_ms"`
	AsyncTimeoutMs     int                      `yaml:"async_timeout_ms"`
	Services           map[string]ServiceConfig `yaml:"services"`
}

// 限流配置
type RateLimitConfig struct {
	Enabled            bool  `yaml:"enabled"`              // 是否启用限流
	GlobalRate         int64 `yaml:"global_rate"`          // 全局速率（每秒）
	GlobalCapacity     int64 `yaml:"global_capacity"`      // 全局容量
	ServiceRate        int64 `yaml:"service_rate"`         // 单服务速率（每秒）
	ServiceCapacity    int64 `yaml:"service_capacity"`     // 单服务容量
	PlayerRate         int64 `yaml:"player_rate"`          // 单玩家速率（每秒）
	PlayerCapacity     int64 `yaml:"player_capacity"`      // 单玩家容量
	CleanupIntervalMin int   `yaml:"cleanup_interval_min"` // 清理间隔（分钟）
	BucketIdleTimeMin  int   `yaml:"bucket_idle_time_min"` // 令牌桶空闲时间（分钟）
}

// 认证配置
type AuthConfig struct {
	Enabled             bool     `yaml:"enabled"`               // 是否启用认证
	SecretKey           string   `yaml:"secret_key"`            // JWT密钥
	TokenExpireHours    int      `yaml:"token_expire_hours"`    // Token过期时间（小时）
	CleanupIntervalMin  int      `yaml:"cleanup_interval_min"`  // 清理间隔（分钟）
	RequireAuth         []string `yaml:"require_auth"`          // 需要认证的消息类型
	WhitelistServices   []string `yaml:"whitelist_services"`    // 免认证的服务列表
	MaxTokensPerPlayer  int      `yaml:"max_tokens_per_player"` // 单玩家最大Token数
	EnableAutoRefresh   bool     `yaml:"enable_auto_refresh"`   // 是否启用自动刷新
	RefreshThresholdMin int      `yaml:"refresh_threshold_min"` // 自动刷新阈值（分钟）
}

type AppConfig struct {
	Nacos             NacosConfig             `yaml:"nacos"`
	Kafka             KafkaConfig             `yaml:"kafka"`
	EnableIPWhitelist bool                    `yaml:"enable_ip_whitelist"`
	EnableTokenCheck  bool                    `yaml:"enable_token_check"`
	ServiceForwarding ServiceForwardingConfig `yaml:"service_forwarding"`
	RateLimit         RateLimitConfig         `yaml:"rate_limit"`
	Auth              AuthConfig              `yaml:"auth"`
}

var (
	Global     AppConfig
	configPath = "./config.yaml"
	mu         sync.RWMutex
	GatesvrID  string
)

// LoadConfig 加载配置
func LoadConfig() error {
	data, err := ioutil.ReadFile(configPath)
	if err != nil {
		return err
	}
	var cfg AppConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return err
	}
	mu.Lock()
	Global = cfg
	mu.Unlock()
	log.Println("配置加载成功")
	return nil
}

// WatchConfig 热更配置（定时检测文件变化）
func WatchConfig() {
	go func() {
		var lastModTime time.Time
		for {
			fi, err := os.Stat(configPath)
			if err == nil && fi.ModTime() != lastModTime {
				if err := LoadConfig(); err == nil {
					lastModTime = fi.ModTime()
				}
			}
			time.Sleep(3 * time.Second)
		}
	}()
}

// GetConfig 获取配置快照
func GetConfig() AppConfig {
	mu.RLock()
	defer mu.RUnlock()
	return Global
}

// SetGatesvrID 设置网关服务器ID
func SetGatesvrID(id string) {
	GatesvrID = id
}

// GetGatesvrID 获取网关服务器ID
func GetGatesvrID() string {
	return GatesvrID
}

// GetServiceConfig 获取服务配置，如果没有配置则返回默认值
func GetServiceConfig(serviceName string) ServiceConfig {
	cfg := GetConfig()
	if serviceConfig, exists := cfg.ServiceForwarding.Services[serviceName]; exists {
		return serviceConfig
	}
	// 返回默认配置
	return ServiceConfig{
		Group:       "DEFAULT_GROUP",
		LoadBalance: cfg.ServiceForwarding.DefaultLoadBalance,
		TimeoutMs:   cfg.ServiceForwarding.RequestTimeoutMs,
	}
}

// GetRateLimitConfig 获取限流配置，如果没有配置则返回默认值
func GetRateLimitConfig() RateLimitConfig {
	cfg := GetConfig()
	rateLimitCfg := cfg.RateLimit

	// 设置默认值
	if rateLimitCfg.GlobalRate == 0 {
		rateLimitCfg.GlobalRate = 500
	}
	if rateLimitCfg.GlobalCapacity == 0 {
		rateLimitCfg.GlobalCapacity = 1000
	}
	if rateLimitCfg.ServiceRate == 0 {
		rateLimitCfg.ServiceRate = 100
	}
	if rateLimitCfg.ServiceCapacity == 0 {
		rateLimitCfg.ServiceCapacity = 200
	}
	if rateLimitCfg.PlayerRate == 0 {
		rateLimitCfg.PlayerRate = 10
	}
	if rateLimitCfg.PlayerCapacity == 0 {
		rateLimitCfg.PlayerCapacity = 20
	}
	if rateLimitCfg.CleanupIntervalMin == 0 {
		rateLimitCfg.CleanupIntervalMin = 1
	}
	if rateLimitCfg.BucketIdleTimeMin == 0 {
		rateLimitCfg.BucketIdleTimeMin = 5
	}

	return rateLimitCfg
}

// GetAuthConfig 获取认证配置，如果没有配置则返回默认值
func GetAuthConfig() AuthConfig {
	cfg := GetConfig()
	authCfg := cfg.Auth

	// 设置默认值
	if authCfg.SecretKey == "" {
		authCfg.SecretKey = "mua-gatesvr-default-secret-key-2024"
	}
	if authCfg.TokenExpireHours == 0 {
		authCfg.TokenExpireHours = 24 // 默认24小时
	}
	if authCfg.CleanupIntervalMin == 0 {
		authCfg.CleanupIntervalMin = 10 // 默认10分钟清理一次
	}
	if authCfg.MaxTokensPerPlayer == 0 {
		authCfg.MaxTokensPerPlayer = 3 // 默认单玩家最大3个Token
	}
	if authCfg.RefreshThresholdMin == 0 {
		authCfg.RefreshThresholdMin = 60 // 默认60分钟内自动刷新
	}

	// 默认需要认证的消息类型
	if len(authCfg.RequireAuth) == 0 {
		authCfg.RequireAuth = []string{"SERVICE_MESSAGE"}
	}

	return authCfg
}
