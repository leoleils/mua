package kafka

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"
	"log"
	"time"
	"io"

	"github.com/segmentio/kafka-go"
	"github.com/golang/protobuf/proto"
	"mua/gatesvr/internal/pb"
	"mua/gatesvr/config"
)

const (
	KafkaTopicPlayerStatusChanged = "player_status_changed"
)

var (
	kafkaBrokers []string
	tlsConfig    *tls.Config
	localGateSvrID string
	localGateSvrIP string
)

// 初始化Kafka配置，需在配置加载后调用
func Init() {
	cfg := config.GetConfig().Kafka
	kafkaBrokers = cfg.Brokers
	if cfg.CaCert == "" {
		log.Fatalf("Kafka配置缺少caCert路径，请检查config.yaml")
	}
	var err error
	tlsConfig, err = newTLSConfig(cfg.CaCert)
	if err != nil {
		log.Fatalf("加载Kafka CA证书失败[%s]: %v", cfg.CaCert, err)
	}
}

// 加载CA证书
func newTLSConfig(caFile string) (*tls.Config, error) {
	caCert, err := ioutil.ReadFile(caFile)
	if err != nil {
		return nil, err
	}
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)
	return &tls.Config{
		RootCAs: caCertPool,
	}, nil
}

func SetLocalGateSvrID(id string) {
	localGateSvrID = id
}

func SetLocalGateSvrIP(ip string) {
	localGateSvrIP = ip
}

// 广播玩家上下线事件
func BroadcastPlayerStatusChanged(event *pb.PlayerStatusChanged) {
	if event.GatesvrId == "" {
		event.GatesvrId = localGateSvrID
	}
	if event.GatesvrIp == "" {
		event.GatesvrIp = localGateSvrIP
	}
	data, err := proto.Marshal(event)
	if err != nil {
		log.Printf("PlayerStatusChanged序列化失败: %v", err)
		return
	}
	cfg := config.GetConfig().Kafka
	writeKafka(cfg.Topic, string(data))
}

func writeKafka(topic, msg string) {
	w := &kafka.Writer{
		Addr:      kafka.TCP(kafkaBrokers...),
		Topic:     topic,
		Transport: &kafka.Transport{TLS: tlsConfig},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := w.WriteMessages(ctx, kafka.Message{Value: []byte(msg)})
	if err != nil {
		log.Printf("Kafka写入失败: %v", err)
	}
	w.Close()
}

// 订阅玩家上下线事件
func Subscribe(handler func(event *pb.PlayerStatusChanged)) {
	dialer := &kafka.Dialer{
		Timeout: 10 * time.Second,
		TLS:    tlsConfig,
	}
	cfg := config.GetConfig().Kafka
	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers: kafkaBrokers,
		Topic:   cfg.Topic,
		GroupID: "gatesvr-group",
		Dialer:  dialer,
	})
	go func() {
		for {
			m, err := r.ReadMessage(context.Background())
			if err != nil {
				if err == io.EOF {
					time.Sleep(100 * time.Millisecond)
					continue
				}
				log.Printf("Kafka读取失败: %v", err)
				continue
			}
			var event pb.PlayerStatusChanged
			if err := proto.Unmarshal(m.Value, &event); err != nil {
				log.Printf("PlayerStatusChanged反序列化失败: %v", err)
				continue
			}
			handler(&event)
		}
	}()
}

func split2(s string, sep byte) []string {
	idx := -1
	for i := 0; i < len(s); i++ {
		if s[i] == sep {
			idx = i
			break
		}
	}
	if idx == -1 {
		return []string{s}
	}
	return []string{s[:idx], s[idx+1:]}
} 