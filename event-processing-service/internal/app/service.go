package app

import (
	"context"
	"encoding/json"
	"fmt"

	"gorm.io/gorm"

	"event-processing-service/db"
)

type RawEvent struct {
	TenantID string `json:"tenant_id"`
}

type Service struct {
	DB *gorm.DB
}

func NewService(gdb *gorm.DB) *Service {
	return &Service{DB: gdb}
}

func (s *Service) Ingest(ctx context.Context, payload []byte) (int64, error) {
	var event RawEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		return 0, fmt.Errorf("unmarshal: %w", err)
	}

	id, err := db.InsertEvent(ctx, s.DB, event.TenantID, payload)
	if err != nil {
		return 0, fmt.Errorf("insert event: %w", err)
	}

	return id, nil
}
