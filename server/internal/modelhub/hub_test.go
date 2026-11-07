package modelhub

import (
	"strings"
	"testing"

	"github.com/gxfdev/aidash/server/internal/model"
)

func TestValidateRequest(t *testing.T) {
	cases := []struct {
		name string
		req  CreateRequest
		ok   bool
	}{
		{"modelscope ok", CreateRequest{Source: "modelscope", Repo: "Qwen/Qwen2.5-0.5B-Instruct-GGUF", Name: "qwen"}, true},
		{"modelscope missing repo", CreateRequest{Source: "modelscope", Name: "qwen"}, false},
		{"modelscope injection", CreateRequest{Source: "modelscope", Repo: "Qwen/Qwen; rm -rf /", Name: "qwen"}, false},
		{"hf ok", CreateRequest{Source: "huggingface", Repo: "Qwen/Qwen2.5-0.5B-Instruct-GGUF"}, true},
		{"ollama ok", CreateRequest{Source: "ollama", Repo: "qwen2.5:0.5b"}, true},
		{"local ok", CreateRequest{Source: "local", HostPath: "/opt/models/qwen"}, true},
		{"local relative", CreateRequest{Source: "local", HostPath: "models/qwen"}, false},
		{"url ok", CreateRequest{Source: "url", URL: "https://example.com/model.gguf"}, true},
		{"url bad", CreateRequest{Source: "url", URL: "ftp://example.com/model.gguf"}, false},
		{"unknown source", CreateRequest{Source: "s3"}, false},
	}
	for _, c := range cases {
		err := validateRequest(c.req)
		if c.ok && err != nil {
			t.Fatalf("%s: unexpected error %v", c.name, err)
		}
		if !c.ok && err == nil {
			t.Fatalf("%s: expected error", c.name)
		}
	}
}

func TestDownloadPlan(t *testing.T) {
	asset := &model.ModelAsset{
		ID: "mabc", Name: "qwen", Source: "modelscope", Repo: "Qwen/Qwen2.5-0.5B-Instruct-GGUF",
		Revision: "master", File: "*.gguf", LocalPath: "/models/mabc", Runtime: "llamacpp",
	}
	image, cmd, env := downloadPlan(asset, CreateRequest{})
	if image != "python:3.12-slim" {
		t.Fatalf("image = %s", image)
	}
	if !strings.Contains(cmd, "modelscope download") || !strings.Contains(cmd, "Qwen/Qwen2.5-0.5B-Instruct-GGUF") || !strings.Contains(cmd, "--include *.gguf") {
		t.Fatalf("modelscope cmd = %s", cmd)
	}
	if !strings.Contains(strings.Join(env, ","), "PIP_INDEX_URL=") {
		t.Fatalf("env = %v", env)
	}

	asset.Source = "huggingface"
	asset.Mirror = "https://hf-mirror.com"
	image, cmd, env = downloadPlan(asset, CreateRequest{Token: "hf_xxx"})
	if !strings.Contains(cmd, "snapshot_download") || !strings.Contains(cmd, "local_dir='/models/mabc'") {
		t.Fatalf("hf cmd = %s", cmd)
	}
	joined := strings.Join(env, ",")
	if !strings.Contains(joined, "HF_ENDPOINT=https://hf-mirror.com") || !strings.Contains(joined, "HF_TOKEN=hf_xxx") {
		t.Fatalf("hf env = %v", env)
	}

	asset.Source = "url"
	image, cmd, _ = downloadPlan(asset, CreateRequest{URL: "https://example.com/model.tar.gz"})
	if image != "alpine:3.20" || !strings.Contains(cmd, "tar -xf") {
		t.Fatalf("url plan image=%s cmd=%s", image, cmd)
	}
}

func TestParseScanOutput(t *testing.T) {
	out := "504658657\t/models/mabc\n3\n/models/mabc/qwen2.5-0.5b-q4_k_m.gguf\n"
	size, files, gguf := parseScanOutput(out)
	if size != 504658657 || files != 3 || gguf != "/models/mabc/qwen2.5-0.5b-q4_k_m.gguf" {
		t.Fatalf("size=%d files=%d gguf=%s", size, files, gguf)
	}
}

func TestSlugify(t *testing.T) {
	cases := map[string]string{
		"Qwen2.5 7B":      "qwen2-5-7b",
		"Qwen/Qwen2.5-7B": "qwen-qwen2-5-7b",
		"deepseek-r1:7b":  "deepseek-r1-7b",
		"___":             "model",
	}
	for in, want := range cases {
		if got := slugify(in); got != want {
			t.Fatalf("slugify(%q)=%q want %q", in, got, want)
		}
	}
}
