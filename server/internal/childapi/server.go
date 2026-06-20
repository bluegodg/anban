package childapi

import (
	"net/http"
	"strings"

	"github.com/bluegodg/anban/server/internal/domains/account"
	"github.com/bluegodg/anban/server/internal/domains/devicebinding"
	sharedtypes "github.com/bluegodg/anban/server/pkg/types"
	"github.com/gin-gonic/gin"
)

// Deps 是 childapi 装配所需的依赖（各域 handler 之后注入这里）。
// 地基期为空骨架；域 follow-on 计划往这里加自己的 handler 字段并注册路由。
type Deps struct {
	AccessCode           string
	AllowedOrigins       []string
	MessageRoutes        RouteRegistrar
	GreetingRoutes       RouteRegistrar
	ReminderRoutes       RouteRegistrar
	StatusRoutes         RouteRegistrar
	ProfileRoutes        RouteRegistrar
	MemoryRoutes         RouteRegistrar
	VisionRoutes         RouteRegistrar
	AccountService       *account.Service
	DeviceBindingService *devicebinding.Service
	TimelineRoutes       RouteRegistrar
}

type RouteRegistrar interface {
	RegisterRoutes(r gin.IRoutes)
}

func NewRouter(d Deps) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(AllowCORS(d.AllowedOrigins))
	r.OPTIONS("/*path", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	// 健康检查无需访问码
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	api := r.Group("/api", NoStoreAPIResponses())
	if d.AccountService != nil {
		registerAccountRoutes(api, d.AccountService, d.DeviceBindingService)
	}

	// —— 各业务域路由占位（域 follow-on 计划逐个替换为真 handler）——
	// 返回 501，让前端先对着 URL 形状开发。
	notImpl := func(c *gin.Context) {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "未实现（地基占位）"})
	}
	deviceAPI := api.Group("", RequireAccountBindingOrAccessCode(d.AccessCode, d.AccountService, d.DeviceBindingService), RequireAdminForProfileWrites())
	if d.MessageRoutes != nil {
		d.MessageRoutes.RegisterRoutes(deviceAPI)
	} else {
		deviceAPI.POST("/messages", notImpl) // message 域
		deviceAPI.GET("/messages", notImpl)  // message 域
	}
	if d.ReminderRoutes != nil {
		d.ReminderRoutes.RegisterRoutes(deviceAPI)
	} else {
		deviceAPI.POST("/reminders", notImpl)         // reminder 域
		deviceAPI.GET("/reminders", notImpl)          // reminder 域
		deviceAPI.DELETE("/reminders/:id", notImpl)   // reminder 域
		deviceAPI.POST("/reminders/:id/ack", notImpl) // reminder 域
	}
	if d.GreetingRoutes != nil {
		d.GreetingRoutes.RegisterRoutes(deviceAPI)
	} else {
		deviceAPI.POST("/greetings/trigger", notImpl) // greeting 域
		deviceAPI.GET("/greetings/schedule", notImpl) // greeting 域
		deviceAPI.PUT("/greetings/schedule", notImpl) // greeting 域
	}
	if d.ProfileRoutes != nil {
		d.ProfileRoutes.RegisterRoutes(deviceAPI)
	} else {
		deviceAPI.GET("/profile", notImpl)  // profile 域
		deviceAPI.PUT("/profile", notImpl)  // profile 域
		deviceAPI.POST("/profile", notImpl) // profile 域
	}
	if d.MemoryRoutes != nil {
		d.MemoryRoutes.RegisterRoutes(deviceAPI)
	} else {
		deviceAPI.GET("/memory/facts", notImpl)        // memory 域
		deviceAPI.POST("/memory/facts", notImpl)       // memory 域
		deviceAPI.PUT("/memory/facts/:id", notImpl)    // memory 域
		deviceAPI.DELETE("/memory/facts/:id", notImpl) // memory 域
	}
	if d.StatusRoutes != nil {
		d.StatusRoutes.RegisterRoutes(deviceAPI)
	} else {
		deviceAPI.GET("/status", notImpl)         // status 域
		deviceAPI.GET("/device/status", notImpl)  // status 域（PRD 路径）
		deviceAPI.GET("/device/history", notImpl) // status 域（开发期对话记录）
	}
	if d.VisionRoutes != nil {
		d.VisionRoutes.RegisterRoutes(deviceAPI)
	} else {
		deviceAPI.POST("/vision/capture", notImpl)        // vision 域（拍照 MCP 入口）
		deviceAPI.POST("/vision/check-presence", notImpl) // vision 域（采帧后读取 presence）
		deviceAPI.POST("/vision/presence", notImpl)       // vision 域（VLM presence 状态入口）
	}
	if d.TimelineRoutes != nil {
		d.TimelineRoutes.RegisterRoutes(deviceAPI)
	}

	return r
}

func NoStoreAPIResponses() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Cache-Control", "no-store")
		c.Header("Pragma", "no-cache")
		c.Header("Expires", "0")
		c.Next()
	}
}

func AllowCORS(allowedOrigins []string) gin.HandlerFunc {
	allowed := make(map[string]struct{}, len(allowedOrigins))
	allowAll := false
	for _, origin := range allowedOrigins {
		origin = strings.TrimSpace(origin)
		if origin == "" {
			continue
		}
		if origin == "*" {
			allowAll = true
			continue
		}
		allowed[origin] = struct{}{}
	}

	return func(c *gin.Context) {
		origin := strings.TrimSpace(c.GetHeader("Origin"))
		if origin != "" {
			if allowAll {
				c.Header("Access-Control-Allow-Origin", "*")
			} else if _, ok := allowed[origin]; ok {
				c.Header("Access-Control-Allow-Origin", origin)
				c.Header("Vary", "Origin")
			}
			if c.Writer.Header().Get("Access-Control-Allow-Origin") != "" {
				c.Header("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
				c.Header("Access-Control-Allow-Headers", "Content-Type, X-Access-Code, Authorization")
			}
		}

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

func RequireAccountBindingOrAccessCode(code string, accountService *account.Service, bindingService *devicebinding.Service) gin.HandlerFunc {
	accessCodeMiddleware := RequireAccessCode(code)
	return func(c *gin.Context) {
		authHeader := strings.TrimSpace(c.GetHeader("Authorization"))
		if authHeader == "" {
			accessCodeMiddleware(c)
			return
		}
		if accountService == nil || bindingService == nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "账号体系未启用"})
			return
		}
		acct, err := accountService.Authenticate(c.Request.Context(), authHeader)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "登录已失效"})
			return
		}
		binding, err := bindingService.CurrentBinding(c.Request.Context(), acct.ID)
		if err != nil {
			if err == devicebinding.ErrNotBound {
				c.AbortWithStatusJSON(http.StatusConflict, gin.H{"error": "device_not_bound", "message": "请先绑定安伴设备"})
				return
			}
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "设备绑定读取失败"})
			return
		}
		c.Set(sharedtypes.GinContextAuthMode, "account")
		c.Set(sharedtypes.GinContextAccountID, acct.ID)
		c.Set(sharedtypes.GinContextDeviceID, binding.DeviceID)
		c.Set(sharedtypes.GinContextDeviceRole, string(binding.Role))
		c.Set(sharedtypes.GinContextSenderDisplayName, account.DisplayName(acct))
		c.Set(sharedtypes.GinContextSenderAvatarColor, acct.AvatarColor)
		c.Set(sharedtypes.GinContextElderDisplayName, binding.ElderDisplayName)
		c.Next()
	}
}

func RequireBearerAccount(accountService *account.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		if accountService == nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "账号体系未启用"})
			return
		}
		acct, err := accountService.Authenticate(c.Request.Context(), c.GetHeader("Authorization"))
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "登录已失效"})
			return
		}
		c.Set(sharedtypes.GinContextAuthMode, "account")
		c.Set(sharedtypes.GinContextAccountID, acct.ID)
		c.Set(sharedtypes.GinContextSenderDisplayName, account.DisplayName(acct))
		c.Next()
	}
}

func RequireAdminForProfileWrites() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.GetString(sharedtypes.GinContextAuthMode) == "account" &&
			isAdminOnlyWritePath(c.Request.Method, c.Request.URL.Path) &&
			c.GetString(sharedtypes.GinContextDeviceRole) != string(devicebinding.RoleAdmin) {
			message := "只有家庭管理员可以编辑家人资料和记忆"
			if isVisionCaptureDeletePath(c.Request.Method, c.Request.URL.Path) {
				message = "只有家庭管理员可以删除原图记录"
			}
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "admin_required", "message": message})
			return
		}
		c.Next()
	}
}

func isAdminOnlyWritePath(method, path string) bool {
	if method != http.MethodPut && method != http.MethodPost && method != http.MethodDelete {
		return false
	}
	if path == "/api/profile" {
		return true
	}
	return path == "/api/memory/facts" || strings.HasPrefix(path, "/api/memory/facts/") || isVisionCaptureDeletePath(method, path)
}

func isVisionCaptureDeletePath(method, path string) bool {
	return method == http.MethodDelete && strings.HasPrefix(path, "/api/vision/captures/")
}
