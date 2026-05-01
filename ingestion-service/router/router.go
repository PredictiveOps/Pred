package router

import (
	"ingestion-service/handlers"

	"github.com/gin-gonic/gin"
)

func NewRouter() *gin.Engine {
	r := gin.Default()

	r.GET("/health", handlers.HealthCheck)
	r.POST("/devices", handlers.RegisterDevice)
	r.GET("/devices", handlers.GetDevices)
	r.GET("/devices/:id", handlers.GetDeviceByID)

	return r
}
