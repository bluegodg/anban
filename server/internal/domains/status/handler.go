package status

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
	r.GET("/status", h.get)
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
