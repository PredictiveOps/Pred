package processor

import (
	"math"
	"testing"
)

func makeReadings(vrms, temp, hz1, hz2, hz3 float64, n int) []SensorEvent {
	readings := make([]SensorEvent, n)
	for i := range readings {
		readings[i] = SensorEvent{
			DeviceID: "1",
			TenantID: "test-tenant",
			VRMS:     vrms,
			TempC:    temp,
			PeakHz1:  hz1,
			PeakHz2:  hz2,
			PeakHz3:  hz3,
		}
	}
	return readings
}

func approxEqual(a, b, tol float64) bool {
	return math.Abs(a-b) <= tol
}

func TestCompute_FeatureCount(t *testing.T) {
	readings := makeReadings(1.0, 50.0, 120, 240, 450, 10)
	result := Compute(readings)
	f := result.Features
	if got := len(f.ToSlice()); got != 51 {
		t.Errorf("ToSlice() length = %d, want 51", got)
	}
	if result.DataFormat != DataFormatOld {
		t.Errorf("DataFormat = %v, want DataFormatOld for old-format readings", result.DataFormat)
	}
}

func TestCompute_EmptyReadings(t *testing.T) {
	result := Compute(nil)
	f := result.Features
	for i, v := range f.ToSlice() {
		if v != 0 {
			t.Errorf("field[%d] = %v, want 0 for empty input", i, v)
		}
	}
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 for empty input", result.Count)
	}
}

func TestCompute_ConstantVRMS(t *testing.T) {
	// With constant v_rms=1.0: mean=1, std=0, min=max=1, rms=1.
	readings := makeReadings(1.0, 50.0, 120, 240, 450, 20)
	result := Compute(readings)
	f := result.Features

	tol := 1e-9
	if !approxEqual(f.VibrationResultantMean, 1.0, tol) {
		t.Errorf("ResultantMean = %v, want 1.0", f.VibrationResultantMean)
	}
	if !approxEqual(f.VibrationResultantStdDev, 0.0, tol) {
		t.Errorf("ResultantStdDev = %v, want 0.0", f.VibrationResultantStdDev)
	}
	if !approxEqual(f.VibrationResultantRMS, 1.0, tol) {
		t.Errorf("ResultantRMS = %v, want 1.0", f.VibrationResultantRMS)
	}
	if !approxEqual(f.VibrationResultantPeakToPeak, 0.0, tol) {
		t.Errorf("ResultantPeakToPeak = %v, want 0.0", f.VibrationResultantPeakToPeak)
	}
	if result.Count != 20 {
		t.Errorf("Count = %d, want 20", result.Count)
	}
}

func TestCompute_AxisSplit(t *testing.T) {
	// X and Y RMS should be resultant RMS / √2.
	readings := makeReadings(1.0, 50.0, 120, 240, 450, 10)
	result := Compute(readings)
	f := result.Features

	want := 1.0 / math.Sqrt2
	tol := 1e-9
	if !approxEqual(f.VibrationXRMS, want, tol) {
		t.Errorf("VibrationXRMS = %v, want %v", f.VibrationXRMS, want)
	}
	if !approxEqual(f.VibrationYRMS, want, tol) {
		t.Errorf("VibrationYRMS = %v, want %v", f.VibrationYRMS, want)
	}
}

func TestCompute_AxisEnergySplit(t *testing.T) {
	// X energy should be resultant energy / 2 (k² = 0.5).
	readings := makeReadings(2.0, 50.0, 120, 240, 450, 5)
	result := Compute(readings)
	f := result.Features

	if !approxEqual(f.VibrationXEnergy, f.VibrationResultantEnergy/2, 1e-9) {
		t.Errorf("XEnergy=%v, ResultantEnergy/2=%v", f.VibrationXEnergy, f.VibrationResultantEnergy/2)
	}
}

func TestCompute_TemperatureBearing(t *testing.T) {
	// 10 readings, temp rises from 50 to 59.
	readings := make([]SensorEvent, 10)
	for i := range readings {
		readings[i] = SensorEvent{
			DeviceID: "1",
			TenantID: "t",
			VRMS:     0.5,
			TempC:    float64(50 + i),
			PeakHz1:  120, PeakHz2: 240, PeakHz3: 450,
		}
	}
	result := Compute(readings)
	f := result.Features

	tol := 1e-9
	if !approxEqual(f.TemperatureBearingMean, 54.5, tol) {
		t.Errorf("BearingMean = %v, want 54.5", f.TemperatureBearingMean)
	}
	if !approxEqual(f.TemperatureBearingMin, 50.0, tol) {
		t.Errorf("BearingMin = %v, want 50.0", f.TemperatureBearingMin)
	}
	if !approxEqual(f.TemperatureBearingMax, 59.0, tol) {
		t.Errorf("BearingMax = %v, want 59.0", f.TemperatureBearingMax)
	}
	// Trend should be positive (rising temperature).
	if f.TemperatureBearingTrend <= 0 {
		t.Errorf("BearingTrend = %v, want > 0 (rising)", f.TemperatureBearingTrend)
	}
}

func TestCompute_AtmosphericAndDifferenceAreZero(t *testing.T) {
	readings := makeReadings(1.0, 50.0, 120, 240, 450, 5)
	result := Compute(readings)
	f := result.Features

	zeros := []float64{
		f.TemperatureAtmosphericMean, f.TemperatureAtmosphericMin,
		f.TemperatureAtmosphericMax, f.TemperatureAtmosphericStd,
		f.TemperatureDifferenceMean, f.TemperatureDifferenceMax,
		f.TemperatureDifferenceTrend,
	}
	for i, v := range zeros {
		if v != 0 {
			t.Errorf("atmospheric/difference field[%d] = %v, want 0", i, v)
		}
	}
}

func TestCompute_SpectralCentroid(t *testing.T) {
	// Centroid of (120+240+450)/3 = 270 for all readings.
	readings := makeReadings(1.0, 50.0, 120, 240, 450, 5)
	result := Compute(readings)
	f := result.Features

	want := (120.0 + 240.0 + 450.0) / 3.0
	tol := 1e-9
	if !approxEqual(f.VibrationResultantSpectralCentroid, want, tol) {
		t.Errorf("SpectralCentroid = %v, want %v", f.VibrationResultantSpectralCentroid, want)
	}
}

func TestCompute_DominantFrequency(t *testing.T) {
	readings := makeReadings(1.0, 50.0, 120, 240, 450, 5)
	result := Compute(readings)
	f := result.Features

	if !approxEqual(f.VibrationResultantDominantFreqIdx, 120.0, 1e-9) {
		t.Errorf("DominantFreqIdx = %v, want 120.0", f.VibrationResultantDominantFreqIdx)
	}
}

func TestCompute_NewDataFormat(t *testing.T) {
	// Test new format detection with VibrationX/Y and TempMotor/Atmospheric
	readings := make([]SensorEvent, 5)
	for i := range readings {
		readings[i] = SensorEvent{
			DeviceID:        "d",
			TenantID:        "t",
			VibrationX:      1.0,
			VibrationY:      1.0,
			TempMotor:       55.0,
			TempAtmospheric: 25.0,
		}
	}
	result := Compute(readings)

	if result.DataFormat != DataFormatNew {
		t.Errorf("DataFormat = %v, want DataFormatNew", result.DataFormat)
	}
	if result.Count != 5 {
		t.Errorf("Count = %d, want 5", result.Count)
	}
}

func TestCompute_MixedDataFormat(t *testing.T) {
	// Test mixed format detection - some old, some new format readings
	readings := make([]SensorEvent, 4)
	// First 2: old format
	for i := 0; i < 2; i++ {
		readings[i] = SensorEvent{
			DeviceID: "d",
			TenantID: "t",
			VRMS:     1.0,
			TempC:    50.0,
			PeakHz1:  120, PeakHz2: 240, PeakHz3: 450,
		}
	}
	// Last 2: new format
	for i := 2; i < 4; i++ {
		readings[i] = SensorEvent{
			DeviceID:        "d",
			TenantID:        "t",
			VibrationX:      1.0,
			VibrationY:      1.0,
			TempMotor:       55.0,
			TempAtmospheric: 25.0,
		}
	}
	result := Compute(readings)

	if result.DataFormat != DataFormatMixed {
		t.Errorf("DataFormat = %v, want DataFormatMixed", result.DataFormat)
	}
}

func TestSensorEvent_DetectFormat(t *testing.T) {
	tests := []struct {
		name     string
		event    SensorEvent
		expected DataFormat
	}{
		{
			name:     "old format",
			event:    SensorEvent{VRMS: 1.0, TempC: 50.0},
			expected: DataFormatOld,
		},
		{
			name:     "new format",
			event:    SensorEvent{VibrationX: 1.0, TempMotor: 55.0},
			expected: DataFormatNew,
		},
		{
			name:     "mixed format",
			event:    SensorEvent{VRMS: 1.0, VibrationX: 1.0},
			expected: DataFormatMixed,
		},
		{
			name:     "empty has all zero values",
			event:    SensorEvent{},
			expected: DataFormatOld, // all fields zero, so hasOld path triggers
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.event.DetectFormat()
			if got != tt.expected {
				t.Errorf("DetectFormat() = %v, want %v", got, tt.expected)
			}
		})
	}
}
