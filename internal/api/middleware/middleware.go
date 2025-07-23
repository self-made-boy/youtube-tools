package middleware

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/self-made-boy/youtube-tools/internal/api/response"
)

// Logger 创建一个日志中间件
func Logger(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 开始时间
		start := time.Now()

		// 生成请求 ID
		requestID := uuid.New().String()
		c.Set("request_id", requestID)
		c.Header("X-Request-ID", requestID)

		// 处理请求
		c.Next()

		// 结束时间
		end := time.Now()
		latency := end.Sub(start)

		// 获取客户端 IP
		clientIP := c.ClientIP()

		// 获取用户代理
		userAgent := c.Request.UserAgent()

		// 获取状态码
		statusCode := c.Writer.Status()

		// 获取错误信息
		errors := c.Errors.String()

		// 记录日志
		logFields := []zap.Field{
			zap.String("request_id", requestID),
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.String("query", c.Request.URL.RawQuery),
			zap.Int("status", statusCode),
			zap.String("latency", latency.String()),
			zap.String("ip", clientIP),
			zap.String("user_agent", userAgent),
		}

		if errors != "" {
			logFields = append(logFields, zap.String("errors", errors))
		}

		if statusCode >= 500 {
			logger.Error("Request failed", logFields...)
		} else if statusCode >= 400 {
			logger.Warn("Request warning", logFields...)
		} else {
			logger.Info("Request completed", logFields...)
		}
	}
}

// Recovery 创建一个恢复中间件
func Recovery(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				// 获取请求 ID
				requestID, _ := c.Get("request_id")

				// 记录错误日志
				logger.Error("Request panic",
					zap.Any("request_id", requestID),
					zap.String("error", fmt.Sprintf("%v", err)),
					zap.String("method", c.Request.Method),
					zap.String("path", c.Request.URL.Path),
				)

				// 返回 500 错误
				response.ServerError(c, fmt.Errorf("%v", err))

				// 终止请求
				c.Abort()
			}
		}()

		c.Next()
	}
}

// CORS 创建一个 CORS 中间件
func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
