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

func TestRegisterDeviceForTenant(t *testing.T) {
	openTestDB(t)

	device, err := db.RegisterDeviceForTenant(5001, 1001)
	if err != nil {
		t.Fatalf("RegisterDeviceForTenant: %v", err)
	}
	if device.DeviceID == 0 {
		t.Fatal("expected non-zero device ID")
	}
	if device.TenantID != 1001 {
		t.Fatalf("expected tenant_id 1001, got %d", device.TenantID)
	}
	if device.PublicKey != nil {
		t.Fatal("expected empty public key on HTTP registration")
	}
	if device.IsActive {
		t.Fatal("expected device to be inactive until MQTT registration")
	}
}

func TestGetDevicesByTenantID(t *testing.T) {
	gdb := openTestDB(t)

	pk1 := "key1"
	pk2 := "key2"
	devices := []db.Device{
		{DeviceID: 100, TenantID: 2001, PublicKey: &pk1, IsActive: true},
		{DeviceID: 101, TenantID: 2001, PublicKey: &pk2, IsActive: false},
		{DeviceID: 102, TenantID: 2002, PublicKey: &pk1, IsActive: true},
	}
	for i := range devices {
		if err := gdb.Create(&devices[i]).Error; err != nil {
			t.Fatalf("seed device: %v", err)
		}
	}

	got, err := db.GetDevicesByTenantID(2001)
	if err != nil {
		t.Fatalf("GetDevicesByTenantID: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d devices, want 2", len(got))
	}
}

func TestGetDeviceByID(t *testing.T) {
	gdb := openTestDB(t)

	pk := "test-key"
	device := db.Device{DeviceID: 4001, TenantID: 4001, PublicKey: &pk, IsActive: true}
	if err := gdb.Create(&device).Error; err != nil {
		t.Fatalf("seed device: %v", err)
	}

	found, err := db.GetDeviceByID(4001)
	if err != nil {
		t.Fatalf("GetDeviceByID: %v", err)
	}
	if found.DeviceID != 4001 {
		t.Fatalf("got device_id %d, want 4001", found.DeviceID)
	}
	if found.TenantID != 4001 {
		t.Fatalf("got tenant_id %d, want 4001", found.TenantID)
	}

	newPK := "updated-key"
	if err := db.UpdateDevicePublicKey(4001, newPK); err != nil {
		t.Fatalf("UpdateDevicePublicKey: %v", err)
	}

	found, err = db.GetDeviceByID(4001)
	if err != nil {
		t.Fatalf("GetDeviceByID after update: %v", err)
	}
	if found.PublicKey == nil || *found.PublicKey != newPK {
		t.Fatalf("unexpected updated device: %+v", found)
	}

	if err := db.DeleteDeviceByID(4001); err != nil {
		t.Fatalf("DeleteDevice: %v", err)
	}
	if _, err := db.GetDeviceByID(4001); err == nil {
		t.Fatal("expected error after delete, got nil")
	}
}
