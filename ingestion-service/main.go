package main

import (
	"context"
	"fmt"
	"ingestion-service/config"
	"ingestion-service/db"
	"ingestion-service/handlers"
	"ingestion-service/router"
	"ingestion-service/services"
	"log"
)

func main() {
	config.LoadConfig()

	if config.DatabaseURL == "" {
		log.Fatal("DATABASE_URL is required")
	}

	gdb, err := db.Open(context.Background(), config.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}

	if gdb == nil {
		log.Fatal("Database connection not established")
	}

	// Migrate the schema
	if err := gdb.AutoMigrate(&db.Device{}); err != nil {
		log.Fatalf("Failed to migrate database schema: %v", err)
	}

	kafkaProducer := services.NewKafkaProducer(config.KafkaBrokers, config.KafkaTopic)
	handlers.SetKafkaProducer(kafkaProducer)
	defer func() {
		if err := kafkaProducer.Close(); err != nil {
			log.Printf("Failed to close Kafka producer: %v", err)
		}
	}()

	mqttClient, err := services.CreateMQTTClient(
		config.MQTTBroker,
		config.MQTTClientID,
		config.MQTTUsername,
		config.MQTTPassword,
		config.MQTTCACert,
	)
	if err != nil {
		log.Fatalf("Failed to create MQTT client: %v", err)
	}

	if err := services.ConnectMQTTClient(mqttClient); err != nil {
		log.Fatalf("MQTT connection failed: %v", err)
	}

	if err := services.SubscribeMQTTTopic(mqttClient, config.MQTTTopic, handlers.HandleMQTTMessage); err != nil {
		log.Fatalf("MQTT subscribe failed: %v", err)
	}
	defer services.DisconnectMQTTClient(mqttClient)

	r := router.NewRouter()
	log.Fatal(r.Run(fmt.Sprintf(":%s", config.Port)))
}
