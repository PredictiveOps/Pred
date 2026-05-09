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
	Features         datatypes.JSON `gorm:"type:jsonb;not null"`                         // {rms, kurtosis, crest_factor, spectral_energy, temperature, ...}
	FeatureVersion   string         `gorm:"default:v1"`                                  // schema version for features
	DataFormat       string         `gorm:"default:old;index:processed_features_format"` // old, new, mixed - tracks sensor data format
	CreatedAt        time.Time      `gorm:"not null;default:now();index:processed_features_timestamp"`
	FeatureTimestamp time.Time      `gorm:"not null"` // when the feature was collected
}

// Prediction stores ML model predictions before review
type Prediction struct {
	ID              int64     `gorm:"primaryKey" json:"id"`
	PredictionID    string    `gorm:"uniqueIndex;not null" json:"prediction_id"` // auto-generated unique ID
	TenantID        string    `gorm:"not null;index:predictions_tenant" json:"tenant_id"`
	DeviceID        uint      `gorm:"not null;index:predictions_asset,priority:1" json:"device_id"`
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
	DeviceID           uint      `gorm:"not null" json:"device_id"`
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
	ID                     int64  `gorm:"primaryKey"`
	TenantID               string `gorm:"uniqueIndex;not null"`
	MinimumReviewedRecords int    `gorm:"not null;default:500"`
	AutoRetrainEnabled     bool   `gorm:"not null;default:false"`
	RequiresManualApproval bool   `gorm:"not null;default:true"`
	UpdatedBy              string
	UpdatedAt              time.Time `gorm:"not null;default:now()"`
}

// RetrainingRequest tracks retraining workflow
type RetrainingRequest struct {
	ID                int64  `gorm:"primaryKey"`
	RequestID         string `gorm:"uniqueIndex;not null"`
	TenantID          string `gorm:"not null;index:retraining_requests_tenant"`
	Status            string `gorm:"not null;default:created"` // created, approved, in_progress, completed, failed, rejected
	TrainingDataCount int
	RequestedBy       string
	ApprovedBy        *string
	RejectionReason   *string
	CompletedAt       *time.Time
	CreatedAt         time.Time `gorm:"not null;default:now()"`
	UpdatedAt         time.Time `gorm:"not null;default:now()"`
}

// ModelVersion stores versioned model metadata
type ModelVersion struct {
	ID                int64  `gorm:"primaryKey"`
	ModelID           string `gorm:"not null;index:model_versions_model,priority:1"`
	TenantID          string `gorm:"not null;index:model_versions_model,priority:2"`
	ModelName         string `gorm:"not null"`
	ModelVersion      string `gorm:"not null"` // v1, v2, etc.
	ModelPath         string // path to saved model artifacts
	TrainingDataCount int
	TrainingDate      time.Time
	ValidationScore   *float64
	ApprovedBy        *string
	DeploymentStatus  string `gorm:"not null;default:trained"` // trained, pending_approval, approved, deployed, rejected
	ActiveUntil       *time.Time
	CreatedAt         time.Time `gorm:"not null;default:now()"`
	UpdatedAt         time.Time `gorm:"not null;default:now()"`
}

// Index to find latest active model version for a tenant
type ActiveModelVersion struct {
	ID            int64  `gorm:"primaryKey"`
	TenantID      string `gorm:"uniqueIndex;not null"`
	ActiveModelID string `gorm:"not null"` // FK to ModelVersion.ModelID
	ActiveVersion string `gorm:"not null"` // current version string (v1, v2, etc.)
	UpdatedAt     time.Time
}
