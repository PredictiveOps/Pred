package db

import (
	"time"

	"gorm.io/datatypes"
)

type DeviceToken struct {
	ID        int64     `gorm:"primaryKey"`
	TenantID  string    `gorm:"not null;index:device_tokens_tenant_user,priority:1"`
	UserID    string    `gorm:"not null;index:device_tokens_tenant_user,priority:2"`
	Token     string    `gorm:"not null;uniqueIndex"`
	Platform  string    `gorm:"not null"`
	CreatedAt time.Time `gorm:"not null;default:now()"`
	UpdatedAt time.Time `gorm:"not null;default:now()"`
}

type Notification struct {
	ID        int64          `gorm:"primaryKey"`
	TenantID  string         `gorm:"not null;index:notifications_tenant"`
	Type      string         `gorm:"not null"`
	Payload   datatypes.JSON `gorm:"type:jsonb;not null"`
	CreatedAt time.Time      `gorm:"not null;default:now()"`
}

type NotificationDelivery struct {
	ID             int64 `gorm:"primaryKey"`
	NotificationID int64 `gorm:"not null;index:notification_deliveries_notification"`
	Notification   Notification
	TenantID       string `gorm:"not null;index:notification_deliveries_tenant_user,priority:1"`
	UserID         string `gorm:"not null;index:notification_deliveries_tenant_user,priority:2"`
	Recipient      string `gorm:"not null"`
	DeviceTokenID  *int64
	DeviceToken    *DeviceToken `gorm:"constraint:OnDelete:SET NULL"`
	Status         string       `gorm:"not null;default:pending"`
	Error          *string
	CreatedAt      time.Time `gorm:"not null;default:now()"`
	UpdatedAt      time.Time `gorm:"not null;default:now()"`
}
