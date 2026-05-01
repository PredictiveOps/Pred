package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"ingestion-service/db"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func RegisterDevice(c *gin.Context) {
	var device db.Device
	if err := c.ShouldBindJSON(&device); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request payload"})
		return
	}

	if device.Name == "" || device.TenantID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name and tenant_id are required"})
		return
	}

	if err := db.AddDevice(&device); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to register device"})
		return
	}

	c.JSON(http.StatusCreated, device)
}

func GetDevices(c *gin.Context) {
	tenantIDParam := c.Query("tenant_id")
	if tenantIDParam == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tenant_id query param is required"})
		return
	}

	tenantID64, err := strconv.ParseUint(tenantIDParam, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid tenant_id"})
		return
	}

	devices, err := db.GetAllDevicesByTenantID(uint(tenantID64))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve devices"})
		return
	}

	c.JSON(http.StatusOK, devices)
}

func GetDeviceByID(c *gin.Context) {
	idParam := c.Param("id")
	id64, err := strconv.ParseUint(idParam, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid device id"})
		return
	}

	device, err := db.GetDeviceByID(uint(id64))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "device not found"})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve device"})
		return
	}

	c.JSON(http.StatusOK, device)
}
