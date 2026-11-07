// Command demo boots AIDash against an in-memory Docker engine and seeds a
// realistic single-host AI OS state. It is used for UI screenshots, manual
// evaluation and CI smoke tests when no Docker daemon is available.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gxfdev/aidash/server/internal/api"
	"github.com/gxfdev/aidash/server/internal/auth"
	"github.com/gxfdev/aidash/server/internal/catalog"
	"github.com/gxfdev/aidash/server/internal/collector"
	"github.com/gxfdev/aidash/server/internal/config"
	"github.com/gxfdev/aidash/server/internal/engine"
	"github.com/gxfdev/aidash/server/internal/gateway"
	"github.com/gxfdev/aidash/server/internal/mockdocker"
	"github.com/gxfdev/aidash/server/internal/model"
	"github.com/gxfdev/aidash/server/internal/modelhub"
	"github.com/gxfdev/aidash/server/internal/orchestrator"
	"github.com/gxfdev/aidash/server/internal/store"
)

func main() {
	gin.SetMode(gin.ReleaseMode)
	os.Setenv("AIDASH_HEALTH_TIMEOUT", "2s")
	os.Setenv("AIDASH_TARGET_OS", "linux")

	dir := os.Getenv("AIDASH_DEMO_DIR")
	if dir == "" {
		dir = filepath.Join("..", "web", "dist")
	}
	cfg := &config.Config{
		Port:      envOr("PORT", "9090"),
		DataDir:   filepath.Join(os.TempDir(), "aidash-demo"),
		DBPath:    filepath.Join(os.TempDir(), "aidash-demo", "aidash.db"),
		StaticDir: dir,
		JWTSecret: "aidash-demo-secret-0123456789abcdef",
		AdminUser: "admin", AdminPass: "admin123",
		Edition: "AIDash 单机 AI 操作系统", Version: "2.0.0",
		CollectEvery: 5 * time.Second,
	}
	_ = os.RemoveAll(cfg.DataDir)
	_ = os.MkdirAll(cfg.DataDir, 0o755)

	st, err := store.Open(cfg.DBPath)
	if err != nil {
		log.Fatal(err)
	}
	if err := st.EnsureAdmin(cfg.AdminUser, cfg.AdminPass); err != nil {
		log.Fatal(err)
	}
	cat, err := catalog.Load()
	if err != nil {
		log.Fatal(err)
	}
	mock := mockdocker.New()
	dc, err := engine.New(mock.URL())
	if err != nil {
		log.Fatal(err)
	}
	orch := orchestrator.New(st, dc, cat)
	hub := modelhub.New(st, dc)
	gw := gateway.New(st, orch)
	col := collector.New()

	snap := &model.HostSnapshot{}
	handler := api.NewHandler(cfg, st, orch, auth.NewManager(cfg.JWTSecret), cat, gw,
		gateway.NewCopilot(st, orch, gw, func() *model.HostSnapshot { return snap }), hub)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Fake OpenAI-compatible upstream so model services look healthy.
	llm := startFakeLLM()
	defer llm.Close()
	llmPort := portOf(llm.URL)

	seedMetrics(st)
	seedUsage(st)
	seedAudit(st)
	seedKeys(st)
	seedUsers(st)

	seedDeployments(orch, llmPort)

	go orch.Monitor(ctx, 6*time.Second)

	// Live host sampling.
	go func() {
		for {
			if s, err := col.Collect(); err == nil {
				s.Containers = orch.ContainerCounts(ctx)
				snap = s
				handler.SetSnapshot(s)
				st.InsertMetric(model.MetricPoint{
					TS: s.Timestamp.Unix(), CPU: s.CPU.UsagePercent, Memory: s.Memory.UsagePercent,
					Disk: s.Disk.UsagePercent, NetRX: s.Network.RecvRateMB, NetTX: s.Network.SentRateMB,
				})
			}
			time.Sleep(5 * time.Second)
		}
	}()

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(corsForDemo())
	handler.Register(r)
	registerStatic(r, cfg.StaticDir)

	log.Printf("AIDash DEMO 启动于 http://127.0.0.1:%s（admin / admin123）", cfg.Port)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatal(err)
	}
}

func envOr(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

func portOf(rawURL string) int {
	i := strings.LastIndex(rawURL, ":")
	port := 0
	for _, c := range rawURL[i+1:] {
		port = port*10 + int(c-'0')
	}
	return port
}

func seedDeployments(orch *orchestrator.Orchestrator, llmPort int) {
	ctx := context.Background()
	trueVal := true
	deploy := func(req orchestrator.DeployRequest) {
		svc, err := orch.Deploy(ctx, req)
		if err != nil {
			log.Printf("seed deploy %s: %v", req.Name, err)
			return
		}
		log.Printf("seed service %s (%s)", svc.Name, svc.ID)
	}
	deploy(orchestrator.DeployRequest{
		CatalogID: "qwen2.5-7b", Name: "qwen-7b", Owner: "admin",
		ModelName: "qwen2.5-7b",
		Ports:     []model.PortMap{{HostPort: 0, ContPort: llmPort, Protocol: "tcp", Accessible: true}},
	})
	deploy(orchestrator.DeployRequest{
		CatalogID: "deepseek-r1-7b", Name: "deepseek-r1", Owner: "admin",
		ModelName: "deepseek-r1-7b",
		Ports:     []model.PortMap{{HostPort: 0, ContPort: llmPort, Protocol: "tcp", Accessible: true}},
	})
	deploy(orchestrator.DeployRequest{
		CatalogID: "open-webui", Name: "open-webui", Owner: "admin", AutoStart: &trueVal,
	})
	deploy(orchestrator.DeployRequest{
		CatalogID: "ai-ops-agent", Name: "ops-sandbox", Owner: "admin",
	})
	deploy(orchestrator.DeployRequest{
		CatalogID: "qdrant", Name: "vector-db", Owner: "admin",
	})
	time.Sleep(2500 * time.Millisecond)
	// Put the vector DB to sleep to exercise the "stopped" visual state.
	if svcs, err := orch.Store().ListServices(); err == nil {
		for _, svc := range svcs {
			if svc.Name == "vector-db" {
				_ = orch.Stop(context.Background(), svc.ID)
			}
		}
	}
}

func seedMetrics(st *store.Store) {
	now := time.Now()
	for i := 24 * 60; i >= 0; i-- {
		ts := now.Add(-time.Duration(i) * time.Minute)
		phase := float64(i) / 60.0
		cpu := 24 + 14*math.Sin(phase*0.9) + 6*math.Sin(phase*2.7)
		mem := 46 + 9*math.Sin(phase*0.5+1.2)
		disk := 38 + 4*math.Sin(phase*0.2)
		rx := 1.8 + 1.6*math.Abs(math.Sin(phase*1.3))
		tx := 0.9 + 1.1*math.Abs(math.Cos(phase*1.1))
		if cpu < 5 {
			cpu = 5
		}
		st.InsertMetric(model.MetricPoint{
			TS: ts.Unix(), CPU: round(cpu), Memory: round(mem), Disk: round(disk),
			NetRX: round(rx), NetTX: round(tx),
		})
	}
}

func seedUsage(st *store.Store) {
	now := time.Now()
	models := []string{"qwen2.5-7b", "deepseek-r1-7b"}
	for i := 0; i < 46; i++ {
		prompt := int64(320 + (i*97)%900)
		completion := int64(180 + (i*53)%600)
		u := &model.TokenUsage{
			KeyName: "控制台", KeyID: "", Model: models[i%2],
			PromptTokens: prompt, CompletionTokens: completion, TotalTokens: prompt + completion,
			Status: 200, LatencyMS: int64(700 + (i*37)%2400),
			TS: now.Add(-time.Duration(i*31) * time.Minute),
		}
		if i%5 == 0 {
			u.KeyName = "team-a"
		}
		_ = st.InsertUsage(u)
	}
}

func seedUsers(st *store.Store) {
	_, _ = st.CreateUser("ops01", "operator123", "operator", false)
	_, _ = st.CreateUser("guest", "viewer12345", "viewer", false)
}

func seedKeys(st *store.Store) {
	for _, name := range []string{"team-a", "ci-pipeline"} {
		plain, prefix, hash, id := gateway.GenerateKey()
		_ = plain
		_ = st.CreateAPIKey(&model.APIKey{
			ID: id, Name: name, Prefix: prefix, KeyHash: hash, Role: "operator",
			CreatedAt: time.Now().Add(-48 * time.Hour),
		})
	}
}

func seedAudit(st *store.Store) {
	events := []struct{ action, target, detail, result string }{
		{"登录", "admin", "登录成功", "ok"},
		{"部署服务", "qwen-7b", "已授权宿主机权限=false", "ok"},
		{"部署服务", "deepseek-r1", "", "ok"},
		{"创建 API Key", "team-a", "", "ok"},
		{"服务操作", "vector-db", "stop", "ok"},
		{"更新系统设置", "site_name", "", "ok"},
	}
	for i, e := range events {
		_ = st.InsertAudit(&model.AuditLog{
			User: "admin", Role: "admin", Action: e.action, Target: e.target,
			Detail: e.detail, IP: "127.0.0.1", Result: e.result,
			CreatedAt: time.Now().Add(-time.Duration(len(events)-i) * 23 * time.Minute),
		})
	}
}

func round(v float64) float64 {
	return math.Round(v*100) / 100
}

func startFakeLLM() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/v1/models" {
			_, _ = w.Write([]byte(`{"object":"list","data":[{"id":"qwen2.5-7b","object":"model"},{"id":"deepseek-r1-7b","object":"model"}]}`))
			return
		}
		if r.URL.Path == "/" {
			_, _ = w.Write([]byte(`{"status":"ok"}`))
			return
		}
		var req struct {
			Model    string `json:"model"`
			Stream   bool   `json:"stream"`
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
		}
		body, _ := json.Marshal(nil)
		_ = body
		_ = json.NewDecoder(r.Body).Decode(&req)
		answer := "这是 AIDash 演示模型的回复。\n\n当前主机运行状态正常：CPU 与内存均在安全水位，2 个模型服务已通过网关对外提供 OpenAI 兼容接口。你可以继续提问，或在「模型服务」中查看容器日志与终端。"
		if len(req.Messages) > 0 {
			last := req.Messages[len(req.Messages)-1].Content
			answer = fmt.Sprintf("收到：「%s」\n\n（演示模型回复）当前的 AIDash 平台已就绪：模型服务通过 Docker 容器运行，网关负责鉴权与 Token 计量。可以在左侧「模型服务」里进入容器终端，或在「AI 运维助手」中让我执行诊断命令。", truncate(last, 60))
		}
		if req.Stream {
			w.Header().Set("Content-Type", "text/event-stream")
			for _, part := range strings.Split(answer, "") {
				payload, _ := json.Marshal(map[string]any{
					"id": "chatcmpl-demo", "object": "chat.completion.chunk", "model": req.Model,
					"choices": []map[string]any{{"index": 0, "delta": map[string]string{"content": part}}},
				})
				_, _ = fmt.Fprintf(w, "data: %s\n\n", payload)
				time.Sleep(8 * time.Millisecond)
			}
			_, _ = w.Write([]byte("data: [DONE]\n\n"))
			return
		}
		payload := map[string]any{
			"id": "chatcmpl-demo", "object": "chat.completion", "model": req.Model,
			"choices": []map[string]any{{
				"index": 0, "message": map[string]string{"role": "assistant", "content": answer},
				"finish_reason": "stop",
			}},
			"usage": map[string]int{"prompt_tokens": 26, "completion_tokens": len([]rune(answer)) / 2, "total_tokens": 26 + len([]rune(answer))/2},
		}
		_ = json.NewEncoder(w).Encode(payload)
	}))
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

func corsForDemo() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

func registerStatic(r *gin.Engine, dir string) {
	if abs, err := filepath.Abs(dir); err == nil {
		dir = abs
	}
	r.Static("/assets", filepath.Join(dir, "assets"))
	r.StaticFile("/favicon.svg", filepath.Join(dir, "favicon.svg"))
	r.NoRoute(func(c *gin.Context) {
		p := c.Request.URL.Path
		if strings.HasPrefix(p, "/api") || strings.HasPrefix(p, "/v1") || strings.HasPrefix(p, "/ws") {
			c.JSON(404, gin.H{"error": "not found"})
			return
		}
		c.File(filepath.Join(dir, "index.html"))
	})
}
