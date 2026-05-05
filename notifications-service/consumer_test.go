package main

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/segmentio/kafka-go"
	"gorm.io/gorm"

	"notifications-service/db"
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

func TestHandleMessage_Email(t *testing.T) {
	gdb := openTestDB(t)
	ctx := context.Background()

	event := AlertEvent{
		TenantID: "t-email",
		Type:     "email",
		Payload:  json.RawMessage(`{"subject":"Test","body":"Hello"}`),
		Recipients: []Recipient{
			{UserID: "u1", Email: "u1@example.com"},
			{UserID: "u2", Email: "u2@example.com"},
		},
	}

	if err := handleMessage(ctx, gdb, nil, makeMessage(t, event)); err != nil {
		t.Fatalf("handleMessage: %v", err)
	}

	var notif db.Notification
	if err := gdb.Where("tenant_id = ? AND type = ?", "t-email", "email").Last(&notif).Error; err != nil {
		t.Fatalf("fetch notification: %v", err)
	}

	var deliveries []db.NotificationDelivery
	gdb.Where("notification_id = ?", notif.ID).Find(&deliveries)
	if len(deliveries) != 2 {
		t.Fatalf("delivery count: got %d, want 2", len(deliveries))
	}
	for _, d := range deliveries {
		if d.Status != "delivered" {
			t.Errorf("delivery %d status: got %q, want %q", d.ID, d.Status, "delivered")
		}
	}
}

func TestHandleMessage_Push(t *testing.T) {
	gdb := openTestDB(t)
	ctx := context.Background()

	// Seed device tokens.
	tokens := []db.DeviceToken{
		{TenantID: "t-push", UserID: "u10", Token: "push-tok-u10", Platform: "ios"},
		{TenantID: "t-push", UserID: "u11", Token: "push-tok-u11", Platform: "android"},
	}
	for i := range tokens {
		if err := gdb.Create(&tokens[i]).Error; err != nil {
			t.Fatalf("seed token: %v", err)
		}
	}
	t.Cleanup(func() {
		for _, tok := range tokens {
			gdb.Delete(&db.DeviceToken{}, tok.ID)
		}
	})

	event := AlertEvent{
		TenantID: "t-push",
		Type:     "push",
		Payload:  json.RawMessage(`{"title":"Alert","body":"Check this out"}`),
		Recipients: []Recipient{
			{UserID: "u10"},
			{UserID: "u11"},
		},
	}

	if err := handleMessage(ctx, gdb, nil, makeMessage(t, event)); err != nil {
		t.Fatalf("handleMessage: %v", err)
	}

	var notif db.Notification
	if err := gdb.Where("tenant_id = ? AND type = ?", "t-push", "push").Last(&notif).Error; err != nil {
		t.Fatalf("fetch notification: %v", err)
	}

	var deliveries []db.NotificationDelivery
	gdb.Where("notification_id = ?", notif.ID).Find(&deliveries)
	if len(deliveries) != 2 {
		t.Fatalf("delivery count: got %d, want 2", len(deliveries))
	}
	for _, d := range deliveries {
		if d.Status != "delivered" {
			t.Errorf("delivery %d status: got %q, want %q", d.ID, d.Status, "delivered")
		}
		if d.DeviceTokenID == nil {
			t.Errorf("delivery %d: expected DeviceTokenID to be set", d.ID)
		}
	}
}

func TestHandleMessage_UnknownType(t *testing.T) {
	gdb := openTestDB(t)
	ctx := context.Background()

	event := AlertEvent{
		TenantID:   "t-unknown",
		Type:       "sms",
		Payload:    json.RawMessage(`{}`),
		Recipients: []Recipient{{UserID: "u1"}},
	}

	err := handleMessage(ctx, gdb, nil, makeMessage(t, event))
	if err == nil {
		t.Fatal("expected error for unknown notification type, got nil")
	}
}

func TestHandleMessage_InvalidJSON(t *testing.T) {
	gdb := openTestDB(t)
	ctx := context.Background()

	msg := kafka.Message{Value: []byte(`not valid json`)}
	err := handleMessage(ctx, gdb, nil, msg)
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

func TestHandleMessage_MissingTenantID(t *testing.T) {
	gdb := openTestDB(t)
	ctx := context.Background()

	event := AlertEvent{
		Type:       "email",
		Payload:    json.RawMessage(`{}`),
		Recipients: []Recipient{{UserID: "u1", Email: "u1@example.com"}},
	}
	err := handleMessage(ctx, gdb, nil, makeMessage(t, event))
	if err == nil {
		t.Fatal("expected error for missing tenant_id, got nil")
	}
}

func TestHandleMessage_EmptyRecipients(t *testing.T) {
	gdb := openTestDB(t)
	ctx := context.Background()

	event := AlertEvent{
		TenantID:   "t-empty-recip",
		Type:       "email",
		Payload:    json.RawMessage(`{}`),
		Recipients: []Recipient{},
	}
	err := handleMessage(ctx, gdb, nil, makeMessage(t, event))
	if err == nil {
		t.Fatal("expected error for empty recipients, got nil")
	}
}

func TestHandleMessage_LowFailureProbability(t *testing.T) {
	gdb := openTestDB(t)
	ctx := context.Background()
	t.Setenv("FAILURE_THRESHOLD", "0.9")

	event := AlertEvent{
		TenantID:   "t-low-prob",
		Type:       "email",
		Payload:    json.RawMessage(`{"failure_probability":0.5}`),
		Recipients: []Recipient{{UserID: "u1", Email: "u1@example.com"}},
	}
	if err := handleMessage(ctx, gdb, nil, makeMessage(t, event)); err != nil {
		t.Fatalf("handleMessage: %v", err)
	}

	var count int64
	gdb.Model(&db.Notification{}).Where("tenant_id = ?", "t-low-prob").Count(&count)
	if count != 0 {
		t.Errorf("expected no notification for low-probability event, got %d", count)
	}
}

func TestHandleMessage_HighFailureProbability(t *testing.T) {
	gdb := openTestDB(t)
	ctx := context.Background()
	t.Setenv("FAILURE_THRESHOLD", "0.8")

	event := AlertEvent{
		TenantID:   "t-high-prob",
		Type:       "email",
		Payload:    json.RawMessage(`{"failure_probability":0.95}`),
		Recipients: []Recipient{{UserID: "u1", Email: "u1@example.com"}},
	}
	if err := handleMessage(ctx, gdb, nil, makeMessage(t, event)); err != nil {
		t.Fatalf("handleMessage: %v", err)
	}

	var count int64
	gdb.Model(&db.Notification{}).Where("tenant_id = ?", "t-high-prob").Count(&count)
	if count != 1 {
		t.Errorf("expected 1 notification for high-probability event, got %d", count)
	}
}

func TestHandleMessage_EmailSkipsEmptyAddress(t *testing.T) {
	gdb := openTestDB(t)
	ctx := context.Background()

	event := AlertEvent{
		TenantID: "t-email-skip",
		Type:     "email",
		Payload:  json.RawMessage(`{"subject":"Test"}`),
		Recipients: []Recipient{
			{UserID: "u1", Email: "u1@example.com"},
			{UserID: "u2", Email: ""},
		},
	}
	if err := handleMessage(ctx, gdb, nil, makeMessage(t, event)); err != nil {
		t.Fatalf("handleMessage: %v", err)
	}

	var notif db.Notification
	if err := gdb.Where("tenant_id = ? AND type = ?", "t-email-skip", "email").Last(&notif).Error; err != nil {
		t.Fatalf("fetch notification: %v", err)
	}

	var deliveries []db.NotificationDelivery
	gdb.Where("notification_id = ?", notif.ID).Find(&deliveries)
	if len(deliveries) != 1 {
		t.Fatalf("delivery count: got %d, want 1 (empty address should be skipped)", len(deliveries))
	}
	if deliveries[0].Recipient != "u1@example.com" {
		t.Errorf("expected delivery to u1@example.com, got %q", deliveries[0].Recipient)
	}
}

func TestHandleMessage_PushNoDeviceTokens(t *testing.T) {
	gdb := openTestDB(t)
	ctx := context.Background()

	event := AlertEvent{
		TenantID:   "t-push-notokens",
		Type:       "push",
		Payload:    json.RawMessage(`{"title":"Alert"}`),
		Recipients: []Recipient{{UserID: "u-no-token"}},
	}
	if err := handleMessage(ctx, gdb, nil, makeMessage(t, event)); err != nil {
		t.Fatalf("handleMessage: %v", err)
	}

	var notif db.Notification
	if err := gdb.Where("tenant_id = ? AND type = ?", "t-push-notokens", "push").Last(&notif).Error; err != nil {
		t.Fatalf("fetch notification: %v", err)
	}

	var deliveries []db.NotificationDelivery
	gdb.Where("notification_id = ?", notif.ID).Find(&deliveries)
	if len(deliveries) != 0 {
		t.Errorf("delivery count: got %d, want 0 (no device tokens)", len(deliveries))
	}
}

func TestHandleMessage_PushMultipleTokensPerUser(t *testing.T) {
	gdb := openTestDB(t)
	ctx := context.Background()

	tokens := []db.DeviceToken{
		{TenantID: "t-push-multi", UserID: "u20", Token: "multi-tok-ios", Platform: "ios"},
		{TenantID: "t-push-multi", UserID: "u20", Token: "multi-tok-android", Platform: "android"},
	}
	for i := range tokens {
		if err := gdb.Create(&tokens[i]).Error; err != nil {
			t.Fatalf("seed token: %v", err)
		}
	}
	t.Cleanup(func() {
		for _, tok := range tokens {
			gdb.Delete(&db.DeviceToken{}, tok.ID)
		}
	})

	event := AlertEvent{
		TenantID:   "t-push-multi",
		Type:       "push",
		Payload:    json.RawMessage(`{"title":"Multi"}`),
		Recipients: []Recipient{{UserID: "u20"}},
	}
	if err := handleMessage(ctx, gdb, nil, makeMessage(t, event)); err != nil {
		t.Fatalf("handleMessage: %v", err)
	}

	var notif db.Notification
	if err := gdb.Where("tenant_id = ? AND type = ?", "t-push-multi", "push").Last(&notif).Error; err != nil {
		t.Fatalf("fetch notification: %v", err)
	}

	var deliveries []db.NotificationDelivery
	gdb.Where("notification_id = ?", notif.ID).Find(&deliveries)
	if len(deliveries) != 2 {
		t.Fatalf("delivery count: got %d, want 2 (one per device token)", len(deliveries))
	}
}

func TestNormalizeKafkaMessage(t *testing.T) {
	cases := []struct {
		name  string
		input []byte
		want  []byte
	}{
		{"plain", []byte(`{"a":1}`), []byte(`{"a":1}`)},
		{"leading whitespace", []byte("  \n{\"a\":1}"), []byte(`{"a":1}`)},
		{"trailing whitespace", []byte("{\"a\":1}\n  "), []byte(`{"a":1}`)},
		{"UTF-8 BOM", append([]byte{0xEF, 0xBB, 0xBF}, []byte(`{"a":1}`)...), []byte(`{"a":1}`)},
		{"BOM and whitespace", append([]byte{0xEF, 0xBB, 0xBF}, []byte("  {\"a\":1}  ")...), []byte(`{"a":1}`)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := normalizeKafkaMessage(tc.input)
			if string(got) != string(tc.want) {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestFailureThreshold(t *testing.T) {
	cases := []struct {
		env  string
		want float64
	}{
		{"0.5", 0.5},
		{"0.0", 0.0},
		{"1.0", 1.0},
		{"not-a-number", 0.8},
		{"-0.1", 0.8},
		{"1.1", 0.8},
		{"", 0.8},
	}
	for _, tc := range cases {
		t.Run(tc.env, func(t *testing.T) {
			if tc.env == "" {
				t.Setenv("FAILURE_THRESHOLD", "")
			} else {
				t.Setenv("FAILURE_THRESHOLD", tc.env)
			}
			got := failureThreshold()
			if got != tc.want {
				t.Errorf("FAILURE_THRESHOLD=%q: got %v, want %v", tc.env, got, tc.want)
			}
		})
	}
}
