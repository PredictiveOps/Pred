package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"ingestion-service/db"
	"ingestion-service/handlers"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func openTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	gdb, err := db.Open(context.Background(), url)
	if err != nil {
		t.Fatalf("openTestDB: %v", err)
	}
	if err := gdb.AutoMigrate(&db.Device{}); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}
	t.Cleanup(func() {
		sqlDB, _ := gdb.DB()
		sqlDB.Close()
	})
	return gdb
}

func newTestRouter(gdb *gorm.DB) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/devices/register", handlers.RegisterDeviceHTTP(gdb))
	r.GET("/tenants/:tenant_id/devices", handlers.GetDevicesByTenantIDHandler(gdb))
	r.GET("/devices/:device_id", handlers.GetDeviceByIDHandler(gdb))
	return r
}

func TestRegisterDevice_Success(t *testing.T) {
	gdb := openTestDB(t)
	t.Cleanup(func() { gdb.Delete(&db.Device{}, "device_id = ?", 70001) })

	body := `{"device_id":70001,"tenant_id":7001}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/devices/register", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	newTestRouter(gdb).ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusCreated, w.Body.String())
	}

	var resp db.DeviceRegistrationResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("response not valid JSON: %v", err)
	}
	if resp.RegistrationStatus != "ok" {
		t.Errorf("registration_status = %q, want %q", resp.RegistrationStatus, "ok")
	}
}

func TestRegisterDevice_MissingDeviceID(t *testing.T) {
	body := `{"tenant_id":7002}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/devices/register", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	newTestRouter(nil).ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestRegisterDevice_MissingTenantID(t *testing.T) {
	body := `{"device_id":1}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/devices/register", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	newTestRouter(nil).ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestRegisterDevice_InvalidJSON(t *testing.T) {
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/devices/register", bytes.NewBufferString("not-json"))
	req.Header.Set("Content-Type", "application/json")
	newTestRouter(nil).ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestGetDevices_ReturnsTenantDevices(t *testing.T) {
	gdb := openTestDB(t)
	t.Cleanup(func() {
		gdb.Delete(&db.Device{}, "tenant_id IN ?", []uint{7003, 7004})
	})

	for _, d := range []db.Device{
		{DeviceID: 70031, TenantID: 7003},
		{DeviceID: 70032, TenantID: 7003},
		{DeviceID: 70041, TenantID: 7004},
	} {
		if err := gdb.Create(&d).Error; err != nil {
			t.Fatalf("seed: %v", err)
		}
	}

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/tenants/7003/devices", nil)
	newTestRouter(gdb).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	var devices []db.DeviceDetails
	if err := json.Unmarshal(w.Body.Bytes(), &devices); err != nil {
		t.Fatalf("response not valid JSON: %v", err)
	}
	if len(devices) != 2 {
		t.Errorf("got %d devices, want 2", len(devices))
	}
	for _, d := range devices {
		if d.TenantID != 7003 {
			t.Errorf("device has tenant_id=%d, want 7003", d.TenantID)
		}
	}
}

func TestGetDevices_InvalidTenantID(t *testing.T) {
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/tenants/abc/devices", nil)
	newTestRouter(nil).ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestGetDeviceByID_Found(t *testing.T) {
	gdb := openTestDB(t)

	device := db.Device{DeviceID: 70051, TenantID: 7005}
	if err := gdb.Create(&device).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}
	t.Cleanup(func() { gdb.Delete(&db.Device{}, "device_id = ?", device.DeviceID) })

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, fmt.Sprintf("/devices/%d", device.DeviceID), nil)
	newTestRouter(gdb).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	var resp db.DeviceDetails
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("response not valid JSON: %v", err)
	}
	if resp.DeviceID != device.DeviceID {
		t.Errorf("device_id = %d, want %d", resp.DeviceID, device.DeviceID)
	}
	if resp.TenantID != 7005 {
		t.Errorf("tenant_id = %d, want 7005", resp.TenantID)
	}
}

func TestGetDeviceByID_NotFound(t *testing.T) {
	openTestDB(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/devices/999999999", nil)
	newTestRouter(nil).ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestGetDeviceByID_InvalidID(t *testing.T) {
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/devices/not-a-number", nil)
	newTestRouter(nil).ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}
