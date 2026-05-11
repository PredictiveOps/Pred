package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"event-processing-service/db"
)

type route struct {
	method  string
	path    string
	handler func(*gorm.DB) gin.HandlerFunc
}

var routes = []route{
	{http.MethodGet, "/health", health},
	{http.MethodGet, "/events", listAllEvents},
	{http.MethodGet, "/tenants/:tenantID/events", listEvents},

	// Prediction & Review Management
	// {http.MethodGet, "/tenants/:tenantID/predictions/pending", getPendingPredictions},
	// {http.MethodGet, "/tenants/:tenantID/reviews", getReviewedPredictions},
	// {http.MethodGet, "/tenants/:tenantID/reviews/training-eligible-count", countTrainingEligibleReviews},

	// Retraining Configuration & Workflow
	// {http.MethodGet, "/tenants/:tenantID/retraining/config", getRetrainingConfig},
	// {http.MethodPut, "/tenants/:tenantID/retraining/config", updateRetrainingConfig},
	// {http.MethodGet, "/tenants/:tenantID/retraining/:requestID", getRetrainingRequest},
	// {http.MethodPost, "/tenants/:tenantID/retraining/:requestID/approve", approveRetrainingRequest},

	// Model Version Management
	// {http.MethodGet, "/tenants/:tenantID/models/versions", getModelVersions},
	// {http.MethodGet, "/tenants/:tenantID/models/active", getActiveModelVersion},
	// {http.MethodPost, "/tenants/:tenantID/models/:modelID/:version/approve", approveModelVersion},
	// {http.MethodPost, "/tenants/:tenantID/models/:modelID/:version/deploy", deployModelVersion},

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

func listAllEvents(gdb *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		filter, ok := eventFilterFromQuery(c, "")
		if !ok {
			return
		}

		events, err := db.GetEvents(c.Request.Context(), gdb, filter)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"count":  len(events),
			"events": events,
		})
	}
}

func listEvents(gdb *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := c.Param("tenantID")
		filter, ok := eventFilterFromQuery(c, tenantID)
		if !ok {
			return
		}

		events, err := db.GetEvents(c.Request.Context(), gdb, filter)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"tenant_id": tenantID,
			"count":     len(events),
			"events":    events,
		})
	}
}

func eventFilterFromQuery(c *gin.Context, tenantID string) (db.EventFilter, bool) {
	filter := db.EventFilter{
		TenantID: tenantID,
		DeviceID: c.Query("device_id"),
		Status:   c.Query("status"),
		Mode:     c.Query("mode"),
		Limit:    100,
	}

	if filter.TenantID == "" {
		filter.TenantID = c.Query("tenant_id")
	}

	if limit := c.Query("limit"); limit != "" {
		parsed, err := strconv.Atoi(limit)
		if err != nil || parsed < 1 || parsed > 1000 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "limit must be between 1 and 1000"})
			return db.EventFilter{}, false
		}
		filter.Limit = parsed
	}

	if offset := c.Query("offset"); offset != "" {
		parsed, err := strconv.Atoi(offset)
		if err != nil || parsed < 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "offset must be 0 or greater"})
			return db.EventFilter{}, false
		}
		filter.Offset = parsed
	}

	from, ok := parseEventTimeQuery(c, "from")
	if !ok {
		return db.EventFilter{}, false
	}
	to, ok := parseEventTimeQuery(c, "to")
	if !ok {
		return db.EventFilter{}, false
	}
	filter.From = from
	filter.To = to

	return filter, true
}

func parseEventTimeQuery(c *gin.Context, key string) (*time.Time, bool) {
	value := c.Query(key)
	if value == "" {
		return nil, true
	}

	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": key + " must be an RFC3339 timestamp"})
		return nil, false
	}

	return &parsed, true
}
