package gateway_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gxfdev/aidash/server/internal/catalog"
	"github.com/gxfdev/aidash/server/internal/engine"
	"github.com/gxfdev/aidash/server/internal/gateway"
	"github.com/gxfdev/aidash/server/internal/mockdocker"
	"github.com/gxfdev/aidash/server/internal/model"
	"github.com/gxfdev/aidash/server/internal/orchestrator"
	"github.com/gxfdev/aidash/server/internal/store"
)

func TestCopilotToolLoop(t *testing.T) {
	var calls int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/models":
			_, _ = w.Write([]byte(`{"object":"list","data":[{"id":"tool-model"}]}`))
			return
		case "/v1/chat/completions":
		default:
			w.WriteHeader(404)
			return
		}
		body, _ := io.ReadAll(r.Body)
		var req struct {
			Messages []map[string]any `json:"messages"`
		}
		_ = json.Unmarshal(body, &req)
		sawToolResult := false
		for _, m := range req.Messages {
			if m["role"] == "tool" {
				sawToolResult = true
			}
		}
		n := atomic.AddInt32(&calls, 1)
		if !sawToolResult {
			// Ask for a tool call: host_overview.
			_, _ = fmt.Fprintf(w, `{"id":"c%d","object":"chat.completion","model":"tool-model","choices":[{"index":0,"message":{"role":"assistant","content":"","tool_calls":[{"id":"call_1","type":"function","function":{"name":"host_overview","arguments":"{}"}}]},"finish_reason":"tool_calls"}]}`, n)
			return
		}
		_, _ = fmt.Fprintf(w, `{"id":"c%d","object":"chat.completion","model":"tool-model","choices":[{"index":0,"message":{"role":"assistant","content":"根据实时指标，CPU 与内存均处于安全水位。"},"finish_reason":"stop"}],"usage":{"prompt_tokens":12,"completion_tokens":9,"total_tokens":21}}`, n)
	}))
	defer upstream.Close()

	port := 0
	_, _ = fmt.Sscanf(strings.TrimPrefix(upstream.URL, "http://127.0.0.1:"), "%d", &port)
	if port == 0 {
		t.Fatalf("cannot parse port from %s", upstream.URL)
	}

	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	cat, _ := catalog.Load()
	mock := mockdocker.New()
	defer mock.Close()
	dc, _ := engine.New(mock.URL())
	orch := orchestrator.New(st, dc, cat)
	gw := gateway.New(st, orch)

	// Health probes for this deployment are irrelevant; keep them short.
	t.Setenv("AIDASH_HEALTH_TIMEOUT", "500ms")
	t.Setenv("AIDASH_TARGET_OS", "linux")
	svc, err := orch.Deploy(context.Background(), orchestrator.DeployRequest{
		Name: "tool-llm", Kind: "model", Image: "example/llm:1", ModelName: "tool-model",
		Ports: []model.PortMap{{ContPort: port, Protocol: "tcp", Accessible: true}},
	})
	if err != nil {
		t.Fatalf("deploy: %v", err)
	}
	deadline := time.Now().Add(10 * time.Second)
	for {
		cur, _ := st.GetService(svc.ID)
		if cur != nil && cur.Status == orchestrator.StatusRunning {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("service not running: %+v", cur)
		}
		time.Sleep(100 * time.Millisecond)
	}

	snap := &model.HostSnapshot{
		Timestamp: time.Now(),
		Host:      model.HostInfo{Hostname: "test", Platform: "linux"},
		CPU:       model.CPU{Cores: 8, UsagePercent: 12.5},
		Memory:    model.Memory{TotalGB: 32, UsedGB: 8, UsagePercent: 25},
	}
	cp := gateway.NewCopilot(st, orch, gw, func() *model.HostSnapshot { return snap })

	resp, err := cp.Chat(context.Background(), gateway.CopilotRequest{
		Model:    "tool-model",
		Messages: []gateway.ChatMessage{{Role: "user", Content: "现在主机状态如何？"}},
	})
	if err != nil {
		t.Fatalf("chat: %v", err)
	}
	if resp.Reply == "" || !strings.Contains(resp.Reply, "安全水位") {
		t.Fatalf("reply = %q", resp.Reply)
	}
	if len(resp.Steps) != 1 || resp.Steps[0].Tool != "host_overview" {
		t.Fatalf("steps = %+v", resp.Steps)
	}
	if !strings.Contains(resp.Steps[0].Result, "usage_percent") {
		t.Fatalf("tool result = %s", resp.Steps[0].Result)
	}

}

func TestCopilotExecDeniedWithoutPermission(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/v1/models" {
			_, _ = w.Write([]byte(`{"object":"list","data":[{"id":"tool-model"}]}`))
			return
		}
		body, _ := io.ReadAll(r.Body)
		if strings.Contains(string(body), `"role":"tool"`) {
			_, _ = w.Write([]byte(`{"id":"c2","object":"chat.completion","choices":[{"index":0,"message":{"role":"assistant","content":"已停止执行。"},"finish_reason":"stop"}]}`))
			return
		}
		_, _ = w.Write([]byte(`{"id":"c1","object":"chat.completion","choices":[{"index":0,"message":{"role":"assistant","content":"","tool_calls":[{"id":"call_x","type":"function","function":{"name":"run_command","arguments":"{\"service_id\":\"ops-sandbox\",\"command\":\"id\"}"}}]},"finish_reason":"tool_calls"}]}`))
	}))
	defer srv.Close()
	port := 0
	_, _ = fmt.Sscanf(strings.TrimPrefix(srv.URL, "http://127.0.0.1:"), "%d", &port)

	dir := t.TempDir()
	st, _ := store.Open(filepath.Join(dir, "t.db"))
	defer st.Close()
	cat, _ := catalog.Load()
	mock := mockdocker.New()
	defer mock.Close()
	dc, _ := engine.New(mock.URL())
	orch := orchestrator.New(st, dc, cat)
	gw := gateway.New(st, orch)
	svc, err := orch.Deploy(context.Background(), orchestrator.DeployRequest{
		Name: "tool-llm", Kind: "model", Image: "example/llm:1", ModelName: "tool-model",
		Ports: []model.PortMap{{ContPort: port, Protocol: "tcp", Accessible: true}},
	})
	if err != nil {
		t.Fatal(err)
	}
	waitRunning(t, st, svc.ID)
	cp := gateway.NewCopilot(st, orch, gw, func() *model.HostSnapshot { return nil })

	resp, err := cp.Chat(context.Background(), gateway.CopilotRequest{
		Model:     "tool-model",
		Messages:  []gateway.ChatMessage{{Role: "user", Content: "在沙箱执行 id"}},
		AllowExec: false,
	})
	if err != nil {
		t.Fatalf("chat: %v", err)
	}
	if len(resp.Steps) != 1 || !strings.Contains(resp.Steps[0].Result, "未开启") {
		t.Fatalf("expected permission denial, got %+v", resp.Steps)
	}
}

func waitRunning(t *testing.T, st *store.Store, id string) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		cur, _ := st.GetService(id)
		if cur != nil && cur.Status == orchestrator.StatusRunning {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("service did not become running")
}
