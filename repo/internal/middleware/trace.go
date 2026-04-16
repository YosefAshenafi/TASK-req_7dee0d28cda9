package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/fulfillops/fulfillops/internal/service"
)

// RequestID propagates X-Request-Id through the request lifecycle.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		reqID := c.GetHeader("X-Request-Id")
		if reqID == "" {
			reqID = uuid.New().String()
		}
		c.Header("X-Request-Id", reqID)

		// Store in context for audit logging.
		ctx := service.WithRequestID(c.Request.Context(), reqID)
		c.Request = c.Request.WithContext(ctx)

		c.Next()
	}
}

// ClientIP captures the client IP and stores it in the request context.
func ClientIP() gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()
		ctx := service.WithIPAddress(c.Request.Context(), ip)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}
