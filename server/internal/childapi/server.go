package childapi

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Deps 是 childapi 装配所需的依赖（各域 handler 之后注入这里）。
// 地基期为空骨架；域 follow-on 计划往这里加自己的 handler 字段并注册路由。
type Deps struct {
	AccessCode     string
	MessageRoutes  RouteRegistrar
	GreetingRoutes RouteRegistrar
	ReminderRoutes RouteRegistrar
	StatusRoutes   RouteRegistrar
	ProfileRoutes  RouteRegistrar
}

type RouteRegistrar interface {
	RegisterRoutes(r gin.IRoutes)
}

func NewRouter(d Deps) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())

	// 健康检查无需访问码
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// 子女端 API：全部走访问码
	api := r.Group("/api", RequireAccessCode(d.AccessCode))

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
		api.POST("/reminders", notImpl) // reminder 域
		api.GET("/reminders", notImpl)  // reminder 域
	}
	if d.GreetingRoutes != nil {
		d.GreetingRoutes.RegisterRoutes(api)
	} else {
		api.POST("/greetings/trigger", notImpl) // greeting 域
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
		api.GET("/status", notImpl)        // status 域
		api.GET("/device/status", notImpl) // status 域（PRD 路径）
	}

	return r
}
