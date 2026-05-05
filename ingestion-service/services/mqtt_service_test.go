package services

import (
	"crypto/tls"
	"os"
	"strings"
	"testing"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

func TestCreateTLSConfig_EmptyCACert(t *testing.T) {
	cfg, err := createTLSConfig("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.MinVersion != tls.VersionTLS12 {
		t.Errorf("MinVersion = %d, want %d", cfg.MinVersion, tls.VersionTLS12)
	}
}

func TestCreateTLSConfig_NonExistentCACert_ReturnsError(t *testing.T) {
	_, err := createTLSConfig("/nonexistent/ca.pem")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "read MQTT CA certificate") {
		t.Errorf("error %q does not contain %q", err.Error(), "read MQTT CA certificate")
	}
}

func TestCreateTLSConfig_InvalidPEMFile_ReturnsError(t *testing.T) {
	f, err := os.CreateTemp("", "ca-*.pem")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	defer os.Remove(f.Name())
	f.WriteString("not-a-pem")
	f.Close()

	_, err = createTLSConfig(f.Name())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to append MQTT CA certificate") {
		t.Errorf("error %q does not contain %q", err.Error(), "failed to append MQTT CA certificate")
	}
}

func TestConnectMQTTClient_NilClient_ReturnsError(t *testing.T) {
	err := ConnectMQTTClient(nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.Error() != "mqtt client is nil" {
		t.Errorf("error = %q, want %q", err.Error(), "mqtt client is nil")
	}
}

type alwaysConnected struct{ mqtt.Client }

func (alwaysConnected) IsConnected() bool { return true }

func TestConnectMQTTClient_AlreadyConnected_ReturnsNil(t *testing.T) {
	err := ConnectMQTTClient(alwaysConnected{})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestPublishMQTTMessage_NilClient_ReturnsError(t *testing.T) {
	err := PublishMQTTMessage(nil, "topic", []byte("data"))
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestDisconnectMQTTClient_NilClient_NoOp(t *testing.T) {
	DisconnectMQTTClient(nil) // must not panic
}

func TestCreateMQTTClient_PlaintextBroker_NoError(t *testing.T) {
	client, err := CreateMQTTClient("tcp://localhost:1883", "test-id", "", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client == nil {
		t.Error("expected non-nil client")
	}
}

func TestCreateMQTTClient_TLSBroker_NoCACert_NoError(t *testing.T) {
	client, err := CreateMQTTClient("ssl://localhost:8883", "test-id", "", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client == nil {
		t.Error("expected non-nil client")
	}
}
