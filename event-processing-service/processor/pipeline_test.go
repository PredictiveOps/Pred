package processor

import (
	"context"
	"testing"
	"time"
)

// TestPipeline_WindowAggregatesAndSendsToML verifies the full path:
//   WindowManager.Add  →  Compute  →  FeatureSink.Send
//
// A short window (300ms) is used so the test completes quickly.
func TestPipeline_WindowAggregatesAndSendsToML(t *testing.T) {
	// --- 1. Wire up a fake sink and WindowManager ---
	requests := make(chan MLRequest, 5)

	sink := &recordingSink{requests: requests}
	windowDuration := 300 * time.Millisecond

	wm := NewWindowManager(windowDuration, func(tenantID, deviceID string, readings []SensorEvent) {
		features := Compute(readings)
		payload := MLRequest{DeviceID: deviceID, TenantID: tenantID, Features: features}
		if err := sink.Send(context.Background(), payload); err != nil {
			t.Errorf("sink.Send: %v", err)
		}
	})
	defer wm.Stop()

	// --- 3. Push 10 readings for one device ---
	for i := 0; i < 10; i++ {
		wm.Add(SensorEvent{
			DeviceID: "MTR-01",
			TenantID: "factory-a",
			VRMS:     0.45 + float64(i)*0.01,
			TempC:    52.0 + float64(i)*0.1,
			PeakHz1:  120,
			PeakHz2:  240,
			PeakHz3:  450,
		})
	}

	// --- 4. Wait for the window to flush (window + flusher tick + margin) ---
	select {
	case payload := <-requests:
		if payload.DeviceID != "MTR-01" {
			t.Errorf("DeviceID = %q, want MTR-01", payload.DeviceID)
		}
		if payload.TenantID != "factory-a" {
			t.Errorf("TenantID = %q, want factory-a", payload.TenantID)
		}

		slice := payload.Features.ToSlice()
		if len(slice) != 51 {
			t.Errorf("feature slice length = %d, want 51", len(slice))
		}

		// Resultant RMS should be close to the mean of the VRMS inputs (~0.495)
		rRMS := payload.Features.VibrationResultantRMS
		if rRMS < 0.44 || rRMS > 0.56 {
			t.Errorf("VibrationResultantRMS = %v, expected ~0.495", rRMS)
		}

		// Bearing temperature mean should be ~52.45
		tMean := payload.Features.TemperatureBearingMean
		if tMean < 52.0 || tMean > 53.0 {
			t.Errorf("TemperatureBearingMean = %v, expected ~52.45", tMean)
		}

		t.Logf("✓ ML request enqueued: device=%s features=%d rms=%.4f temp=%.4f",
			payload.DeviceID, len(slice), rRMS, tMean)

	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for ML request — window never flushed")
	}
}

type recordingSink struct {
	requests chan<- MLRequest
}

func (s *recordingSink) Send(_ context.Context, payload MLRequest) error {
	s.requests <- payload
	return nil
}

// TestPipeline_MultipleDevicesFlushedIndependently verifies that two devices
// each get their own independent window and flush independently.
func TestPipeline_MultipleDevicesFlushedIndependently(t *testing.T) {
	flushed := make(chan string, 10)

	wm := NewWindowManager(300*time.Millisecond, func(tenantID, deviceID string, readings []SensorEvent) {
		flushed <- deviceID
	})
	defer wm.Stop()

	// Feed events for two different devices.
	for i := 0; i < 5; i++ {
		wm.Add(SensorEvent{DeviceID: "MTR-01", TenantID: "t", VRMS: 0.4, TempC: 50})
		wm.Add(SensorEvent{DeviceID: "MTR-02", TenantID: "t", VRMS: 0.6, TempC: 60})
	}

	seen := map[string]bool{}
	timeout := time.After(3 * time.Second)
	for len(seen) < 2 {
		select {
		case id := <-flushed:
			seen[id] = true
			t.Logf("✓ flushed device %q", id)
		case <-timeout:
			t.Fatalf("timed out; only flushed: %v", seen)
		}
	}
}
