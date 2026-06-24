package status

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

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
	r.GET("/status", h.get)
	r.GET("/device/status", h.get)
	r.GET("/device/history", h.history)
	r.GET("/device/panel", h.panel)
	r.POST("/device/volume", h.setVolume)
}

func (h *Handler) panel(c *gin.Context) {
	panel, err := h.service.GetDevicePanel(c.Request.Context(), deviceIDFromContext(c, c.Query("deviceId")))
	if errors.Is(err, ErrInvalidInput) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "deviceId 必填"})
		return
	}
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "设备状态读取失败"})
		return
	}
	c.JSON(http.StatusOK, panel)
}

func (h *Handler) setVolume(c *gin.Context) {
	var body struct {
		DeviceID string `json:"deviceId"`
		Volume   *int   `json:"volume"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.Volume == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "volume 必填（0-100 整数）"})
		return
	}
	err := h.service.SetVolume(c.Request.Context(), SetVolumeRequest{
		DeviceID: deviceIDFromContext(c, body.DeviceID),
		Volume:   *body.Volume,
	})
	if errors.Is(err, ErrInvalidInput) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "volume 必须是 0-100 的整数"})
		return
	}
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "设置音量失败，请确认设备在线"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "volume": *body.Volume})
}

func (h *Handler) get(c *gin.Context) {
	snapshot, err := h.service.Get(c.Request.Context(), GetRequest{DeviceID: deviceIDFromContext(c, c.Query("deviceId"))})
	if errors.Is(err, ErrInvalidInput) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "deviceId 必填"})
		return
	}
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "设备状态读取失败"})
		return
	}

	c.JSON(http.StatusOK, snapshot)
}

func (h *Handler) history(c *gin.Context) {
	limit, err := parseHistoryLimit(c.Query("limit"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "limit 必须是正整数"})
		return
	}

	history, err := h.service.GetHistory(c.Request.Context(), HistoryRequest{
		DeviceID: deviceIDFromContext(c, c.Query("deviceId")),
		Limit:    limit,
	})
	if errors.Is(err, ErrInvalidInput) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "deviceId 必填"})
		return
	}
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "对话记录读取失败"})
		return
	}

	c.JSON(http.StatusOK, history)
}

func parseHistoryLimit(raw string) (int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, nil
	}
	limit, err := strconv.Atoi(raw)
	if err != nil || limit <= 0 {
		return 0, ErrInvalidInput
	}
	return limit, nil
}

func deviceIDFromContext(c *gin.Context, fallback string) string {
	if c.GetString(sharedtypes.GinContextAuthMode) == "account" {
		return c.GetString(sharedtypes.GinContextDeviceID)
	}
	return fallback
}
