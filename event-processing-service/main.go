package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/segmentio/kafka-go"
	"gorm.io/gorm"

	"event-processing-service/api"
	"event-processing-service/db"
	"event-processing-service/processor"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found")
	}

	brokers := getEnv("KAFKA_BROKERS", "localhost:9092")
	topic := getEnv("KAFKA_TOPIC", "events")
	groupID := getEnv("KAFKA_GROUP_ID", "event-processing-service")
	dbURL := getEnv("DATABASE_URL", "postgres://localhost:5432/events")
	httpPort := getEnv("HTTP_PORT", "8082")
	mlTopic := getEnv("ML_FEATURES_TOPIC", "ml-features")
	aggregationIntervalSecs := getEnvInt("AGGREGATION_INTERVAL_SECONDS", getEnvInt("WINDOW_DURATION_SECONDS", 5))
	aggregationBatchSize := getEnvInt("AGGREGATION_BATCH_SIZE", 500)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	gdb, err := db.Open(ctx, dbURL)
	if err != nil {
		log.Fatalf("database init failed: %v", err)
	}
	if sqlDB, err := gdb.DB(); err == nil {
		defer sqlDB.Close()
	}
	log.Println("connected to database")

	mlSink := processor.NewKafkaFeatureSink(strings.Split(brokers, ","), mlTopic)
	defer func() {
		if err := mlSink.Close(); err != nil {
			log.Printf("ml sink close error: %v", err)
		}
	}()
	aggregationInterval := time.Duration(aggregationIntervalSecs) * time.Second
	aggregator := processor.NewDBAggregator(gdb, mlSink, aggregationInterval, aggregationBatchSize)
	go aggregator.Run(ctx)

	server := &http.Server{
		Addr:    ":" + httpPort,
		Handler: api.NewRouter(gdb),
	}
	go func() {
		log.Printf("http listening on :%s", httpPort)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("http server error: %v", err)
		}
	}()
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Printf("http shutdown error: %v", err)
		}
	}()

	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers: strings.Split(brokers, ","),
		Topic:   topic,
		GroupID: groupID,
	})
	defer reader.Close()

	log.Printf("consuming topic %q from %s (group %q), aggregating DB rows every %s in batches of %d, publishing features to %q", topic, brokers, groupID, aggregationInterval, aggregationBatchSize, mlTopic)

	for {
		msg, err := reader.ReadMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				log.Println("shutting down")
				return
			}
			log.Printf("read error: %v", err)
			continue
		}

		log.Printf("[kafka] received message topic=%q partition=%d offset=%d key=%q len=%d", msg.Topic, msg.Partition, msg.Offset, msg.Key, len(msg.Value))
		if err := handleMessage(ctx, gdb, msg); err != nil {
			log.Printf("[kafka] handle error topic=%q partition=%d offset=%d: %v", msg.Topic, msg.Partition, msg.Offset, err)
		}
	}
}

func handleMessage(ctx context.Context, gdb *gorm.DB, msg kafka.Message) error {
	var event processor.SensorEvent
	if err := json.Unmarshal(msg.Value, &event); err != nil {
		return fmt.Errorf("unmarshal: %w", err)
	}
	log.Printf("[event] parsed device=%d tenant=%q offset=%d", event.DeviceID, event.TenantID, msg.Offset)

	id, err := db.InsertEvent(ctx, gdb, event.TenantID, msg.Value)
	if err != nil {
		return fmt.Errorf("insert event: %w", err)
	}
	log.Printf("[db] event stored id=%d device=%d tenant=%q", id, event.DeviceID, event.TenantID)
	return nil
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v, ok := os.LookupEnv(key); ok {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}
