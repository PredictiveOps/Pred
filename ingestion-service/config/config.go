package config

import (
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
)

var Port, DatabaseURL, KafkaBrokers, KafkaTopic, MQTTBroker, MQTTClientID, MQTTTopic, MQTTUsername, MQTTPassword string

func LoadConfig() {
	err := godotenv.Load(".env")
	if err != nil {
		log.Fatalf("Error loading .env file: %v", err)
	}

	Port = os.Getenv("PORT")
	DatabaseURL = os.Getenv("DATABASE_URL")
	KafkaBrokers = os.Getenv("KAFKA_BROKERS")
	KafkaTopic = os.Getenv("KAFKA_TOPIC")
	MQTTBroker = os.Getenv("MQTT_BROKER")
	MQTTClientID = os.Getenv("MQTT_CLIENT_ID")
	MQTTTopic = os.Getenv("MQTT_TOPIC")
	MQTTUsername = os.Getenv("MQTT_USERNAME")
	MQTTPassword = os.Getenv("MQTT_PASSWORD")

	fmt.Printf("Configuration loaded: PORT=%s\n", Port)
}
