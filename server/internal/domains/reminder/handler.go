package reminder

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
	r.POST("/reminders", h.create)
	r.GET("/reminders", h.list)
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
