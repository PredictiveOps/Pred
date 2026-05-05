package db

import (
	"context"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

func DeviceTokensForUsers(ctx context.Context, gdb *gorm.DB, tenantID string, userIDs []string) ([]DeviceToken, error) {
	var tokens []DeviceToken
	err := gdb.WithContext(ctx).
		Where("tenant_id = ? AND user_id IN ?", tenantID, userIDs).
		Find(&tokens).Error
	return tokens, err
}

func InsertNotification(ctx context.Context, gdb *gorm.DB, tenantID, notifType string, payload []byte) (int64, error) {
	n := Notification{TenantID: tenantID, Type: notifType, Payload: datatypes.JSON(payload)}
	if err := gdb.WithContext(ctx).Create(&n).Error; err != nil {
		return 0, err
	}
	return n.ID, nil
}

func InsertDelivery(ctx context.Context, gdb *gorm.DB, notificationID int64, tenantID, userID, recipient string, deviceTokenID *int64) (int64, error) {
	d := NotificationDelivery{
		NotificationID: notificationID,
		TenantID:       tenantID,
		UserID:         userID,
		Recipient:      recipient,
		DeviceTokenID:  deviceTokenID,
		Status:         "pending",
	}
	if err := gdb.WithContext(ctx).Create(&d).Error; err != nil {
		return 0, err
	}
	return d.ID, nil
}

func UpdateDeliveryStatus(ctx context.Context, gdb *gorm.DB, id int64, status, errMsg string) error {
	updates := map[string]any{"status": status, "updated_at": gorm.Expr("NOW()")}
	if errMsg != "" {
		updates["error"] = errMsg
	} else {
		updates["error"] = nil
	}
	return gdb.WithContext(ctx).Model(&NotificationDelivery{}).Where("id = ?", id).Updates(updates).Error
}
