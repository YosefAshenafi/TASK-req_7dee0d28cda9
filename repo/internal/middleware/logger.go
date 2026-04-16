package middleware

import (
	"log"
	"time"

	"github.com/gin-gonic/gin"
)

// Logger logs HTTP requests with method, path, status, and duration.
func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path

		c.Next()

		latency := time.Since(start)
		status := c.Writer.Status()
		reqID := c.GetHeader("X-Request-Id")

		log.Printf("[%d] %s %s %v (req_id=%s)",
			status, c.Request.Method, path, latency, reqID)
	}
}
