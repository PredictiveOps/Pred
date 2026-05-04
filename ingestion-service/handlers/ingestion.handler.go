package handlers

import (
	"context"
	"log"
	"strings"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

type KafkaPublisher interface {
	Publish(ctx context.Context, key string, payload []byte) error
}

var kafkaProducer KafkaPublisher

func SetKafkaProducer(producer KafkaPublisher) {
	kafkaProducer = producer
}

// TODO: Need to check the deviceID with the database and only publish to Kafka if the device is registered.
// This will prevent unregistered devices from flooding Kafka with data.
func HandleMQTTMessage(_ mqtt.Client, msg mqtt.Message) {
	deviceID := extractDeviceIDFromTopic(msg.Topic())
	log.Printf("mqtt message received: topic=%s deviceID=%s payload_bytes=%d", msg.Topic(), deviceID, len(msg.Payload()))

	if kafkaProducer == nil {
		log.Printf("kafka producer is not initialized; skipping publish")
		return
	}
	if err := kafkaProducer.Publish(context.Background(), deviceID, msg.Payload()); err != nil {
		log.Printf("failed to publish message to kafka: %v", err)
		return
	}

	log.Printf("published message to kafka with key=%s", deviceID)
}

// sample topic: devices/{deviceID}/data
func extractDeviceIDFromTopic(topic string) string {
	parts := strings.Split(topic, "/")
	if len(parts) >= 2 && parts[0] == "devices" {
		return parts[1]
	}

	return topic
}
