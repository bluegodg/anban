package greeting

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterRoutes(r gin.IRoutes) {
	r.POST("/greetings/trigger", h.trigger)
	r.GET("/greetings/schedule", h.getSchedule)
	r.PUT("/greetings/schedule", h.updateSchedule)
}

func (h *Handler) trigger(c *gin.Context) {
	var req TriggerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求体无效"})
		return
	}

	greeting, err := h.service.Trigger(c.Request.Context(), req)
	if errors.Is(err, ErrInvalidInput) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "deviceId 必填"})
		return
	}
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "问候触发失败", "greeting": greeting})
		return
	}

	c.JSON(http.StatusCreated, greeting)
}

func (h *Handler) getSchedule(c *gin.Context) {
	schedule, err := h.service.GetSchedule(c.Request.Context(), c.Query("deviceId"))
	if errors.Is(err, ErrInvalidInput) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "deviceId 必填"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "问候时段读取失败"})
		return
	}

	c.JSON(http.StatusOK, schedule)
}

func (h *Handler) updateSchedule(c *gin.Context) {
	var req ScheduleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求体无效"})
		return
	}

	schedule, err := h.service.UpdateSchedule(c.Request.Context(), req)
	if errors.Is(err, ErrInvalidInput) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "deviceId、slots 和 HH:MM 时间必填"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "问候时段保存失败"})
		return
	}

	c.JSON(http.StatusOK, schedule)
}
