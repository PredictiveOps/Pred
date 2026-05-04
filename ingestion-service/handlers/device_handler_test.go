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

func newTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/devices", handlers.RegisterDevice)
	r.GET("/devices", handlers.GetDevices)
	r.GET("/devices/:id", handlers.GetDeviceByID)
	return r
}

func TestRegisterDevice_Success(t *testing.T) {
	gdb := openTestDB(t)
	t.Cleanup(func() { gdb.Delete(&db.Device{}, "tenant_id = ?", 7001) })

	body := `{"name":"Pump A","tenant_id":7001,"is_active":true}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/devices", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	newTestRouter().ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusCreated, w.Body.String())
	}

	var resp db.Device
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("response not valid JSON: %v", err)
	}
	if resp.ID == 0 {
		t.Error("expected non-zero ID")
	}
	if resp.Name != "Pump A" {
		t.Errorf("name = %q, want %q", resp.Name, "Pump A")
	}
}

func TestRegisterDevice_MissingName(t *testing.T) {
	openTestDB(t)

	body := `{"tenant_id":7002}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/devices", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	newTestRouter().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestRegisterDevice_MissingTenantID(t *testing.T) {
	openTestDB(t)

	body := `{"name":"X"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/devices", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	newTestRouter().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestRegisterDevice_InvalidJSON(t *testing.T) {
	openTestDB(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/devices", bytes.NewBufferString("not-json"))
	req.Header.Set("Content-Type", "application/json")
	newTestRouter().ServeHTTP(w, req)

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
		{Name: "D1", TenantID: 7003},
		{Name: "D2", TenantID: 7003},
		{Name: "D3", TenantID: 7004},
	} {
		if err := gdb.Create(&d).Error; err != nil {
			t.Fatalf("seed: %v", err)
		}
	}

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/devices?tenant_id=7003", nil)
	newTestRouter().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	var devices []db.Device
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

func TestGetDevices_MissingTenantID(t *testing.T) {
	openTestDB(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/devices", nil)
	newTestRouter().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestGetDevices_InvalidTenantID(t *testing.T) {
	openTestDB(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/devices?tenant_id=abc", nil)
	newTestRouter().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestGetDeviceByID_Found(t *testing.T) {
	gdb := openTestDB(t)

	device := db.Device{Name: "Sensor A", TenantID: 7005}
	if err := gdb.Create(&device).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}
	t.Cleanup(func() { gdb.Delete(&db.Device{}, device.ID) })

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, fmt.Sprintf("/devices/%d", device.ID), nil)
	newTestRouter().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	var resp db.Device
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("response not valid JSON: %v", err)
	}
	if resp.Name != "Sensor A" {
		t.Errorf("name = %q, want %q", resp.Name, "Sensor A")
	}
}

func TestGetDeviceByID_NotFound(t *testing.T) {
	openTestDB(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/devices/999999999", nil)
	newTestRouter().ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestGetDeviceByID_InvalidID(t *testing.T) {
	openTestDB(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/devices/not-a-number", nil)
	newTestRouter().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}
