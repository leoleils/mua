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

type AppConfig struct {
	Nacos             NacosConfig             `yaml:"nacos"`
	Kafka             KafkaConfig             `yaml:"kafka"`
	EnableIPWhitelist bool                    `yaml:"enable_ip_whitelist"`
	EnableTokenCheck  bool                    `yaml:"enable_token_check"`
	ServiceForwarding ServiceForwardingConfig `yaml:"service_forwarding"`
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
