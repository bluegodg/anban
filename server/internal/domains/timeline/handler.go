package timeline

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

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
	r.GET("/timeline", h.get)
}

func (h *Handler) get(c *gin.Context) {
	limit, err := parseLimit(c.Query("limit"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "limit 必须是正整数"})
		return
	}
	before, err := parseBefore(c.Query("before"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "before 必须是 RFC3339 时间"})
		return
	}
	deviceID := strings.TrimSpace(c.Query("deviceId"))
	elderName := strings.TrimSpace(c.Query("elderDisplayName"))
	if c.GetString(sharedtypes.GinContextAuthMode) == "account" {
		deviceID = c.GetString(sharedtypes.GinContextDeviceID)
		elderName = c.GetString(sharedtypes.GinContextElderDisplayName)
	}
	resp, err := h.service.Get(c.Request.Context(), Request{
		DeviceID:         deviceID,
		ElderDisplayName: elderName,
		Limit:            limit,
		Before:           before,
	})
	if errors.Is(err, ErrInvalidInput) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "deviceId 必填"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "消息时间线读取失败"})
		return
	}
	c.JSON(http.StatusOK, resp)
}

func parseLimit(raw string) (int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return 0, ErrInvalidInput
	}
	return value, nil
}

func parseBefore(raw string) (*time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	value, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return nil, err
	}
	return &value, nil
}
