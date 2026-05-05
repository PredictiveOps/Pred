package handlers

import (
	"context"
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

func TestParseDeviceTopic_ValidTopics(t *testing.T) {
	cases := []struct {
		topic    string
		wantID   uint
		wantKind string
	}{
		{"devices/42/data", 42, "data"},
		{"devices/99/registration", 99, "registration"},
		{"devices/1/data", 1, "data"},
	}
	for _, tc := range cases {
		gotID, gotKind, err := parseDeviceTopic(tc.topic)
		if err != nil {
			t.Errorf("parseDeviceTopic(%q): unexpected error: %v", tc.topic, err)
			continue
		}
		if gotID != tc.wantID || gotKind != tc.wantKind {
			t.Errorf("parseDeviceTopic(%q) = (%d, %q), want (%d, %q)", tc.topic, gotID, gotKind, tc.wantID, tc.wantKind)
		}
	}
}

func TestParseDeviceTopic_InvalidTopics(t *testing.T) {
	cases := []string{
		"noSlash",
		"other/x/data",
		"devices/abc/data",
		"devices/42/unknown",
		"devices/42",
		"",
	}
	for _, topic := range cases {
		if _, _, err := parseDeviceTopic(topic); err == nil {
			t.Errorf("parseDeviceTopic(%q): expected error, got none", topic)
		}
	}
}

func TestHandleMQTTMessage_InvalidTopicDoesNotPanic(t *testing.T) {
	msg := &fakeMQTTMessage{topic: "not-a-valid-topic", payload: []byte("hello")}
	HandleMQTTMessage(nil, msg)
}

func TestHandleMQTTMessage_NonNumericDeviceIDDoesNotPanic(t *testing.T) {
	msg := &fakeMQTTMessage{topic: "devices/abc/data", payload: []byte("hello")}
	HandleMQTTMessage(nil, msg)
}

