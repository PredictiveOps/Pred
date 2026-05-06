package processor

import (
	"sync"
	"testing"
	"time"
)

// recordingFlush is a FlushFunc that records each call into a channel.
type recordingFlush struct {
	mu   sync.Mutex
	calls []flushCall
	ch   chan flushCall
}

type flushCall struct {
	tenantID string
	deviceID string
	count    int
}

func newRecordingFlush(buf int) *recordingFlush {
	return &recordingFlush{ch: make(chan flushCall, buf)}
}

func (r *recordingFlush) fn() FlushFunc {
	return func(tenantID, deviceID string, readings []SensorEvent) {
		call := flushCall{tenantID: tenantID, deviceID: deviceID, count: len(readings)}
		r.mu.Lock()
		r.calls = append(r.calls, call)
		r.mu.Unlock()
		r.ch <- call
	}
}

func makeEvent(deviceID, tenantID string) SensorEvent {
	return SensorEvent{DeviceID: deviceID, TenantID: tenantID, VRMS: 0.5, TempC: 50}
}

// TestWindowManager_SingleDeviceFlushesAfterExpiry verifies that events for one
// device are collected and flushed once the window duration elapses.
func TestWindowManager_SingleDeviceFlushesAfterExpiry(t *testing.T) {
	rec := newRecordingFlush(4)
	wm := NewWindowManager(100*time.Millisecond, rec.fn())
	defer wm.Stop()

	for range 5 {
		wm.Add(makeEvent("DEV-1", "tenant-a"))
	}

	select {
	case call := <-rec.ch:
		if call.deviceID != "DEV-1" {
			t.Errorf("deviceID = %q, want DEV-1", call.deviceID)
		}
		if call.tenantID != "tenant-a" {
			t.Errorf("tenantID = %q, want tenant-a", call.tenantID)
		}
		if call.count != 5 {
			t.Errorf("readings count = %d, want 5", call.count)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for window flush")
	}
}

// TestWindowManager_TwoDevicesFlushIndependently verifies that each device has
// its own window and both flush independently without interfering.
func TestWindowManager_TwoDevicesFlushIndependently(t *testing.T) {
	rec := newRecordingFlush(4)
	wm := NewWindowManager(100*time.Millisecond, rec.fn())
	defer wm.Stop()

	for range 3 {
		wm.Add(makeEvent("DEV-1", "tenant-a"))
		wm.Add(makeEvent("DEV-2", "tenant-a"))
	}

	seen := map[string]int{}
	timeout := time.After(2 * time.Second)
	for len(seen) < 2 {
		select {
		case call := <-rec.ch:
			seen[call.deviceID] = call.count
		case <-timeout:
			t.Fatalf("timed out; flushed devices: %v", seen)
		}
	}

	for _, dev := range []string{"DEV-1", "DEV-2"} {
		if seen[dev] != 3 {
			t.Errorf("device %q flushed %d readings, want 3", dev, seen[dev])
		}
	}
}

// TestWindowManager_StopFlushesRemainingEvents verifies that calling Stop()
// synchronously flushes any open windows that have not yet expired.
func TestWindowManager_StopFlushesRemainingEvents(t *testing.T) {
	rec := newRecordingFlush(4)
	// Use a very long window so the background flusher never fires before Stop.
	wm := NewWindowManager(10*time.Second, rec.fn())

	wm.Add(makeEvent("DEV-1", "tenant-a"))
	wm.Add(makeEvent("DEV-1", "tenant-a"))

	wm.Stop()

	// Stop() dispatches flushes in goroutines; give them a moment to land.
	select {
	case call := <-rec.ch:
		if call.deviceID != "DEV-1" {
			t.Errorf("deviceID = %q, want DEV-1", call.deviceID)
		}
		if call.count != 2 {
			t.Errorf("readings count = %d, want 2", call.count)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Stop() did not flush remaining events")
	}
}

// TestWindowManager_EventsAfterStopAreDiscarded verifies that events added
// after Stop() are never forwarded to the flush callback.
func TestWindowManager_EventsAfterStopAreDiscarded(t *testing.T) {
	rec := newRecordingFlush(4)
	wm := NewWindowManager(100*time.Millisecond, rec.fn())

	wm.Stop()

	// Add events after the manager is stopped.
	for range 5 {
		wm.Add(makeEvent("DEV-1", "tenant-a"))
	}

	// Wait longer than the original window duration; nothing should arrive.
	select {
	case call := <-rec.ch:
		t.Errorf("unexpected flush after Stop(): device=%q count=%d", call.deviceID, call.count)
	case <-time.After(400 * time.Millisecond):
		// expected: no flush
	}
}
