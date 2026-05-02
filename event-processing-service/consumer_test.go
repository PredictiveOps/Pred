package main

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/segmentio/kafka-go"
	"gorm.io/gorm"

	"event-processing-service/db"
	"event-processing-service/internal/app"
)

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

	body := map[string]any{
		"tenant_id": "t-events",
		"source":    "sensor-1",
		"value":     42,
	}

	svc := app.NewService(gdb)
	if err := handleMessage(ctx, svc, makeMessage(t, body)); err != nil {
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

func TestHandleMessage_InvalidJSON(t *testing.T) {
	gdb := openTestDB(t)
	ctx := context.Background()

	msg := kafka.Message{Value: []byte(`not valid json`)}
	svc := app.NewService(gdb)
	err := handleMessage(ctx, svc, msg)
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}
