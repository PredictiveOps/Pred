package processor

import (
	"log"
	"sync"
	"time"
)

// FlushFunc is called with all readings collected during a closed window.
// It is always invoked in its own goroutine so it never blocks the Kafka consumer.
type FlushFunc func(tenantID string, deviceID uint, readings []SensorEvent)

// windowBuffer holds readings for a single device's current open window.
type windowBuffer struct {
	tenantID string
	readings []SensorEvent
	openedAt time.Time
}

// WindowManager manages per-device tumbling windows of a fixed duration.
// When a window closes, FlushFunc is called with the accumulated readings.
type WindowManager struct {
	mu       sync.Mutex
	windows  map[aggregationKey]*windowBuffer
	duration time.Duration
	onFlush  FlushFunc
	stop     chan struct{}
	wg       sync.WaitGroup
}

// NewWindowManager creates a WindowManager and starts the background flusher goroutine.
// Call Stop() during graceful shutdown to flush remaining open windows.
func NewWindowManager(duration time.Duration, onFlush FlushFunc) *WindowManager {
	wm := &WindowManager{
		windows:  make(map[aggregationKey]*windowBuffer),
		duration: duration,
		onFlush:  onFlush,
		stop:     make(chan struct{}),
	}
	wm.wg.Add(1)
	go wm.flusher()
	return wm
}

// Add appends a sensor event to the device's current window.
// If no window exists, one is opened. If the current window has expired,
// it is flushed and a new window is opened before appending.
func (wm *WindowManager) Add(event SensorEvent) {
	wm.mu.Lock()
	defer wm.mu.Unlock()

	now := time.Now()
	key := aggregationKey{tenantID: event.TenantID, deviceID: event.DeviceID}
	buf, exists := wm.windows[key]

	if !exists {
		wm.windows[key] = &windowBuffer{
			tenantID: event.TenantID,
			readings: []SensorEvent{event},
			openedAt: now,
		}
		return
	}

	if now.Sub(buf.openedAt) >= wm.duration {
		// Window expired on message arrival — flush and open a new one.
		wm.dispatchFlush(event.DeviceID, buf)
		wm.windows[key] = &windowBuffer{
			tenantID: event.TenantID,
			readings: []SensorEvent{event},
			openedAt: now,
		}
		return
	}

	buf.readings = append(buf.readings, event)
}

// Stop signals the background flusher to stop and flushes all remaining open windows.
// Blocks until the flusher goroutine exits.
func (wm *WindowManager) Stop() {
	close(wm.stop)
	wm.wg.Wait()

	// Final flush of any windows still open.
	wm.mu.Lock()
	defer wm.mu.Unlock()
	for key, buf := range wm.windows {
		if len(buf.readings) > 0 {
			wm.dispatchFlush(key.deviceID, buf)
		}
	}
	wm.windows = make(map[aggregationKey]*windowBuffer)
}

// flusher runs in a background goroutine and ticks every second to close
// any windows that have exceeded their duration without a new message arriving.
func (wm *WindowManager) flusher() {
	defer wm.wg.Done()
	ticker := time.NewTicker(wm.duration / 5)
	defer ticker.Stop()

	for {
		select {
		case <-wm.stop:
			return
		case <-ticker.C:
			wm.flushExpired()
		}
	}
}

func (wm *WindowManager) flushExpired() {
	wm.mu.Lock()
	defer wm.mu.Unlock()

	now := time.Now()
	for key, buf := range wm.windows {
		if now.Sub(buf.openedAt) >= wm.duration {
			wm.dispatchFlush(key.deviceID, buf)
			delete(wm.windows, key)
		}
	}
}

// dispatchFlush copies the readings and calls onFlush in a new goroutine.
// Must be called with wm.mu held.
func (wm *WindowManager) dispatchFlush(deviceID uint, buf *windowBuffer) {
	if len(buf.readings) == 0 {
		return
	}
	// Copy slice so the caller can safely reclaim the buffer.
	snapshot := make([]SensorEvent, len(buf.readings))
	copy(snapshot, buf.readings)
	tenantID := buf.tenantID

	log.Printf("[window] flushing device=%d tenant=%q readings=%d", deviceID, tenantID, len(snapshot))

	go wm.onFlush(tenantID, deviceID, snapshot)
}
