package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/segmentio/kafka-go"
	"gorm.io/gorm"

	"event-processing-service/db"
	"event-processing-service/processor"
)

// noopWindowManager returns a WindowManager whose flush callback does nothing.
// Used in integration tests that only care about DB insertion, not ML forwarding.
func noopWindowManager() *processor.WindowManager {
	return processor.NewWindowManager(5*time.Second, func(_, _ string, _ []processor.SensorEvent) {})
}

func openTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping consumer integration tests")
	}
	ctx := context.Background()
	gdb, err := db.Open(ctx, url)
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() {
		if sqlDB, err := gdb.DB(); err == nil {
			sqlDB.Close()
		}
	})
	return gdb
}

func makeMessage(t *testing.T, v any) kafka.Message {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal message: %v", err)
	}
	return kafka.Message{Value: b}
}

func TestHandleMessage_InsertsEvent(t *testing.T) {
	gdb := openTestDB(t)
	ctx := context.Background()
	wm := noopWindowManager()
	defer wm.Stop()

	body := map[string]any{
		"tenant_id": "t-events",
		"device_id": "MTR-01",
		"v_rms":     0.45,
		"temp_c":    52.3,
		"peak_hz_1": 120,
		"peak_hz_2": 240,
		"peak_hz_3": 450,
		"status":    "nominal",
	}

	if err := handleMessage(ctx, gdb, wm, makeMessage(t, body)); err != nil {
		t.Fatalf("handleMessage: %v", err)
	}

	var event db.Event
	if err := gdb.Where("tenant_id = ?", "t-events").Last(&event).Error; err != nil {
		t.Fatalf("fetch event: %v", err)
	}

	if len(event.Payload) == 0 {
		t.Fatal("payload was empty")
	}
}

// TestCrossService_IngestionToEventProcessing verifies the ingestion → event-processing
// boundary: bytes in the format ingestion publishes are written to the real test Kafka
// topic, read back as a kafka.Message, and fed to handleMessage, which must persist
// the event row to the events_test DB.
func TestCrossService_IngestionToEventProcessing(t *testing.T) {
	brokersCSV := os.Getenv("TEST_KAFKA_BROKERS")
	if brokersCSV == "" {
		t.Skip("TEST_KAFKA_BROKERS not set; skipping cross-service integration test")
	}
	gdb := openTestDB(t)
	ctx := context.Background()

	// Use a unique topic per run so parallel test runs don't interfere.
	topic := fmt.Sprintf("cross-service-test-%d", time.Now().UnixNano())
	brokers := strings.Split(brokersCSV, ",")

	event := processor.SensorEvent{
		TenantID: "t-cross",
		DeviceID: "MTR-X1",
		VRMS:     1.23,
		TempC:    45.6,
		PeakHz1:  60,
		PeakHz2:  120,
		PeakHz3:  240,
		Status:   "nominal",
	}
	payload, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("marshal sensor event: %v", err)
	}

	conn, err := kafka.Dial("tcp", brokers[0])
	if err != nil {
		t.Fatalf("kafka dial: %v", err)
	}
	if err := conn.CreateTopics(kafka.TopicConfig{
		Topic:             topic,
		NumPartitions:     1,
		ReplicationFactor: 1,
	}); err != nil {
		t.Fatalf("create topic: %v", err)
	}
	conn.Close()

	writer := &kafka.Writer{
		Addr:  kafka.TCP(brokers...),
		Topic: topic,
	}
	defer writer.Close()

	writeCtx, wCancel := context.WithTimeout(ctx, 10*time.Second)
	defer wCancel()
	if err := writer.WriteMessages(writeCtx, kafka.Message{Value: payload}); err != nil {
		t.Fatalf("write to kafka: %v", err)
	}

	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:   brokers,
		Topic:     topic,
		Partition: 0,
	})
	defer reader.Close()

	readCtx, rCancel := context.WithTimeout(ctx, 10*time.Second)
	defer rCancel()
	msg, err := reader.ReadMessage(readCtx)
	if err != nil {
		t.Fatalf("read from kafka: %v", err)
	}

	wm := noopWindowManager()
	defer wm.Stop()

	if err := handleMessage(ctx, gdb, wm, msg); err != nil {
		t.Fatalf("handleMessage: %v", err)
	}

	var stored db.Event
	if err := gdb.Where("tenant_id = ?", "t-cross").Last(&stored).Error; err != nil {
		t.Fatalf("fetch event from DB: %v", err)
	}
	if len(stored.Payload) == 0 {
		t.Fatal("stored payload was empty")
	}
}

func TestHandleMessage_InvalidJSON(t *testing.T) {
	gdb := openTestDB(t)
	ctx := context.Background()
	wm := noopWindowManager()
	defer wm.Stop()

	msg := kafka.Message{Value: []byte(`not valid json`)}
	err := handleMessage(ctx, gdb, wm, msg)
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}
