package mindview

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/bluegodg/anban/server/internal/mind"
	sharedtypes "github.com/bluegodg/anban/server/pkg/types"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *mind.ReadService
}

func NewHandler(service *mind.ReadService) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterRoutes(r gin.IRoutes) {
	r.GET("/mind/snapshot", h.snapshot)
	r.GET("/mind/timeline", h.timeline)
}

func (h *Handler) snapshot(c *gin.Context) {
	resp, err := h.service.Snapshot(c.Request.Context(), requestDeviceID(c))
	if errors.Is(err, mind.ErrInvalidInput) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "device_not_bound"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "mind_snapshot_failed"})
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) timeline(c *gin.Context) {
	kind, err := parseTimelineKind(c.Query("kind"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_kind"})
		return
	}
	limit, err := parseTimelineLimit(c.Query("limit"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_limit"})
		return
	}
	resp, err := h.service.Timeline(c.Request.Context(), mind.TimelineQuery{
		DeviceID: requestDeviceID(c),
		Kind:     kind,
		Limit:    limit,
		Cursor:   c.Query("cursor"),
	})
	if errors.Is(err, mind.ErrInvalidCursor) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_cursor"})
		return
	}
	if errors.Is(err, mind.ErrInvalidInput) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "device_not_bound"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "mind_timeline_failed"})
		return
	}
	c.JSON(http.StatusOK, resp)
}

func requestDeviceID(c *gin.Context) string {
	if c.GetString(sharedtypes.GinContextAuthMode) == "account" {
		return c.GetString(sharedtypes.GinContextDeviceID)
	}
	return strings.TrimSpace(c.Query("deviceId"))
}

func parseTimelineKind(raw string) (mind.TimelineKind, error) {
	switch kind := mind.TimelineKind(strings.TrimSpace(raw)); kind {
	case "", mind.TimelineKindAll:
		return mind.TimelineKindAll, nil
	case mind.TimelineKindThought, mind.TimelineKindAction, mind.TimelineKindEvent, mind.TimelineKindReflection:
		return kind, nil
	default:
		return "", mind.ErrInvalidInput
	}
}

func parseTimelineLimit(raw string) (int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, nil
	}
	limit, err := strconv.Atoi(raw)
	if err != nil || limit <= 0 {
		return 0, mind.ErrInvalidInput
	}
	return limit, nil
}
