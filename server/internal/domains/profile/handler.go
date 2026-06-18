package profile

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
	r.GET("/profile", h.get)
	r.PUT("/profile", h.update)
	r.POST("/profile", h.update)
}

func (h *Handler) get(c *gin.Context) {
	profile, err := h.service.Get(c.Request.Context(), deviceIDFromContext(c, c.Query("deviceId")))
	if errors.Is(err, ErrInvalidInput) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "deviceId 必填"})
		return
	}
	if errors.Is(err, ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "画像不存在"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "画像读取失败"})
		return
	}

	c.JSON(http.StatusOK, profile)
}

func (h *Handler) update(c *gin.Context) {
	var req UpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求体无效"})
		return
	}
	if c.GetString(sharedtypes.GinContextAuthMode) == "account" {
		req.DeviceID = c.GetString(sharedtypes.GinContextDeviceID)
	}

	profile, err := h.service.Update(c.Request.Context(), req)
	if errors.Is(err, ErrInvalidInput) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "deviceId 必填"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "画像保存失败", "profile": profile})
		return
	}

	c.JSON(http.StatusOK, profile)
}

func deviceIDFromContext(c *gin.Context, fallback string) string {
	if c.GetString(sharedtypes.GinContextAuthMode) == "account" {
		return c.GetString(sharedtypes.GinContextDeviceID)
	}
	return fallback
}
