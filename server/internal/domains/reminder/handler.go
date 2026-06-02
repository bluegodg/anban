package reminder

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterRoutes(r gin.IRoutes) {
	r.POST("/reminders", h.create)
	r.GET("/reminders", h.list)
	r.DELETE("/reminders/:id", h.cancel)
	r.POST("/reminders/:id/ack", h.ack)
}

func (h *Handler) create(c *gin.Context) {
	var req CreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求体无效"})
		return
	}

	rem, err := h.service.Create(c.Request.Context(), req)
	if errors.Is(err, ErrInvalidInput) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "deviceId、scheduledAt 和 content 必填"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "提醒创建失败", "reminder": rem})
		return
	}

	c.JSON(http.StatusCreated, rem)
}

func (h *Handler) list(c *gin.Context) {
	filter := ListFilter{
		DeviceID: c.Query("deviceId"),
		Status:   Status(c.Query("status")),
	}
	reminders, err := h.service.List(c.Request.Context(), filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "提醒列表读取失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"reminders": reminders})
}

func (h *Handler) cancel(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "reminder id 无效"})
		return
	}

	rem, err := h.service.Cancel(c.Request.Context(), uint(id))
	if errors.Is(err, ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "提醒不存在"})
		return
	}
	if errors.Is(err, ErrInvalidInput) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "reminder id 无效"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "提醒撤销失败"})
		return
	}

	c.JSON(http.StatusOK, rem)
}

func (h *Handler) ack(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "reminder id 无效"})
		return
	}

	var req AckRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求体无效"})
		return
	}

	rem, err := h.service.Acknowledge(c.Request.Context(), uint(id), req)
	if errors.Is(err, ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "提醒不存在"})
		return
	}
	if errors.Is(err, ErrInvalidInput) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "提醒尚不可确认"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "提醒确认失败"})
		return
	}

	c.JSON(http.StatusOK, rem)
}
