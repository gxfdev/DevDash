package middleware

import (
	"bytes"
	"io"

	"github.com/gin-gonic/gin"
)

func CharsetUTF8() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Content-Type", "application/json; charset=utf-8")
		c.Next()
	}
}

func FixEncoding() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Body != nil {
			bodyBytes, err := io.ReadAll(c.Request.Body)
			if err == nil {
				c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
			}
		}

		c.Header("Content-Type", "application/json; charset=utf-8")
		c.Header("X-Content-Type-Options", "nosniff")

		c.Next()
	}
}
