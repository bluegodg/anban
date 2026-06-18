package vision

import (
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"io"
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
	r.POST("/vision/look", h.look)
	r.POST("/vision/capture", h.capture)
	r.POST("/vision/check-presence", h.checkPresence)
	r.POST("/vision/presence", h.observePresence)
	r.GET("/vision/captures", h.listCaptures)
	r.GET("/vision/captures/:captureId", h.getCapture)
	r.GET("/vision/captures/:captureId/image", h.captureImage)
	r.POST("/vision/captures/:captureId/reanalyze", h.reanalyzeCapture)
}

func (h *Handler) RegisterDeviceRoutes(r gin.IRoutes, ingressToken string) {
	r.POST("/device/vision", func(c *gin.Context) {
		h.deviceVisionUpload(c, ingressToken)
	})
}

func (h *Handler) deviceVisionUpload(c *gin.Context, ingressToken string) {
	providedToken := c.Query("ingress_token")
	if !secureTokenEqual(providedToken, ingressToken) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	if err := c.Request.ParseMultipartForm(maxVisionImageBytes); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "image_upload_invalid"})
		return
	}
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "image_upload_invalid"})
		return
	}
	defer file.Close()

	image, err := io.ReadAll(io.LimitReader(file, maxVisionImageBytes+1))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "image_upload_invalid"})
		return
	}
	if len(image) > maxVisionImageBytes {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "image_too_large"})
		return
	}
	contentType := strings.TrimSpace(header.Header.Get("Content-Type"))
	if !isSupportedImageContentType(contentType) {
		if detected := http.DetectContentType(image); isSupportedImageContentType(detected) {
			contentType = detected
		}
	}

	resp, err := h.service.HandleDeviceVisionUpload(c.Request.Context(), DeviceVisionUpload{
		DeviceID:      c.GetHeader("Device-Id"),
		ClientID:      c.GetHeader("Client-Id"),
		Authorization: c.GetHeader("Authorization"),
		Question:      c.PostForm("question"),
		FileName:      header.Filename,
		ContentType:   contentType,
		Image:         image,
	})
	if errors.Is(err, ErrImageTooLarge) {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "image_too_large"})
		return
	}
	if errors.Is(err, ErrImageUploadInvalid) || errors.Is(err, ErrInvalidInput) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "image_upload_invalid"})
		return
	}
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "vision_analysis_failed"})
		return
	}
	if resp.ContentType != "" {
		c.Header("Content-Type", resp.ContentType)
	}
	c.Status(resp.StatusCode)
	_, _ = c.Writer.Write(resp.Body)
}

func secureTokenEqual(provided, expected string) bool {
	if expected == "" {
		return false
	}
	providedHash := sha256.Sum256([]byte(provided))
	expectedHash := sha256.Sum256([]byte(expected))
	return subtle.ConstantTimeCompare(providedHash[:], expectedHash[:]) == 1
}

func (h *Handler) look(c *gin.Context) {
	var req LookRequest
	if c.Request.ContentLength != 0 {
		if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "请求体无效"})
			return
		}
	}
	applyLookDeviceContext(c, &req)

	result, err := h.service.Look(c.Request.Context(), req)
	if errors.Is(err, ErrCaptureInProgress) {
		c.JSON(http.StatusConflict, gin.H{"error": "capture_in_progress", "capture": result})
		return
	}
	if errors.Is(err, ErrInvalidInput) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "device_not_bound"})
		return
	}
	if err != nil {
		code := result.FailureCode
		if code == "" {
			code = "camera_tool_unavailable"
		}
		c.JSON(http.StatusBadGateway, gin.H{"error": code, "capture": result})
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *Handler) getCapture(c *gin.Context) {
	deviceID := requestDeviceID(c)
	result, err := h.service.GetCapture(c.Request.Context(), deviceID, c.Param("captureId"))
	if errors.Is(err, ErrInvalidInput) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "device_not_bound"})
		return
	}
	if errors.Is(err, ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "capture_not_found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "capture_read_failed"})
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *Handler) listCaptures(c *gin.Context) {
	limit, _ := strconv.Atoi(c.Query("limit"))
	result, err := h.service.ListCaptures(c.Request.Context(), CaptureListRequest{
		DeviceID: requestDeviceID(c),
		Limit:    limit,
	})
	if errors.Is(err, ErrInvalidInput) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "device_not_bound"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "capture_read_failed"})
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *Handler) captureImage(c *gin.Context) {
	deviceID := requestDeviceID(c)
	image, err := h.service.ReadCaptureImage(c.Request.Context(), deviceID, c.Param("captureId"))
	if errors.Is(err, ErrInvalidInput) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "device_not_bound"})
		return
	}
	if errors.Is(err, ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "capture_not_found"})
		return
	}
	if errors.Is(err, ErrCaptureExpired) {
		c.JSON(http.StatusGone, gin.H{"error": "capture_expired"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "capture_image_read_failed"})
		return
	}
	c.Header("Content-Type", image.ContentType)
	c.Header("Content-Length", strconv.FormatInt(image.Size, 10))
	c.Header("Cache-Control", "private, max-age=60")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Status(http.StatusOK)
	_, _ = c.Writer.Write(image.Bytes)
}

func (h *Handler) reanalyzeCapture(c *gin.Context) {
	result, err := h.service.ReanalyzeCapture(c.Request.Context(), ReanalyzeRequest{
		DeviceID:  requestDeviceID(c),
		CaptureID: c.Param("captureId"),
	})
	if errors.Is(err, ErrInvalidInput) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "device_not_bound"})
		return
	}
	if errors.Is(err, ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "capture_not_found"})
		return
	}
	if errors.Is(err, ErrCaptureExpired) {
		c.JSON(http.StatusGone, gin.H{"error": "capture_expired"})
		return
	}
	if errors.Is(err, ErrStoreUnavailable) {
		c.JSON(http.StatusBadGateway, gin.H{"error": "vision_analysis_failed", "capture": result})
		return
	}
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "vision_analysis_failed", "capture": result})
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *Handler) capture(c *gin.Context) {
	var req CaptureRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求体无效"})
		return
	}
	applyDeviceContext(c, &req)

	result, err := h.service.Capture(c.Request.Context(), req)
	if errors.Is(err, ErrInvalidInput) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "deviceId 必填"})
		return
	}
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "视觉采帧失败"})
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *Handler) checkPresence(c *gin.Context) {
	var req CaptureRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求体无效"})
		return
	}
	applyDeviceContext(c, &req)

	result, err := h.service.CaptureAndObservePresence(c.Request.Context(), req)
	if errors.Is(err, ErrInvalidInput) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "deviceId 必填"})
		return
	}
	if errors.Is(err, ErrPresenceUnavailable) {
		c.JSON(http.StatusBadGateway, gin.H{"error": "视觉 presence 缺失", "result": result})
		return
	}
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "视觉采帧判定失败", "result": result})
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *Handler) observePresence(c *gin.Context) {
	var req PresenceObservationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求体无效"})
		return
	}
	if c.GetString(sharedtypes.GinContextAuthMode) == "account" {
		req.DeviceID = c.GetString(sharedtypes.GinContextDeviceID)
	}

	result, err := h.service.ObservePresence(c.Request.Context(), req)
	if errors.Is(err, ErrInvalidInput) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "deviceId 和 presence 必填"})
		return
	}
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "视觉触发问候失败", "result": result})
		return
	}
	c.JSON(http.StatusOK, result)
}

func applyDeviceContext(c *gin.Context, req *CaptureRequest) {
	if c.GetString(sharedtypes.GinContextAuthMode) == "account" {
		req.DeviceID = c.GetString(sharedtypes.GinContextDeviceID)
	}
}

func applyLookDeviceContext(c *gin.Context, req *LookRequest) {
	if c.GetString(sharedtypes.GinContextAuthMode) == "account" {
		req.DeviceID = c.GetString(sharedtypes.GinContextDeviceID)
	}
}

func requestDeviceID(c *gin.Context) string {
	if c.GetString(sharedtypes.GinContextAuthMode) == "account" {
		return c.GetString(sharedtypes.GinContextDeviceID)
	}
	return c.Query("deviceId")
}
