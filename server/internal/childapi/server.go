package childapi

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// Deps 是 childapi 装配所需的依赖（各域 handler 之后注入这里）。
// 地基期为空骨架；域 follow-on 计划往这里加自己的 handler 字段并注册路由。
type Deps struct {
	AccessCode     string
	AllowedOrigins []string
	MessageRoutes  RouteRegistrar
	GreetingRoutes RouteRegistrar
	ReminderRoutes RouteRegistrar
	StatusRoutes   RouteRegistrar
	ProfileRoutes  RouteRegistrar
	VisionRoutes   RouteRegistrar
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

	// 子女端 API：全部走访问码，并禁止浏览器缓存状态类响应。
	api := r.Group("/api", NoStoreAPIResponses(), RequireAccessCode(d.AccessCode))

	// —— 各业务域路由占位（域 follow-on 计划逐个替换为真 handler）——
	// 返回 501，让前端先对着 URL 形状开发。
	notImpl := func(c *gin.Context) {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "未实现（地基占位）"})
	}
	if d.MessageRoutes != nil {
		d.MessageRoutes.RegisterRoutes(api)
	} else {
		api.POST("/messages", notImpl) // message 域
		api.GET("/messages", notImpl)  // message 域
	}
	if d.ReminderRoutes != nil {
		d.ReminderRoutes.RegisterRoutes(api)
	} else {
		api.POST("/reminders", notImpl)         // reminder 域
		api.GET("/reminders", notImpl)          // reminder 域
		api.DELETE("/reminders/:id", notImpl)   // reminder 域
		api.POST("/reminders/:id/ack", notImpl) // reminder 域
	}
	if d.GreetingRoutes != nil {
		d.GreetingRoutes.RegisterRoutes(api)
	} else {
		api.POST("/greetings/trigger", notImpl) // greeting 域
		api.GET("/greetings/schedule", notImpl) // greeting 域
		api.PUT("/greetings/schedule", notImpl) // greeting 域
	}
	if d.ProfileRoutes != nil {
		d.ProfileRoutes.RegisterRoutes(api)
	} else {
		api.GET("/profile", notImpl)  // profile 域
		api.PUT("/profile", notImpl)  // profile 域
		api.POST("/profile", notImpl) // profile 域
	}
	if d.StatusRoutes != nil {
		d.StatusRoutes.RegisterRoutes(api)
	} else {
		api.GET("/status", notImpl)         // status 域
		api.GET("/device/status", notImpl)  // status 域（PRD 路径）
		api.GET("/device/history", notImpl) // status 域（开发期对话记录）
	}
	if d.VisionRoutes != nil {
		d.VisionRoutes.RegisterRoutes(api)
	} else {
		api.POST("/vision/capture", notImpl)        // vision 域（拍照 MCP 入口）
		api.POST("/vision/check-presence", notImpl) // vision 域（采帧后读取 presence）
		api.POST("/vision/presence", notImpl)       // vision 域（VLM presence 状态入口）
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
				c.Header("Access-Control-Allow-Headers", "Content-Type, X-Access-Code")
			}
		}

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
