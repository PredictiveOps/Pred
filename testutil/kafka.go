package testutil

import (
	"context"
	"time"

	"github.com/segmentio/kafka-go"
)

// CreateTestTopic creates a topic with a single partition on the given brokers.
// If the topic already exists the call is a no-op.
func CreateTestTopic(brokers []string, topic string) error {
	conn, err := kafka.Dial("tcp", brokers[0])
	if err != nil {
		return err
	}
	defer conn.Close()
	return conn.CreateTopics(kafka.TopicConfig{
		Topic:             topic,
		NumPartitions:     1,
		ReplicationFactor: 1,
	})
}

// ProduceMessage writes a single message to the given topic.
func ProduceMessage(brokers []string, topic, key string, value []byte) error {
	w := &kafka.Writer{
		Addr:                   kafka.TCP(brokers...),
		Topic:                  topic,
		AllowAutoTopicCreation: true,
	}
	defer w.Close()
	return w.WriteMessages(context.Background(), kafka.Message{
		Key:   []byte(key),
		Value: value,
	})
}

// ConsumeOne reads a single message from the topic and returns its value.
// It returns an error if no message arrives within timeout.
// Each call reads from the earliest available offset with no consumer group,
// so it is safe to call multiple times in the same test process without
// offset tracking side-effects.
func ConsumeOne(brokers []string, topic string, timeout time.Duration) ([]byte, error) {
	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers:     brokers,
		Topic:       topic,
		StartOffset: kafka.FirstOffset,
		MaxWait:     200 * time.Millisecond,
	})
	defer r.Close()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	msg, err := r.ReadMessage(ctx)
	if err != nil {
		return nil, err
	}
	return msg.Value, nil
}
