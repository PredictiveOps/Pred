package processor

import (
	"encoding/json"
	"testing"

	"gorm.io/datatypes"

	"event-processing-service/db"
)

func TestGroupEvents_ByTenantAndDevice(t *testing.T) {
	events := []db.Event{
		storedEvent(t, 1, SensorEvent{TenantID: "factory-a", DeviceID: "MTR-01", VRMS: 0.4}),
		storedEvent(t, 2, SensorEvent{TenantID: "factory-a", DeviceID: "MTR-01", VRMS: 0.5}),
		storedEvent(t, 3, SensorEvent{TenantID: "factory-b", DeviceID: "MTR-01", VRMS: 0.6}),
		storedEvent(t, 4, SensorEvent{TenantID: "factory-a", DeviceID: "MTR-02", VRMS: 0.7}),
	}

	groups := groupEvents(events)
	if len(groups) != 3 {
		t.Fatalf("group count = %d, want 3", len(groups))
	}

	if got := len(groups[0].readings); got != 2 {
		t.Fatalf("first group readings = %d, want 2", got)
	}
	if groups[0].tenantID != "factory-a" || groups[0].deviceID != "MTR-01" {
		t.Fatalf("first group = tenant %q device %q, want factory-a/MTR-01", groups[0].tenantID, groups[0].deviceID)
	}
	if len(groups[0].ids) != 2 || groups[0].ids[0] != 1 || groups[0].ids[1] != 2 {
		t.Fatalf("first group ids = %v, want [1 2]", groups[0].ids)
	}
}

func storedEvent(t *testing.T, id int64, event SensorEvent) db.Event {
	t.Helper()
	body, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("marshal event: %v", err)
	}
	return db.Event{ID: id, TenantID: event.TenantID, Payload: datatypes.JSON(body)}
}
