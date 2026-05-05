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
	
	// AutoMigrate all models for predictions pipeline
	if err := gdb.WithContext(ctx).AutoMigrate(
		&Event{},
		&ProcessedFeatures{},
		&Prediction{},
		&PredictionReview{},
		&RetrainingConfig{},
		&RetrainingRequest{},
		&ModelVersion{},
		&ActiveModelVersion{},
	); err != nil {
		return nil, err
	}
	return gdb, nil
}
