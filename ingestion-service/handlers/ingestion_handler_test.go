package handlers

import (
	"context"
	"errors"
	"testing"
)

type fakeMQTTMessage struct {
	topic   string
	payload []byte
}

func (m *fakeMQTTMessage) Duplicate() bool   { return false }
func (m *fakeMQTTMessage) Qos() byte         { return 0 }
func (m *fakeMQTTMessage) Retained() bool    { return false }
func (m *fakeMQTTMessage) Topic() string     { return m.topic }
func (m *fakeMQTTMessage) MessageID() uint16 { return 0 }
func (m *fakeMQTTMessage) Payload() []byte   { return m.payload }
func (m *fakeMQTTMessage) Ack()              {}

type publishCall struct {
	key     string
	payload []byte
}

type spyKafkaPublisher struct {
	calls     []publishCall
	returnErr error
}

func (s *spyKafkaPublisher) Publish(_ context.Context, key string, payload []byte) error {
	s.calls = append(s.calls, publishCall{key: key, payload: payload})
	return s.returnErr
}

func TestExtractDeviceIDFromTopic(t *testing.T) {
	cases := []struct {
		topic string
		want  string
	}{
		{"devices/abc/data", "abc"},
		{"devices/sensor-99/data", "sensor-99"},
		{"devices/dev_01/data", "dev_01"},
		{"noSlash", "noSlash"},
		{"other/x/data", "other/x/data"},
	}
	for _, tc := range cases {
		got := extractDeviceIDFromTopic(tc.topic)
		if got != tc.want {
			t.Errorf("extractDeviceIDFromTopic(%q) = %q, want %q", tc.topic, got, tc.want)
		}
	}
}

func TestHandleMQTTMessage_PublishesWithDeviceIDAsKey(t *testing.T) {
	spy := &spyKafkaPublisher{}
	SetKafkaProducer(spy)
	defer SetKafkaProducer(nil)

	msg := &fakeMQTTMessage{topic: "devices/dev42/data", payload: []byte("hello")}
	HandleMQTTMessage(nil, msg)

	if len(spy.calls) != 1 {
		t.Fatalf("expected 1 publish call, got %d", len(spy.calls))
	}
	if spy.calls[0].key != "dev42" {
		t.Errorf("key = %q, want %q", spy.calls[0].key, "dev42")
	}
	if string(spy.calls[0].payload) != "hello" {
		t.Errorf("payload = %q, want %q", spy.calls[0].payload, "hello")
	}
}

func TestHandleMQTTMessage_VariousDeviceIDs(t *testing.T) {
	cases := []struct {
		topic    string
		deviceID string
		payload  string
	}{
		{"devices/pump-01/data", "pump-01", `{"temp":22}`},
		{"devices/sensor_99/data", "sensor_99", `{"vibration":0.5}`},
		{"devices/abc123/data", "abc123", `{}`},
	}

	for _, tc := range cases {
		spy := &spyKafkaPublisher{}
		SetKafkaProducer(spy)

		msg := &fakeMQTTMessage{topic: tc.topic, payload: []byte(tc.payload)}
		HandleMQTTMessage(nil, msg)

		SetKafkaProducer(nil)

		if len(spy.calls) != 1 {
			t.Errorf("topic %q: expected 1 publish call, got %d", tc.topic, len(spy.calls))
			continue
		}
		if spy.calls[0].key != tc.deviceID {
			t.Errorf("topic %q: key = %q, want %q", tc.topic, spy.calls[0].key, tc.deviceID)
		}
	}
}

func TestHandleMQTTMessage_KafkaErrorIsLogged(t *testing.T) {
	spy := &spyKafkaPublisher{returnErr: errors.New("broker down")}
	SetKafkaProducer(spy)
	defer SetKafkaProducer(nil)

	msg := &fakeMQTTMessage{topic: "devices/d1/data", payload: []byte("data")}
	HandleMQTTMessage(nil, msg)

	if len(spy.calls) != 1 {
		t.Errorf("expected 1 publish attempt, got %d", len(spy.calls))
	}
}

func TestHandleMQTTMessage_NilProducer(t *testing.T) {
	SetKafkaProducer(nil)

	msg := &fakeMQTTMessage{topic: "devices/d1/data", payload: []byte("data")}
	HandleMQTTMessage(nil, msg) // must not panic
}
