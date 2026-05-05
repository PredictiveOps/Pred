package handlers

import (
	"context"
	"encoding/json"
	"log"
	"strconv"

	mqtt "github.com/eclipse/paho.mqtt.golang"

	"ingestion-service/db"
	"ingestion-service/services"
)

var redisCache *services.RedisCache

type KafkaPublisher interface {
	Publish(ctx context.Context, key string, payload []byte) error
}

var kafkaProducer KafkaPublisher

func SetKafkaProducer(producer KafkaPublisher) {
	kafkaProducer = producer
}

func SetRedisCache(cache *services.RedisCache) {
	redisCache = cache
}

// HandleMQTTMessage is the single MQTT entrypoint.
// It parses topic once, then dispatches work based on the topic kind.
func HandleMQTTMessage(client mqtt.Client, msg mqtt.Message) {
	deviceID, topicKind, err := parseDeviceTopic(msg.Topic())
	if err != nil {
		log.Printf("invalid mqtt topic: %s: %v", msg.Topic(), err)
		return
	}

	log.Printf("mqtt message received: topic=%s kind=%s deviceID=%d payload_bytes=%d", msg.Topic(), topicKind, deviceID, len(msg.Payload()))

	switch topicKind {
	case mqttTopicRegistration:
		go HandleMQTTDeviceRegistrationWithTemplate(client, msg, registrationResponseTopicTemplate)
	case mqttTopicData:
		go handleMQTTDataMessage(deviceID, msg)
	default:
		log.Printf("unsupported mqtt topic kind: %s", topicKind)
	}
}

func handleMQTTDataMessage(deviceID uint, msg mqtt.Message) {
	fallbackPublicKey, ok := loadActiveDevicePublicKey(deviceID)
	if !ok {
		return
	}

	message, err := verifyDeviceData(deviceID, fallbackPublicKey, msg.Payload())
	if err != nil {
		log.Printf("failed to verify/process device data: device_id=%d err=%v", deviceID, err)
		return
	}

	// Publish to Kafka
	if kafkaProducer == nil {
		log.Printf("kafka producer is not initialized; skipping publish")
		return
	}

	var sensorData db.SensorDeviceData
	if err := json.Unmarshal(message.Data, &sensorData); err != nil {
		log.Printf("failed to unmarshal sensor data: device_id=%d err=%v", deviceID, err)
		return
	}

	deviceKey := strconv.FormatUint(uint64(deviceID), 10)
	kafkaPayload := prepareKafkaPayload(deviceID, message.Timestamp, sensorData)
	kafkaJSON, err := json.Marshal(kafkaPayload)
	if err != nil {
		log.Printf("failed to marshal kafka message: %v", err)
		return
	}
	if err := kafkaProducer.Publish(context.Background(), deviceKey, kafkaJSON); err != nil {
		log.Printf("failed to publish message to kafka: %v", err)
		return
	}

	log.Printf("published message to kafka with key=%s", deviceKey)
}

func loadActiveDevicePublicKey(deviceID uint) (*string, bool) {
	if redisCache != nil {
		isActive, publicKey, found, err := redisCache.GetDeviceState(context.Background(), deviceID)
		if err != nil {
			log.Printf("failed to read device state cache: device_id=%d err=%v", deviceID, err)
		} else if found {
			if !isActive {
				log.Printf("device is inactive: device_id=%d", deviceID)
				return nil, false
			}
			if publicKey == "" {
				log.Printf("device has no public key registered: device_id=%d", deviceID)
				return nil, false
			}

			cachedKey := publicKey
			return &cachedKey, true
		}
	}

	device, err := db.GetDeviceByID(deviceID)
	if err != nil {
		log.Printf("device not found: device_id=%d err=%v", deviceID, err)
		return nil, false
	}

	if !device.IsActive {
		log.Printf("device is inactive: device_id=%d", deviceID)
		return nil, false
	}

	if device.PublicKey == nil {
		log.Printf("device has no public key registered: device_id=%d", deviceID)
		return nil, false
	}

	if redisCache != nil {
		if err := redisCache.CacheDeviceState(context.Background(), deviceID, device.IsActive, *device.PublicKey); err != nil {
			log.Printf("failed to cache device state: device_id=%d err=%v", deviceID, err)
		}
	}

	return device.PublicKey, true
}
