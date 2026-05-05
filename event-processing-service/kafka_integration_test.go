package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/segmentio/kafka-go"

	"event-processing-service/db"
	"event-processing-service/processor"
)

func TestIntegration_KafkaToDBToML(t *testing.T) {
	ctx := context.Background()
	brokers := strings.Split(getEnv("KAFKA_BROKERS", "localhost:9092"), ",")
	eventsTopic := getEnv("KAFKA_TOPIC", "events")
	mlTopic := getEnv("ML_FEATURES_TOPIC", "ml-features")
	dbURL := getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5433/events?sslmode=disable")

	gdb, err := db.Open(ctx, dbURL)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if sqlDB, err := gdb.DB(); err == nil {
		defer sqlDB.Close()
	}

	tenantID := fmt.Sprintf("it-%d", time.Now().UnixNano())
	startTime := time.Now().UTC()

	mlReader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:     brokers,
		Topic:       mlTopic,
		StartOffset: kafka.LastOffset,
	})
	defer mlReader.Close()

	writer := &kafka.Writer{
		Addr:  kafka.TCP(brokers...),
		Topic: eventsTopic,
	}
	defer writer.Close()

	events := []processor.SensorEvent{
		{TenantID: tenantID, DeviceID: "MTR-01", VRMS: 0.38, TempC: 46, PeakHz1: 110, PeakHz2: 195, PeakHz3: 390, Status: "nominal"},
		{TenantID: tenantID, DeviceID: "MTR-02", VRMS: 0.62, TempC: 62, PeakHz1: 130, PeakHz2: 210, PeakHz3: 430, Status: "warning"},
		{TenantID: tenantID, DeviceID: "MTR-03", VRMS: 0.85, TempC: 78, PeakHz1: 145, PeakHz2: 225, PeakHz3: 460, Status: "critical"},
		{TenantID: tenantID, DeviceID: "PMP-01", VRMS: 0.41, TempC: 48, PeakHz1: 115, PeakHz2: 205, PeakHz3: 395, Status: "nominal"},
		{TenantID: tenantID, DeviceID: "PMP-02", VRMS: 0.58, TempC: 60, PeakHz1: 125, PeakHz2: 215, PeakHz3: 420, Status: "warning"},
		{TenantID: tenantID, DeviceID: "PMP-03", VRMS: 0.92, TempC: 82, PeakHz1: 155, PeakHz2: 235, PeakHz3: 475, Status: "critical"},
		{TenantID: tenantID, DeviceID: "FAN-01", VRMS: 0.33, TempC: 42, PeakHz1: 100, PeakHz2: 180, PeakHz3: 360, Status: "nominal"},
		{TenantID: tenantID, DeviceID: "FAN-02", VRMS: 0.55, TempC: 58, PeakHz1: 120, PeakHz2: 205, PeakHz3: 410, Status: "warning"},
		{TenantID: tenantID, DeviceID: "CMP-01", VRMS: 0.47, TempC: 52, PeakHz1: 118, PeakHz2: 198, PeakHz3: 402, Status: "nominal"},
		{TenantID: tenantID, DeviceID: "CMP-02", VRMS: 0.88, TempC: 80, PeakHz1: 150, PeakHz2: 230, PeakHz3: 470, Status: "critical"},
	}

	messages := make([]kafka.Message, 0, len(events))
	for _, event := range events {
		body, err := json.Marshal(event)
		if err != nil {
			t.Fatalf("marshal event: %v", err)
		}
		messages = append(messages, kafka.Message{Value: body})
	}

	if err := writer.WriteMessages(ctx, messages...); err != nil {
		t.Fatalf("write kafka events: %v", err)
	}

	waitFor(t, 15*time.Second, 250*time.Millisecond, func() (bool, error) {
		var count int64
		err := gdb.Model(&db.Event{}).
			Where("tenant_id = ? AND created_at >= ?", tenantID, startTime).
			Count(&count).Error
		if err != nil {
			return false, err
		}
		return count >= int64(len(events)), nil
	})

	expected := map[string]struct{}{}
	for _, event := range events {
		expected[event.DeviceID] = struct{}{}
	}

	found := map[string]bool{}
	deadline := time.Now().Add(20 * time.Second)
	for len(found) < len(expected) {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			missing := make([]string, 0, len(expected)-len(found))
			for deviceID := range expected {
				if !found[deviceID] {
					missing = append(missing, deviceID)
				}
			}
			t.Fatalf("timed out waiting for ML messages, missing=%v", missing)
		}

		readCtx, cancel := context.WithTimeout(ctx, remaining)
		msg, err := mlReader.ReadMessage(readCtx)
		cancel()
		if err != nil {
			t.Fatalf("read ml topic: %v", err)
		}

		var payload processor.MLRequest
		if err := json.Unmarshal(msg.Value, &payload); err != nil {
			t.Fatalf("unmarshal ml payload: %v", err)
		}

		if payload.TenantID != tenantID {
			continue
		}
		if _, ok := expected[payload.DeviceID]; ok {
			found[payload.DeviceID] = true
		}
	}

	waitFor(t, 15*time.Second, 250*time.Millisecond, func() (bool, error) {
		var count int64
		err := gdb.Model(&db.Event{}).
			Where("tenant_id = ? AND created_at >= ? AND processed_at IS NOT NULL", tenantID, startTime).
			Count(&count).Error
		if err != nil {
			return false, err
		}
		return count >= int64(len(events)), nil
	})
}

func waitFor(t *testing.T, timeout, interval time.Duration, cond func() (bool, error)) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		ok, err := cond()
		if err != nil {
			t.Fatalf("wait condition error: %v", err)
		}
		if ok {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out after %s", timeout)
		}
		time.Sleep(interval)
	}
}
