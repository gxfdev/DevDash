package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gxfdev/DevDash/server/internal/alert"
	"github.com/gxfdev/DevDash/server/internal/api"
	"github.com/gxfdev/DevDash/server/internal/auth"
	"github.com/gxfdev/DevDash/server/internal/collector"
	"github.com/gxfdev/DevDash/server/internal/config"
	"github.com/gxfdev/DevDash/server/internal/filemgr"
	"github.com/gxfdev/DevDash/server/internal/logger"
	"github.com/gxfdev/DevDash/server/internal/store"

	"github.com/gin-gonic/gin"
)

func main() {
	cfg := config.Load()
	logger.Setup()

	if os.Getenv("GIN_MODE") == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	if err := auth.InitSecret(cfg.JWTSecret); err != nil {
		log.Fatalf("FATAL: %v", err)
	}

	s := store.NewStore(cfg)
	defer s.Close()

	filemgr.SetOpCallback(func(op, path, name, ext string, size int64, isDir bool) {
		s.RecordFileOp(map[string]any{
			"operation": op,
			"path":      path,
			"name":      name,
			"ext":       ext,
			"size":      size,
			"is_dir":    isDir,
		})
	})

	c := collector.NewCollector()
	alertEngine := alert.NewEngine(s)

	go startMetricsCleanup(s)

	r := gin.Default()

	r.Use(gin.RecoveryWithWriter(gin.DefaultErrorWriter, func(c *gin.Context, err any) {
		log.Printf("[panic] recovered: %v", err)
		c.AbortWithStatusJSON(500, gin.H{"error": "internal server error"})
	}))
	r.Use(corsMiddleware())
	r.Use(auth.SecureHeadersMiddleware())
	r.Use(auth.AuditLogMiddleware())
	r.Use(logger.Middleware())

	auditFile, auditErr := os.OpenFile("audit.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if auditErr == nil {
		auth.InitAuditLog(auditFile)
	}

	handler := api.NewHandler(c, s, cfg)
	handler.SetAlertEngine(alertEngine)
	handler.RegisterRoutes(r)

	go startCollection(c, s, cfg, alertEngine, handler)

	staticDir := os.Getenv("STATIC_DIR")
	if staticDir == "" {
		staticDir = "../web/dist"
	}
	r.Static("/assets", staticDir+"/assets")
	r.NoRoute(func(c *gin.Context) {
		p := c.Request.URL.Path
		if strings.HasPrefix(p, "/api") || strings.HasPrefix(p, "/ws") {
			c.AbortWithStatusJSON(404, gin.H{"error": "not found"})
			return
		}
		c.File(staticDir + "/index.html")
	})

	addr := ":" + cfg.ServerPort
	srv := &http.Server{
		Addr:    addr,
		Handler: r,
	}

	go func() {
		log.Printf("DevDash 启动，监听 %s", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("正在关闭服务器...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("服务器强制关闭: %v", err)
	}

	log.Println("服务器已优雅关闭")
}

func startMetricsCleanup(s *store.Store) {
	retentionDays := 30
	ticker := time.NewTicker(6 * time.Hour)
	defer ticker.Stop()
	for range ticker.C {
		if err := s.CleanupOldMetrics(retentionDays); err != nil {
			log.Printf("[cleanup] metrics cleanup failed: %v", err)
		} else {
			log.Printf("[cleanup] cleaned up metrics older than %d days", retentionDays)
		}
	}
}

func startCollection(c *collector.Collector, s *store.Store, cfg *config.Config, alertEngine *alert.Engine, h *api.Handler) {
	collectOnce := func() {
		snap, err := c.Collect()
		if err != nil {
			log.Printf("采集失败: %v", err)
			return
		}
		snap.NodeID = "self"
		if err := s.SaveSnapshot("self", snap); err != nil {
			log.Printf("[store] save snapshot failed: %v", err)
		}
		h.UpdateCache(snap)
		alertEngine.Evaluate(snap)
	}

	collectOnce()
	log.Println("首次采集完成")

	// Dynamic interval: check config every second and reset ticker on change
	var currentInterval int
	getInterval := func() time.Duration {
		cfgVal := cfg.CollectInterval
		if cfgVal < 3 {
			cfgVal = 5
		}
		d := time.Duration(cfgVal) * time.Second
		if d < 3*time.Second {
			d = 5 * time.Second
		}
		return d
	}
	currentInterval = cfg.CollectInterval

	ticker := time.NewTicker(getInterval())
	defer ticker.Stop()

	for range ticker.C {
		collectOnce()
		// Check if interval changed
		if cfg.CollectInterval != currentInterval {
			currentInterval = cfg.CollectInterval
			ticker.Reset(getInterval())
			log.Printf("[collect] interval updated to %ds", currentInterval)
		}
	}
}

func corsMiddleware() gin.HandlerFunc {
	allowedOrigins := os.Getenv("CORS_ORIGINS")
	allowedMap := make(map[string]bool)
	if allowedOrigins != "" {
		for o := range strings.SplitSeq(allowedOrigins, ",") {
			allowedMap[strings.TrimSpace(o)] = true
		}
	} else {
		for _, o := range []string{"http://localhost:3000", "http://localhost:5173", "http://127.0.0.1:3000", "http://127.0.0.1:5173"} {
			allowedMap[o] = true
		}
	}
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin != "" && allowedMap[origin] {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Credentials", "true")
		}
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, PATCH, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
		c.Header("Access-Control-Max-Age", "86400")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	}
}
