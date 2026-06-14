package status

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

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
}

func (h *Handler) get(c *gin.Context) {
	snapshot, err := h.service.Get(c.Request.Context(), GetRequest{DeviceID: c.Query("deviceId")})
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
		DeviceID: c.Query("deviceId"),
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
