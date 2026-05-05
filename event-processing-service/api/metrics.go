package api

import (
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"gorm.io/gorm"
)

func metricsHandler(_ *gorm.DB) gin.HandlerFunc {
	return gin.WrapH(promhttp.Handler())
}
