package config

import (
	"log"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config 配置结构
type Config struct {
	Service      ServiceConfig      `yaml:"service"`
	Nacos        NacosConfig        `yaml:"nacos"`
	Kafka        KafkaConfig        `yaml:"kafka"`
	Notification NotificationConfig `yaml:"notification"`
	Game         GameConfig         `yaml:"game"`
	Log          LogConfig          `yaml:"log"`
	Performance  PerformanceConfig  `yaml:"performance"`
}

// ServiceConfig 服务配置
type ServiceConfig struct {
	Name    string `yaml:"name"`
	Port    int    `yaml:"port"`
	Version string `yaml:"version"`
}

// NacosConfig Nacos配置
type NacosConfig struct {
	Addr                string  `yaml:"addr"`
	Port                int     `yaml:"port"`
	Username            string  `yaml:"username"`
	Password            string  `yaml:"password"`
	Service             string  `yaml:"service"`
	Group               string  `yaml:"group"`
	Cluster             string  `yaml:"cluster"`
	Weight              float64 `yaml:"weight"`
	EnableRegister      bool    `yaml:"enable_register"`
	EnableHeartbeat     bool    `yaml:"enable_heartbeat"`
	HeartbeatIntervalMs int     `yaml:"heartbeat_interval_ms"`
}

// KafkaConfig Kafka配置
type KafkaConfig struct {
	Brokers    []string         `yaml:"brokers"`
	CaCert     string           `yaml:"caCert"`
	Username   string           `yaml:"username"`
	Password   string           `yaml:"password"`
	Producer   ProducerConfig   `yaml:"producer"`
	Consumer   ConsumerConfig   `yaml:"consumer"`
	Connection ConnectionConfig `yaml:"connection"`
}

// ProducerConfig 生产者配置
type ProducerConfig struct {
	Enabled bool              `yaml:"enabled"`
	Topics  map[string]string `yaml:"topics"`
}

// ConsumerConfig 消费者配置
type ConsumerConfig struct {
	Enabled         bool              `yaml:"enabled"`
	GroupID         string            `yaml:"group_id"`
	Topics          map[string]string `yaml:"topics"`
	AutoOffsetReset string            `yaml:"auto_offset_reset"`
}

// ConnectionConfig 连接配置
type ConnectionConfig struct {
	TimeoutMs       int `yaml:"timeout_ms"`
	RetryTimes      int `yaml:"retry_times"`
	RetryIntervalMs int `yaml:"retry_interval_ms"`
}

// NotificationConfig 消息推送配置
type NotificationConfig struct {
	Enabled    bool          `yaml:"enabled"`
	QueueSize  int           `yaml:"queue_size"`
	Workers    int           `yaml:"workers"`
	TimeoutSec int           `yaml:"timeout_sec"`
	RetryTimes int           `yaml:"retry_times"`
	Gatesvr    GatesvrConfig `yaml:"gatesvr"`
}

// GatesvrConfig gatesvr配置
type GatesvrConfig struct {
	ServiceName string `yaml:"service_name"`
	Group       string `yaml:"group"`
	LoadBalance string `yaml:"load_balance"`
}

// GameConfig 游戏配置
type GameConfig struct {
	Room  RoomConfig  `yaml:"room"`
	Board BoardConfig `yaml:"board"`
	Rules RulesConfig `yaml:"rules"`
}

// RoomConfig 房间配置
type RoomConfig struct {
	MaxRooms               int `yaml:"max_rooms"`
	MaxPlayersPerRoom      int `yaml:"max_players_per_room"`
	RoomTimeoutMin         int `yaml:"room_timeout_min"`
	AutoCleanupIntervalMin int `yaml:"auto_cleanup_interval_min"`
}

// BoardConfig 棋盘配置
type BoardConfig struct {
	Size                 int  `yaml:"size"`
	WinCondition         int  `yaml:"win_condition"`
	EnableForbiddenMoves bool `yaml:"enable_forbidden_moves"`
}

// RulesConfig 游戏规则配置
type RulesConfig struct {
	MoveTimeoutSec  int  `yaml:"move_timeout_sec"`
	GameTimeoutMin  int  `yaml:"game_timeout_min"`
	EnableUndo      bool `yaml:"enable_undo"`
	EnableSurrender bool `yaml:"enable_surrender"`
}

// LogConfig 日志配置
type LogConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
	Output string `yaml:"output"`
}

// PerformanceConfig 性能配置
type PerformanceConfig struct {
	MaxConnections   int `yaml:"max_connections"`
	ReadTimeoutSec   int `yaml:"read_timeout_sec"`
	WriteTimeoutSec  int `yaml:"write_timeout_sec"`
	MaxRequestSizeMb int `yaml:"max_request_size_mb"`
}

var globalConfig *Config

// LoadConfig 加载配置文件
func LoadConfig() error {
	configFile := "config-local.yaml"

	// 尝试从当前目录和上级目录加载配置
	configPaths := []string{
		configFile,
		filepath.Join("..", configFile),
		filepath.Join("minigame", "gomokusvr", configFile),
	}

	var configData []byte
	var err error

	for _, path := range configPaths {
		if configData, err = os.ReadFile(path); err == nil {
			log.Printf("成功加载配置文件: %s", path)
			break
		}
	}

	if err != nil {
		return err
	}

	var config Config
	if err := yaml.Unmarshal(configData, &config); err != nil {
		return err
	}

	globalConfig = &config
	log.Printf("配置加载完成: %+v", config)
	return nil
}

// GetConfig 获取全局配置
func GetConfig() *Config {
	if globalConfig == nil {
		log.Fatal("配置未初始化，请先调用LoadConfig()")
	}
	return globalConfig
}

// GetServiceConfig 获取服务配置
func GetServiceConfig() ServiceConfig {
	return GetConfig().Service
}

// GetNacosConfig 获取Nacos配置
func GetNacosConfig() NacosConfig {
	return GetConfig().Nacos
}

// GetKafkaConfig 获取Kafka配置
func GetKafkaConfig() KafkaConfig {
	return GetConfig().Kafka
}

// GetNotificationConfig 获取通知配置
func GetNotificationConfig() NotificationConfig {
	return GetConfig().Notification
}

// GetGameConfig 获取游戏配置
func GetGameConfig() GameConfig {
	return GetConfig().Game
}

// GetRoomConfig 获取房间配置
func GetRoomConfig() RoomConfig {
	return GetConfig().Game.Room
}

// GetLogConfig 获取日志配置
func GetLogConfig() LogConfig {
	return GetConfig().Log
}

// GetPerformanceConfig 获取性能配置
func GetPerformanceConfig() PerformanceConfig {
	return GetConfig().Performance
}
