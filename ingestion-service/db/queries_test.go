package db_test

import (
	"context"
	"os"
	"testing"

	"ingestion-service/db"

	"gorm.io/gorm"
)

func openTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping DB integration tests")
	}

	gdb, err := db.Open(context.Background(), url)
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if err := gdb.AutoMigrate(&db.Device{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	t.Cleanup(func() {
		if sqlDB, err := gdb.DB(); err == nil {
			sqlDB.Close()
		}
	})
	return gdb
}

func TestAddDevice(t *testing.T) {
	openTestDB(t)

	device := &db.Device{Name: "Pump A", Description: "Main pump", TenantID: 1001, IsActive: true}
	if err := db.AddDevice(device); err != nil {
		t.Fatalf("AddDevice: %v", err)
	}
	if device.ID == 0 {
		t.Fatal("expected non-zero device ID")
	}
}

func TestGetAllDevicesByTenantID(t *testing.T) {
	gdb := openTestDB(t)

	devices := []db.Device{
		{Name: "Device 1", TenantID: 2001, IsActive: true},
		{Name: "Device 2", TenantID: 2001, IsActive: false},
		{Name: "Other Tenant", TenantID: 2002, IsActive: true},
	}
	for i := range devices {
		if err := gdb.Create(&devices[i]).Error; err != nil {
			t.Fatalf("seed device: %v", err)
		}
	}

	got, err := db.GetAllDevicesByTenantID(2001)
	if err != nil {
		t.Fatalf("GetAllDevicesByTenantID: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d devices, want 2", len(got))
	}
}

func TestGetActiveDevicesByTenantID(t *testing.T) {
	gdb := openTestDB(t)

	devices := []db.Device{
		{Name: "Active", TenantID: 3001, IsActive: true},
		{Name: "Inactive", TenantID: 3001, IsActive: false},
	}
	for i := range devices {
		if err := gdb.Create(&devices[i]).Error; err != nil {
			t.Fatalf("seed device: %v", err)
		}
	}

	got, err := db.GetActiveDevicesByTenantID(3001)
	if err != nil {
		t.Fatalf("GetActiveDevicesByTenantID: %v", err)
	}
	if len(got) != 1 || got[0].Name != "Active" {
		t.Fatalf("unexpected active devices: %+v", got)
	}
}

func TestGetDeviceByID_UpdateDevice_DeleteDevice(t *testing.T) {
	gdb := openTestDB(t)

	device := db.Device{Name: "Original", Description: "desc", TenantID: 4001, IsActive: true}
	if err := gdb.Create(&device).Error; err != nil {
		t.Fatalf("seed device: %v", err)
	}

	found, err := db.GetDeviceByID(device.ID)
	if err != nil {
		t.Fatalf("GetDeviceByID: %v", err)
	}
	if found.Name != "Original" {
		t.Fatalf("got device %q, want %q", found.Name, "Original")
	}

	updated := &db.Device{Name: "Updated", Description: "new desc", TenantID: 4001, IsActive: false}
	if err := db.UpdateDevice(device.ID, updated); err != nil {
		t.Fatalf("UpdateDevice: %v", err)
	}

	found, err = db.GetDeviceByID(device.ID)
	if err != nil {
		t.Fatalf("GetDeviceByID after update: %v", err)
	}
	if found.Name != "Updated" || found.Description != "new desc" || found.IsActive {
		t.Fatalf("unexpected updated device: %+v", found)
	}

	if err := db.DeleteDevice(device.ID); err != nil {
		t.Fatalf("DeleteDevice: %v", err)
	}
	if _, err := db.GetDeviceByID(device.ID); err == nil {
		t.Fatal("expected error after delete, got nil")
	}
}
