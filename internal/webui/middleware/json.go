package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// JSONMiddleware - JSON内容类型中间件
func JSONMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 设置响应头
		c.Header("Content-Type", "application/json; charset=utf-8")

		// 检查请求内容类型
		if c.Request.Method == http.MethodPost || c.Request.Method == http.MethodPut || c.Request.Method == http.MethodPatch {
			contentType := c.GetHeader("Content-Type")
			if contentType != "" && contentType != "application/json" {
				c.JSON(http.StatusUnsupportedMediaType, gin.H{
					"success": false,
					"error":   "Content-Type must be application/json",
				})
				c.Abort()
				return
			}
		}

		c.Next()
	}
}