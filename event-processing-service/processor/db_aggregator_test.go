package processor

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"event-processing-service/db"

	"testutil"
)

func TestDBAggregator_GroupsByTenantAndDevice(t *testing.T) {
	gdb := testutil.OpenTestDB(t, db.Open)
	ctx := context.Background()
	tenantA := "tenant-agg-a"
	tenantB := "tenant-agg-b"
	gdb.Where("tenant_id IN ?", []string{tenantA, tenantB}).Delete(&db.Event{})
	t.Cleanup(func() {
		gdb.Where("tenant_id IN ?", []string{tenantA, tenantB}).Delete(&db.Event{})
	})

	for _, tenantID := range []string{tenantA, tenantB} {
		for i := 0; i < 2; i++ {
			raw, err := json.Marshal(SensorEvent{
				TenantID: tenantID,
				DeviceID: 7,
				VRMS:     0.4 + float64(i)*0.1,
				TempC:    50 + float64(i),
				PeakHz1:  120,
				PeakHz2:  240,
				PeakHz3:  450,
			})
			if err != nil {
				t.Fatalf("marshal event: %v", err)
			}
			if _, err := db.InsertEvent(ctx, gdb, tenantID, raw); err != nil {
				t.Fatalf("InsertEvent: %v", err)
			}
		}
	}

	requests := make(chan MLRequest, 4)
	aggregator := NewDBAggregator(gdb, &recordingSink{requests: requests}, time.Second, 10)
	if err := aggregator.ProcessOnce(ctx); err != nil {
		t.Fatalf("ProcessOnce: %v", err)
	}

	seen := map[string]MLRequest{}
	for i := 0; i < 2; i++ {
		select {
		case request := <-requests:
			seen[request.TenantID] = request
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for ML request")
		}
	}

	for _, tenantID := range []string{tenantA, tenantB} {
		request, ok := seen[tenantID]
		if !ok {
			t.Fatalf("missing ML request for tenant %q", tenantID)
		}
		if request.DeviceID != 7 {
			t.Fatalf("tenant %q device_id = %d, want 7", tenantID, request.DeviceID)
		}
	}

	var unprocessed int64
	if err := gdb.Model(&db.Event{}).
		Where("tenant_id IN ? AND aggregation_processed = ?", []string{tenantA, tenantB}, false).
		Count(&unprocessed).Error; err != nil {
		t.Fatalf("count unprocessed: %v", err)
	}
	if unprocessed != 0 {
		t.Fatalf("unprocessed rows = %d, want 0", unprocessed)
	}
}
