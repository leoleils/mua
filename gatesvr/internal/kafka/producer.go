package kafka

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"
	"log"
	"time"

	"mua/gatesvr/config"
	"mua/gatesvr/internal/pb"

	"github.com/golang/protobuf/proto"
	"github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/sasl/plain"
)

const (
	KafkaTopicPlayerStatusChanged = "player_status_changed"
)

var (
	kafkaBrokers   []string
	tlsConfig      *tls.Config
	localGateSvrID string
	localGateSvrIP string
)

// Init 初始化Kafka配置，需在配置加载后调用
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

// SetLocalGateSvrID 设置本地网关服务器ID
func SetLocalGateSvrID(id string) {
	localGateSvrID = id
}

// SetLocalGateSvrIP 设置本地网关服务器IP
func SetLocalGateSvrIP(ip string) {
	localGateSvrIP = ip
}

// BroadcastPlayerStatusChanged 广播玩家状态变更事件
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
	cfg := config.GetConfig().Kafka
	dialer := &kafka.Dialer{
		Timeout: 10 * time.Second,
		TLS:     tlsConfig,
		SASLMechanism: plain.Mechanism{
			Username: cfg.Username,
			Password: cfg.Password,
		},
	}
	w := &kafka.Writer{
		Addr:      kafka.TCP(kafkaBrokers...),
		Topic:     topic,
		Transport: &kafka.Transport{TLS: tlsConfig, SASL: dialer.SASLMechanism},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := w.WriteMessages(ctx, kafka.Message{Value: []byte(msg)})
	if err != nil {
		log.Printf("Kafka写入失败: %v", err)
	}
	w.Close()
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
