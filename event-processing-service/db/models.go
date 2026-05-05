package db

import (
	"time"

	"gorm.io/datatypes"
)

type Event struct {
	ID        int64          `gorm:"primaryKey"`
	TenantID  string         `gorm:"not null;index:events_tenant"`
	Payload   datatypes.JSON `gorm:"type:jsonb;not null"`
	CreatedAt time.Time      `gorm:"not null;default:now()"`
}

// ProcessedFeatures stores time-series sensor feature data
type ProcessedFeatures struct {
	ID               int64          `gorm:"primaryKey"`
	TenantID         string         `gorm:"not null;index:processed_features_tenant_asset,priority:1"`
	DeviceID         string         `gorm:"not null;index:processed_features_tenant_asset,priority:2"`
	AssetID          string         `gorm:"not null;index:processed_features_tenant_asset,priority:3"`
	Features         datatypes.JSON `gorm:"type:jsonb;not null"` // {rms, kurtosis, crest_factor, spectral_energy, temperature, ...}
	FeatureVersion   string         `gorm:"default:v1"`          // schema version for features
	CreatedAt        time.Time      `gorm:"not null;default:now();index:processed_features_timestamp"`
	FeatureTimestamp time.Time      `gorm:"not null"` // when the feature was collected
}

// Prediction stores ML model predictions before review
type Prediction struct {
	ID              int64     `gorm:"primaryKey" json:"id"`
	PredictionID    string    `gorm:"uniqueIndex;not null" json:"prediction_id"` // auto-generated unique ID
	TenantID        string    `gorm:"not null;index:predictions_tenant" json:"tenant_id"`
	DeviceID        string    `gorm:"not null;index:predictions_asset,priority:1" json:"device_id"`
	AssetID         string    `gorm:"not null;index:predictions_asset,priority:2" json:"asset_id"`
	ModelName       string    `gorm:"not null" json:"model_name"`
	ModelVersion    string    `gorm:"not null" json:"model_version"`
	AnomalyScore    float64   `gorm:"not null" json:"anomaly_score"`
	PredictedStatus string    `gorm:"not null" json:"predicted_status"`                     // normal, warning, critical
	ReviewStatus    string    `gorm:"not null;default:pending_review" json:"review_status"` // pending_review, reviewed, archived
	Reviewed        bool      `gorm:"not null;default:false" json:"reviewed"`
	CreatedAt       time.Time `gorm:"not null;default:now()" json:"created_at"`
	UpdatedAt       time.Time `gorm:"not null;default:now()" json:"updated_at"`
}

// PredictionReview stores user reviews of predictions
type PredictionReview struct {
	ID                 int64     `gorm:"primaryKey" json:"id"`
	ReviewID           string    `gorm:"uniqueIndex;not null" json:"review_id"` // auto-generated unique ID
	TenantID           string    `gorm:"not null;index:reviews_tenant" json:"tenant_id"`
	PredictionID       string    `gorm:"not null;uniqueIndex:reviews_prediction,priority:1" json:"prediction_id"`
	DeviceID           string    `gorm:"not null" json:"device_id"`
	AssetID            string    `gorm:"not null" json:"asset_id"`
	ModelPrediction    string    `gorm:"not null" json:"model_prediction"` // original model prediction
	ReviewedLabel      string    `gorm:"not null" json:"reviewed_label"`   // corrected label by user (normal, warning, critical)
	ReviewedBy         string    `gorm:"not null" json:"reviewed_by"`      // user ID
	ReviewComment      string    `json:"review_comment"`
	IsTrainingEligible *bool     `gorm:"not null;default:true" json:"is_training_eligible"` // pointer so GORM stores false explicitly
	ReviewedAt         time.Time `gorm:"not null;default:now()" json:"reviewed_at"`
	CreatedAt          time.Time `gorm:"not null;default:now()" json:"created_at"`
}

// RetrainingConfig stores configuration for retraining triggers
type RetrainingConfig struct {
	ID                     int64     `gorm:"primaryKey" json:"id"`
	TenantID               string    `gorm:"uniqueIndex;not null" json:"tenant_id"`
	MinimumReviewedRecords int       `gorm:"not null;default:500" json:"minimum_reviewed_records"`
	AutoRetrainEnabled     bool      `gorm:"not null;default:false" json:"auto_retrain_enabled"`
	RequiresManualApproval bool      `gorm:"not null;default:true" json:"requires_manual_approval"`
	UpdatedBy              string    `json:"updated_by"`
	UpdatedAt              time.Time `gorm:"not null;default:now()" json:"updated_at"`
}

// RetrainingRequest tracks retraining workflow
type RetrainingRequest struct {
	ID                int64      `gorm:"primaryKey" json:"id"`
	RequestID         string     `gorm:"uniqueIndex;not null" json:"request_id"`
	TenantID          string     `gorm:"not null;index:retraining_requests_tenant" json:"tenant_id"`
	Status            string     `gorm:"not null;default:created" json:"status"` // created, approved, in_progress, completed, failed, rejected
	TrainingDataCount int        `json:"training_data_count"`
	RequestedBy       string     `json:"requested_by"`
	ApprovedBy        *string    `json:"approved_by"`
	RejectionReason   *string    `json:"rejection_reason"`
	CompletedAt       *time.Time `json:"completed_at"`
	CreatedAt         time.Time  `gorm:"not null;default:now()" json:"created_at"`
	UpdatedAt         time.Time  `gorm:"not null;default:now()" json:"updated_at"`
}

// ModelVersion stores versioned model metadata
type ModelVersion struct {
	ID                int64      `gorm:"primaryKey" json:"id"`
	ModelID           string     `gorm:"not null;index:model_versions_model,priority:1" json:"model_id"`
	TenantID          string     `gorm:"not null;index:model_versions_model,priority:2" json:"tenant_id"`
	ModelName         string     `gorm:"not null" json:"model_name"`
	ModelVersion      string     `gorm:"not null" json:"model_version"` // v1, v2, etc.
	ModelPath         string     `json:"model_path"`                    // path to saved model artifacts
	TrainingDataCount int        `json:"training_data_count"`
	TrainingDate      time.Time  `json:"training_date"`
	ValidationScore   *float64   `json:"validation_score"`
	ApprovedBy        *string    `json:"approved_by"`
	DeploymentStatus  string     `gorm:"not null;default:trained" json:"deployment_status"` // trained, pending_approval, approved, deployed, rejected
	ActiveUntil       *time.Time `json:"active_until"`
	CreatedAt         time.Time  `gorm:"not null;default:now()" json:"created_at"`
	UpdatedAt         time.Time  `gorm:"not null;default:now()" json:"updated_at"`
}

// Index to find latest active model version for a tenant
type ActiveModelVersion struct {
	ID            int64     `gorm:"primaryKey" json:"id"`
	TenantID      string    `gorm:"uniqueIndex;not null" json:"tenant_id"`
	ActiveModelID string    `gorm:"not null" json:"active_model_id"` // FK to ModelVersion.ModelID
	ActiveVersion string    `gorm:"not null" json:"active_version"`  // current version string (v1, v2, etc.)
	UpdatedAt     time.Time `json:"updated_at"`
}
