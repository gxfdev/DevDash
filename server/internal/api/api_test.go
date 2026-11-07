package api_test

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gxfdev/aidash/server/internal/orchestrator"
	"github.com/gxfdev/aidash/server/internal/testutil"
)

func TestAuthAndRBAC(t *testing.T) {
	env := testutil.New(t)

	if res := env.Do(t, http.MethodGet, "/api/system/overview", "", nil); res.Status != http.StatusUnauthorized {
		t.Fatalf("unauthenticated overview = %d", res.Status)
	}
	if res := env.Do(t, http.MethodPost, "/api/auth/login", "", map[string]string{"username": "admin", "password": "wrong"}); res.Status != http.StatusUnauthorized {
		t.Fatalf("bad login = %d", res.Status)
	}

	me := env.Do(t, http.MethodGet, "/api/auth/me", env.Token, nil)
	if me.Status != 200 {
		t.Fatalf("me = %d %s", me.Status, me.Body)
	}

	// Create a viewer account and verify read-only enforcement.
	res := env.Do(t, http.MethodPost, "/api/users", env.Token, map[string]string{
		"username": "viewer1", "password": "viewer123456", "role": "viewer",
	})
	if res.Status != 201 {
		t.Fatalf("create viewer = %d %s", res.Status, res.Body)
	}
	viewerToken := env.Login(t, "viewer1", "viewer123456")
	if res := env.Do(t, http.MethodGet, "/api/catalog", viewerToken, nil); res.Status != 200 {
		t.Fatalf("viewer catalog = %d", res.Status)
	}
	deployBody := map[string]any{"catalog_id": "ai-ops-agent", "name": "viewer-deploy"}
	if res := env.Do(t, http.MethodPost, "/api/services", viewerToken, deployBody); res.Status != http.StatusForbidden {
		t.Fatalf("viewer deploy = %d %s", res.Status, res.Body)
	}
	if res := env.Do(t, http.MethodGet, "/api/users", viewerToken, nil); res.Status != http.StatusForbidden {
		t.Fatalf("viewer users = %d", res.Status)
	}
}

func TestOverviewAndMetrics(t *testing.T) {
	env := testutil.New(t)
	res := env.Do(t, http.MethodGet, "/api/system/overview", env.Token, nil)
	if res.Status != 200 {
		t.Fatalf("overview = %d %s", res.Status, res.Body)
	}
	var payload struct {
		Data struct {
			Edition  string         `json:"edition"`
			Services map[string]any `json:"services"`
			Engine   map[string]any `json:"engine"`
			Metrics  map[string]any `json:"metrics"`
			Usage    map[string]any `json:"usage"`
			Alerts   []any          `json:"alerts"`
		} `json:"data"`
	}
	env.JSON(t, res, &payload)
	if payload.Data.Edition == "" {
		t.Fatal("edition missing")
	}
	if payload.Data.Services == nil || payload.Data.Engine == nil || payload.Data.Metrics == nil {
		t.Fatalf("overview incomplete: %+v", payload.Data)
	}
	if connected, _ := payload.Data.Engine["connected"].(bool); !connected {
		t.Fatalf("engine should be connected to the mock: %+v", payload.Data.Engine)
	}

	res = env.Do(t, http.MethodGet, "/api/system/metrics?range=24h", env.Token, nil)
	if res.Status != 200 {
		t.Fatalf("metrics = %d %s", res.Status, res.Body)
	}
	res = env.Do(t, http.MethodGet, "/api/system/engine", env.Token, nil)
	if res.Status != 200 || !strings.Contains(string(res.Body), "27.0.0-mock") {
		t.Fatalf("engine = %d %s", res.Status, res.Body)
	}
	res = env.Do(t, http.MethodGet, "/api/health", "", nil)
	if res.Status != 200 {
		t.Fatalf("health = %d", res.Status)
	}
}

func TestServicesAPI(t *testing.T) {
	env := testutil.New(t)
	res := env.Do(t, http.MethodPost, "/api/services", env.Token, map[string]any{
		"catalog_id": "ai-ops-agent",
		"name":       "api-sandbox",
	})
	if res.Status != http.StatusAccepted {
		t.Fatalf("deploy = %d %s", res.Status, res.Body)
	}
	var created struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	env.JSON(t, res, &created)
	env.WaitStatus(t, created.Data.ID, orchestrator.StatusRunning, 15*time.Second)

	res = env.Do(t, http.MethodGet, "/api/services", env.Token, nil)
	if res.Status != 200 || !strings.Contains(string(res.Body), "api-sandbox") {
		t.Fatalf("list = %d %s", res.Status, res.Body)
	}
	res = env.Do(t, http.MethodGet, "/api/services/"+created.Data.ID, env.Token, nil)
	if res.Status != 200 {
		t.Fatalf("get = %d %s", res.Status, res.Body)
	}
	res = env.Do(t, http.MethodGet, "/api/services/"+created.Data.ID+"/events", env.Token, nil)
	if res.Status != 200 || !strings.Contains(string(res.Body), "部署任务") {
		t.Fatalf("events = %d %s", res.Status, res.Body)
	}
	res = env.Do(t, http.MethodPost, "/api/services/"+created.Data.ID+"/action", env.Token, map[string]string{"action": "stop"})
	if res.Status != 200 {
		t.Fatalf("stop = %d %s", res.Status, res.Body)
	}
	res = env.Do(t, http.MethodDelete, "/api/services/"+created.Data.ID, env.Token, nil)
	if res.Status != 200 {
		t.Fatalf("delete = %d %s", res.Status, res.Body)
	}
	res = env.Do(t, http.MethodGet, "/api/services", env.Token, nil)
	if strings.Contains(string(res.Body), "api-sandbox") {
		t.Fatal("service should be gone")
	}
}

func TestAdminSettingsAndAudit(t *testing.T) {
	env := testutil.New(t)
	res := env.Do(t, http.MethodGet, "/api/settings", env.Token, nil)
	if res.Status != 200 {
		t.Fatalf("settings = %d", res.Status)
	}
	res = env.Do(t, http.MethodPut, "/api/settings", env.Token, map[string]string{"site_name": "实验室 AI OS"})
	if res.Status != 200 || !strings.Contains(string(res.Body), "实验室 AI OS") {
		t.Fatalf("update settings = %d %s", res.Status, res.Body)
	}
	env.Do(t, http.MethodGet, "/api/audit", env.Token, nil)
	res = env.Do(t, http.MethodGet, "/api/audit?q=登录", env.Token, nil)
	if res.Status != 200 || !strings.Contains(string(res.Body), "登录") {
		t.Fatalf("audit = %d %s", res.Status, res.Body)
	}
}

func TestDockerAPI(t *testing.T) {
	env := testutil.New(t)
	for _, path := range []string{"/api/docker/containers", "/api/docker/images", "/api/docker/networks", "/api/docker/volumes", "/api/docker/df"} {
		res := env.Do(t, http.MethodGet, path, env.Token, nil)
		if res.Status != 200 {
			t.Fatalf("%s = %d %s", path, res.Status, res.Body)
		}
	}
	res := env.Do(t, http.MethodPost, "/api/docker/images/pull", env.Token, map[string]string{"image": "nginx", "tag": "alpine"})
	if res.Status != 200 || !strings.Contains(string(res.Body), "done") {
		t.Fatalf("pull = %d %s", res.Status, res.Body)
	}
}

func TestCopilotGracefulWithoutModel(t *testing.T) {
	env := testutil.New(t)
	res := env.Do(t, http.MethodPost, "/api/copilot/chat", env.Token, map[string]any{
		"messages": []map[string]string{{"role": "user", "content": "现在服务器状态如何？"}},
	})
	if res.Status != 200 {
		t.Fatalf("copilot = %d %s", res.Status, res.Body)
	}
	if !strings.Contains(string(res.Body), "error") {
		t.Fatalf("expected graceful error, got %s", res.Body)
	}
}
