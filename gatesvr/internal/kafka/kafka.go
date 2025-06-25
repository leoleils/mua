package kafka

import (
	"context"
	"log"
	"time"

	"github.com/segmentio/kafka-go"
)

const (
	KafkaTopicOnline  = "player_online"
	KafkaTopicOffline = "player_offline"
)

var (
	kafkaBrokers = []string{"localhost:9092"} // 可配置
)

// 广播玩家上线
func BroadcastOnline(playerID, gatesvrID string) {
	msg := playerID + "," + gatesvrID
	writeKafka(KafkaTopicOnline, msg)
}

// 广播玩家下线
func BroadcastOffline(playerID, gatesvrID string) {
	msg := playerID + "," + gatesvrID
	writeKafka(KafkaTopicOffline, msg)
}

func writeKafka(topic, msg string) {
	w := kafka.Writer{
		Addr:     kafka.TCP(kafkaBrokers...),
		Topic:    topic,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := w.WriteMessages(ctx, kafka.Message{Value: []byte(msg)})
	if err != nil {
		log.Printf("Kafka写入失败: %v", err)
	}
	w.Close()
}

// 订阅玩家上下线
func Subscribe(topic string, handler func(playerID, gatesvrID string)) {
	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers: kafkaBrokers,
		Topic:   topic,
		GroupID: "gatesvr-group",
	})
	go func() {
		for {
			m, err := r.ReadMessage(context.Background())
			if err != nil {
				log.Printf("Kafka读取失败: %v", err)
				continue
			}
			parts := string(m.Value)
			arr := make([]string, 2)
			copy(arr, split2(parts, ','))
			if len(arr) == 2 {
				handler(arr[0], arr[1])
			}
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