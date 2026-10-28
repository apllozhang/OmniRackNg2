package middleware

import (
	"log"
	"time"

	"github.com/gin-gonic/gin"
)

func RequestLog() gin.HandlerFunc {
	return func(c *gin.Context) {
		started := time.Now()
		c.Next()
		log.Printf("requestId=%s method=%s path=%s status=%d durationMs=%d", c.GetString("requestId"), c.Request.Method, c.Request.URL.Path, c.Writer.Status(), time.Since(started).Milliseconds())
	}
}
