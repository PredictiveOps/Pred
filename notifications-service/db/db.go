package db

import (
	"context"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func Open(ctx context.Context, url string) (*gorm.DB, error) {
	gdb, err := gorm.Open(postgres.Open(url), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	if err := gdb.WithContext(ctx).AutoMigrate(&DeviceToken{}, &Notification{}, &NotificationDelivery{}); err != nil {
		return nil, err
	}
	return gdb, nil
}
