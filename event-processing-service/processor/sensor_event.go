package processor

// SensorEvent is the payload published to Kafka by the Ingestion Service.
// It represents a single reading from an edge device.
type OldSensorEvent struct {
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

type NewSensorEvent struct {
	DeviceID        string  `json:"device_id"`
	TenantID        string  `json:"tenant_id"`
	VibrationX      float64 `json:"vibration_x"`
	VibrationY      float64 `json:"vibration_y"`
	TempMotor       float64 `json:"temp_motor"`
	TempAtmospheric float64 `json:"temp_atmospheric"`
}

// SensorEvent is the generic structure used by the processor.
type SensorEvent struct {
	DeviceID        string
	TenantID        string
	Mode            string
	VRMS            float64
	TempC           float64
	PeakHz1         float64
	PeakHz2         float64
	PeakHz3         float64
	Status          string
	VibrationX      float64
	VibrationY      float64
	TempMotor       float64
	TempAtmospheric float64
}
