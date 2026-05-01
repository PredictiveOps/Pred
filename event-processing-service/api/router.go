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
