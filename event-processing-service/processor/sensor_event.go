package processor

// DataFormat indicates which sensor data format was detected for a window.
type DataFormat string

const (
	// DataFormatOld indicates the legacy format with VRMS, TempC fields
	DataFormatOld DataFormat = "old"
	// DataFormatNew indicates the new format with VibrationX/Y, TempMotor/Atmospheric
	DataFormatNew DataFormat = "new"
	// DataFormatMixed indicates a window containing both old and new format readings
	DataFormatMixed DataFormat = "mixed"
	// DataFormatUnknown indicates no format could be determined (e.g., empty window)
	DataFormatUnknown DataFormat = "unknown"
)

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
	// Format indicates which source format this event originated from
	Format DataFormat `json:"-"`
}

// DetectFormat determines the data format of a SensorEvent based on populated fields.
func (s SensorEvent) DetectFormat() DataFormat {
	hasOld := s.VRMS != 0 || s.TempC != 0
	hasNew := s.VibrationX != 0 || s.VibrationY != 0 || s.TempMotor != 0 || s.TempAtmospheric != 0

	if hasOld && hasNew {
		return DataFormatMixed
	}
	if hasNew {
		return DataFormatNew
	}
	return DataFormatOld
}
