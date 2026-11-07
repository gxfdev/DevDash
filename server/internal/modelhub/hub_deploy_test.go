package modelhub_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/gxfdev/aidash/server/internal/model"
	"github.com/gxfdev/aidash/server/internal/orchestrator"
	"github.com/gxfdev/aidash/server/internal/testutil"
)

// TestDeployFromDownloadedAsset proves that a completed local model asset is
// mounted into a real inference container with the right runtime command.
func TestDeployFromDownloadedAsset(t *testing.T) {
	env := testutil.New(t)
	asset := &model.ModelAsset{
		ID: "mtest01", Name: "qwen-local", Source: "modelscope", Repo: "Qwen/Qwen2.5-0.5B-Instruct-GGUF",
		Status: "completed", Runtime: "llamacpp", Volume: "aidash-models",
		LocalPath: "/models/mtest01", GGUF: "/models/mtest01/qwen2.5-0.5b-instruct-q4_k_m.gguf",
		SizeBytes: 491400000, FileCount: 1,
	}
	if err := env.Store.CreateAsset(asset); err != nil {
		t.Fatalf("create asset: %v", err)
	}

	svc, err := env.Orch.Deploy(context.Background(), orchestrator.DeployRequest{
		Name: "qwen-local", Kind: "model", ModelName: "qwen-local",
		ModelAssetID: asset.ID, ModelRuntime: "llamacpp",
	})
	if err != nil {
		t.Fatalf("deploy: %v", err)
	}
	env.WaitStatus(t, svc.ID, orchestrator.StatusRunning, 20*time.Second)

	containers := env.Mock.ContainerList()
	if len(containers) != 1 {
		t.Fatalf("containers = %d", len(containers))
	}
	c := containers[0]
	if !strings.Contains(c.Image, "llama.cpp") {
		t.Fatalf("image = %s", c.Image)
	}
	cmd := strings.Join(c.Config.Cmd, " ")
	if !strings.Contains(cmd, "-m /models/mtest01/qwen2.5-0.5b-instruct-q4_k_m.gguf") || !strings.Contains(cmd, "--port 8080") {
		t.Fatalf("cmd = %s", cmd)
	}
	binds := strings.Join(c.Config.HostConfig.Binds, ",")
	if !strings.Contains(binds, "aidash-models:/models") {
		t.Fatalf("binds = %s", binds)
	}
	if len(c.Config.HostConfig.PortBindings["8080/tcp"]) == 0 {
		t.Fatalf("missing 8080 binding: %+v", c.Config.HostConfig.PortBindings)
	}
	stored, _ := env.Store.GetService(svc.ID)
	if stored.Spec.ModelAssetID != asset.ID || stored.Spec.ModelRuntime != "llamacpp" {
		t.Fatalf("spec asset fields = %+v", stored.Spec)
	}
	models := env.GW.ListModels(context.Background())
	if len(models) != 1 || models[0]["id"] != "qwen-local" {
		t.Fatalf("gateway models = %+v", models)
	}
}

func TestDeployFromVLLMAssetRequiresGPU(t *testing.T) {
	env := testutil.New(t)
	asset := &model.ModelAsset{
		ID: "mtest02", Name: "qwen-hf", Source: "huggingface", Repo: "Qwen/Qwen2.5-0.5B-Instruct",
		Status: "completed", Runtime: "vllm", Volume: "aidash-models",
		LocalPath: "/models/mtest02", SizeBytes: 1 << 30, FileCount: 12,
	}
	if err := env.Store.CreateAsset(asset); err != nil {
		t.Fatal(err)
	}
	svc, err := env.Orch.Deploy(context.Background(), orchestrator.DeployRequest{
		Name: "qwen-vllm", Kind: "model", ModelName: "qwen-vllm",
		ModelAssetID: asset.ID, ModelRuntime: "vllm",
	})
	if err != nil {
		t.Fatalf("deploy: %v", err)
	}
	env.WaitStatus(t, svc.ID, orchestrator.StatusRunning, 20*time.Second)
	c := env.Mock.ContainerList()[0]
	cmd := strings.Join(c.Config.Cmd, " ")
	if !strings.Contains(cmd, "--model /models/mtest02") || !strings.Contains(cmd, "--served-model-name qwen-vllm") {
		t.Fatalf("cmd = %s", cmd)
	}
	if len(c.Config.HostConfig.DeviceRequests) == 0 || c.Config.HostConfig.DeviceRequests[0].Count != 1 {
		t.Fatalf("expected 1 GPU device request: %+v", c.Config.HostConfig.DeviceRequests)
	}
	if c.Config.HostConfig.ShmSize < 2<<30 {
		t.Fatalf("shm = %d", c.Config.HostConfig.ShmSize)
	}
}

func TestDeployRejectsIncompleteAsset(t *testing.T) {
	env := testutil.New(t)
	asset := &model.ModelAsset{
		ID: "mtest03", Name: "downloading", Source: "modelscope", Status: "downloading",
		LocalPath: "/models/mtest03",
	}
	if err := env.Store.CreateAsset(asset); err != nil {
		t.Fatal(err)
	}
	_, err := env.Orch.Deploy(context.Background(), orchestrator.DeployRequest{
		Name: "too-early", Kind: "model", ModelName: "too-early", ModelAssetID: asset.ID,
	})
	if err == nil || !strings.Contains(err.Error(), "尚未准备完成") {
		t.Fatalf("expected incomplete-asset error, got %v", err)
	}
}
