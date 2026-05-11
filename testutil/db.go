package testutil

import (
	"context"
	"os"
	"testing"

	"gorm.io/gorm"
)

// OpenTestDB opens a test database using TEST_DATABASE_URL and registers a
// cleanup to close it. The test is skipped when TEST_DATABASE_URL is unset.
// Pass the service-specific db.Open as openFunc.
func OpenTestDB(t *testing.T, openFunc func(context.Context, string) (*gorm.DB, error)) *gorm.DB {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping integration tests")
	}
	gdb, err := openFunc(context.Background(), url)
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() {
		if sqlDB, err := gdb.DB(); err == nil {
			sqlDB.Close()
		}
	})
	return gdb
}
