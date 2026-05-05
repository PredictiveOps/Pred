package handlers

import (
	"context"
	"crypto/ecdsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"ingestion-service/db"
	"log"
	"strconv"
	"strings"
)

const (
	mqttTopicData         = "data"
	mqttTopicRegistration = "registration"
)

// parseDeviceTopic parses device MQTT topic in one place.
// Supported topics:
// - devices/{deviceID}/data
// - devices/{deviceID}/registration
func parseDeviceTopic(topic string) (uint, string, error) {
	parts := strings.Split(topic, "/")
	if len(parts) != 3 || parts[0] != "devices" {
		return 0, "", fmt.Errorf("expected devices/{deviceID}/{kind}")
	}

	deviceID64, err := strconv.ParseUint(parts[1], 10, 32)
	if err != nil {
		return 0, "", fmt.Errorf("invalid device id: %w", err)
	}

	switch parts[2] {
	case mqttTopicData, mqttTopicRegistration:
		return uint(deviceID64), parts[2], nil
	default:
		return 0, "", fmt.Errorf("unsupported topic kind: %s", parts[2])
	}
}

func verifySignature(pub *ecdsa.PublicKey, payload []byte, sig []byte) bool { // Placeholder for signature verification logic.
	hash := sha256.Sum256(payload)

	return ecdsa.VerifyASN1(pub, hash[:], sig)
}

func ParsePublicKey(pemStr string) (*ecdsa.PublicKey, error) {
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return nil, fmt.Errorf("invalid PEM public key")
	}

	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}

	ecdsaPub, ok := pub.(*ecdsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("public key is not ECDSA")
	}
	return ecdsaPub, nil
}

func verifyDeviceData(deviceID uint, fallbackPublicKey *string, payload []byte) (*db.MQTTPayload, error) {
	message, err := verifyRawDataStructure(payload)
	if err != nil {
		return nil, err
	}

	publicKeyPEM, err := resolveDevicePublicKey(deviceID, fallbackPublicKey)
	if err != nil {
		return nil, err
	}

	publicKey, err := ParsePublicKey(publicKeyPEM)
	if err != nil {
		return nil, fmt.Errorf("failed to parse public key: %w", err)
	}

	if redisCache != nil {
		exists, err := redisCache.NonceExists(context.Background(), deviceID, message.Nonce)
		if err != nil {
			return nil, fmt.Errorf("nonce check failed: %w", err)
		}
		if exists {
			return nil, fmt.Errorf("replayed nonce")
		}
	}

	signatureBytes, err := base64.StdEncoding.DecodeString(message.Signature)
	if err != nil {
		return nil, fmt.Errorf("invalid signature encoding: %w", err)
	}

	// Signature is computed over the exact bytes of the 'data' field
	if !verifySignature(publicKey, message.Data, signatureBytes) {
		return nil, fmt.Errorf("signature verification failed")
	}

	if redisCache != nil {
		marked, err := redisCache.MarkNonceUsed(context.Background(), deviceID, message.Nonce)
		if err != nil {
			return nil, fmt.Errorf("failed to mark nonce used: %w", err)
		}
		if !marked {
			return nil, fmt.Errorf("replayed nonce")
		}
	}

	log.Printf("verified signed payload for device_id=%d payload_bytes=%d", deviceID, len(message.Data))
	return message, nil
}

func resolveDevicePublicKey(deviceID uint, fallbackPublicKey *string) (string, error) {
	if redisCache != nil {
		cachedKey, err := redisCache.GetDevicePublicKey(context.Background(), deviceID)
		if err == nil && cachedKey != "" {
			return cachedKey, nil
		}
	}

	if fallbackPublicKey == nil || *fallbackPublicKey == "" {
		return "", fmt.Errorf("device public key not found")
	}

	key := *fallbackPublicKey
	if redisCache != nil {
		if err := redisCache.CacheDevicePublicKey(context.Background(), deviceID, key); err != nil {
			log.Printf("failed to cache public key for device_id=%d: %v", deviceID, err)
		}
	}

	return key, nil
}

func prepareKafkaPayload(deviceID uint, timestamp int64, data db.SensorDeviceData) *db.KafkaPayload {
	return &db.KafkaPayload{
		DeviceID:  deviceID,
		Timestamp: timestamp,
		Mode:      data.Mode,
		VRMS:      data.VRMS,
		TempC:     data.TempC,
		PeakHz1:   data.PeakHz1,
		PeakHz2:   data.PeakHz2,
		PeakHz3:   data.PeakHz3,
		Status:    data.Status,
	}
}

func verifyRawDataStructure(payload []byte) (*db.MQTTPayload, error) {
	var message db.MQTTPayload

	if err := json.Unmarshal(payload, &message); err != nil {
		return nil, fmt.Errorf("invalid signed payload: %w", err)
	}
	if len(message.Data) == 0 {
		return nil, fmt.Errorf("data field is required")
	}
	if message.Nonce == "" {
		return nil, fmt.Errorf("nonce is required")
	}
	if message.Signature == "" {
		return nil, fmt.Errorf("signature is required")
	}
	if message.Timestamp == 0 {
		return nil, fmt.Errorf("timestamp is required")
	}

	return &message, nil
}
