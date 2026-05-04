package db

import (
	"encoding/json"
	"time"
)

type Device struct {
	DeviceID  uint      `gorm:"primaryKey" json:"device_id"`
	TenantID  uint      `json:"tenant_id"`
	PublicKey *string   `json:"public_key"`
	IsActive  bool      `json:"is_active" gorm:"default:false"`
	CreatedAt time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

type DeviceDetails struct {
	DeviceID  uint      `json:"device_id"`
	TenantID  uint      `json:"tenant_id"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type DeviceHTTPRegistrationRequest struct {
	DeviceID uint `json:"device_id"`
	TenantID uint `json:"tenant_id"`
}

type DeviceRegistrationRequest struct {
	PublicKey string `json:"public_key"`
}

type DeviceRegistrationResponse struct {
	RegistrationStatus string `json:"registration_status"`
}

type SensorDeviceData struct {
	Mode    string  `json:"mode"`
	VRMS    float64 `json:"v_rms"`
	TempC   float64 `json:"temp_c"`
	PeakHz1 int     `json:"peak_hz_1"`
	PeakHz2 int     `json:"peak_hz_2"`
	PeakHz3 int     `json:"peak_hz_3"`
	Status  string  `json:"status"`
}

type MQTTPayload struct {
	Timestamp int64           `json:"timestamp"`
	Nonce     string          `json:"nonce"`
	Data      json.RawMessage `json:"data"`
	Signature string          `json:"signature"`
}

type KafkaPayload struct {
	DeviceID  uint    `json:"device_id"`
	Timestamp int64   `json:"timestamp"`
	Mode      string  `json:"mode"`
	VRMS      float64 `json:"v_rms"`
	TempC     float64 `json:"temp_c"`
	PeakHz1   int     `json:"peak_hz_1"`
	PeakHz2   int     `json:"peak_hz_2"`
	PeakHz3   int     `json:"peak_hz_3"`
	Status    string  `json:"status"`
}
