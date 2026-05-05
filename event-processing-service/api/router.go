package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type route struct {
	method  string
	path    string
	handler func(*gorm.DB) gin.HandlerFunc
}

var routes = []route{
	{http.MethodGet, "/health", health},
	{http.MethodGet, "/tenants/:tenantID/events", listEvents},

	// Prediction & Review Management
	{http.MethodGet, "/tenants/:tenantID/predictions/pending", getPendingPredictions},
	{http.MethodGet, "/tenants/:tenantID/reviews", getReviewedPredictions},
	{http.MethodGet, "/tenants/:tenantID/reviews/training-eligible-count", countTrainingEligibleReviews},

	// Retraining Configuration & Workflow
	{http.MethodGet, "/tenants/:tenantID/retraining/config", getRetrainingConfig},
	{http.MethodPut, "/tenants/:tenantID/retraining/config", updateRetrainingConfig},
	{http.MethodGet, "/tenants/:tenantID/retraining/:requestID", getRetrainingRequest},
	{http.MethodPost, "/tenants/:tenantID/retraining/:requestID/approve", approveRetrainingRequest},

	// Model Version Management
	{http.MethodGet, "/tenants/:tenantID/models/versions", getModelVersions},
	{http.MethodGet, "/tenants/:tenantID/models/active", getActiveModelVersion},
	{http.MethodPost, "/tenants/:tenantID/models/:modelID/:version/approve", approveModelVersion},
	{http.MethodPost, "/tenants/:tenantID/models/:modelID/:version/deploy", deployModelVersion},
}

func NewRouter(gdb *gorm.DB) *gin.Engine {
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())

	for _, rt := range routes {
		r.Handle(rt.method, rt.path, rt.handler(gdb))
	}

	return r
}

func health(_ *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	}
}

func listEvents(_ *gorm.DB) gin.HandlerFunc {
	// TODO: query events scoped by tenant via the db package.
	return func(c *gin.Context) {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "not implemented"})
	}
}
