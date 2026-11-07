package orchestrator_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gxfdev/aidash/server/internal/model"
	"github.com/gxfdev/aidash/server/internal/orchestrator"
	"github.com/gxfdev/aidash/server/internal/testutil"
)

func deploy(t *testing.T, env *testutil.Env, req orchestrator.DeployRequest) *model.Service {
	t.Helper()
	svc, err := env.Orch.Deploy(context.Background(), req)
	if err != nil {
		t.Fatalf("deploy: %v", err)
	}
	return svc
}

func TestDeployOllamaModel(t *testing.T) {
	env := testutil.New(t)
	svc := deploy(t, env, orchestrator.DeployRequest{
		CatalogID: "qwen2.5-7b",
		Name:      "qwen-demo",
		Owner:     "admin",
	})
	if svc.Status != orchestrator.StatusPending {
		t.Fatalf("initial status = %s", svc.Status)
	}
	done := env.WaitStatus(t, svc.ID, orchestrator.StatusRunning, 15*time.Second)
	if done.Health == "" {
		t.Fatal("health should be set")
	}

	containers := env.Mock.ContainerList()
	if len(containers) != 1 {
		t.Fatalf("expected 1 container, got %d", len(containers))
	}
	ct := containers[0]
	if ct.Name != "aidash-qwen-demo" {
		t.Fatalf("container name = %q", ct.Name)
	}
	if ct.Config.Labels["aios.service_id"] != svc.ID {
		t.Fatalf("missing service label: %+v", ct.Config.Labels)
	}
	if ct.Config.Labels["aios.model"] != "qwen2.5-7b" {
		t.Fatalf("missing model label: %+v", ct.Config.Labels)
	}
	if ct.Config.HostConfig.Privileged {
		t.Fatal("model must not get privileged mode by default")
	}
	if ct.Config.HostConfig.NanoCPUs != int64(4*1e9) {
		t.Fatalf("cpu limit = %d", ct.Config.HostConfig.NanoCPUs)
	}
	if ct.Config.HostConfig.Memory != 8192*1024*1024 {
		t.Fatalf("memory limit = %d", ct.Config.HostConfig.Memory)
	}
	if len(ct.Config.HostConfig.PortBindings) != 1 {
		t.Fatalf("expected 1 published port, got %d", len(ct.Config.HostConfig.PortBindings))
	}
	for key := range ct.Config.HostConfig.PortBindings {
		if key != "11434/tcp" {
			t.Fatalf("unexpected port key %s", key)
		}
	}

	// Events should record the pull + model download + ready state.
	events, err := env.Store.ListServiceEvents(svc.ID, 100)
	if err != nil {
		t.Fatalf("events: %v", err)
	}
	var messages []string
	for _, e := range events {
		messages = append(messages, e.Message)
	}
	joined := strings.Join(messages, "\n")
	for _, want := range []string{"拉取镜像", "模型", "服务已就绪"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("events missing %q:\n%s", want, joined)
		}
	}

	// Logs via mock.
	env.Mock.SetContainerLogs(ct.ID, "ollama listening on 11434")
	logs, err := env.Orch.Logs(context.Background(), svc.ID, 20)
	if err != nil {
		t.Fatalf("logs: %v", err)
	}
	if !strings.Contains(logs, "ollama listening") {
		t.Fatalf("logs = %q", logs)
	}

	// Stats.
	stats, err := env.Orch.Stats(context.Background(), svc.ID)
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	if stats.MemoryMB <= 0 {
		t.Fatalf("stats memory = %v", stats.MemoryMB)
	}

	// Console command.
	out, err := env.Orch.RunCommand(context.Background(), svc.ID, "uname -a", 10*time.Second)
	if err != nil {
		t.Fatalf("run command: %v", err)
	}
	if !strings.Contains(out, "mock") {
		t.Fatalf("command output = %q", out)
	}

	// Stop / start / restart lifecycle.
	if err := env.Orch.Stop(context.Background(), svc.ID); err != nil {
		t.Fatalf("stop: %v", err)
	}
	stopped := env.WaitStatus(t, svc.ID, orchestrator.StatusStopped, 5*time.Second)
	if stopped.Health == "healthy" {
		t.Fatal("stopped service should not be healthy")
	}
	if err := env.Orch.Start(context.Background(), svc.ID); err != nil {
		t.Fatalf("start: %v", err)
	}
	env.WaitStatus(t, svc.ID, orchestrator.StatusRunning, 10*time.Second)

	// Remove.
	if err := env.Orch.Remove(context.Background(), svc.ID, true); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if _, err := env.Store.GetService(svc.ID); err == nil {
		t.Fatal("service should be deleted")
	}
	if len(env.Mock.ContainerList()) != 0 {
		t.Fatal("container should be removed")
	}
}

func TestDeployHostAccessAgent(t *testing.T) {
	env := testutil.New(t)
	svc := deploy(t, env, orchestrator.DeployRequest{
		CatalogID: "ai-ops-agent",
		Name:      "ops-sandbox",
		Owner:     "admin",
	})
	env.WaitStatus(t, svc.ID, orchestrator.StatusRunning, 15*time.Second)

	containers := env.Mock.ContainerList()
	if len(containers) != 1 {
		t.Fatalf("containers = %d", len(containers))
	}
	hc := containers[0].Config.HostConfig
	if !hc.Privileged {
		t.Fatal("host-access workload must be privileged")
	}
	if hc.PidMode != "host" {
		t.Fatalf("pid mode = %q", hc.PidMode)
	}
	joined := strings.Join(hc.Binds, ",")
	for _, want := range []string{"/:/host:rw", "/var/run/docker.sock", "/proc:/host/proc:ro"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("binds missing %q: %v", want, hc.Binds)
		}
	}
	caps := strings.Join(hc.CapAdd, ",")
	if !strings.Contains(caps, "SYS_ADMIN") || !strings.Contains(caps, "SYS_PTRACE") {
		t.Fatalf("caps = %v", hc.CapAdd)
	}
	if dc := strings.Join(hc.SecurityOpt, ","); !strings.Contains(dc, "seccomp=unconfined") {
		t.Fatalf("security opts = %v", hc.SecurityOpt)
	}
	envs := strings.Join(containers[0].Config.Env, ",")
	if !strings.Contains(envs, "HOST_ROOT=/host") || !strings.Contains(envs, "AIDASH_HOST_ACCESS=true") {
		t.Fatalf("env = %v", containers[0].Config.Env)
	}
	stored, _ := env.Store.GetService(svc.ID)
	if !stored.HostAccess || !stored.Spec.HostAccess.Enabled {
		t.Fatal("service should record host access")
	}
}

func TestDeployCustomSpecValidations(t *testing.T) {
	env := testutil.New(t)
	if _, err := env.Orch.Deploy(context.Background(), orchestrator.DeployRequest{
		Name: "Bad Name", Image: "nginx:latest",
	}); err == nil {
		t.Fatal("invalid name should be rejected")
	}
	if _, err := env.Orch.Deploy(context.Background(), orchestrator.DeployRequest{
		Name: "no-image",
	}); err == nil {
		t.Fatal("missing catalog/image should be rejected")
	}
	svc := deploy(t, env, orchestrator.DeployRequest{
		Name: "custom-llm", Kind: "model", Image: "example/llm:1.0", ModelName: "custom-model",
		Ports:     []model.PortMap{{ContPort: 8000, Protocol: "tcp", Accessible: true}},
		Resources: &model.ResourceSpec{CPUs: 2, MemoryMB: 4096},
	})
	env.WaitStatus(t, svc.ID, orchestrator.StatusRunning, 15*time.Second)
	if svc.ModelName != "custom-model" {
		t.Fatalf("model name = %q", svc.ModelName)
	}
	models := env.GW.ListModels(context.Background())
	if len(models) != 1 || models[0]["id"] != "custom-model" {
		t.Fatalf("gateway models = %+v", models)
	}
}

func TestGatewayProxyAndMetering(t *testing.T) {
	env := testutil.New(t)
	_, port := testutil.FakeOpenAI(t, "你好，我是测试模型")
	svc := deploy(t, env, orchestrator.DeployRequest{
		Name: "fake-llm", Kind: "model", Image: "example/llm:1.0", ModelName: "fake-model",
		Ports: []model.PortMap{{ContPort: port, Protocol: "tcp", Accessible: true}},
	})
	env.WaitStatus(t, svc.ID, orchestrator.StatusRunning, 15*time.Second)

	// Create an API key through the API.
	res := env.Do(t, http.MethodPost, "/api/gateway/keys", env.Token, map[string]any{"name": "ci-key", "token_limit": 0})
	if res.Status != 201 {
		t.Fatalf("create key: %d %s", res.Status, res.Body)
	}
	var keyResp struct {
		Data struct {
			Secret string `json:"secret"`
		} `json:"data"`
	}
	env.JSON(t, res, &keyResp)
	if keyResp.Data.Secret == "" {
		t.Fatal("secret not returned")
	}

	chat := map[string]any{
		"model":    "fake-model",
		"messages": []map[string]string{{"role": "user", "content": "你好"}},
	}
	res = env.Do(t, http.MethodPost, "/v1/chat/completions", keyResp.Data.Secret, chat)
	if res.Status != 200 {
		t.Fatalf("chat: %d %s", res.Status, res.Body)
	}
	if !strings.Contains(string(res.Body), "你好，我是测试模型") {
		t.Fatalf("response = %s", res.Body)
	}
	var upstream map[string]any
	_ = json.Unmarshal(res.Body, &upstream)
	if upstream["usage"] == nil {
		t.Fatal("usage should be forwarded")
	}

	totals, err := env.Store.UsageTotalsSince(0)
	if err != nil {
		t.Fatalf("usage: %v", err)
	}
	if totals.Requests != 1 || totals.TotalTokens != 18 {
		t.Fatalf("usage = %+v", totals)
	}
	keys, _ := env.Store.ListAPIKeys()
	if len(keys) != 1 || keys[0].TokensUsed != 18 {
		t.Fatalf("key usage = %+v", keys)
	}

	// Invalid key rejected.
	res = env.Do(t, http.MethodPost, "/v1/chat/completions", "sk-aidash-invalid-key-0000", chat)
	if res.Status != http.StatusUnauthorized {
		t.Fatalf("invalid key status = %d", res.Status)
	}

	// Unknown model rejected with 404.
	chat["model"] = "nope"
	res = env.Do(t, http.MethodPost, "/v1/chat/completions", keyResp.Data.Secret, chat)
	if res.Status != http.StatusNotFound {
		t.Fatalf("unknown model status = %d %s", res.Status, res.Body)
	}

	// Quota enforcement: a key with a tiny limit is blocked once usage exceeds it.
	chat["model"] = "fake-model"
	res = env.Do(t, http.MethodPost, "/api/gateway/keys", env.Token, map[string]any{"name": "tiny", "token_limit": 1})
	var tiny struct {
		Data struct {
			Secret string `json:"secret"`
		} `json:"data"`
	}
	env.JSON(t, res, &tiny)
	res = env.Do(t, http.MethodPost, "/v1/chat/completions", tiny.Data.Secret, chat)
	if res.Status != http.StatusOK {
		t.Fatalf("first tiny call = %d %s", res.Status, res.Body)
	}
	res = env.Do(t, http.MethodPost, "/v1/chat/completions", tiny.Data.Secret, chat)
	if res.Status != http.StatusTooManyRequests {
		t.Fatalf("quota status = %d %s", res.Status, res.Body)
	}
}
