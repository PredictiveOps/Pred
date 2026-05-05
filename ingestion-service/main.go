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
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

func main() {
	config.LoadConfig()

	if err := config.Validate(); err != nil {
		log.Fatalf("configuration invalid: %v", err)
	}

	log.Printf("Connecting to database...")
	gdb, err := db.Open(context.Background(), config.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	log.Printf("Database connected successfully")

	// Migrate the schema
	if err := gdb.AutoMigrate(&db.Device{}); err != nil {
		log.Fatalf("Failed to migrate database schema: %v", err)
	}

	log.Printf("Initializing Kafka producer...")
	kafkaProducer := services.NewKafkaProducer(config.KafkaBrokers, config.KafkaTopic)
	handlers.SetKafkaProducer(kafkaProducer)
	log.Printf("Kafka producer initialized")

	pubKeyTTL, err := time.ParseDuration(config.RedisPubKeyTTL)
	if err != nil {
		log.Fatalf("Invalid REDIS_PUBKEY_TTL: %v", err)
	}
	nonceTTL, err := time.ParseDuration(config.RedisNonceTTL)
	if err != nil {
		log.Fatalf("Invalid REDIS_NONCE_TTL: %v", err)
	}

	log.Printf("Initializing Redis cache...")
	redisCache, err := services.NewRedisCache(
		config.RedisAddr,
		config.RedisPassword,
		config.RedisDB,
		pubKeyTTL,
		nonceTTL,
	)
	if err != nil {
		log.Fatalf("Failed to initialize Redis cache: %v", err)
	}
	handlers.SetRedisCache(redisCache)
	log.Printf("Redis cache initialized")

	// Start HTTP server as early as possible so health checks can succeed even
	// while MQTT initialization is still connecting or retrying.
	r := router.NewRouter(gdb)
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%s", config.Port),
		Handler: r,
	}

	go func() {
		log.Printf("HTTP server listening on %s", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server ListenAndServe: %v", err)
		}
	}()

	log.Printf("Creating MQTT client...")
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
	log.Printf("MQTT client created")

	log.Printf("Connecting to MQTT broker...")
	if err := services.ConnectMQTTClient(mqttClient); err != nil {
		log.Fatalf("MQTT connection failed: %v", err)
	}
	log.Printf("MQTT connected successfully")

	handlers.SetRegistrationResponseTopicTemplate(config.MQTTDeviceRegistrationResponseTopic)

	// Subscribe to configured data topic
	if config.MQTTTopic != "" {
		if err := services.SubscribeMQTTTopic(mqttClient, config.MQTTTopic, handlers.HandleMQTTMessage); err != nil {
			log.Fatalf("MQTT subscribe to data topic failed: %v", err)
		}
	} else {
		log.Printf("MQTT data topic not configured; skipping subscription")
	}

	// Subscribe to registration topic with a wrapper that supplies the response template
	if config.MQTTDeviceRegistrationTopic != "" {
		regHandler := func(c mqtt.Client, m mqtt.Message) {
			handlers.HandleMQTTDeviceRegistrationWithTemplate(c, m, config.MQTTDeviceRegistrationResponseTopic)
		}
		if err := services.SubscribeMQTTTopic(mqttClient, config.MQTTDeviceRegistrationTopic, regHandler); err != nil {
			log.Fatalf("MQTT subscribe to registration topic failed: %v", err)
		}
	} else {
		log.Printf("MQTT registration topic not configured; skipping subscription")
	}

	// wait for termination signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Printf("shutdown signal received, stopping service")

	// give components up to 10s to shut down
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Shutdown HTTP server
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("HTTP server shutdown error: %v", err)
	}

	// Disconnect MQTT
	services.DisconnectMQTTClient(mqttClient)

	// Close Kafka producer
	if err := kafkaProducer.Close(); err != nil {
		log.Printf("Failed to close Kafka producer: %v", err)
	}

	// Close Redis
	if err := redisCache.Close(); err != nil {
		log.Printf("Failed to close Redis cache: %v", err)
	}

	// Close DB connection
	if sqlDB, err := gdb.DB(); err == nil {
		sqlDB.Close()
	}

	log.Printf("service stopped")
}
