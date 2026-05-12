package config

import (
	"os"
	"testing"
)

func setEnv(t *testing.T, pairs map[string]string) {
	t.Helper()
	for k, v := range pairs {
		t.Setenv(k, v)
	}
}

func requiredEnv() map[string]string {
	return map[string]string{
		"DATABASE_URL":                             "postgres://localhost/test",
		"KAFKA_BROKERS":                            "localhost:9092",
		"KAFKA_TOPIC":                              "test-topic",
		"MQTT_CLIENT_ID":                           "test-client",
		"MQTT_TOPIC":                               "test/topic",
		"MQTT_DEVICE_REGISTRATION_TOPIC":           "test/register",
		"MQTT_DEVICE_REGISTRATION_RESPONSE_TOPIC":  "test/register/response",
	}
}

func TestLoadConfig_Defaults(t *testing.T) {
	// Clear vars that have defaults so we can verify them.
	os.Unsetenv("PORT")
	os.Unsetenv("REDIS_ADDR")
	os.Unsetenv("REDIS_PUBKEY_TTL")
	os.Unsetenv("REDIS_NONCE_TTL")
	os.Unsetenv("REDIS_DB")

	LoadConfig()

	if Port != "8081" {
		t.Errorf("Port: got %q, want %q", Port, "8081")
	}
	if RedisAddr != "localhost:6379" {
		t.Errorf("RedisAddr: got %q, want %q", RedisAddr, "localhost:6379")
	}
	if RedisPubKeyTTL != "30m" {
		t.Errorf("RedisPubKeyTTL: got %q, want %q", RedisPubKeyTTL, "30m")
	}
	if RedisNonceTTL != "60s" {
		t.Errorf("RedisNonceTTL: got %q, want %q", RedisNonceTTL, "60s")
	}
	if RedisDB != 0 {
		t.Errorf("RedisDB: got %d, want 0", RedisDB)
	}
}

func TestLoadConfig_EnvOverrides(t *testing.T) {
	setEnv(t, map[string]string{
		"PORT":             "9000",
		"REDIS_ADDR":       "redis:6380",
		"REDIS_PUBKEY_TTL": "5m",
		"REDIS_NONCE_TTL":  "120s",
		"REDIS_DB":         "3",
	})

	LoadConfig()

	if Port != "9000" {
		t.Errorf("Port: got %q, want %q", Port, "9000")
	}
	if RedisAddr != "redis:6380" {
		t.Errorf("RedisAddr: got %q, want %q", RedisAddr, "redis:6380")
	}
	if RedisPubKeyTTL != "5m" {
		t.Errorf("RedisPubKeyTTL: got %q, want %q", RedisPubKeyTTL, "5m")
	}
	if RedisNonceTTL != "120s" {
		t.Errorf("RedisNonceTTL: got %q, want %q", RedisNonceTTL, "120s")
	}
	if RedisDB != 3 {
		t.Errorf("RedisDB: got %d, want 3", RedisDB)
	}
}

func TestLoadConfig_InvalidRedisDB(t *testing.T) {
	t.Setenv("REDIS_DB", "not-a-number")

	LoadConfig()

	if RedisDB != 0 {
		t.Errorf("RedisDB: got %d, want 0 for invalid input", RedisDB)
	}
}

func TestLoadConfig_MQTTFields(t *testing.T) {
	setEnv(t, map[string]string{
		"MQTT_BROKER":                              "tcp://broker:1883",
		"MQTT_CLIENT_ID":                           "my-client",
		"MQTT_TOPIC":                               "sensors/#",
		"MQTT_DEVICE_REGISTRATION_TOPIC":           "devices/register",
		"MQTT_DEVICE_REGISTRATION_RESPONSE_TOPIC":  "devices/register/response",
		"MQTT_USERNAME":                            "user",
		"MQTT_PASSWORD":                            "pass",
		"MQTT_CA_CERT":                             "/certs/ca.pem",
	})

	LoadConfig()

	checks := map[string]string{
		"MQTTBroker":                              MQTTBroker,
		"MQTTClientID":                            MQTTClientID,
		"MQTTTopic":                               MQTTTopic,
		"MQTTDeviceRegistrationTopic":             MQTTDeviceRegistrationTopic,
		"MQTTDeviceRegistrationResponseTopic":     MQTTDeviceRegistrationResponseTopic,
		"MQTTUsername":                            MQTTUsername,
		"MQTTPassword":                            MQTTPassword,
		"MQTTCACert":                              MQTTCACert,
	}
	expected := map[string]string{
		"MQTTBroker":                              "tcp://broker:1883",
		"MQTTClientID":                            "my-client",
		"MQTTTopic":                               "sensors/#",
		"MQTTDeviceRegistrationTopic":             "devices/register",
		"MQTTDeviceRegistrationResponseTopic":     "devices/register/response",
		"MQTTUsername":                            "user",
		"MQTTPassword":                            "pass",
		"MQTTCACert":                              "/certs/ca.pem",
	}
	for field, got := range checks {
		if got != expected[field] {
			t.Errorf("%s: got %q, want %q", field, got, expected[field])
		}
	}
}

func TestValidate_AllPresent(t *testing.T) {
	setEnv(t, requiredEnv())
	LoadConfig()

	if err := Validate(); err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
}

func TestValidate_MissingDatabaseURL(t *testing.T) {
	setEnv(t, requiredEnv())
	t.Setenv("DATABASE_URL", "")
	LoadConfig()

	err := Validate()
	if err == nil {
		t.Fatal("expected error for missing DATABASE_URL")
	}
	assertContains(t, err.Error(), "DATABASE_URL")
}

func TestValidate_MissingKafkaFields(t *testing.T) {
	for _, field := range []struct {
		env  string
		want string
	}{
		{"KAFKA_BROKERS", "KAFKA_BROKERS"},
		{"KAFKA_TOPIC", "KAFKA_TOPIC"},
	} {
		t.Run(field.env, func(t *testing.T) {
			setEnv(t, requiredEnv())
			t.Setenv(field.env, "")
			LoadConfig()

			err := Validate()
			if err == nil {
				t.Fatalf("expected error for missing %s", field.env)
			}
			assertContains(t, err.Error(), field.want)
		})
	}
}

func TestValidate_MissingMQTTFields(t *testing.T) {
	for _, field := range []struct {
		env  string
		want string
	}{
		{"MQTT_CLIENT_ID", "MQTT_CLIENT_ID"},
		{"MQTT_TOPIC", "MQTT_TOPIC"},
		{"MQTT_DEVICE_REGISTRATION_TOPIC", "MQTT_DEVICE_REGISTRATION_TOPIC"},
		{"MQTT_DEVICE_REGISTRATION_RESPONSE_TOPIC", "MQTT_DEVICE_REGISTRATION_RESPONSE_TOPIC"},
	} {
		t.Run(field.env, func(t *testing.T) {
			setEnv(t, requiredEnv())
			t.Setenv(field.env, "")
			LoadConfig()

			err := Validate()
			if err == nil {
				t.Fatalf("expected error for missing %s", field.env)
			}
			assertContains(t, err.Error(), field.want)
		})
	}
}

func TestValidate_MultipleMissing(t *testing.T) {
	// Clear all required fields.
	for k := range requiredEnv() {
		t.Setenv(k, "")
	}
	LoadConfig()

	err := Validate()
	if err == nil {
		t.Fatal("expected error when all required fields are missing")
	}
	for _, name := range []string{"DATABASE_URL", "KAFKA_BROKERS", "KAFKA_TOPIC", "MQTT_CLIENT_ID", "MQTT_TOPIC"} {
		assertContains(t, err.Error(), name)
	}
}

func assertContains(t *testing.T, s, substr string) {
	t.Helper()
	if len(s) == 0 || len(substr) == 0 {
		t.Errorf("assertContains: got %q, want it to contain %q", s, substr)
		return
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return
		}
	}
	t.Errorf("got %q, want it to contain %q", s, substr)
}
