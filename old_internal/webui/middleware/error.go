package middleware

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

// APIResponse - 本地API响应类型，避免循环导入
type APIResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// ErrorHandlingMiddleware - 错误处理中间件
func ErrorHandlingMiddleware() gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, recovered interface{}) {
		if err, ok := recovered.(string); ok {
			log.Printf("Panic recovered: %s", err)
			c.JSON(http.StatusInternalServerError, APIResponse{
				Success: false,
				Error:   "Internal server error",
			})
		} else {
			log.Printf("Panic recovered: %v", recovered)
			c.JSON(http.StatusInternalServerError, APIResponse{
				Success: false,
				Error:   "Internal server error",
			})
		}
		c.Abort()
	})
}

// RateLimitMiddleware - 简单的速率限制中间件 (可选)
func RateLimitMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 这里可以实现更复杂的速率限制逻辑
		// 目前只是一个占位符
		c.Next()
	}
}

// AuthMiddleware - 认证中间件 (可选)
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 这里可以实现认证逻辑
		// 目前只是一个占位符
		c.Next()
	}
}
