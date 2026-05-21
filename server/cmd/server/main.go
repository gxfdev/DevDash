package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"devdash/internal/agent"
	"devdash/internal/api"
	"devdash/internal/alert"
	"devdash/internal/auth"
	"devdash/internal/collector"
	"devdash/internal/config"
	"devdash/internal/filemgr"
	"devdash/internal/logger"
	"devdash/internal/model"
	"devdash/internal/node"
	"devdash/internal/store"

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
		s.RecordFileOp(map[string]interface{}{
			"operation": op,
			"path":      path,
			"name":      name,
			"ext":       ext,
			"size":      size,
			"is_dir":    isDir,
		})
	})

	nm := node.NewNodeManager(s)
	nm.SyncFromDB()

	c := collector.NewCollector()
	alertEngine := alert.NewEngine(s)
	agentMgr := agent.NewAgentManager()

	registerSelfNode(nm, c)

	go startCollection(c, s, nm, cfg, alertEngine)
	go startMetricsCleanup(s)

	agentMgr.StartPeriodicCollection(30 * time.Second)

	r := gin.Default()
	
	r.Use(corsMiddleware())
	r.Use(logger.Middleware())
	
	handler := api.NewHandler(c, s, nm)
	handler.RegisterRoutes(r)

	agentHandler := api.NewAgentHandler(agentMgr)
	agentHandler.RegisterRoutes(r)

	agentServer := agent.NewAgentServer(nil, nil, c)
	agentServer.RegisterRoutes(r)

	// Serve frontend SPA from dist/ (supports both local dev and Docker)
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

// registerSelfNode registers the local machine as a node and triggers an initial collection
func registerSelfNode(nm *node.NodeManager, c *collector.Collector) {
	selfNode := &model.Node{
		ID:     "self",
		Name:   hostname(),
		OS:     runtime.GOOS,
		Arch:   runtimeArch(),
		Status: "online",
	}
	nm.Register(selfNode)

	// Do an initial collection to populate the snapshot cache
	go func() {
		snap, err := c.Collect()
		if err != nil {
			log.Printf("[init] 初始采集失败: %v", err)
			return
		}
		log.Printf("[init] 初始采集完成: CPU %v%% | 内存 %v%% | 磁盘 %v%%",
			snap.CPU.UsagePercent, snap.Memory.UsagePercent, snap.Disk.UsagePercent)
	}()
}

func hostname() string {
	h, _ := os.Hostname()
	if h == "" {
		return "localhost"
	}
	return h
}

func runtimeArch() string {
	arch := runtime.GOARCH
	if arch == "amd64" {
		return "x86_64"
	}
	return arch
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

func startCollection(c *collector.Collector, s *store.Store, nm *node.NodeManager, cfg *config.Config, alertEngine *alert.Engine) {
	interval := time.Duration(cfg.CollectInterval) * time.Second
	if interval < 5*time.Second {
		interval = 5 * time.Second
	}

	sem := make(chan struct{}, 5)

	collectOnce := func() {
		nodes := nm.ListNodes()
		var wg sync.WaitGroup
		for _, n := range nodes {
			if n.Role == "agent" || n.Role == "full" || n.ID == "self" {
				wg.Add(1)
				go func(node *model.Node) {
					defer wg.Done()
					sem <- struct{}{}
					defer func() { <-sem }()
					snap, err := c.Collect()
					if err != nil {
						log.Printf("采集失败 node=%s: %v", node.ID, err)
						return
					}
					snap.NodeID = node.ID
					s.SaveSnapshot(node.ID, snap)
					alertEngine.Evaluate(snap)
					nm.UpdateHeartbeat(node.ID)
				}(n)
			}
		}
		wg.Wait()
	}

	collectOnce()
	log.Println("首次采集完成")

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for range ticker.C {
		collectOnce()
	}
}

func corsMiddleware() gin.HandlerFunc {
	allowedOrigins := os.Getenv("CORS_ORIGINS")
	allowedMap := make(map[string]bool)
	if allowedOrigins != "" {
		for _, o := range strings.Split(allowedOrigins, ",") {
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
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("X-XSS-Protection", "1; mode=block")
		c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		if strings.HasPrefix(c.Request.URL.Path, "/api") {
			c.Header("Content-Type", "application/json; charset=utf-8")
		}
		c.Header("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline' 'unsafe-eval'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; connect-src 'self' ws: wss:; frame-ancestors 'none'")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Header("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	}
}