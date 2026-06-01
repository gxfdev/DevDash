package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"

	"webshell/internal/api"
	"webshell/internal/config"
	"webshell/internal/filemgr"
	"webshell/internal/store"
)

var version = "dev"

func main() {
	cfg := config.Load()

	if cfg.JWTSecret == "" {
		log.Println("WARNING: JWT_SECRET not set, using random secret (sessions will not survive restarts)")
		cfg.JWTSecret = fmt.Sprintf("auto-%d", os.Getpid())
	}

	gin.SetMode(cfg.GinMode)

	s, err := store.New(cfg.DBPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer s.Close()

	filemgr.SetRoot("/")

	r := gin.Default()

	r.Use(corsMiddleware(cfg.CORSOrigins))

	r.GET("/api/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "version": version})
	})

	r.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path
		if strings.HasPrefix(path, "/api") || strings.HasPrefix(path, "/ws") {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		c.FileFromFS(path, gin.Dir("./web/dist", false))
	})
	r.GET("/", func(c *gin.Context) {
		c.FileFromFS("/", gin.Dir("./web/dist", false))
	})

	h := api.NewHandler(s, cfg.JWTSecret, cfg.DataDir)
	h.RegisterRoutes(r)

	addr := fmt.Sprintf("0.0.0.0:%s", cfg.Port)
	log.Printf("WebShell v%s starting on %s", version, addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

func corsMiddleware(origins string) gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		allowed := origins == "*"
		if !allowed {
			for _, o := range strings.Split(origins, ",") {
				if strings.TrimSpace(o) == origin {
					allowed = true
					break
				}
			}
		}
		if allowed {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type")
			c.Header("Access-Control-Allow-Credentials", "true")
		}
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
