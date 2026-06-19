// Package openmemory exposes AnBan companion context through the MemOS HTTP shape
// already supported by xiaozhi. Conversation ingestion remains owned by AnBan's
// history poller; these endpoints are a read-only context projection.
package openmemory

import (
	"context"
	"crypto/subtle"
	"errors"
	"net/http"
	"strings"

	"github.com/bluegodg/anban/server/internal/domains/profile"
	"github.com/gin-gonic/gin"
)

type ProfileReader interface {
	Get(ctx context.Context, deviceID string) (profile.Profile, error)
}

type Handler struct {
	token           string
	defaultDeviceID string
	reader          ProfileReader
}

type request struct {
	UserID         string `json:"user_id"`
	ConversationID string `json:"conversation_id"`
	Query          string `json:"query"`
}

func NewHandler(token, defaultDeviceID string, reader ProfileReader) *Handler {
	return &Handler{
		token:           strings.TrimSpace(token),
		defaultDeviceID: strings.TrimSpace(defaultDeviceID),
		reader:          reader,
	}
}

func (h *Handler) RegisterRoutes(r *gin.RouterGroup) {
	r.Use(h.authorize)
	r.POST("/search/memory", h.search)
	r.POST("/get/messages", h.getMessages)
	r.POST("/add/message", h.acknowledgeWrite)
	r.POST("/flush", h.acknowledgeWrite)
	r.POST("/reset/memory", h.acknowledgeWrite)
}

func (h *Handler) authorize(c *gin.Context) {
	want := "Bearer " + h.token
	got := c.GetHeader("Authorization")
	if h.token == "" || len(got) != len(want) || subtle.ConstantTimeCompare([]byte(got), []byte(want)) != 1 {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	c.Next()
}

func (h *Handler) search(c *gin.Context) {
	var req request
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	deviceID := requestDeviceID(req)
	if deviceID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user_id required"})
		return
	}

	// MemOS GetContext calls Search with an empty query at session start. Returning
	// no static copy keeps profile edits effective immediately through per-turn search.
	if strings.TrimSpace(req.Query) == "" {
		writeSearchResult(c, "")
		return
	}

	current, err := h.getProfile(c.Request.Context(), deviceID)
	if errors.Is(err, profile.ErrNotFound) {
		writeSearchResult(c, "")
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "context unavailable"})
		return
	}
	writeSearchResult(c, profile.BuildPromptWith(current.Fields, current.MemoryFacts, current.MindContext))
}

func (h *Handler) getProfile(ctx context.Context, identity string) (profile.Profile, error) {
	current, err := h.reader.Get(ctx, identity)
	if !errors.Is(err, profile.ErrNotFound) || h.defaultDeviceID == "" || h.defaultDeviceID == identity {
		return current, err
	}
	return h.reader.Get(ctx, h.defaultDeviceID)
}

func (h *Handler) getMessages(c *gin.Context) {
	if _, ok := bindIdentity(c); !ok {
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"messages": []any{}}})
}

func (h *Handler) acknowledgeWrite(c *gin.Context) {
	if _, ok := bindIdentity(c); !ok {
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "ok",
		"ingest":  "history_poller",
	})
}

func bindIdentity(c *gin.Context) (string, bool) {
	var req request
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return "", false
	}
	deviceID := requestDeviceID(req)
	if deviceID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user_id required"})
		return "", false
	}
	return deviceID, true
}

func requestDeviceID(req request) string {
	if value := strings.TrimSpace(req.UserID); value != "" {
		return value
	}
	return strings.TrimSpace(req.ConversationID)
}

func writeSearchResult(c *gin.Context, value string) {
	items := []any{}
	if value = strings.TrimSpace(value); value != "" {
		items = append(items, gin.H{"memory_value": value})
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"memory_detail_list": items}})
}
