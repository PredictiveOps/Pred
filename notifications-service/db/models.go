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
	ID        int64          `gorm:"primaryKey" json:"id"`
	TenantID  string         `gorm:"not null;index:notifications_tenant" json:"tenant_id"`
	Type      string         `gorm:"not null" json:"type"`
	Payload   datatypes.JSON `gorm:"type:jsonb;not null" json:"payload,omitempty"`
	CreatedAt time.Time      `gorm:"not null;default:now()" json:"created_at,omitempty"`
}

type NotificationDelivery struct {
	ID             int64        `gorm:"primaryKey"`
	NotificationID int64        `gorm:"not null;index:notification_deliveries_notification"`
	Notification   Notification `gorm:"constraint:OnDelete:CASCADE"`
	TenantID       string       `gorm:"not null;index:notification_deliveries_tenant_user,priority:1"`
	UserID         string       `gorm:"not null;index:notification_deliveries_tenant_user,priority:2"`
	Recipient      string       `gorm:"not null"`
	DeviceTokenID  *int64
	DeviceToken    *DeviceToken `gorm:"constraint:OnDelete:SET NULL"`
	Status         string       `gorm:"not null;default:pending"`
	Error          *string
	CreatedAt      time.Time `gorm:"not null;default:now()"`
	UpdatedAt      time.Time `gorm:"not null;default:now()"`
}
