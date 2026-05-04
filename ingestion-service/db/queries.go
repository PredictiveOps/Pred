package db

import "gorm.io/gorm"

// RegisterDeviceForTenant creates the device record with device and tenant IDs only.
// The public key is attached later when the device first registers over MQTT.
func RegisterDeviceForTenant(deviceID, tenantID uint) (*Device, error) {
	if deviceID == 0 || tenantID == 0 {
		return nil, gorm.ErrInvalidDB
	}

	device := &Device{
		DeviceID: deviceID,
		TenantID: tenantID,
	}

	result := ORM.Create(device)
	if result.Error != nil {
		return nil, result.Error
	}

	return device, nil
}

// GetDeviceByID retrieves a device by its device ID.
func GetDeviceByID(deviceID uint) (*Device, error) {
	var device Device
	result := ORM.First(&device, "device_id = ?", deviceID)
	if result.Error != nil {
		return nil, result.Error
	}
	return &device, nil
}

// GetDevicesByTenantID retrieves all devices for a tenant.
func GetDevicesByTenantID(tenantID uint) ([]Device, error) {
	var devices []Device
	result := ORM.Where("tenant_id = ?", tenantID).Find(&devices)
	return devices, result.Error
}

// GetDeviceDetailsByTenantID retrieves device details for all devices in a tenant.
func GetDeviceDetailsByTenantID(tenantID uint) ([]DeviceDetails, error) {
	var devices []Device
	result := ORM.Where("tenant_id = ?", tenantID).Find(&devices)
	if result.Error != nil {
		return nil, result.Error
	}

	details := make([]DeviceDetails, len(devices))
	for i, dev := range devices {
		details[i] = DeviceDetails{
			DeviceID:  dev.DeviceID,
			TenantID:  dev.TenantID,
			IsActive:  dev.IsActive,
			CreatedAt: dev.CreatedAt,
			UpdatedAt: dev.UpdatedAt,
		}
	}

	return details, nil
}

// UpdateDevicePublicKey updates the public key for a device.
func UpdateDevicePublicKey(deviceID uint, publicKey string) error {
	result := ORM.Model(&Device{}).Where("device_id = ?", deviceID).Updates(map[string]any{
		"public_key": publicKey,
		"is_active":  true,
	})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// UpdateDeviceActiveStatus updates the is_active flag for a device.
func UpdateDeviceActiveStatus(deviceID uint, isActive bool) error {
	result := ORM.Model(&Device{}).Where("device_id = ?", deviceID).Update("is_active", isActive)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// DeleteDeviceByID deletes a device by its device ID.
func DeleteDeviceByID(deviceID uint) error {
	result := ORM.Delete(&Device{}, "device_id = ?", deviceID)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
