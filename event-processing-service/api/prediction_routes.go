package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"event-processing-service/db"
)

// GetPendingPredictions retrieves predictions pending human review
func getPendingPredictions(gdb *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := c.Param("tenantID")
		limit := 50

		if l := c.Query("limit"); l != "" {
			if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 1000 {
				limit = parsed
			}
		}

		ctx := c.Request.Context()
		predictions, err := db.GetPendingPredictions(ctx, gdb, tenantID, limit)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"tenant_id":    tenantID,
			"count":        len(predictions),
			"predictions":  predictions,
		})
	}
}

// GetReviewedPredictions retrieves reviewed predictions
func getReviewedPredictions(gdb *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := c.Param("tenantID")
		limit := 50

		if l := c.Query("limit"); l != "" {
			if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 1000 {
				limit = parsed
			}
		}

		ctx := c.Request.Context()
		reviews, err := db.GetReviewsByTenant(ctx, gdb, tenantID, limit)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"tenant_id": tenantID,
			"count":     len(reviews),
			"reviews":   reviews,
		})
	}
}

// CountTrainingEligibleReviews returns count of reviews eligible for retraining
func countTrainingEligibleReviews(gdb *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := c.Param("tenantID")
		ctx := c.Request.Context()

		count, err := db.CountTrainingEligibleReviews(ctx, gdb, tenantID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"tenant_id":                  tenantID,
			"training_eligible_count":    count,
		})
	}
}

// GetRetrainingConfig retrieves retraining configuration for a tenant
func getRetrainingConfig(gdb *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := c.Param("tenantID")
		ctx := c.Request.Context()

		config, err := db.GetRetrainingConfig(ctx, gdb, tenantID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		if config == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "configuration not found"})
			return
		}

		c.JSON(http.StatusOK, config)
	}
}

// UpdateRetrainingConfigRequest is the request body for updating config
type UpdateRetrainingConfigRequest struct {
	MinimumReviewedRecords int    `json:"minimum_reviewed_records" binding:"required,min=1"`
	AutoRetrainEnabled     bool   `json:"auto_retrain_enabled"`
	RequiresManualApproval bool   `json:"requires_manual_approval"`
	UpdatedBy              string `json:"updated_by" binding:"required"`
}

// UpdateRetrainingConfig updates retraining configuration
func updateRetrainingConfig(gdb *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := c.Param("tenantID")

		var req UpdateRetrainingConfigRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		ctx := c.Request.Context()

		config := &db.RetrainingConfig{
			TenantID:               tenantID,
			MinimumReviewedRecords: req.MinimumReviewedRecords,
			AutoRetrainEnabled:     req.AutoRetrainEnabled,
			RequiresManualApproval: req.RequiresManualApproval,
			UpdatedBy:              req.UpdatedBy,
		}

		err := db.UpsertRetrainingConfig(ctx, gdb, config)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, config)
	}
}

// GetRetrainingRequestByIDRequest retrieves a specific retraining request
func getRetrainingRequest(gdb *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := c.Param("tenantID")
		requestID := c.Param("requestID")

		ctx := c.Request.Context()
		req, err := db.GetRetrainingRequestByID(ctx, gdb, tenantID, requestID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		if req == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "request not found"})
			return
		}

		c.JSON(http.StatusOK, req)
	}
}

// ApproveRetrainingRequestBody is the request body for approving retraining
type ApproveRetrainingRequestBody struct {
	ApprovedBy string `json:"approved_by" binding:"required"`
}

// ApproveRetrainingRequest approves a retraining request
func approveRetrainingRequest(gdb *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := c.Param("tenantID")
		requestID := c.Param("requestID")

		var req ApproveRetrainingRequestBody
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		ctx := c.Request.Context()
		err := db.UpdateRetrainingRequestStatus(ctx, gdb, tenantID, requestID, "approved")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// Update approved_by field
		dbReq, err := db.GetRetrainingRequestByID(ctx, gdb, tenantID, requestID)
		if err == nil && dbReq != nil {
			dbReq.ApprovedBy = &req.ApprovedBy
			gdb.Save(dbReq)
		}

		c.JSON(http.StatusOK, gin.H{"status": "approved", "request_id": requestID})
	}
}

// GetModelVersionsRequest retrieves model versions for a tenant
func getModelVersions(gdb *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := c.Param("tenantID")
		status := c.Query("status")
		limit := 100

		if l := c.Query("limit"); l != "" {
			if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 1000 {
				limit = parsed
			}
		}

		ctx := c.Request.Context()
		var versions []db.ModelVersion

		if status != "" {
			versions, _ = db.GetModelVersionsByStatus(ctx, gdb, tenantID, status, limit)
		} else {
			versions = []db.ModelVersion{}
		}

		c.JSON(http.StatusOK, gin.H{
			"tenant_id": tenantID,
			"count":     len(versions),
			"versions":  versions,
		})
	}
}

// GetActiveModelVersion retrieves the active model for a tenant
func getActiveModelVersion(gdb *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := c.Param("tenantID")
		ctx := c.Request.Context()

		active, err := db.GetActiveModelVersion(ctx, gdb, tenantID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		if active == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "no active model set"})
			return
		}

		c.JSON(http.StatusOK, active)
	}
}

// ApproveModelVersionRequest is the request body for model approval
type ApproveModelVersionRequest struct {
	ApprovedBy string `json:"approved_by" binding:"required"`
}

// ApproveModelVersion marks a model version as approved
func approveModelVersion(gdb *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := c.Param("tenantID")
		modelID := c.Param("modelID")
		version := c.Param("version")

		var req ApproveModelVersionRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		ctx := c.Request.Context()
		err := db.UpdateModelVersionStatus(ctx, gdb, tenantID, modelID, version, "approved")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"status": "approved", "model_id": modelID, "version": version})
	}
}

// DeployModelVersion marks a model version as deployed (active)
func deployModelVersion(gdb *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := c.Param("tenantID")
		modelID := c.Param("modelID")
		version := c.Param("version")

		ctx := c.Request.Context()
		err := db.SetActiveModelVersion(ctx, gdb, tenantID, modelID, version)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		err = db.UpdateModelVersionStatus(ctx, gdb, tenantID, modelID, version, "deployed")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"status": "deployed", "model_id": modelID, "version": version})
	}
}
