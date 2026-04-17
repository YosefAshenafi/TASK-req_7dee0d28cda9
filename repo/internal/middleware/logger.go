package middleware

import (
	"log"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/fulfillops/fulfillops/internal/service"
)

// Logger logs HTTP requests with method, path, status, and duration.
// Request ID is resolved from the request context (set by RequestID middleware)
// and falls back to the outbound response header so generated IDs are logged,
// not only IDs supplied by the caller.
func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path

		c.Next()

		latency := time.Since(start)
		status := c.Writer.Status()

		reqID := service.RequestIDFromContext(c.Request.Context())
		if reqID == "" {
			reqID = c.Writer.Header().Get("X-Request-Id")
		}
		if reqID == "" {
			reqID = c.GetHeader("X-Request-Id")
		}

		log.Printf("[%d] %s %s %v (req_id=%s)",
			status, c.Request.Method, path, latency, reqID)
	}
}
