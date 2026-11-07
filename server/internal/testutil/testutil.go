package testutil

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gxfdev/aidash/server/internal/api"
	"github.com/gxfdev/aidash/server/internal/auth"
	"github.com/gxfdev/aidash/server/internal/catalog"
	"github.com/gxfdev/aidash/server/internal/config"
	"github.com/gxfdev/aidash/server/internal/engine"
	"github.com/gxfdev/aidash/server/internal/gateway"
	"github.com/gxfdev/aidash/server/internal/mockdocker"
	"github.com/gxfdev/aidash/server/internal/model"
	"github.com/gxfdev/aidash/server/internal/modelhub"
	"github.com/gxfdev/aidash/server/internal/orchestrator"
	"github.com/gxfdev/aidash/server/internal/store"
)

type Env struct {
	Cfg     *config.Config
	Store   *store.Store
	Mock    *mockdocker.Mock
	Orch    *orchestrator.Orchestrator
	GW      *gateway.Gateway
	Copilot *gateway.Copilot
	Handler *api.Handler
	Router  *gin.Engine
	Server  *httptest.Server
	Token   string
}

func New(t *testing.T) *Env {
	t.Helper()
	gin.SetMode(gin.TestMode)
	dir := t.TempDir()
	t.Setenv("AIDASH_HEALTH_TIMEOUT", "1s")
	t.Setenv("AIDASH_TARGET_OS", "linux")

	cfg := &config.Config{
		Port: "0", DataDir: dir, DBPath: filepath.Join(dir, "test.db"),
		JWTSecret: "test-secret-key-0123456789abcdef",
		AdminUser: "admin", AdminPass: "admin123",
		Edition: "AIDash Test", Version: "test",
		CollectEvery: 30 * time.Second,
	}
	st, err := store.Open(cfg.DBPath)
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	if err := st.EnsureAdmin(cfg.AdminUser, cfg.AdminPass); err != nil {
		t.Fatalf("admin: %v", err)
	}
	cat, err := catalog.Load()
	if err != nil {
		t.Fatalf("catalog: %v", err)
	}
	mock := mockdocker.New()
	t.Cleanup(mock.Close)
	dc, err := engine.New(mock.URL())
	if err != nil {
		t.Fatalf("engine: %v", err)
	}
	orch := orchestrator.New(st, dc, cat)
	hub := modelhub.New(st, dc)
	gw := gateway.New(st, orch)
	snap := &model.HostSnapshot{
		Timestamp: time.Now(),
		Host:      model.HostInfo{Hostname: "test-host", Platform: "linux", PlatformVersion: "mock"},
	}
	cp := gateway.NewCopilot(st, orch, gw, func() *model.HostSnapshot { return snap })
	mgr := auth.NewManager(cfg.JWTSecret)
	h := api.NewHandler(cfg, st, orch, mgr, cat, gw, cp, hub)

	r := gin.New()
	r.Use(gin.Recovery())
	h.Register(r)

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	env := &Env{Cfg: cfg, Store: st, Mock: mock, Orch: orch, GW: gw, Copilot: cp, Handler: h, Router: r, Server: srv}
	env.Token = env.Login(t, "admin", "admin123")
	return env
}

type httpResult struct {
	Status int
	Body   []byte
}

func (e *Env) Do(t *testing.T, method, path, token string, body any) httpResult {
	t.Helper()
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, e.Server.URL+path, reader)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do %s %s: %v", method, path, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	return httpResult{Status: resp.StatusCode, Body: raw}
}

func (e *Env) Login(t *testing.T, user, pass string) string {
	t.Helper()
	res := e.Do(t, http.MethodPost, "/api/auth/login", "", map[string]string{"username": user, "password": pass})
	if res.Status != 200 {
		t.Fatalf("login failed: %d %s", res.Status, res.Body)
	}
	var payload struct {
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	if err := json.Unmarshal(res.Body, &payload); err != nil {
		t.Fatalf("login decode: %v", err)
	}
	return payload.Data.Token
}

func (e *Env) JSON(t *testing.T, res httpResult, out any) {
	t.Helper()
	if err := json.Unmarshal(res.Body, out); err != nil {
		t.Fatalf("decode %s: %v", res.Body, err)
	}
}

// WaitStatus polls the store until the service reaches the wanted status.
func (e *Env) WaitStatus(t *testing.T, id, status string, timeout time.Duration) *model.Service {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		svc, err := e.Store.GetService(id)
		if err == nil {
			if svc.Status == status {
				return svc
			}
			if svc.Status == orchestrator.StatusError && status == orchestrator.StatusRunning {
				t.Fatalf("service %s failed: %s", svc.Name, svc.Error)
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	svc, _ := e.Store.GetService(id)
	if svc != nil {
		t.Fatalf("timeout waiting for status %q, current=%q err=%q", status, svc.Status, svc.Error)
	}
	t.Fatalf("service %s not found", id)
	return nil
}

// FakeOpenAI starts a minimal OpenAI-compatible model server and returns
// its port. It serves /v1/models and /v1/chat/completions.
func FakeOpenAI(t *testing.T, content string) (url string, port int) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/models":
			_, _ = w.Write([]byte(`{"object":"list","data":[{"id":"fake-model","object":"model"}]}`))
		case "/v1/chat/completions":
			body, _ := io.ReadAll(r.Body)
			var req struct {
				Stream bool   `json:"stream"`
				Model  string `json:"model"`
			}
			_ = json.Unmarshal(body, &req)
			if req.Stream {
				w.Header().Set("Content-Type", "text/event-stream")
				_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"" + content + "\"}}]}\n\n"))
				_, _ = w.Write([]byte("data: [DONE]\n\n"))
				return
			}
			_, _ = w.Write([]byte(`{"id":"chatcmpl-1","object":"chat.completion","model":"fake-model","choices":[{"index":0,"message":{"role":"assistant","content":"` + content + `"},"finish_reason":"stop"}],"usage":{"prompt_tokens":11,"completion_tokens":7,"total_tokens":18}}`))
		default:
			w.WriteHeader(404)
			_, _ = w.Write([]byte(`{"error":"not found"}`))
		}
	}))
	t.Cleanup(srv.Close)
	u := srv.URL
	port = portOf(t, u)
	return u, port
}

func portOf(t *testing.T, rawURL string) int {
	t.Helper()
	i := len(rawURL) - 1
	for i >= 0 && rawURL[i] != ':' {
		i--
	}
	if i < 0 {
		t.Fatalf("no port in %s", rawURL)
	}
	port := 0
	for _, c := range rawURL[i+1:] {
		port = port*10 + int(c-'0')
	}
	return port
}

func WriteFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
