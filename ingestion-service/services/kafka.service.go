package services

import (
	"context"
	"strings"
	"time"

	"github.com/segmentio/kafka-go"
)

type KafkaProducer struct {
	writer *kafka.Writer
	topic  string
}

func NewKafkaProducer(brokersCSV, topic string) *KafkaProducer {
	brokers := strings.Split(brokersCSV, ",")
	w := &kafka.Writer{
		Addr:         kafka.TCP(brokers...),
		Topic:        topic,
		Balancer:     &kafka.LeastBytes{},
		RequiredAcks: kafka.RequireOne,
		Async:        true,
	}

	return &KafkaProducer{
		writer: w,
		topic:  topic,
	}
}

func (p *KafkaProducer) Publish(ctx context.Context, key string, payload []byte) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	return p.writer.WriteMessages(ctx, kafka.Message{
		Key:   []byte(key),
		Value: payload,
		Time:  time.Now(),
	})
}

func (p *KafkaProducer) Close() error {
	return p.writer.Close()
}
