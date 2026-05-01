package db

import "gorm.io/gorm"

func AddDevice(device *Device) error {
	result := ORM.Create(device)
	return result.Error
}

func GetAllDevicesByTenantID(tenantID uint) ([]Device, error) {
	var devices []Device
	result := ORM.Where("tenant_id = ?", tenantID).Find(&devices)
	return devices, result.Error
}

func GetActiveDevicesByTenantID(tenantID uint) ([]Device, error) {
	var devices []Device
	result := ORM.Where("tenant_id = ? AND is_active = ?", tenantID, true).Find(&devices)
	return devices, result.Error
}

func GetDeviceByID(id uint) (*Device, error) {
	var device Device
	result := ORM.First(&device, id)
	if result.Error != nil {
		return nil, result.Error
	}
	return &device, nil
}

func UpdateDevice(id uint, updatedDevice *Device) error {
	var device Device
	result := ORM.First(&device, id)
	if result.Error != nil {
		return result.Error
	}
	device.Name = updatedDevice.Name
	device.Description = updatedDevice.Description
	device.TenantID = updatedDevice.TenantID
	device.IsActive = updatedDevice.IsActive
	return ORM.Save(&device).Error
}

func DeleteDevice(id uint) error {
	return ORM.Delete(&Device{}, id).Error
}

func ensureORM() error {
	if ORM == nil {
		return gorm.ErrInvalidDB
	}
	return nil
}
