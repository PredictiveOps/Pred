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
	"strings"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/segmentio/kafka-go"
	"gorm.io/gorm"

	"event-processing-service/api"
	"event-processing-service/db"
)

// RawEvent is the minimal envelope extracted from each Kafka message.
// The full message body is stored verbatim as the payload.
type RawEvent struct {
	TenantID string `json:"tenant_id"`
}

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found")
	}

	brokers := getEnv("KAFKA_BROKERS", "localhost:9092")
	topic := getEnv("KAFKA_TOPIC", "events")
	groupID := getEnv("KAFKA_GROUP_ID", "event-processing-service")
	dbURL := getEnv("DATABASE_URL", "postgres://localhost:5432/events")
	httpPort := getEnv("HTTP_PORT", "8080")

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

	log.Printf("consuming topic %q from %s (group %q)", topic, brokers, groupID)

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

		if err := handleMessage(ctx, gdb, msg); err != nil {
			log.Printf("handle error (offset %d): %v", msg.Offset, err)
		}
	}
}

func handleMessage(ctx context.Context, gdb *gorm.DB, msg kafka.Message) error {
	var event RawEvent
	if err := json.Unmarshal(msg.Value, &event); err != nil {
		return fmt.Errorf("unmarshal: %w", err)
	}

	id, err := db.InsertEvent(ctx, gdb, event.TenantID, msg.Value)
	if err != nil {
		return fmt.Errorf("insert event: %w", err)
	}

	return process(ctx, gdb, id, event, msg.Value)
}

// process is a stub for downstream event processing logic.
// TODO: implement event processing.
func process(_ context.Context, _ *gorm.DB, id int64, event RawEvent, payload []byte) error {
	log.Printf("event stored (id %d, tenant %q, %d bytes)", id, event.TenantID, len(payload))
	return nil
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return fallback
}
