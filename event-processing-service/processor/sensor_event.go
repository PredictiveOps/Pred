package processor

// SensorEvent is the payload published to Kafka by the Ingestion Service.
// It represents a single reading from an edge device.
type SensorEvent struct {
	DeviceID string  `json:"device_id"`
	TenantID string  `json:"tenant_id"`
	Mode     string  `json:"mode"`
	VRMS     float64 `json:"v_rms"`
	TempC    float64 `json:"temp_c"`
	PeakHz1  float64 `json:"peak_hz_1"`
	PeakHz2  float64 `json:"peak_hz_2"`
	PeakHz3  float64 `json:"peak_hz_3"`
	Status   string  `json:"status"`
}
