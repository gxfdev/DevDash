package logger

import (
	"io"
	"log"
	"os"
	"time"

	"github.com/gin-gonic/gin"
)

func Setup() {
	log.SetOutput(os.Stdout)
	log.SetFlags(log.LstdFlags | log.Lshortfile | log.Lmicroseconds)
}

func Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		c.Next()

		latency := time.Since(start)
		statusCode := c.Writer.Status()
		clientIP := c.ClientIP()
		method := c.Request.Method

		if query != "" {
			path = path + "?" + query
		}

		log.Printf("[GIN] %d | %13v | %15s | %-7s %s",
			statusCode,
			latency,
			clientIP,
			method,
			path,
		)

		for _, err := range c.Errors.ByType(gin.ErrorTypePrivate).String() {
			log.Printf("[ERROR] %s", err)
		}
	}
}

func RequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		log.Printf("[REQUEST] %s %s from %s",
			c.Request.Method,
			c.Request.URL.Path,
			c.ClientIP(),
		)
		c.Next()
	}
}

func ErrorLogger(err error, context string) {
	log.Printf("[ERROR] [%s] %v", context, err)
}

func InfoLogger(message string) {
	log.Printf("[INFO] %s", message)
}
