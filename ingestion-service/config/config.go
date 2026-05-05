package config

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

var Port, DatabaseURL, KafkaBrokers, KafkaTopic, MQTTBroker, MQTTClientID, MQTTTopic, MQTTDeviceRegistrationTopic, MQTTDeviceRegistrationResponseTopic, MQTTUsername, MQTTPassword, MQTTCACert string
var RedisAddr, RedisPassword, RedisPubKeyTTL, RedisNonceTTL string
var RedisDB int

func LoadConfig() {
	// Load .env if present; do not fail when it's missing (allow env-based configs)
	if err := godotenv.Load(".env"); err != nil {
		log.Printf(".env file not loaded (proceeding to env vars): %v", err)
	}

	Port = os.Getenv("PORT")
	if Port == "" {
		Port = "2500"
	}
	DatabaseURL = os.Getenv("DATABASE_URL")
	KafkaBrokers = os.Getenv("KAFKA_BROKERS")
	KafkaTopic = os.Getenv("KAFKA_TOPIC")
	MQTTBroker = os.Getenv("MQTT_BROKER")
	MQTTClientID = os.Getenv("MQTT_CLIENT_ID")
	MQTTTopic = os.Getenv("MQTT_TOPIC")
	MQTTDeviceRegistrationTopic = os.Getenv("MQTT_DEVICE_REGISTRATION_TOPIC")
	MQTTDeviceRegistrationResponseTopic = os.Getenv("MQTT_DEVICE_REGISTRATION_RESPONSE_TOPIC")
	MQTTUsername = os.Getenv("MQTT_USERNAME")
	MQTTPassword = os.Getenv("MQTT_PASSWORD")
	MQTTCACert = os.Getenv("MQTT_CA_CERT")
	RedisAddr = getEnv("REDIS_ADDR", "localhost:6379")
	RedisPassword = os.Getenv("REDIS_PASSWORD")
	RedisPubKeyTTL = getEnv("REDIS_PUBKEY_TTL", "30m")
	RedisNonceTTL = getEnv("REDIS_NONCE_TTL", "60s")

	redisDB, err := strconv.Atoi(getEnv("REDIS_DB", "0"))
	if err != nil {
		log.Printf("invalid REDIS_DB value, using default 0: %v", err)
		redisDB = 0
	}
	RedisDB = redisDB

	fmt.Printf("Configuration loaded: PORT=%s\n", Port)
}

// Validate returns an error describing missing required configuration values.
func Validate() error {
	missing := []string{}
	if DatabaseURL == "" {
		missing = append(missing, "DATABASE_URL")
	}
	if KafkaBrokers == "" {
		missing = append(missing, "KAFKA_BROKERS")
	}
	if KafkaTopic == "" {
		missing = append(missing, "KAFKA_TOPIC")
	}
	if MQTTClientID == "" {
		missing = append(missing, "MQTT_CLIENT_ID")
	}
	if MQTTTopic == "" {
		missing = append(missing, "MQTT_TOPIC")
	}
	if MQTTDeviceRegistrationTopic == "" {
		missing = append(missing, "MQTT_DEVICE_REGISTRATION_TOPIC")
	}
	if MQTTDeviceRegistrationResponseTopic == "" {
		missing = append(missing, "MQTT_DEVICE_REGISTRATION_RESPONSE_TOPIC")
	}

	if len(missing) == 0 {
		return nil
	}

	return fmt.Errorf("missing required configuration: %s", strings.Join(missing, ", "))
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}

	return fallback
}
