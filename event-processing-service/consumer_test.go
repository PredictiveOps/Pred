package main

import (
	"context"
	"encoding/json"
	"os"
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
