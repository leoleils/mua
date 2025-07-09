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

	"mua/gatesvr/internal/pb"

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

func StartPlayerEventConsumer(brokers []string, topic string, groupID string, handler PlayerEventHandler) {
	log.Printf("---topic: %s, groupID: %s ---", topic, groupID)

	// 1. 读取配置
	cfg := config.GetConfig().Kafka

	// 2. 加载CA证书
	caCert, err := ioutil.ReadFile(cfg.CaCert)
	if err != nil {
		log.Fatalf("加载Kafka CA证书失败: %v", err)
	}
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)
	tlsConfig := &tls.Config{
		RootCAs: caCertPool,
	}

	// 3. 构造Dialer
	dialer := &kafka.Dialer{
		Timeout: 10 * time.Second,
		TLS:     tlsConfig,
		SASLMechanism: plain.Mechanism{
			Username: cfg.Username,
			Password: cfg.Password,
		},
	}

	go func() {
		r := kafka.NewReader(kafka.ReaderConfig{
			Brokers:     brokers,
			Topic:       topic,
			GroupID:     groupID,
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
				continue
			}
			log.Printf("Kafka消息 offset=%d key=%s value=%s", m.Offset, string(m.Key), string(m.Value))

			var evt pb.PlayerStatusChanged
			if err := proto.Unmarshal(m.Value, &evt); err != nil {
				log.Printf("Kafka pb消息解析失败: %v", err)
				continue
			}

			gateOnline := nacos.IsGatesvrInstanceOnline(evt.GatesvrId)

			handler(&evt, gateOnline)
		}
	}()
}
