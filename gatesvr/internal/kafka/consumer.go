package kafka

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"
	"log"
	"mua/gatesvr/config"
	"mua/gatesvr/internal/nacos"
	"time"

	"mua/gatesvr/pb"

	"github.com/golang/protobuf/proto"
	"github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/sasl/plain"
)

func init() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
}

type PlayerEvent struct {
	PlayerID string `json:"player_id"`
	GateID   string `json:"gate_id"`
	Event    string `json:"event"` // "online" or "offline"
}

type PlayerEventHandler func(event *pb.PlayerStatusChanged, gateOnline bool)

// StartPlayerEventConsumer 启动玩家事件消费者（带重试机制）
func StartPlayerEventConsumer(handler PlayerEventHandler) {
	cfg := config.GetConfig().Kafka
	log.Printf("---topic: %s, groupID: %s ---", cfg.Topic, cfg.GroupID)

	var tlsConfig *tls.Config
	if cfg.CaCert == "" {
		log.Println("Kafka CA证书路径为空，跳过TLS配置（本地开发模式，明文连接Kafka）")
		tlsConfig = nil
	} else {
		caCert, err := ioutil.ReadFile(cfg.CaCert)
		if err != nil {
			log.Fatalf("加载Kafka CA证书失败: %v", err)
		}
		caCertPool := x509.NewCertPool()
		caCertPool.AppendCertsFromPEM(caCert)
		tlsConfig = &tls.Config{
			RootCAs: caCertPool,
		}
	}

	// 构造Dialer
	dialer := &kafka.Dialer{
		Timeout: 10 * time.Second,
		TLS:     tlsConfig,
		SASLMechanism: plain.Mechanism{
			Username: cfg.Username,
			Password: cfg.Password,
		},
	}

	// 添加重试逻辑
	maxRetries := 3
	baseDelay := time.Second

	go func() {
		for attempt := 0; attempt < maxRetries; attempt++ {
			if err := consumeMessages(cfg, dialer, handler); err != nil {
				delay := baseDelay * time.Duration(1<<uint(attempt)) // 指数退避
				log.Printf("Kafka消费失败，%v后重试(%d/%d): %v", delay, attempt+1, maxRetries, err)
				time.Sleep(delay)
				continue
			}
			break
		}
	}()
}

// consumeMessages 消费Kafka消息的核心逻辑
func consumeMessages(cfg config.KafkaConfig, dialer *kafka.Dialer, handler PlayerEventHandler) error {
	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers:     cfg.Brokers,
		Topic:       cfg.Topic,
		GroupID:     cfg.GroupID,
		MinBytes:    1e3,               // 1KB
		MaxBytes:    10e6,              // 10MB
		StartOffset: kafka.FirstOffset, // 从最早的消息开始消费
		Dialer:      dialer,            // 关键：加上认证
	})

	defer r.Close()
	for {
		m, err := r.ReadMessage(context.Background())
		if err != nil {
			log.Printf("Kafka 读取消息失败: %v", err)
			return err // 返回错误触发重试
		}

		var evt pb.PlayerStatusChanged
		if err := proto.Unmarshal(m.Value, &evt); err != nil {
			log.Printf("Kafka pb消息解析失败: %v", err)
			continue
		}
		log.Printf("Kafka消息 offset=%d key=%s value=%s", m.Offset, string(m.Key), string(evt.String()))

		gateOnline := nacos.IsGatesvrInstanceOnline(evt.GatesvrId)

		handler(&evt, gateOnline)
	}
}
