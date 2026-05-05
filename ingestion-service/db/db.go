package db

import (
	"context"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// ORM kept for backward compatibility with existing code.
var ORM *gorm.DB

// Open opens a GORM DB connection using the provided DSN and returns the *gorm.DB.
// It also sets the package-level `ORM` variable for compatibility.
func Open(ctx context.Context, url string) (*gorm.DB, error) {
	gdb, err := gorm.Open(postgres.Open(url), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	ORM = gdb
	return gdb, nil
}
