package vision

import (
	"errors"
	"net/http"

	sharedtypes "github.com/bluegodg/anban/server/pkg/types"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterRoutes(r gin.IRoutes) {
	r.POST("/vision/capture", h.capture)
	r.POST("/vision/check-presence", h.checkPresence)
	r.POST("/vision/presence", h.observePresence)
}

func (h *Handler) capture(c *gin.Context) {
	var req CaptureRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求体无效"})
		return
	}
	applyDeviceContext(c, &req)

	result, err := h.service.Capture(c.Request.Context(), req)
	if errors.Is(err, ErrInvalidInput) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "deviceId 必填"})
		return
	}
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "视觉采帧失败"})
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *Handler) checkPresence(c *gin.Context) {
	var req CaptureRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求体无效"})
		return
	}
	applyDeviceContext(c, &req)

	result, err := h.service.CaptureAndObservePresence(c.Request.Context(), req)
	if errors.Is(err, ErrInvalidInput) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "deviceId 必填"})
		return
	}
	if errors.Is(err, ErrPresenceUnavailable) {
		c.JSON(http.StatusBadGateway, gin.H{"error": "视觉 presence 缺失", "result": result})
		return
	}
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "视觉采帧判定失败", "result": result})
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *Handler) observePresence(c *gin.Context) {
	var req PresenceObservationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求体无效"})
		return
	}
	if c.GetString(sharedtypes.GinContextAuthMode) == "account" {
		req.DeviceID = c.GetString(sharedtypes.GinContextDeviceID)
	}

	result, err := h.service.ObservePresence(c.Request.Context(), req)
	if errors.Is(err, ErrInvalidInput) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "deviceId 和 presence 必填"})
		return
	}
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "视觉触发问候失败", "result": result})
		return
	}
	c.JSON(http.StatusOK, result)
}

func applyDeviceContext(c *gin.Context, req *CaptureRequest) {
	if c.GetString(sharedtypes.GinContextAuthMode) == "account" {
		req.DeviceID = c.GetString(sharedtypes.GinContextDeviceID)
	}
}
