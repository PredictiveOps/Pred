package main_test

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/segmentio/kafka-go"

	"ingestion-service/db"
	"ingestion-service/handlers"
	"ingestion-service/router"
	"ingestion-service/services"

	"testutil"
)

// ---- in-memory Redis client ------------------------------------------------

type inMemRedisClient struct {
	mu sync.Mutex
	kv map[string]string
	h  map[string]map[string]string
}

func newInMemRedisClient() *inMemRedisClient {
	return &inMemRedisClient{kv: make(map[string]string), h: make(map[string]map[string]string)}
}

var errKeyNotFound = fmt.Errorf("key not found")

func (r *inMemRedisClient) Ping(_ context.Context) error { return nil }
func (r *inMemRedisClient) Close() error                 { return nil }

func (r *inMemRedisClient) Set(_ context.Context, key string, value interface{}, _ time.Duration) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.kv[key] = fmt.Sprint(value)
	return nil
}

func (r *inMemRedisClient) Get(_ context.Context, key string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	v, ok := r.kv[key]
	if !ok {
		return "", errKeyNotFound
	}
	return v, nil
}

func (r *inMemRedisClient) HSet(_ context.Context, key string, values map[string]interface{}) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.h[key]; !ok {
		r.h[key] = make(map[string]string)
	}
	for k, v := range values {
		r.h[key][k] = fmt.Sprint(v)
	}
	return nil
}

func (r *inMemRedisClient) HGetAll(_ context.Context, key string) (map[string]string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	m, ok := r.h[key]
	if !ok {
		return map[string]string{}, nil
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out, nil
}

func (r *inMemRedisClient) Expire(_ context.Context, _ string, _ time.Duration) (bool, error) {
	return true, nil
}

func (r *inMemRedisClient) SetNX(_ context.Context, key string, value interface{}, _ time.Duration) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.kv[key]; ok {
		return false, nil
	}
	r.kv[key] = fmt.Sprint(value)
	return true, nil
}

func (r *inMemRedisClient) Exists(_ context.Context, keys ...string) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var n int64
	for _, k := range keys {
		if _, ok := r.kv[k]; ok {
			n++
		}
	}
	return n, nil
}

// ---- MQTT fakes ------------------------------------------------------------

type fakeMQTTMsg struct {
	topic   string
	payload []byte
}

func (m *fakeMQTTMsg) Duplicate() bool   { return false }
func (m *fakeMQTTMsg) Qos() byte         { return 0 }
func (m *fakeMQTTMsg) Retained() bool    { return false }
func (m *fakeMQTTMsg) Topic() string     { return m.topic }
func (m *fakeMQTTMsg) MessageID() uint16 { return 0 }
func (m *fakeMQTTMsg) Payload() []byte   { return m.payload }
func (m *fakeMQTTMsg) Ack()              {}

type nullToken struct{}

func (nullToken) Wait() bool                     { return false }
func (nullToken) WaitTimeout(time.Duration) bool { return false }
func (nullToken) Done() <-chan struct{}          { ch := make(chan struct{}); close(ch); return ch }
func (nullToken) Error() error                   { return nil }

// nullMQTTClient satisfies mqtt.Client by discarding all operations.
type nullMQTTClient struct{}

func (nullMQTTClient) IsConnected() bool                                          { return true }
func (nullMQTTClient) IsConnectionOpen() bool                                     { return true }
func (nullMQTTClient) Connect() mqtt.Token                                        { return nullToken{} }
func (nullMQTTClient) Disconnect(_ uint)                                          {}
func (nullMQTTClient) Publish(_ string, _ byte, _ bool, _ interface{}) mqtt.Token { return nullToken{} }
func (nullMQTTClient) Subscribe(_ string, _ byte, _ mqtt.MessageHandler) mqtt.Token {
	return nullToken{}
}
func (nullMQTTClient) SubscribeMultiple(_ map[string]byte, _ mqtt.MessageHandler) mqtt.Token {
	return nullToken{}
}
func (nullMQTTClient) Unsubscribe(_ ...string) mqtt.Token       { return nullToken{} }
func (nullMQTTClient) AddRoute(_ string, _ mqtt.MessageHandler) {}
func (nullMQTTClient) OptionsReader() mqtt.ClientOptionsReader  { return mqtt.ClientOptionsReader{} }

// ---- crypto helpers --------------------------------------------------------

func publicKeyToPEM(t *testing.T, key *ecdsa.PublicKey) string {
	t.Helper()
	der, err := x509.MarshalPKIXPublicKey(key)
	if err != nil {
		t.Fatalf("marshal public key: %v", err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}))
}

// buildSignedPayload creates the MQTT envelope expected by the ingestion handler.
// The signature covers the exact JSON bytes of the data field.
func buildSignedPayload(t *testing.T, privKey *ecdsa.PrivateKey, nonce string, data db.SensorDeviceData) []byte {
	t.Helper()

	dataBytes, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("marshal sensor data: %v", err)
	}

	hash := sha256.Sum256(dataBytes)
	sig, err := ecdsa.SignASN1(rand.Reader, privKey, hash[:])
	if err != nil {
		t.Fatalf("sign payload: %v", err)
	}

	envelope := db.MQTTPayload{
		Timestamp: time.Now().Unix(),
		Nonce:     nonce,
		Data:      json.RawMessage(dataBytes),
		Signature: base64.StdEncoding.EncodeToString(sig),
	}

	envelopeBytes, err := json.Marshal(envelope)
	if err != nil {
		t.Fatalf("marshal envelope: %v", err)
	}
	return envelopeBytes
}

// ---- Kafka reader helper ---------------------------------------------------

// kafkaTestReader is a stateful consumer that advances through the topic so
// successive calls to consume pick up only new messages.
type kafkaTestReader struct {
	r *kafka.Reader
}

func newKafkaTestReader(brokers, topic string) *kafkaTestReader {
	return &kafkaTestReader{
		r: kafka.NewReader(kafka.ReaderConfig{
			Brokers:     strings.Split(brokers, ","),
			Topic:       topic,
			StartOffset: kafka.FirstOffset,
			MaxWait:     500 * time.Millisecond,
		}),
	}
}

// consume reads from the current position until a message for deviceID is
// found or the timeout elapses.
func (k *kafkaTestReader) consume(deviceID uint, timeout time.Duration) *db.KafkaPayload {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	for {
		msg, err := k.r.ReadMessage(ctx)
		if err != nil {
			return nil
		}
		var payload db.KafkaPayload
		if json.Unmarshal(msg.Value, &payload) == nil && payload.DeviceID == deviceID {
			return &payload
		}
	}
}

func (k *kafkaTestReader) close() { k.r.Close() }

// ---------------------------------------------------------------------------

// TestDeviceTelemetryPipeline covers the complete happy-path and replay
// scenarios from the shell integration test, without requiring a live MQTT
// broker. It calls handler functions directly and verifies side-effects in
// Postgres and Kafka.
//
// Requires TEST_DATABASE_URL and TEST_KAFKA_BROKERS; skipped otherwise.
func TestDeviceTelemetryPipeline(t *testing.T) {
	dbURL := os.Getenv("TEST_DATABASE_URL")
	kafkaBrokers := os.Getenv("TEST_KAFKA_BROKERS")
	if dbURL == "" || kafkaBrokers == "" {
		t.Skip("TEST_DATABASE_URL and TEST_KAFKA_BROKERS required; run via make test")
	}

	// Open DB and migrate.
	gdb, err := db.Open(context.Background(), dbURL)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := gdb.AutoMigrate(&db.Device{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	t.Cleanup(func() {
		if sqlDB, err := gdb.DB(); err == nil {
			sqlDB.Close()
		}
	})

	// Unique device ID to avoid collisions with concurrent test runs.
	deviceID := uint(time.Now().UnixNano()%1_000_000_000) + 9_000_000
	tenantID := "tenant-999"

	t.Cleanup(func() { gdb.Delete(&db.Device{}, "device_id = ?", deviceID) })

	// Each run uses its own Kafka topic so messages from prior runs don't bleed in.
	kafkaTopic := fmt.Sprintf("sensor_data_test_%d", time.Now().UnixNano())

	// Pre-create the topic so the async producer doesn't drop the first message
	// while auto-creation is still in flight.
	if err := testutil.CreateTestTopic(strings.Split(kafkaBrokers, ","), kafkaTopic); err != nil {
		t.Fatalf("create kafka topic %s: %v", kafkaTopic, err)
	}

	producer := services.NewKafkaProducer(kafkaBrokers, kafkaTopic)
	handlers.SetKafkaProducer(producer)
	t.Cleanup(func() { producer.Close() })

	cache := services.NewRedisCacheWithClient(newInMemRedisClient(), 30*time.Minute, 60*time.Second)
	handlers.SetRedisCache(cache)

	privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	pubKeyPEM := publicKeyToPEM(t, &privKey.PublicKey)

	httpRouter := router.NewRouter(gdb)

	kReader := newKafkaTestReader(kafkaBrokers, kafkaTopic)
	t.Cleanup(kReader.close)

	// Step 1: health check.
	t.Run("HealthCheck", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/health", nil)
		httpRouter.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("health: got %d, want 200", w.Code)
		}
		var body map[string]string
		json.Unmarshal(w.Body.Bytes(), &body)
		if body["status"] != "ok" {
			t.Errorf("status = %q, want ok", body["status"])
		}
	})

	// Step 2: register device via HTTP.
	t.Run("DeviceRegistrationHTTP", func(t *testing.T) {
		body := fmt.Sprintf(`{"device_id":%d}`, deviceID)
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/devices/register", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Tenant-Id", tenantID)
		httpRouter.ServeHTTP(w, req)

		if w.Code != http.StatusCreated {
			t.Fatalf("register: got %d, want 201; body: %s", w.Code, w.Body.String())
		}
		var resp db.DeviceRegistrationResponse
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if resp.RegistrationStatus != "ok" {
			t.Errorf("registration_status = %q, want ok", resp.RegistrationStatus)
		}
	})

	// Step 3: register device public key via simulated MQTT message.
	t.Run("PublicKeyRegistrationMQTT", func(t *testing.T) {
		regPayload := db.DeviceRegistrationRequest{PublicKey: pubKeyPEM}
		regBytes, _ := json.Marshal(regPayload)

		msg := &fakeMQTTMsg{
			topic:   fmt.Sprintf("devices/%d/registration", deviceID),
			payload: regBytes,
		}
		handlers.HandleMQTTDeviceRegistrationWithTemplate(nullMQTTClient{}, msg, "")

		device, err := db.GetDeviceByID(deviceID)
		if err != nil {
			t.Fatalf("GetDeviceByID: %v", err)
		}
		if device.PublicKey == nil || *device.PublicKey != pubKeyPEM {
			t.Error("public key not stored in database after MQTT registration")
		}
		if !device.IsActive {
			t.Error("device should be active after public key registration")
		}
	})

	sensorData := db.SensorDeviceData{
		Mode:    "normal",
		VRMS:    1.23,
		TempC:   72.4,
		PeakHz1: 50,
		PeakHz2: 100,
		PeakHz3: 150,
		Status:  "ok",
	}
	nonce := fmt.Sprintf("n-%d", time.Now().UnixMilli())
	signedPayload := buildSignedPayload(t, privKey, nonce, sensorData)

	// Step 4: send signed telemetry and verify it appears in Kafka.
	t.Run("TelemetryPublishAndKafkaMessage", func(t *testing.T) {
		msg := &fakeMQTTMsg{
			topic:   fmt.Sprintf("devices/%d/data", deviceID),
			payload: signedPayload,
		}
		handlers.HandleMQTTMessage(nil, msg)

		got := kReader.consume(deviceID, 15*time.Second)
		if got == nil {
			t.Fatal("no Kafka message received within timeout")
		}
		if got.DeviceID != deviceID {
			t.Errorf("device_id = %d, want %d", got.DeviceID, deviceID)
		}
		if got.Mode != sensorData.Mode {
			t.Errorf("mode = %q, want %q", got.Mode, sensorData.Mode)
		}
		if got.VRMS != sensorData.VRMS {
			t.Errorf("v_rms = %f, want %f", got.VRMS, sensorData.VRMS)
		}
		if got.TempC != sensorData.TempC {
			t.Errorf("temp_c = %f, want %f", got.TempC, sensorData.TempC)
		}
		if got.PeakHz1 != sensorData.PeakHz1 {
			t.Errorf("peak_hz_1 = %d, want %d", got.PeakHz1, sensorData.PeakHz1)
		}
		if got.Status != sensorData.Status {
			t.Errorf("status = %q, want %q", got.Status, sensorData.Status)
		}
	})

	// Step 5: replay protection — re-sending the identical payload (same nonce)
	// must not produce a second Kafka message.
	t.Run("ReplayProtection", func(t *testing.T) {
		msg := &fakeMQTTMsg{
			topic:   fmt.Sprintf("devices/%d/data", deviceID),
			payload: signedPayload,
		}
		handlers.HandleMQTTMessage(nil, msg)

		// kReader is positioned after the first message; a successful replay
		// would deliver a second one within the window.
		got := kReader.consume(deviceID, 3*time.Second)
		if got != nil {
			t.Error("replay protection failed: duplicate nonce produced a Kafka message")
		}
	})
}
