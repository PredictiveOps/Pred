package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/smtp"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"github.com/joho/godotenv"
	"github.com/segmentio/kafka-go"
	"gorm.io/gorm"

	"notifications-service/db"
)

// KeycloakRecipientProvider interface for getting recipients from Keycloak
type KeycloakRecipientProvider interface {
	GetTenantRecipients(ctx context.Context, tenantID string) ([]Recipient, error)
}

// AlertEvent is the message schema consumed from Kafka.
type AlertEvent struct {
	TenantID string          `json:"tenant_id"`
	Type     string          `json:"type"` // "email" or "push"
	Payload  json.RawMessage `json:"payload"`
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Println("No .env file found")
	}

	brokers := getEnv("KAFKA_BROKERS", "localhost:9092")
	topic := getEnv("KAFKA_TOPIC", "notifications")
	groupID := getEnv("KAFKA_GROUP_ID", "notifications-service")
	dbURL := getEnv("DATABASE_URL", "postgres://localhost:5432/notifications")

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	kcURL := getEnv("KEYCLOAK_URL", "http://localhost:8080")
	kcRealm := getEnv("KEYCLOAK_REALM", "pred")
	kcClientID := getEnv("KEYCLOAK_CLIENT_ID", "notifications-service")
	kcClientSecret := getEnv("KEYCLOAK_CLIENT_SECRET", "dev-notifications-service-secret")
	kc := NewKeycloakClient(kcURL, kcRealm, kcClientID, kcClientSecret)

	gdb, err := db.Open(ctx, dbURL)
	if err != nil {
		log.Fatalf("database init failed: %v", err)
	}
	if sqlDB, err := gdb.DB(); err == nil {
		defer sqlDB.Close()
	}
	log.Println("connected to database")

	// Initialize WebSocket hub for broadcasting
	hub := NewHub()

	startHTTPServer(gdb, hub)

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

		if err := handleMessage(ctx, gdb, hub, kc, msg); err != nil {
			log.Printf("handle error (offset %d): %v", msg.Offset, err)
		}
	}
}

func handleMessage(ctx context.Context, gdb *gorm.DB, hub *Hub, kc KeycloakRecipientProvider, msg kafka.Message) error {
	var event AlertEvent
	cleanValue := normalizeKafkaMessage(msg.Value)
	if err := json.Unmarshal(cleanValue, &event); err != nil {
		return fmt.Errorf("unmarshal: %w", err)
	}

	if event.TenantID == "" {
		return fmt.Errorf("missing tenant_id")
	}
	if event.Type != "push" && event.Type != "email" {
		return fmt.Errorf("unknown notification type %q", event.Type)
	}

	if kc == nil {
		return fmt.Errorf("keycloak client not configured")
	}

	recipients, err := kc.GetTenantRecipients(ctx, event.TenantID)
	if err != nil {
		return fmt.Errorf("fetch keycloak recipients: %w", err)
	}
	if len(recipients) == 0 {
		return fmt.Errorf("no users found in keycloak for tenant %s", event.TenantID)
	}

	var payloadMap map[string]interface{}
	if len(event.Payload) > 0 {
		if err := json.Unmarshal(event.Payload, &payloadMap); err != nil {
			return fmt.Errorf("invalid payload: %w", err)
		}
	}

	if prob, ok := payloadMap["failure_probability"].(float64); ok {
		if prob < failureThreshold() {
			log.Println("skipping low-risk event")
			return nil
		}
	}

	notifID, err := db.InsertNotification(ctx, gdb, event.TenantID, event.Type, event.Payload)
	if err != nil {
		return fmt.Errorf("insert notification: %w", err)
	}

	// Broadcast new notification to WebSocket clients
	if hub != nil {
		wsMsg := WSMessage{
			Type: "new_notification",
			Data: event.Payload,
		}
		msgBytes, _ := json.Marshal(wsMsg)
		hub.Broadcast(event.TenantID, msgBytes)
	}

	switch event.Type {
	case "push":
		return fanOutPush(ctx, gdb, notifID, event, recipients)
	case "email":
		return fanOutEmail(ctx, gdb, notifID, event, recipients)
	}

	return nil
}

func fanOutPush(ctx context.Context, gdb *gorm.DB, notifID int64, event AlertEvent, recipients []Recipient) error {
	userIDs := make([]string, len(recipients))
	for i, r := range recipients {
		userIDs[i] = r.UserID
	}

	tokens, err := db.DeviceTokensForUsers(ctx, gdb, event.TenantID, userIDs)
	if err != nil {
		return fmt.Errorf("lookup device tokens: %w", err)
	}

	for _, t := range tokens {
		deliveryID, err := db.InsertDelivery(ctx, gdb, notifID, event.TenantID, t.UserID, t.Token, &t.ID)
		if err != nil {
			log.Printf("insert delivery failed (user %s, token %s): %v", t.UserID, t.Token, err)
			continue
		}

		status, errMsg := "delivered", ""
		if pushErr := sendPush(t.Token, t.Platform, event.Payload); pushErr != nil {
			log.Printf("push failed (user %s, token %s): %v", t.UserID, t.Token, pushErr)
			status, errMsg = "failed", pushErr.Error()
		}

		if err := db.UpdateDeliveryStatus(ctx, gdb, deliveryID, status, errMsg); err != nil {
			log.Printf("update status failed (delivery %d): %v", deliveryID, err)
		}
	}
	return nil
}

func fanOutEmail(ctx context.Context, gdb *gorm.DB, notifID int64, event AlertEvent, recipients []Recipient) error {
	for _, r := range recipients {
		if r.Email == "" {
			log.Printf("skipping recipient with empty email (user %s)", r.UserID)
			continue
		}

		deliveryID, err := db.InsertDelivery(ctx, gdb, notifID, event.TenantID, r.UserID, r.Email, nil)
		if err != nil {
			log.Printf("insert delivery failed (user %s, email %s): %v", r.UserID, r.Email, err)
			continue
		}

		status, errMsg := "delivered", ""
		if emailErr := sendEmail(r.Email, event.Payload); emailErr != nil {
			log.Printf("email failed (user %s, email %s): %v", r.UserID, r.Email, emailErr)
			status, errMsg = "failed", emailErr.Error()
		}

		if err := db.UpdateDeliveryStatus(ctx, gdb, deliveryID, status, errMsg); err != nil {
			log.Printf("update status failed (delivery %d): %v", deliveryID, err)
		}
	}
	return nil
}

func sendPush(token, platform string, _ json.RawMessage) error {
	log.Printf("push -> %s (%s)", token, platform)
	return nil
}

func sendEmail(email string, payload json.RawMessage) error {
	from := getEnv("EMAIL_USER", "")
	password := getEnv("EMAIL_PASS", "")

	if from == "" || password == "" {
		log.Printf("email -> %s (SMTP not configured, skipping)", email)
		return nil
	}

	msg := "Subject: Machine Alert\n\n" + string(payload)

	return smtp.SendMail(
		"smtp.gmail.com:587",
		smtp.PlainAuth("", from, password, "smtp.gmail.com"),
		from,
		[]string{email},
		[]byte(msg),
	)
}

func failureThreshold() float64 {
	v := getEnv("FAILURE_THRESHOLD", "0.8")
	f, err := strconv.ParseFloat(v, 64)
	if err != nil || f < 0 || f > 1 {
		return 0.8
	}
	return f
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return fallback
}

func normalizeKafkaMessage(raw []byte) []byte {
	// Trim UTF-8 BOM and leading/trailing whitespace that can appear in CLI-produced messages.
	raw = bytes.TrimPrefix(raw, []byte{0xEF, 0xBB, 0xBF})
	return bytes.TrimSpace(raw)
}
