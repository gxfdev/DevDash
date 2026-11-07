package orchestrator

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/gxfdev/aidash/server/internal/catalog"
	"github.com/gxfdev/aidash/server/internal/engine"
	"github.com/gxfdev/aidash/server/internal/model"
)

var nameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,40}$`)

// DeployRequest is the user-facing description of a new workload. Only
// Name + (CatalogID or Image) are required; the rest falls back to the
// catalog defaults.
type DeployRequest struct {
	CatalogID    string              `json:"catalog_id"`
	Name         string              `json:"name"`
	Kind         string              `json:"kind"`
	Image        string              `json:"image"`
	ImageTag     string              `json:"image_tag"`
	Command      []string            `json:"command"`
	Env          map[string]string   `json:"env"`
	Ports        []model.PortMap     `json:"ports"`
	Volumes      []model.VolumeMount `json:"volumes"`
	Resources    *model.ResourceSpec `json:"resources"`
	HostAccess   *model.HostAccess   `json:"host_access"`
	ModelName    string              `json:"model_name"`
	ModelAssetID string              `json:"model_asset_id"`
	ModelRuntime string              `json:"model_runtime"`
	AutoStart    *bool               `json:"auto_start"`
	Owner        string              `json:"owner"`
}

func (o *Orchestrator) BuildSpec(req DeployRequest, cat *catalog.Catalog) (*model.ServiceSpec, error) {
	name := strings.ToLower(strings.TrimSpace(req.Name))
	if name == "" {
		return nil, fmt.Errorf("服务名称不能为空")
	}
	if !nameRe.MatchString(name) {
		return nil, fmt.Errorf("服务名称只能包含小写字母、数字和连字符（2-41 位）")
	}

	spec := &model.ServiceSpec{
		Name:      name,
		Env:       map[string]string{},
		Restart:   "unless-stopped",
		AutoStart: true,
	}

	if req.CatalogID != "" {
		it, ok := cat.Get(req.CatalogID)
		if !ok {
			return nil, fmt.Errorf("目录中不存在应用：%s", req.CatalogID)
		}
		spec.Kind = it.Kind
		spec.CatalogID = it.ID
		spec.Image = it.Image + ":" + it.ImageTag
		spec.Command = append([]string{}, it.Command...)
		for k, v := range it.Env {
			spec.Env[k] = v
		}
		spec.Ports = append([]model.PortMap{}, it.Ports...)
		spec.Volumes = append([]model.VolumeMount{}, it.Volumes...)
		spec.Resources = it.Resources
		spec.ModelName = it.ModelName
		spec.Description = it.Summary
		if spec.Description == "" {
			spec.Description = it.Description
		}
		spec.Labels = map[string]string{"aios.catalog": it.ID, "aios.runtime": it.Runtime}
		if it.DefaultHostAccess {
			spec.HostAccess = model.HostAccess{Enabled: true, Privileged: true, HostPID: true, MountHostFS: true, MountDockerSocket: true}
		}
	} else {
		if req.Image == "" && req.ModelAssetID == "" {
			return nil, fmt.Errorf("必须提供 catalog_id、模型资产或镜像地址")
		}
		spec.Image = req.Image
		if req.ImageTag != "" && !strings.Contains(req.Image, ":") {
			spec.Image = req.Image + ":" + req.ImageTag
		}
		spec.Kind = req.Kind
		if spec.Kind == "" {
			spec.Kind = "custom"
		}
	}
	if spec.Kind == "" {
		spec.Kind = "app"
	}
	if req.Command != nil {
		spec.Command = req.Command
	}
	for k, v := range req.Env {
		spec.Env[k] = v
	}
	if req.Ports != nil {
		spec.Ports = req.Ports
	}
	if req.Volumes != nil {
		spec.Volumes = req.Volumes
	}
	if req.Resources != nil {
		spec.Resources = mergeResources(spec.Resources, *req.Resources)
	}
	if req.ModelName != "" {
		spec.ModelName = req.ModelName
	}
	if req.AutoStart != nil {
		spec.AutoStart = *req.AutoStart
	}
	if req.HostAccess != nil {
		spec.HostAccess = *req.HostAccess
	}
	if spec.Resources.MemoryMB <= 0 {
		spec.Resources.MemoryMB = 512
	}
	if spec.Resources.CPUs <= 0 {
		spec.Resources.CPUs = 1
	}
	if spec.Resources.ShmMB <= 0 {
		spec.Resources.ShmMB = 64
		if spec.Kind == "model" {
			spec.Resources.ShmMB = 1024
		}
	}
	if spec.Kind == "model" && spec.ModelName == "" {
		spec.ModelName = name
	}

	// Local model assets: mount the real weights and pick the inference runtime.
	if req.ModelAssetID != "" {
		asset, err := o.store.GetAsset(req.ModelAssetID)
		if err != nil {
			return nil, fmt.Errorf("模型资产不存在")
		}
		if asset.Status != "completed" && asset.Status != "registry" {
			return nil, fmt.Errorf("模型尚未准备完成（状态：%s），请等待下载完成", asset.Status)
		}
		runtime := strings.ToLower(strings.TrimSpace(req.ModelRuntime))
		if runtime == "" {
			runtime = asset.Runtime
		}
		if runtime == "" {
			runtime = "llamacpp"
		}
		spec.Kind = "model"
		spec.CatalogID = "asset:" + asset.ID
		spec.ModelAssetID = asset.ID
		spec.ModelRuntime = runtime
		alias := strings.TrimSpace(req.ModelName)
		if alias == "" {
			alias = name
		}
		spec.ModelName = alias
		spec.Description = "本地模型 · " + asset.Name
		if spec.Labels == nil {
			spec.Labels = map[string]string{}
		}
		spec.Labels["aios.asset_id"] = asset.ID
		spec.Labels["aios.model_dir"] = asset.LocalPath

		switch runtime {
		case "llamacpp":
			if asset.GGUF == "" {
				return nil, fmt.Errorf("模型目录中没有找到 .gguf 文件；请选择 GGUF 模型，或改用 vLLM 运行时")
			}
			gpu := spec.Resources.GPUCount
			if req.Resources != nil && req.Resources.GPUCount > 0 {
				gpu = req.Resources.GPUCount
			}
			image := "ghcr.io/ggml-org/llama.cpp:server"
			if gpu > 0 {
				image = "ghcr.io/ggml-org/llama.cpp:server-cuda"
			}
			spec.Image = image
			spec.Command = []string{
				"-m", asset.GGUF,
				"--host", "0.0.0.0", "--port", "8080",
				"-c", "4096", "-ngl", "999", "--jinja", "--alias", alias,
			}
			spec.Ports = []model.PortMap{{HostPort: 0, ContPort: 8080, Protocol: "tcp", Accessible: true}}
			if spec.Resources.CPUs < 2 {
				spec.Resources.CPUs = 4
			}
			if spec.Resources.MemoryMB < 4096 {
				spec.Resources.MemoryMB = 8192
			}
		case "vllm":
			spec.Image = "vllm/vllm-openai:latest"
			spec.Command = []string{
				"--model", asset.LocalPath,
				"--served-model-name", alias,
				"--host", "0.0.0.0", "--port", "8000",
			}
			spec.Ports = []model.PortMap{{HostPort: 0, ContPort: 8000, Protocol: "tcp", Accessible: true}}
			if spec.Resources.GPUCount == 0 {
				spec.Resources.GPUCount = 1
			}
			if spec.Resources.MemoryMB < 8192 {
				spec.Resources.MemoryMB = 16384
			}
			if spec.Resources.ShmMB < 2048 {
				spec.Resources.ShmMB = 2048
			}
		case "ollama":
			spec.Image = "ollama/ollama:latest"
			spec.Ports = []model.PortMap{{HostPort: 0, ContPort: 11434, Protocol: "tcp", Accessible: true}}
			spec.Labels["aios.runtime"] = "ollama"
			spec.Labels["aios.model_ref"] = asset.Repo
		default:
			return nil, fmt.Errorf("不支持的推理引擎: %s", runtime)
		}

		if asset.HostPath != "" {
			spec.Volumes = []model.VolumeMount{{Source: asset.HostPath, Target: asset.LocalPath, ReadOnly: true}}
		} else if asset.Volume != "" {
			spec.Volumes = []model.VolumeMount{{Source: asset.Volume, Target: "/models"}}
		}
	}

	// Allocate reachable host ports, avoiding ports already published by
	// Docker containers or claimed by existing AIDash services.
	used := map[int]bool{}
	if svcs, err := o.store.ListServices(); err == nil {
		for _, s := range svcs {
			for _, p := range s.Spec.Ports {
				if p.HostPort > 0 {
					used[p.HostPort] = true
				}
			}
		}
	}
	listCtx, cancelList := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancelList()
	if containers, err := o.docker.ListContainers(listCtx, true); err == nil {
		for _, c := range containers {
			for _, port := range c.Ports {
				if port.PublicPort > 0 {
					used[port.PublicPort] = true
				}
			}
		}
	}
	o.portMu.Lock()
	allocated, err := allocatePorts(spec.Ports, used)
	o.portMu.Unlock()
	if err != nil {
		return nil, err
	}
	spec.Ports = allocated
	return spec, nil
}

func mergeResources(base, override model.ResourceSpec) model.ResourceSpec {
	out := base
	if override.CPUs > 0 {
		out.CPUs = override.CPUs
	}
	if override.MemoryMB > 0 {
		out.MemoryMB = override.MemoryMB
	}
	if override.ShmMB > 0 {
		out.ShmMB = override.ShmMB
	}
	if override.GPUCount > 0 {
		out.GPUCount = override.GPUCount
	}
	if override.GPUKind != "" {
		out.GPUKind = override.GPUKind
	}
	return out
}

func allocatePorts(ports []model.PortMap, used map[int]bool) ([]model.PortMap, error) {
	out := make([]model.PortMap, len(ports))
	for i, p := range ports {
		want := p.HostPort
		if want > 0 && !used[want] && portAvailable(want) {
			out[i] = p
			used[want] = true
			continue
		}
		port, err := findFreePort(used)
		if err != nil {
			return nil, err
		}
		p.HostPort = port
		used[port] = true
		out[i] = p
	}
	return out, nil
}

func portAvailable(port int) bool {
	if port <= 0 || port > 65535 {
		return false
	}
	ln, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", port))
	if err != nil {
		return false
	}
	_ = ln.Close()
	return true
}

func findFreePort(used map[int]bool) (int, error) {
	for port := 20000; port < 30000; port++ {
		if used[port] {
			continue
		}
		if portAvailable(port) {
			return port, nil
		}
	}
	return 0, fmt.Errorf("没有可用的宿主机端口（20000-29999 已耗尽）")
}

// BuildCreateRequest translates our deployment spec into a Docker API
// create payload, including the optional host-access (运维模式) grants.
func (o *Orchestrator) BuildCreateRequest(svc *model.Service) engine.CreateRequest {
	spec := svc.Spec
	exposed := map[string]struct{}{}
	bindings := map[string][]engine.PortBinding{}
	for _, p := range spec.Ports {
		proto := p.Protocol
		if proto == "" {
			proto = "tcp"
		}
		key := fmt.Sprintf("%d/%s", p.ContPort, proto)
		exposed[key] = struct{}{}
		bindIP := p.BindHost
		if bindIP == "" {
			bindIP = "0.0.0.0"
		}
		bindings[key] = append(bindings[key], engine.PortBinding{HostIP: bindIP, HostPort: fmt.Sprintf("%d", p.HostPort)})
	}

	env := []string{}
	seen := map[string]bool{}
	addEnv := func(k, v string) {
		if seen[k] {
			return
		}
		seen[k] = true
		env = append(env, k+"="+v)
	}
	for k, v := range spec.Env {
		addEnv(k, v)
	}
	if spec.Resources.GPUCount > 0 {
		addEnv("NVIDIA_VISIBLE_DEVICES", "all")
		addEnv("NVIDIA_DRIVER_CAPABILITIES", "compute,utility")
	}

	binds := []string{}
	for _, v := range spec.Volumes {
		if v.Source == "" || v.Target == "" {
			continue
		}
		entry := v.Source + ":" + v.Target
		if v.ReadOnly {
			entry += ":ro"
		}
		binds = append(binds, entry)
	}

	hostConfig := engine.HostConfig{
		PortBindings:  bindings,
		RestartPolicy: engine.RestartPolicy{Name: "unless-stopped"},
		NanoCPUs:      int64(spec.Resources.CPUs * 1e9),
		Memory:        spec.Resources.MemoryMB * 1024 * 1024,
		ShmSize:       spec.Resources.ShmMB * 1024 * 1024,
		LogConfig:     &engine.LogConfig{Type: "json-file", Config: map[string]string{"max-size": "10m", "max-file": "3"}},
		NetworkMode:   "aidash-net",
		ExtraHosts:    []string{"host.docker.internal:host-gateway"},
	}
	if spec.Resources.GPUCount > 0 {
		hostConfig.DeviceRequests = []engine.DeviceRequest{{
			Count:        spec.Resources.GPUCount,
			Capabilities: [][]string{{"gpu"}},
		}}
	}

	if spec.HostAccess.Enabled {
		hostConfig.Privileged = true
		if spec.HostAccess.HostPID {
			hostConfig.PidMode = "host"
		}
		hostConfig.CapAdd = append(hostConfig.CapAdd, "SYS_ADMIN", "SYS_PTRACE", "NET_ADMIN", "SYS_CHROOT")
		hostConfig.CapAdd = append(hostConfig.CapAdd, spec.HostAccess.CapAdd...)
		hostConfig.SecurityOpt = []string{"seccomp=unconfined", "apparmor=unconfined"}
		addEnv("HOST_ROOT", "/host")
		addEnv("AIDASH_HOST_ACCESS", "true")
		if targetOS() == "windows" {
			if spec.HostAccess.MountDockerSocket {
				binds = append(binds, `\\.\pipe\docker_engine:\\.\pipe\docker_engine`)
			}
			if spec.HostAccess.MountHostFS {
				if drive := os.Getenv("SystemDrive"); drive != "" {
					binds = append(binds, drive+`\:/host`)
				}
			}
			addEnv("DOCKER_HOST", "npipe:////./pipe/docker_engine")
		} else {
			if spec.HostAccess.MountHostFS {
				binds = append(binds, "/:/host:rw")
				addEnv("DOCKER_HOST", "unix:///var/run/docker.sock")
			}
			if spec.HostAccess.MountDockerSocket {
				binds = append(binds, "/var/run/docker.sock:/var/run/docker.sock")
			}
			binds = append(binds,
				"/proc:/host/proc:ro",
				"/sys:/host/sys:ro",
				"/dev:/host/dev:ro",
			)
		}
		for _, m := range spec.HostAccess.ExtraMounts {
			if m.Source == "" || m.Target == "" {
				continue
			}
			entry := m.Source + ":" + m.Target
			if m.ReadOnly {
				entry += ":ro"
			}
			binds = append(binds, entry)
		}
	}
	hostConfig.Binds = binds

	labels := map[string]string{
		"aios.managed":      "true",
		"aios.service_id":   svc.ID,
		"aios.service_name": svc.Name,
		"aios.kind":         svc.Kind,
	}
	if spec.ModelName != "" {
		labels["aios.model"] = spec.ModelName
	}
	if spec.HostAccess.Enabled {
		labels["aios.host_access"] = "true"
	}
	for k, v := range spec.Labels {
		labels[k] = v
	}

	return engine.CreateRequest{
		Config: engine.Config{
			Hostname:     "aidash-" + svc.Name,
			Image:        spec.Image,
			Cmd:          spec.Command,
			Entrypoint:   spec.Entrypoint,
			Env:          env,
			Labels:       labels,
			ExposedPorts: exposed,
			WorkingDir:   "",
			Tty:          false,
			OpenStdin:    false,
		},
		HostConfig: hostConfig,
	}
}

func targetOS() string {
	if v := strings.TrimSpace(os.Getenv("AIDASH_TARGET_OS")); v != "" {
		return strings.ToLower(v)
	}
	return runtime.GOOS
}

func buildEndpoints(spec *model.ServiceSpec, hostIP string) []string {
	out := []string{}
	for _, p := range spec.Ports {
		if !p.Accessible {
			continue
		}
		scheme := "http"
		if p.ContPort == 443 || p.ContPort == 8443 {
			scheme = "https"
		}
		out = append(out, fmt.Sprintf("%s://%s:%d", scheme, hostIP, p.HostPort))
	}
	return out
}

func sanitizeVolumeName(v string) string {
	if strings.ContainsAny(v, `/\.`) {
		clean := filepath.Base(strings.ReplaceAll(v, `\`, "/"))
		return "aidash-" + strings.ToLower(regexp.MustCompile(`[^a-zA-Z0-9_.-]`).ReplaceAllString(clean, "-"))
	}
	return v
}
