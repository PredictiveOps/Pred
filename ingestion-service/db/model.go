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

type OldTelemetryData struct {
	Mode    string  `json:"mode"`
	VRMS    float64 `json:"v_rms"`
	TempC   float64 `json:"temp_c"`
	PeakHz1 int     `json:"peak_hz_1"`
	PeakHz2 int     `json:"peak_hz_2"`
	PeakHz3 int     `json:"peak_hz_3"`
	Status  string  `json:"status"`
}

type NewTelemetryData struct {
	DeviceName      string  `json:"device_name"`
	Timestamp       string  `json:"timestamp"`
	VibrationX      float64 `json:"vibration_x"`
	VibrationY      float64 `json:"vibration_y"`
	TempMotor       float64 `json:"temp_motor"`
	TempAtmospheric float64 `json:"temp_atmospheric"`
}

type MQTTPayload struct {
	Timestamp int64           `json:"timestamp"`
	Nonce     string          `json:"nonce"`
	Data      json.RawMessage `json:"data"`
	Signature string          `json:"signature"`
}

type OldKafkaPayload struct {
	DeviceID  string  `json:"device_id"`
	Timestamp int64   `json:"timestamp"`
	Mode      string  `json:"mode"`
	VRMS      float64 `json:"v_rms"`
	TempC     float64 `json:"temp_c"`
	PeakHz1   int     `json:"peak_hz_1"`
	PeakHz2   int     `json:"peak_hz_2"`
	PeakHz3   int     `json:"peak_hz_3"`
	Status    string  `json:"status"`
}

type NewKafkaPayload struct {
	DeviceID        string  `json:"device_id"`
	Timestamp       int64   `json:"timestamp"`
	VibrationX      float64 `json:"vibration_x"`
	VibrationY      float64 `json:"vibration_y"`
	TempMotor       float64 `json:"temp_motor"`
	TempAtmospheric float64 `json:"temp_atmospheric"`
}
