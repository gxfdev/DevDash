package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gxfdev/aidash/server/internal/api"
	"github.com/gxfdev/aidash/server/internal/auth"
	"github.com/gxfdev/aidash/server/internal/catalog"
	"github.com/gxfdev/aidash/server/internal/collector"
	"github.com/gxfdev/aidash/server/internal/config"
	"github.com/gxfdev/aidash/server/internal/engine"
	"github.com/gxfdev/aidash/server/internal/gateway"
	"github.com/gxfdev/aidash/server/internal/model"
	"github.com/gxfdev/aidash/server/internal/modelhub"
	"github.com/gxfdev/aidash/server/internal/orchestrator"
	"github.com/gxfdev/aidash/server/internal/store"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	cfg := config.Load()
	if os.Getenv("GIN_MODE") == "release" {
		gin.SetMode(gin.ReleaseMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	st, err := store.Open(cfg.DBPath)
	if err != nil {
		log.Fatalf("打开数据库失败: %v", err)
	}
	defer st.Close()
	if err := st.EnsureAdmin(cfg.AdminUser, cfg.AdminPass); err != nil {
		log.Fatalf("初始化管理员失败: %v", err)
	}

	cat, err := catalog.Load()
	if err != nil {
		log.Fatalf("加载应用目录失败: %v", err)
	}

	docker, err := engine.New(cfg.DockerHost)
	if err != nil {
		log.Fatalf("初始化 Docker 客户端失败: %v", err)
	}

	orch := orchestrator.New(st, docker, cat)
	hub := modelhub.New(st, docker)
	gw := gateway.New(st, orch)

	col := collector.New()
	snapCache := &snapshotCache{}

	mgr := auth.NewManager(cfg.JWTSecret)
	handler := api.NewHandler(cfg, st, orch, mgr, cat, gw,
		gateway.NewCopilot(st, orch, gw, snapCache.get), hub)

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(gin.LoggerWithConfig(gin.LoggerConfig{SkipPaths: []string{"/api/health"}}))
	r.Use(corsMiddleware())
	r.Use(auth.SecureHeaders())
	handler.Register(r)
	registerStatic(r, cfg.StaticDir)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Host metrics pipeline.
	go func() {
		collectOnce := func() {
			snap, err := col.Collect()
			if err != nil {
				log.Printf("[collect] %v", err)
				return
			}
			snap.Containers = orch.ContainerCounts(ctx)
			snapCache.set(snap)
			handler.SetSnapshot(snap)
			point := model.MetricPoint{
				TS:     snap.Timestamp.Unix(),
				CPU:    snap.CPU.UsagePercent,
				Memory: snap.Memory.UsagePercent,
				Disk:   snap.Disk.UsagePercent,
				NetRX:  snap.Network.RecvRateMB,
				NetTX:  snap.Network.SentRateMB,
			}
			if snap.GPU != nil {
				point.GPU = snap.GPU.UsagePercent
				if snap.GPU.MemTotalMB > 0 {
					point.GPUMem = snap.GPU.MemUsedMB / snap.GPU.MemTotalMB * 100
				}
			}
			st.InsertMetric(point)
		}
		collectOnce()
		ticker := time.NewTicker(cfg.CollectEvery)
		defer ticker.Stop()
		lastPersist := time.Now()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				collectOnce()
				if time.Since(lastPersist) > 4*time.Hour {
					lastPersist = time.Now()
					_ = st.CleanupMetrics(time.Now().AddDate(0, 0, -30))
				}
			}
		}
	}()

	// Docker state reconciliation.
	go orch.Monitor(ctx, 10*time.Second)
	go hub.Recover()

	// Boot summary + auto-start of managed workloads.
	go func() {
		time.Sleep(2 * time.Second)
		engineSnap := orch.EngineSnapshot(ctx)
		if engineSnap.Connected {
			log.Printf("[engine] Docker 已连接: %s (v%s)", engineSnap.Host, engineSnap.Version)
		} else {
			log.Printf("[engine] Docker 不可用: %s", engineSnap.Error)
		}
		services, _ := st.ListServices()
		log.Printf("[boot] 已加载 %d 个 AI 服务", len(services))
		_ = os.MkdirAll(filepath.Join(cfg.DataDir, "logs"), 0o755)
	}()

	srv := &http.Server{Addr: ":" + cfg.Port, Handler: r}
	go func() {
		log.Printf("%s v%s 启动，监听 :%s（Docker: %s）", cfg.Edition, cfg.Version, cfg.Port, cfg.DockerHost)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("监听失败: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("正在关闭…")
	cancel()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer shutdownCancel()
	_ = srv.Shutdown(shutdownCtx)
	log.Println("已退出")
}

type snapshotCache struct {
	snap *model.HostSnapshot
}

func (s *snapshotCache) set(v *model.HostSnapshot) { s.snap = v }
func (s *snapshotCache) get() *model.HostSnapshot  { return s.snap }

func corsMiddleware() gin.HandlerFunc {
	origins := map[string]bool{
		"http://localhost:5173": true,
		"http://127.0.0.1:5173": true,
		"http://localhost:9495": true,
		"http://127.0.0.1:9495": true,
	}
	if extra := os.Getenv("CORS_ORIGINS"); extra != "" {
		for _, o := range strings.Split(extra, ",") {
			origins[strings.TrimSpace(o)] = true
		}
	}
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin != "" && origins[origin] {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Credentials", "true")
		}
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization, X-API-Key")
		c.Header("Access-Control-Max-Age", "86400")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

func registerStatic(r *gin.Engine, dir string) {
	if dir == "" {
		return
	}
	if abs, err := filepath.Abs(dir); err == nil {
		dir = abs
	}
	assets := filepath.Join(dir, "assets")
	r.Static("/assets", assets)
	r.StaticFile("/favicon.svg", filepath.Join(dir, "favicon.svg"))
	r.NoRoute(func(c *gin.Context) {
		p := c.Request.URL.Path
		if strings.HasPrefix(p, "/api") || strings.HasPrefix(p, "/v1") || strings.HasPrefix(p, "/ws") {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		if _, err := os.Stat(filepath.Join(dir, "index.html")); err != nil {
			c.String(http.StatusOK, "AIDash backend is running. Frontend build not found at %s", dir)
			return
		}
		c.File(filepath.Join(dir, "index.html"))
	})
}
