package main

import (
	"log"
	"os"
	"runtime"
	"strings"
	"time"

	"devdash/internal/api"
	"devdash/internal/auth"
	"devdash/internal/collector"
	"devdash/internal/config"
	"devdash/internal/model"
	"devdash/internal/node"
	"devdash/internal/store"

	"github.com/gin-gonic/gin"
)

func main() {
	cfg := config.Load()
	auth.InitSecret(cfg.JWTSecret)

	s := store.NewStore(cfg)
	defer s.Close()

	nm := node.NewNodeManager(s)
	c := collector.NewCollector()

	// Register the server itself as the "self" node for local monitoring
	registerSelfNode(nm, c)

	go startCollection(c, s, nm, cfg)

	r := gin.Default()
	r.Use(corsMiddleware())
	handler := api.NewHandler(c, s, nm)
	handler.RegisterRoutes(r)

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
	log.Printf("DevDash 启动，监听 %s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatal(err)
	}
}

// registerSelfNode registers the local machine as a node and triggers an initial collection
func registerSelfNode(nm *node.NodeManager, c *collector.Collector) {
	selfNode := &model.Node{
		ID:     "self",
		Name:   hostname(),
		OS:     "windows",
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

func startCollection(c *collector.Collector, s *store.Store, nm *node.NodeManager, cfg *config.Config) {
	ticker := time.NewTicker(time.Duration(cfg.CollectInterval) * time.Second)
	defer ticker.Stop()

	first := true
	for range ticker.C {
		nodes := nm.ListNodes()
		for _, n := range nodes {
			if n.Role == "agent" || n.Role == "full" || n.ID == "self" {
				snap, err := c.Collect()
				if err != nil {
					log.Printf("采集失败 node=%s: %v", n.ID, err)
					continue
				}
				snap.NodeID = n.ID
				s.SaveSnapshot(snap)
				nm.UpdateHeartbeat(n.ID)
			}
		}
		if first {
			first = false
			log.Println("首次采集完成")
		}
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
		c.Header("Content-Type", "application/json; charset=utf-8")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	}
}