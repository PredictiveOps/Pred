package processor

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/segmentio/kafka-go"
)

// FeatureSink delivers aggregated ML feature payloads to downstream consumers.
type FeatureSink interface {
	Send(ctx context.Context, payload MLRequest) error
}

// KafkaFeatureSink publishes MLRequest payloads to a Kafka topic.
type KafkaFeatureSink struct {
	writer *kafka.Writer
}

// NewKafkaFeatureSink creates a KafkaFeatureSink targeting the provided topic.
func NewKafkaFeatureSink(brokers []string, topic string) *KafkaFeatureSink {
	return &KafkaFeatureSink{
		writer: &kafka.Writer{
			Addr:  kafka.TCP(brokers...),
			Topic: topic,
		},
	}
}

// Send marshals the payload to JSON and writes it to Kafka.
func (s *KafkaFeatureSink) Send(ctx context.Context, payload MLRequest) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("kafka_sink: marshal payload: %w", err)
	}

	msg := kafka.Message{
		Key:   []byte(payload.DeviceID),
		Value: body,
	}

	if err := s.writer.WriteMessages(ctx, msg); err != nil {
		return fmt.Errorf("kafka_sink: write message: %w", err)
	}

	return nil
}

// Close releases the underlying Kafka writer resources.
func (s *KafkaFeatureSink) Close() error {
	return s.writer.Close()
}
