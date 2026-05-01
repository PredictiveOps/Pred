package db

type Device struct {
	ID          uint   `gorm:"primaryKey" json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	TenantID    uint   `json:"tenant_id"`
	IsActive    bool   `json:"is_active"`
}
