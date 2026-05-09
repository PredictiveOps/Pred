package router

import (
	"ingestion-service/handlers"
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
	{http.MethodGet, "/health", handlers.HealthCheck},
	{http.MethodPost, "/devices/register", handlers.RegisterDeviceHTTP},
	{http.MethodGet, "/devices/:device_id", handlers.GetDeviceByIDHandler},
	{http.MethodGet, "/devices", handlers.GetDevicesByTenantIDHandler},
	{http.MethodPut, "/devices/:device_id/status", handlers.UpdateDeviceActiveStatusHandler},
	{http.MethodDelete, "/devices/:device_id", handlers.DeleteDeviceHandler},
	{http.MethodGet, "/metrics", handlers.PrometheusHandler},
}

func NewRouter(gdb *gorm.DB) *gin.Engine {
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())

	for _, rt := range routes {
		r.Handle(rt.method, rt.path, rt.handler(gdb))
	}

	return r
}
