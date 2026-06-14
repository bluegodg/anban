package message

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
	r.POST("/messages", h.create)
	r.GET("/messages", h.list)
}

func (h *Handler) create(c *gin.Context) {
	var req SendRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求体无效"})
		return
	}

	msg, err := h.service.Send(c.Request.Context(), req)
	if errors.Is(err, ErrInvalidInput) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "deviceId 和 text 必填"})
		return
	}
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "留言播报失败", "message": msg})
		return
	}

	if msg.Status == StatusPending {
		c.JSON(http.StatusAccepted, msg)
		return
	}
	c.JSON(http.StatusCreated, msg)
}

func (h *Handler) list(c *gin.Context) {
	filter := ListFilter{
		DeviceID: c.Query("deviceId"),
		Status:   Status(c.Query("status")),
	}
	msgs, err := h.service.List(c.Request.Context(), filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "留言列表读取失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"messages": msgs})
}
