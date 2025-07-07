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

type AppConfig struct {
	Nacos             NacosConfig `yaml:"nacos"`
	Kafka             KafkaConfig `yaml:"kafka"`
	EnableIPWhitelist bool        `yaml:"enable_ip_whitelist"`
	EnableTokenCheck  bool        `yaml:"enable_token_check"`
}

var (
	Global     AppConfig
	configPath = "./config.yaml"
	mu         sync.RWMutex
	GatesvrID  string
)

// 加载配置
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

// 热更配置（定时检测文件变化）
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

// 获取配置快照
func GetConfig() AppConfig {
	mu.RLock()
	defer mu.RUnlock()
	return Global
}

func SetGatesvrID(id string) {
	GatesvrID = id
}

func GetGatesvrID() string {
	return GatesvrID
}
