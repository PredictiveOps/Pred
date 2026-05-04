package db

import (
	"context"

	"gorm.io/clause"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

func InsertEvent(ctx context.Context, gdb *gorm.DB, tenantID string, payload []byte) (int64, error) {
	e := Event{TenantID: tenantID, Payload: datatypes.JSON(payload)}
	if err := gdb.WithContext(ctx).Create(&e).Error; err != nil {
		return 0, err
	}
	return e.ID, nil
}

// ProcessedFeatures queries
func InsertProcessedFeatures(ctx context.Context, gdb *gorm.DB, pf *ProcessedFeatures) error {
	return gdb.WithContext(ctx).Create(pf).Error
}

func GetLatestProcessedFeatures(ctx context.Context, gdb *gorm.DB, tenantID, assetID string) (*ProcessedFeatures, error) {
	var pf ProcessedFeatures
	err := gdb.WithContext(ctx).
		Where("tenant_id = ? AND asset_id = ?", tenantID, assetID).
		Order("created_at DESC").
		First(&pf).Error
	if err != nil {
		return nil, err
	}
	return &pf, nil
}

func GetProcessedFeaturesByAsset(ctx context.Context, gdb *gorm.DB, tenantID, assetID string, limit int) ([]ProcessedFeatures, error) {
	var features []ProcessedFeatures
	err := gdb.WithContext(ctx).
		Where("tenant_id = ? AND asset_id = ?", tenantID, assetID).
		Order("created_at DESC").
		Limit(limit).
		Find(&features).Error
	return features, err
}

// Prediction queries
func InsertPrediction(ctx context.Context, gdb *gorm.DB, pred *Prediction) error {
	return gdb.WithContext(ctx).Create(pred).Error
}

func GetPredictionByID(ctx context.Context, gdb *gorm.DB, tenantID, predictionID string) (*Prediction, error) {
	var pred Prediction
	err := gdb.WithContext(ctx).
		Where("tenant_id = ? AND prediction_id = ?", tenantID, predictionID).
		First(&pred).Error
	if err != nil {
		return nil, err
	}
	return &pred, nil
}

func GetPendingPredictions(ctx context.Context, gdb *gorm.DB, tenantID string, limit int) ([]Prediction, error) {
	var preds []Prediction
	err := gdb.WithContext(ctx).
		Where("tenant_id = ? AND review_status = ?", tenantID, "pending_review").
		Order("created_at ASC").
		Limit(limit).
		Find(&preds).Error
	return preds, err
}

func UpdatePredictionStatus(ctx context.Context, gdb *gorm.DB, tenantID, predictionID string, status string, reviewed bool) error {
	return gdb.WithContext(ctx).
		Model(&Prediction{}).
		Where("tenant_id = ? AND prediction_id = ?", tenantID, predictionID).
		Updates(map[string]interface{}{"review_status": status, "reviewed": reviewed}).Error
}

// PredictionReview queries
func InsertPredictionReview(ctx context.Context, gdb *gorm.DB, review *PredictionReview) error {
	return gdb.WithContext(ctx).Create(review).Error
}

func GetReviewsByTenant(ctx context.Context, gdb *gorm.DB, tenantID string, limit int) ([]PredictionReview, error) {
	var reviews []PredictionReview
	err := gdb.WithContext(ctx).
		Where("tenant_id = ?", tenantID).
		Order("reviewed_at DESC").
		Limit(limit).
		Find(&reviews).Error
	return reviews, err
}

func CountTrainingEligibleReviews(ctx context.Context, gdb *gorm.DB, tenantID string) (int64, error) {
	var count int64
	err := gdb.WithContext(ctx).
		Model(&PredictionReview{}).
		Where("tenant_id = ? AND is_training_eligible = true", tenantID).
		Count(&count).Error
	return count, err
}

func GetTrainingEligibleReviews(ctx context.Context, gdb *gorm.DB, tenantID string, limit int) ([]PredictionReview, error) {
	var reviews []PredictionReview
	err := gdb.WithContext(ctx).
		Where("tenant_id = ? AND is_training_eligible = true", tenantID).
		Order("reviewed_at DESC").
		Limit(limit).
		Find(&reviews).Error
	return reviews, err
}

// RetrainingConfig queries
func GetRetrainingConfig(ctx context.Context, gdb *gorm.DB, tenantID string) (*RetrainingConfig, error) {
	var cfg RetrainingConfig
	err := gdb.WithContext(ctx).
		Where("tenant_id = ?", tenantID).
		First(&cfg).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil // not found is not an error
		}
		return nil, err
	}
	return &cfg, nil
}

func UpsertRetrainingConfig(ctx context.Context, gdb *gorm.DB, cfg *RetrainingConfig) error {
	return gdb.WithContext(ctx).
		Clauses(clause.OnConflict{
			UpdateAll: true,
		}).
		Create(cfg).Error
}

// RetrainingRequest queries
func InsertRetrainingRequest(ctx context.Context, gdb *gorm.DB, req *RetrainingRequest) error {
	return gdb.WithContext(ctx).Create(req).Error
}

func GetRetrainingRequestByID(ctx context.Context, gdb *gorm.DB, tenantID, requestID string) (*RetrainingRequest, error) {
	var req RetrainingRequest
	err := gdb.WithContext(ctx).
		Where("tenant_id = ? AND request_id = ?", tenantID, requestID).
		First(&req).Error
	if err != nil {
		return nil, err
	}
	return &req, nil
}

func UpdateRetrainingRequestStatus(ctx context.Context, gdb *gorm.DB, tenantID, requestID string, status string) error {
	return gdb.WithContext(ctx).
		Model(&RetrainingRequest{}).
		Where("tenant_id = ? AND request_id = ?", tenantID, requestID).
		Update("status", status).Error
}

// ModelVersion queries
func InsertModelVersion(ctx context.Context, gdb *gorm.DB, mv *ModelVersion) error {
	return gdb.WithContext(ctx).Create(mv).Error
}

func GetModelVersion(ctx context.Context, gdb *gorm.DB, tenantID, modelID string, version string) (*ModelVersion, error) {
	var mv ModelVersion
	err := gdb.WithContext(ctx).
		Where("tenant_id = ? AND model_id = ? AND model_version = ?", tenantID, modelID, version).
		First(&mv).Error
	if err != nil {
		return nil, err
	}
	return &mv, nil
}

func GetLatestModelVersion(ctx context.Context, gdb *gorm.DB, tenantID, modelID string) (*ModelVersion, error) {
	var mv ModelVersion
	err := gdb.WithContext(ctx).
		Where("tenant_id = ? AND model_id = ?", tenantID, modelID).
		Order("created_at DESC").
		First(&mv).Error
	if err != nil {
		return nil, err
	}
	return &mv, nil
}

func UpdateModelVersionStatus(ctx context.Context, gdb *gorm.DB, tenantID, modelID string, version string, status string) error {
	return gdb.WithContext(ctx).
		Model(&ModelVersion{}).
		Where("tenant_id = ? AND model_id = ? AND model_version = ?", tenantID, modelID, version).
		Update("deployment_status", status).Error
}

func GetActiveModelVersion(ctx context.Context, gdb *gorm.DB, tenantID string) (*ActiveModelVersion, error) {
	var amv ActiveModelVersion
	err := gdb.WithContext(ctx).
		Where("tenant_id = ?", tenantID).
		First(&amv).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &amv, nil
}

func SetActiveModelVersion(ctx context.Context, gdb *gorm.DB, tenantID, modelID, version string) error {
	return gdb.WithContext(ctx).
		Clauses(clause.OnConflict{
			UpdateAll: true,
		}).
		Create(&ActiveModelVersion{
			TenantID:      tenantID,
			ActiveModelID: modelID,
			ActiveVersion: version,
		}).Error
}

func GetModelVersionsByStatus(ctx context.Context, gdb *gorm.DB, tenantID, status string, limit int) ([]ModelVersion, error) {
	var versions []ModelVersion
	err := gdb.WithContext(ctx).
		Where("tenant_id = ? AND deployment_status = ?", tenantID, status).
		Order("created_at DESC").
		Limit(limit).
		Find(&versions).Error
	return versions, err
}
