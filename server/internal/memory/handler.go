package memory

import (
	"errors"
	"net/http"
	"strconv"

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
	r.GET("/memory/facts", h.list)
	r.POST("/memory/facts", h.create)
	r.PUT("/memory/facts/:id", h.update)
	r.DELETE("/memory/facts/:id", h.delete)
}

func (h *Handler) list(c *gin.Context) {
	limit, _ := strconv.Atoi(c.Query("limit"))
	facts, err := h.service.ListFacts(c.Request.Context(), deviceIDFromContext(c, c.Query("deviceId")), limit)
	if errors.Is(err, ErrInvalidInput) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "deviceId 必填"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "记忆读取失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"facts": facts})
}

func (h *Handler) create(c *gin.Context) {
	var req FactRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求体无效"})
		return
	}
	req.DeviceID = deviceIDFromContext(c, req.DeviceID)

	fact, err := h.service.AddManualFact(c.Request.Context(), req)
	if errors.Is(err, ErrInvalidInput) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "记忆内容必填"})
		return
	}
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "记忆同步失败", "fact": fact})
		return
	}
	c.JSON(http.StatusOK, fact)
}

func (h *Handler) update(c *gin.Context) {
	factID, err := parseFactID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "记忆 ID 无效"})
		return
	}
	var req FactRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求体无效"})
		return
	}
	deviceID := deviceIDFromContext(c, req.DeviceID)

	fact, err := h.service.UpdateFact(c.Request.Context(), deviceID, factID, req)
	if errors.Is(err, ErrInvalidInput) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "记忆内容必填"})
		return
	}
	if errors.Is(err, ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "记忆不存在"})
		return
	}
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "记忆同步失败", "fact": fact})
		return
	}
	c.JSON(http.StatusOK, fact)
}

func (h *Handler) delete(c *gin.Context) {
	factID, err := parseFactID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "记忆 ID 无效"})
		return
	}
	deviceID := deviceIDFromContext(c, c.Query("deviceId"))

	err = h.service.DeleteFact(c.Request.Context(), deviceID, factID)
	if errors.Is(err, ErrInvalidInput) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "deviceId 必填"})
		return
	}
	if errors.Is(err, ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "记忆不存在"})
		return
	}
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "记忆同步失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

func deviceIDFromContext(c *gin.Context, fallback string) string {
	if c.GetString(sharedtypes.GinContextAuthMode) == "account" {
		return c.GetString(sharedtypes.GinContextDeviceID)
	}
	return fallback
}

func parseFactID(value string) (uint, error) {
	id, err := strconv.ParseUint(value, 10, 64)
	if err != nil || id == 0 {
		return 0, ErrInvalidInput
	}
	return uint(id), nil
}
