package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"ingestion-service/db"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

var registrationResponseTopicTemplate string

func SetRegistrationResponseTopicTemplate(template string) {
	registrationResponseTopicTemplate = template
}

// RegisterDeviceHTTP handles the HTTP endpoint for device registration.
// Body: { "device_id": uint, "tenant_id": uint }
func RegisterDeviceHTTP(gdb *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req db.DeviceHTTPRegistrationRequest

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request payload"})
			return
		}
		if req.DeviceID == 0 || req.TenantID == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "device_id and tenant_id are required"})
			return
		}

		device, err := db.RegisterDeviceForTenant(req.DeviceID, req.TenantID)
		if err != nil {
			log.Printf("failed to register device: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to register device"})
			return
		}
		if redisCache != nil {
			go func(deviceID uint) {
				if err := redisCache.CacheDeviceState(context.Background(), deviceID, false, ""); err != nil {
					log.Printf("failed to cache initial device state: device_id=%d err=%v", deviceID, err)
				}
			}(req.DeviceID)
		}
		log.Printf("device registered: device_id=%d tenant_id=%d", device.DeviceID, device.TenantID)
		c.JSON(http.StatusCreated, db.DeviceRegistrationResponse{RegistrationStatus: "ok"})
	}
}

// GetDeviceByIDHandler retrieves a device by its device ID.
func GetDeviceByIDHandler(gdb *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		deviceIDParam := c.Param("device_id")
		deviceID64, err := strconv.ParseUint(deviceIDParam, 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid device_id"})
			return
		}

		device, err := db.GetDeviceByID(uint(deviceID64))
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				c.JSON(http.StatusNotFound, gin.H{"error": "device not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve device"})
			return
		}

		// Return DeviceDetails (public fields)
		details := &db.DeviceDetails{
			DeviceID:  device.DeviceID,
			TenantID:  device.TenantID,
			IsActive:  device.IsActive,
			CreatedAt: device.CreatedAt,
			UpdatedAt: device.UpdatedAt,
		}
		c.JSON(http.StatusOK, details)
	}
}

// GetDevicesByTenantIDHandler retrieves all devices (details) for a tenant.
func GetDevicesByTenantIDHandler(gdb *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantIDParam := c.Param("tenant_id")
		tenantID64, err := strconv.ParseUint(tenantIDParam, 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid tenant_id"})
			return
		}

		details, err := db.GetDeviceDetailsByTenantID(uint(tenantID64))
		if err != nil {
			log.Printf("failed to retrieve devices: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve devices"})
			return
		}

		if details == nil {
			details = []db.DeviceDetails{}
		}
		c.JSON(http.StatusOK, details)
	}
}

// UpdateDeviceActiveStatusHandler updates the is_active flag for a device.
// Body: { "is_active": bool }
func UpdateDeviceActiveStatusHandler(gdb *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		deviceIDParam := c.Param("device_id")
		deviceID64, err := strconv.ParseUint(deviceIDParam, 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid device_id"})
			return
		}

		var req struct {
			IsActive bool `json:"is_active"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request payload"})
			return
		}

		if err := db.UpdateDeviceActiveStatus(uint(deviceID64), req.IsActive); err != nil {
			log.Printf("failed to update device status: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update device status"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"status": "updated"})
	}
}

// DeleteDeviceHandler deletes a device by its device ID.
func DeleteDeviceHandler(gdb *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		deviceIDParam := c.Param("device_id")
		deviceID64, err := strconv.ParseUint(deviceIDParam, 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid device_id"})
			return
		}

		deviceID := uint(deviceID64)
		if err := db.DeleteDeviceByID(deviceID); err != nil {
			log.Printf("failed to delete device: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete device"})
			return
		}

		if redisCache != nil {
			go func(did uint) {
				if err := redisCache.CacheDeviceState(context.Background(), did, false, ""); err != nil {
					log.Printf("failed to update cached device state for deleted device_id=%d: %v", did, err)
				}
				if err := redisCache.CacheDevicePublicKey(context.Background(), did, ""); err != nil {
					log.Printf("failed to clear cached public key for deleted device_id=%d: %v", did, err)
				}
			}(deviceID)
		}

		c.JSON(http.StatusOK, gin.H{"status": "deleted"})
	}
}

func HandleMQTTDeviceRegistrationWithTemplate(client mqtt.Client, msg mqtt.Message, responseTopicTemplate string) {
	deviceID, topicKind, err := parseDeviceTopic(msg.Topic())
	if err != nil {
		log.Printf("invalid registration topic: %s: %v", msg.Topic(), err)
		return
	}
	if topicKind != mqttTopicRegistration {
		log.Printf("unexpected topic kind for registration handler: %s", topicKind)
		return
	}

	var req db.DeviceRegistrationRequest
	if err := json.Unmarshal(msg.Payload(), &req); err != nil {
		log.Printf("invalid registration payload for device_id=%d: %v", deviceID, err)
		return
	}
	if req.PublicKey == "" {
		log.Printf("missing public key for device_id=%d", deviceID)
		return
	}

	if err := db.UpdateDevicePublicKey(deviceID, req.PublicKey); err != nil {
		log.Printf("failed to update public key for device_id=%d: %v", deviceID, err)
		return
	}
	if redisCache != nil {
		go func(did uint, pubKey string) {
			if err := redisCache.CacheDevicePublicKey(context.Background(), did, pubKey); err != nil {
				log.Printf("failed to cache public key for device_id=%d: %v", did, err)
			}
			if err := redisCache.CacheDeviceState(context.Background(), did, true, pubKey); err != nil {
				log.Printf("failed to cache active device state for device_id=%d: %v", did, err)
			}
		}(deviceID, req.PublicKey)
	}

	responseTopic := buildRegistrationResponseTopic(msg.Topic(), responseTopicTemplate)
	responsePayload, err := json.Marshal(db.DeviceRegistrationResponse{RegistrationStatus: "ok"})
	if err != nil {
		log.Printf("failed to marshal registration response for device_id=%d: %v", deviceID, err)
		return
	}

	token := client.Publish(responseTopic, 0, false, responsePayload)
	if token.Wait() && token.Error() != nil {
		log.Printf("failed to publish registration response for device_id=%d: %v", deviceID, token.Error())
		return
	}

	log.Printf("registered device public key: device_id=%d response_topic=%s", deviceID, responseTopic)
}

func buildRegistrationResponseTopic(requestTopic, template string) string {
	deviceID, topicKind, err := parseDeviceTopic(requestTopic)
	if err != nil {
		return requestTopic + "/response"
	}
	if topicKind != mqttTopicRegistration {
		return requestTopic + "/response"
	}

	if template != "" {
		return fmt.Sprintf(template, deviceID)
	}

	return fmt.Sprintf("devices/%d/registration/response", deviceID)
}
