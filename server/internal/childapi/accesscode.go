package childapi

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// RequireAccessCode 校验请求头 X-Access-Code 是否等于配置的访问码（简化登录）。
func RequireAccessCode(code string) gin.HandlerFunc {
	code = strings.TrimSpace(code)
	return func(c *gin.Context) {
		if code == "" || strings.TrimSpace(c.GetHeader("X-Access-Code")) != code {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "访问码无效"})
			return
		}
		c.Next()
	}
}
