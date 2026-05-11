package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"notifications-service/db"
	"testutil"
)

func TestNotificationsHandler_MissingTenantID(t *testing.T) {
	gdb := testutil.OpenTestDB(t, db.Open)
	handler := notificationsHandler(gdb)

	req := httptest.NewRequest(http.MethodGet, "/notifications", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestNotificationsHandler_ValidTenant(t *testing.T) {
	gdb := testutil.OpenTestDB(t, db.Open)

	tenantID := "api-test-tenant"
	for i := range 3 {
		_, err := db.InsertNotification(t.Context(), gdb, tenantID, "email", []byte(fmt.Sprintf(`{"i":%d}`, i)))
		if err != nil {
			t.Fatalf("seed notification: %v", err)
		}
	}
	t.Cleanup(func() {
		gdb.Where("tenant_id = ?", tenantID).Delete(&db.Notification{})
	})

	handler := notificationsHandler(gdb)
	req := httptest.NewRequest(http.MethodGet, "/notifications?tenant_id="+tenantID, nil)
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", w.Code, http.StatusOK)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type: got %q, want %q", ct, "application/json")
	}

	var notifs []db.Notification
	if err := json.NewDecoder(w.Body).Decode(&notifs); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(notifs) != 3 {
		t.Errorf("notification count: got %d, want 3", len(notifs))
	}
	for _, n := range notifs {
		if n.TenantID != tenantID {
			t.Errorf("unexpected tenant_id %q in response", n.TenantID)
		}
	}
}

func TestNotificationsHandler_LimitClamped(t *testing.T) {
	gdb := testutil.OpenTestDB(t, db.Open)

	tenantID := "api-test-limit"
	for range 5 {
		_, err := db.InsertNotification(t.Context(), gdb, tenantID, "email", []byte(`{}`))
		if err != nil {
			t.Fatalf("seed notification: %v", err)
		}
	}
	t.Cleanup(func() {
		gdb.Where("tenant_id = ?", tenantID).Delete(&db.Notification{})
	})

	handler := notificationsHandler(gdb)

	// limit=200 should be clamped to 100; all 5 results should still come back
	req := httptest.NewRequest(http.MethodGet, "/notifications?tenant_id="+tenantID+"&limit=200", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", w.Code, http.StatusOK)
	}

	var notifs []db.Notification
	if err := json.NewDecoder(w.Body).Decode(&notifs); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	// All 5 rows are within the clamped limit of 100
	if len(notifs) != 5 {
		t.Errorf("notification count: got %d, want 5", len(notifs))
	}
}
