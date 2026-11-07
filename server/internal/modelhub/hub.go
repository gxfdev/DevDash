// Package modelhub downloads real model weights (ModelScope / HuggingFace /
// direct URLs / existing host directories) and prepares them for serving
// containers. Every download runs in a real Docker container, so the files
// are always visible to the inference runtime through the same volume.
package modelhub

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gxfdev/aidash/server/internal/engine"
	"github.com/gxfdev/aidash/server/internal/model"
	"github.com/gxfdev/aidash/server/internal/store"
)

const (
	VolumeName   = "aidash-models"
	VolumeTarget = "/models"

	StatusQueued      = "queued"
	StatusDownloading = "downloading"
	StatusCompleted   = "completed"
	StatusFailed      = "failed"
	StatusCanceled    = "canceled"
	StatusRegistry    = "registry" // model served directly from a registry (ollama)
)

type Hub struct {
	store  *store.Store
	docker *engine.Client

	mu      sync.Mutex
	workers map[string]context.CancelFunc
}

func New(st *store.Store, dc *engine.Client) *Hub {
	return &Hub{store: st, docker: dc, workers: map[string]context.CancelFunc{}}
}

func (h *Hub) Docker() *engine.Client { return h.docker }
func (h *Hub) Store() *store.Store    { return h.store }

type SourceOption struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Description   string `json:"description"`
	Downloads     bool   `json:"downloads"`
	RequiresRepo  bool   `json:"requires_repo"`
	RequiresURL   bool   `json:"requires_url"`
	RequiresPath  bool   `json:"requires_path"`
	DefaultMirror string `json:"default_mirror"`
	DocsURL       string `json:"docs_url"`
	Badge         string `json:"badge"`
}

func Sources() []SourceOption {
	return []SourceOption{
		{ID: "modelscope", Name: "魔搭 ModelScope", Description: "国内直连，支持 Qwen / DeepSeek / GLM 等社区模型，推荐国内服务器使用。", Downloads: true, RequiresRepo: true, DocsURL: "https://modelscope.cn/models", Badge: "国内推荐"},
		{ID: "huggingface", Name: "HuggingFace", Description: "国际模型社区。国内网络可设置 hf-mirror.com 镜像加速。", Downloads: true, RequiresRepo: true, DefaultMirror: "https://hf-mirror.com", DocsURL: "https://huggingface.co/models", Badge: "镜像可选"},
		{ID: "ollama", Name: "Ollama 仓库", Description: "直接从 Ollama 模型仓库拉取（如 qwen2.5:0.5b），无需管理模型文件。", Downloads: false, RequiresRepo: true, DocsURL: "https://ollama.com/library", Badge: "最省事"},
		{ID: "local", Name: "服务器本地目录", Description: "模型已存在于宿主机目录（离线包、共享存储、已下载的 GGUF）。", Downloads: false, RequiresPath: true, Badge: "离线"},
		{ID: "url", Name: "直链 / 离线包", Description: "通过 HTTP(S) 直链下载 GGUF 或 tar.gz 离线模型包，支持内网仓库。", Downloads: true, RequiresURL: true, Badge: "内网"},
	}
}

type CreateRequest struct {
	Name     string `json:"name"`
	Source   string `json:"source"`
	Repo     string `json:"repo"`
	Revision string `json:"revision"`
	File     string `json:"file"`
	Mirror   string `json:"mirror"`
	Token    string `json:"token"`
	URL      string `json:"url"`
	HostPath string `json:"host_path"`
	Runtime  string `json:"runtime"`
}

var slugRe = regexp.MustCompile(`[^a-z0-9-]+`)
var safeRe = regexp.MustCompile(`^[A-Za-z0-9_./:@-]+$`)
var patternRe = regexp.MustCompile(`^[A-Za-z0-9_./*?@-]+$`)

func newID() string {
	b := make([]byte, 5)
	_, _ = rand.Read(b)
	return "m" + hex.EncodeToString(b)
}

func slugify(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	s = strings.ReplaceAll(s, "_", "-")
	s = strings.ReplaceAll(s, "/", "-")
	s = strings.ReplaceAll(s, ":", "-")
	s = slugRe.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	if s == "" {
		s = "model"
	}
	if len(s) > 48 {
		s = s[:48]
	}
	return s
}

// Create validates a request, persists the asset and starts the download job.
func (h *Hub) Create(ctx context.Context, req CreateRequest) (*model.ModelAsset, error) {
	req.Name = strings.TrimSpace(req.Name)
	req.Source = strings.TrimSpace(req.Source)
	if req.Source == "" {
		req.Source = "modelscope"
	}
	if req.Name == "" {
		req.Name = strings.TrimSpace(req.Repo)
		if req.Name == "" {
			req.Name = path.Base(strings.TrimSuffix(req.URL, "/"))
		}
	}
	if req.Name == "" {
		return nil, fmt.Errorf("请填写模型名称")
	}
	if err := validateRequest(req); err != nil {
		return nil, err
	}

	id := newID()
	asset := &model.ModelAsset{
		ID: id, Name: req.Name, Source: req.Source, Repo: strings.TrimSpace(req.Repo),
		Revision: firstNonEmpty(req.Revision, defaultRevision(req.Source)),
		File:     strings.TrimSpace(req.File), Mirror: strings.TrimSpace(req.Mirror),
		Volume: VolumeName, Status: StatusQueued, Runtime: normalizeRuntime(req.Runtime),
		LocalPath: path.Join(VolumeTarget, id),
	}
	if req.Source == "local" {
		asset.HostPath = strings.TrimSpace(req.HostPath)
		asset.LocalPath = path.Join(VolumeTarget, id)
	}
	if req.Source == "ollama" {
		asset.Status = StatusRegistry
		asset.Runtime = "ollama"
		asset.LocalPath = ""
	}
	if req.Source == "modelscope" && asset.Mirror == "" {
		asset.Mirror = ""
	}
	if err := h.store.CreateAsset(asset); err != nil {
		return nil, err
	}
	if asset.Status == StatusRegistry {
		return asset, nil
	}

	runCtx, cancel := context.WithCancel(context.Background())
	h.mu.Lock()
	h.workers[id] = cancel
	h.mu.Unlock()
	go func() {
		defer func() {
			h.mu.Lock()
			delete(h.workers, id)
			h.mu.Unlock()
		}()
		h.run(runCtx, asset, req)
	}()
	return asset, nil
}

func (h *Hub) run(ctx context.Context, asset *model.ModelAsset, req CreateRequest) {
	if asset.Source == "local" {
		h.importLocal(ctx, asset)
		return
	}
	if err := h.pullDownloaderImage(ctx, asset, req); err != nil {
		h.fail(asset.ID, err.Error())
		return
	}
	image, cmd, env := downloadPlan(asset, req)
	if err := h.docker.CreateVolume(ctx, VolumeName); err != nil {
		h.fail(asset.ID, "创建模型数据卷失败: "+err.Error())
		return
	}
	create := engine.CreateRequest{
		Config: engine.Config{
			Image: image,
			Cmd:   []string{"sh", "-lc", cmd},
			Env:   env,
			Labels: map[string]string{
				"aios.asset_id":   asset.ID,
				"aios.asset_name": asset.Name,
				"aios.role":       "model-downloader",
			},
		},
		HostConfig: engine.HostConfig{
			Binds:         []string{VolumeName + ":" + VolumeTarget},
			RestartPolicy: engine.RestartPolicy{Name: "no"},
		},
	}
	create.HostConfig.NanoCPUs = int64(2e9)
	create.HostConfig.Memory = 2 << 30

	h.patch(asset.ID, StatusDownloading, "", "", "", 0, 0, "", "", "", "")
	created, err := h.docker.CreateContainer(ctx, "aidash-dl-"+asset.ID, create)
	if err != nil {
		h.fail(asset.ID, "创建下载容器失败: "+err.Error())
		return
	}
	asset.ContainerID = created.ID
	h.patch(asset.ID, StatusDownloading, "", "", created.ID, 0, 0, "", "", "", "")
	if err := h.docker.StartContainer(ctx, created.ID); err != nil {
		h.fail(asset.ID, "启动下载容器失败: "+err.Error())
		h.cleanupContainer(ctx, created.ID)
		return
	}

	h.watch(ctx, asset, created.ID)
}

// watch follows an existing downloader container until it exits, keeping
// size/file metadata and logs fresh, then finalizes the asset.
func (h *Hub) watch(ctx context.Context, asset *model.ModelAsset, containerID string) {
	exitCode := -1
	ticks := 0
	var lastSize int64
	var lastFiles int
	for {
		select {
		case <-ctx.Done():
			_ = h.docker.RemoveContainer(context.Background(), containerID, true, false)
			h.patch(asset.ID, StatusCanceled, "已取消", "", containerID, lastSize, lastFiles, asset.GGUF, asset.Runtime, asset.LocalPath, asset.HostPath)
			return
		case <-time.After(2 * time.Second):
		}
		ticks++
		logs, _ := h.docker.ContainerLogsText(context.Background(), containerID, 400)
		ci, err := h.docker.InspectContainer(ctx, containerID)
		if err != nil {
			h.fail(asset.ID, "下载容器异常: "+err.Error())
			h.cleanupContainer(ctx, containerID)
			return
		}
		if ticks%3 == 0 {
			if size, files := h.dirStats(ctx, containerID, asset.LocalPath); size > 0 || files > 0 {
				lastSize, lastFiles = size, files
			}
		}
		h.patch(asset.ID, StatusDownloading, "", tail(logs, 6000), containerID, lastSize, lastFiles, "", asset.Runtime, asset.LocalPath, asset.HostPath)
		if !ci.State.Running {
			exitCode = ci.State.ExitCode
			break
		}
	}

	logs, _ := h.docker.ContainerLogsText(context.Background(), containerID, 400)
	if exitCode == 0 {
		h.cleanupContainer(ctx, containerID)
		size, files, gguf := h.scanVolume(ctx, asset)
		if files == 0 && size == 0 {
			h.fail(asset.ID, "下载完成但目录为空，请检查仓库名 / 分支 / 文件路径\n\n"+tail(logs, 2500))
			return
		}
		runtime := asset.Runtime
		if gguf != "" && runtime == "" {
			runtime = "llamacpp"
		}
		if gguf == "" && runtime == "" {
			runtime = "vllm"
		}
		h.patch(asset.ID, StatusCompleted, "", tail(logs, 6000), containerID, size, files, gguf, runtime, asset.LocalPath, asset.HostPath)
	} else {
		h.patch(asset.ID, StatusFailed, fmt.Sprintf("下载失败（退出码 %d）\n\n%s", exitCode, tail(logs, 2500)), tail(logs, 6000), containerID, lastSize, lastFiles, "", asset.Runtime, asset.LocalPath, asset.HostPath)
	}
}

// Recover reconciles download jobs after a panel restart.
func (h *Hub) Recover() {
	assets, err := h.store.ListAssets()
	if err != nil {
		return
	}
	for _, a := range assets {
		if a.Status != StatusDownloading && a.Status != StatusQueued {
			continue
		}
		if a.ContainerID == "" {
			h.fail(a.ID, "面板重启，下载任务已中断，请重新下载")
			continue
		}
		ci, err := h.docker.InspectContainer(context.Background(), a.ContainerID)
		if err != nil {
			h.fail(a.ID, "下载容器已不存在，请重新下载")
			continue
		}
		asset := a
		if ci.State.Running {
			go h.watch(context.Background(), &asset, a.ContainerID)
			continue
		}
		if ci.State.ExitCode == 0 {
			size, files, gguf := h.scanVolume(context.Background(), &asset)
			runtime := asset.Runtime
			if runtime == "" {
				if gguf != "" {
					runtime = "llamacpp"
				} else {
					runtime = "vllm"
				}
			}
			h.patch(asset.ID, StatusCompleted, "", asset.Log, a.ContainerID, size, files, gguf, runtime, asset.LocalPath, asset.HostPath)
			h.cleanupContainer(context.Background(), a.ContainerID)
		} else {
			h.fail(asset.ID, "下载容器已退出，请重新下载")
		}
	}
}

func (h *Hub) pullDownloaderImage(ctx context.Context, asset *model.ModelAsset, req CreateRequest) error {
	image := downloaderImage(asset.Source)
	last := time.Time{}
	lastMsg := ""
	err := h.docker.PullImage(ctx, image, "", func(p engine.PullProgress) {
		msg := strings.TrimSpace(p.Status + " " + p.Progress)
		if msg == "" || msg == lastMsg || time.Since(last) < 1500*time.Millisecond {
			return
		}
		last = time.Now()
		lastMsg = msg
		h.patch(asset.ID, StatusQueued, "", "准备下载环境: "+msg, "", 0, 0, "", asset.Runtime, asset.LocalPath, asset.HostPath)
	})
	if err != nil {
		return fmt.Errorf("拉取下载镜像 %s 失败: %w", image, err)
	}
	return nil
}

func (h *Hub) importLocal(ctx context.Context, asset *model.ModelAsset) {
	if err := h.pullDownloaderImage(ctx, asset, CreateRequest{}); err != nil {
		h.fail(asset.ID, err.Error())
		return
	}
	image := downloaderImage("local")
	create := engine.CreateRequest{
		Config: engine.Config{
			Image:  image,
			Cmd:    []string{"sh", "-lc", "du -sb /src; find /src -type f | wc -l; find /src -name '*.gguf' | head -1"},
			Labels: map[string]string{"aios.asset_id": asset.ID, "aios.role": "model-scan"},
		},
		HostConfig: engine.HostConfig{
			Binds:         []string{asset.HostPath + ":/src:ro"},
			RestartPolicy: engine.RestartPolicy{Name: "no"},
		},
	}
	create.HostConfig.NanoCPUs = int64(1e9)
	create.HostConfig.Memory = 512 << 20
	created, err := h.docker.CreateContainer(ctx, "aidash-scan-"+asset.ID, create)
	if err != nil {
		h.fail(asset.ID, "宿主机目录不可用: "+err.Error())
		return
	}
	defer h.cleanupContainer(context.Background(), created.ID)
	if err := h.docker.StartContainer(ctx, created.ID); err != nil {
		h.fail(asset.ID, "扫描目录失败: "+err.Error())
		return
	}
	var state *engine.ContainerInspect
	for i := 0; i < 20; i++ {
		time.Sleep(500 * time.Millisecond)
		state, err = h.docker.InspectContainer(ctx, created.ID)
		if err == nil && !state.State.Running {
			break
		}
	}
	if state == nil || state.State.ExitCode != 0 {
		logs, _ := h.docker.ContainerLogsText(ctx, created.ID, 100)
		h.fail(asset.ID, "无法读取宿主机目录（请确认路径存在且 Docker 有权限）\n"+tail(logs, 800))
		return
	}
	logs, _ := h.docker.ContainerLogsText(ctx, created.ID, 100)
	size, files, ggufSrc := parseScanOutput(logs)
	gguf := ""
	if ggufSrc != "" {
		rel := strings.TrimPrefix(strings.TrimPrefix(ggufSrc, "/src/"), "/src")
		gguf = path.Join(asset.LocalPath, rel)
	}
	if files == 0 && size == 0 {
		h.fail(asset.ID, "目录为空："+asset.HostPath)
		return
	}
	runtime := asset.Runtime
	if runtime == "" {
		if gguf != "" {
			runtime = "llamacpp"
		} else {
			runtime = "vllm"
		}
	}
	h.patch(asset.ID, StatusCompleted, "", "已导入本地目录 "+asset.HostPath, "", size, files, gguf, runtime, asset.LocalPath, asset.HostPath)
}

func (h *Hub) Cancel(id string) error {
	h.mu.Lock()
	cancel, ok := h.workers[id]
	h.mu.Unlock()
	if ok && cancel != nil {
		cancel()
		return nil
	}
	asset, err := h.store.GetAsset(id)
	if err != nil {
		return err
	}
	if asset.Status == StatusQueued || asset.Status == StatusDownloading {
		if asset.ContainerID != "" {
			_ = h.docker.RemoveContainer(context.Background(), asset.ContainerID, true, false)
		}
		h.patch(id, StatusCanceled, "已取消", asset.Log, asset.ContainerID, asset.SizeBytes, asset.FileCount, asset.GGUF, asset.Runtime, asset.LocalPath, asset.HostPath)
	}
	return nil
}

func (h *Hub) Remove(ctx context.Context, id string, deleteFiles bool) error {
	asset, err := h.store.GetAsset(id)
	if err != nil {
		return err
	}
	_ = h.Cancel(id)
	if asset.ContainerID != "" {
		_ = h.docker.RemoveContainer(ctx, asset.ContainerID, true, false)
	}
	if deleteFiles && asset.Source != "local" {
		if err := h.removeFiles(ctx, asset); err != nil {
			return err
		}
	}
	return h.store.DeleteAsset(id)
}

func (h *Hub) removeFiles(ctx context.Context, asset *model.ModelAsset) error {
	image := downloaderImage(asset.Source)
	create := engine.CreateRequest{
		Config: engine.Config{
			Image:  image,
			Cmd:    []string{"sh", "-lc", "rm -rf " + asset.LocalPath},
			Labels: map[string]string{"aios.asset_id": asset.ID, "aios.role": "model-cleanup"},
		},
		HostConfig: engine.HostConfig{
			Binds:         []string{VolumeName + ":" + VolumeTarget},
			RestartPolicy: engine.RestartPolicy{Name: "no"},
		},
	}
	created, err := h.docker.CreateContainer(ctx, "aidash-rm-"+asset.ID, create)
	if err != nil {
		return fmt.Errorf("清理模型文件失败: %w", err)
	}
	defer h.cleanupContainer(context.Background(), created.ID)
	if err := h.docker.StartContainer(ctx, created.ID); err != nil {
		return err
	}
	for i := 0; i < 40; i++ {
		time.Sleep(300 * time.Millisecond)
		ci, err := h.docker.InspectContainer(ctx, created.ID)
		if err == nil && !ci.State.Running {
			return nil
		}
	}
	return nil
}

// Scan refreshes size/file metadata for a completed asset.
func (h *Hub) Scan(ctx context.Context, id string) (*model.ModelAsset, error) {
	asset, err := h.store.GetAsset(id)
	if err != nil {
		return nil, err
	}
	if asset.Source == "local" {
		return asset, nil
	}
	image := downloaderImage(asset.Source)
	cmd := "du -sb " + asset.LocalPath + "; find " + asset.LocalPath + " -type f | wc -l; find " + asset.LocalPath + " -name '*.gguf' | head -1"
	create := engine.CreateRequest{
		Config: engine.Config{
			Image:  image,
			Cmd:    []string{"sh", "-lc", cmd},
			Labels: map[string]string{"aios.asset_id": asset.ID, "aios.role": "model-scan"},
		},
		HostConfig: engine.HostConfig{
			Binds:         []string{VolumeName + ":" + VolumeTarget},
			RestartPolicy: engine.RestartPolicy{Name: "no"},
		},
	}
	created, err := h.docker.CreateContainer(ctx, "aidash-scan-"+asset.ID, create)
	if err != nil {
		return nil, err
	}
	defer h.cleanupContainer(context.Background(), created.ID)
	if err := h.docker.StartContainer(ctx, created.ID); err != nil {
		return nil, err
	}
	for i := 0; i < 40; i++ {
		time.Sleep(300 * time.Millisecond)
		ci, err := h.docker.InspectContainer(ctx, created.ID)
		if err == nil && !ci.State.Running {
			break
		}
	}
	logs, _ := h.docker.ContainerLogsText(ctx, created.ID, 100)
	lines := strings.Split(strings.TrimSpace(logs), "\n")
	var size int64
	var files int
	gguf := ""
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if n, err := strconv.ParseInt(line, 10, 64); err == nil {
			if size == 0 {
				size = n
			} else if files == 0 {
				files = int(n)
			}
			continue
		}
		if strings.HasSuffix(line, ".gguf") {
			gguf = line
		}
	}
	_ = h.store.PatchAsset(id, store.AssetPatch{Status: asset.Status, Error: asset.Error, Log: asset.Log, ContainerID: asset.ContainerID, SizeBytes: size, FileCount: files, GGUF: firstNonEmpty(gguf, asset.GGUF), Runtime: asset.Runtime, LocalPath: asset.LocalPath, HostPath: asset.HostPath})
	return h.store.GetAsset(id)
}

func (h *Hub) patch(id, status, errMsg, log, containerID string, size int64, files int, gguf, runtime, localPath, hostPath string) {
	_ = h.store.PatchAsset(id, store.AssetPatch{
		Status: status, Error: errMsg, Log: log, ContainerID: containerID,
		SizeBytes: size, FileCount: files, GGUF: gguf, Runtime: runtime,
		LocalPath: localPath, HostPath: hostPath,
	})
}

func (h *Hub) currentPatch(id string) store.AssetPatch {
	a, err := h.store.GetAsset(id)
	if err != nil {
		return store.AssetPatch{}
	}
	return store.AssetPatch{Status: a.Status, Error: a.Error, Log: a.Log, ContainerID: a.ContainerID, SizeBytes: a.SizeBytes, FileCount: a.FileCount, GGUF: a.GGUF, Runtime: a.Runtime, LocalPath: a.LocalPath, HostPath: a.HostPath}
}

func (h *Hub) fail(id, msg string) {
	a, _ := h.store.GetAsset(id)
	p := store.AssetPatch{Status: StatusFailed, Error: msg}
	if a != nil {
		p = store.AssetPatch{Status: StatusFailed, Error: msg, Log: a.Log, ContainerID: a.ContainerID, SizeBytes: a.SizeBytes, FileCount: a.FileCount, GGUF: a.GGUF, Runtime: a.Runtime, LocalPath: a.LocalPath, HostPath: a.HostPath}
	}
	_ = h.store.PatchAsset(id, p)
}

func (h *Hub) cleanupContainer(ctx context.Context, id string) {
	if id != "" {
		_ = h.docker.RemoveContainer(ctx, id, true, false)
	}
}

func (h *Hub) dirStats(ctx context.Context, containerID, dir string) (int64, int) {
	out, err := h.execCapture(ctx, containerID, []string{"sh", "-lc", "du -sb " + dir + " 2>/dev/null; find " + dir + " -type f 2>/dev/null | wc -l"}, 8*time.Second)
	if err != nil {
		return 0, 0
	}
	size, files, _ := parseScanOutput(out)
	return size, files
}

func (h *Hub) scanAsset(ctx context.Context, containerID, dir string) (int64, int, string) {
	out, err := h.execCapture(ctx, containerID, []string{"sh", "-lc", "du -sb " + dir + " 2>/dev/null; find " + dir + " -type f 2>/dev/null | wc -l; find " + dir + " -name '*.gguf' 2>/dev/null | head -1"}, 15*time.Second)
	if err != nil {
		return 0, 0, ""
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	var size int64
	var files int
	gguf := ""
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if n, err := strconv.ParseInt(line, 10, 64); err == nil {
			if size == 0 {
				size = n
			} else if files == 0 {
				files = int(n)
			}
			continue
		}
		if strings.HasSuffix(line, ".gguf") {
			gguf = line
		}
	}
	return size, files, gguf
}

// execCapture runs a short command in a container and returns stdout+stderr.
func (h *Hub) execCapture(ctx context.Context, containerID string, cmd []string, timeout time.Duration) (string, error) {
	if containerID == "" {
		return "", fmt.Errorf("容器不存在")
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	execID, err := h.docker.ExecCreate(runCtx, containerID, engine.ExecCreateRequest{
		AttachStdout: true, AttachStderr: true, Tty: true, Cmd: cmd,
	})
	if err != nil {
		return "", err
	}
	stream, err := h.docker.ExecStart(runCtx, execID)
	if err != nil {
		return "", err
	}
	defer stream.Close()
	done := make(chan struct{})
	go func() {
		select {
		case <-runCtx.Done():
			_ = stream.Close()
		case <-done:
		}
	}()
	defer close(done)
	raw, err := io.ReadAll(io.LimitReader(stream, 1<<20))
	if err != nil && len(raw) == 0 && runCtx.Err() != nil {
		return "", runCtx.Err()
	}
	return string(raw), nil
}

// ---------- plans ----------

func downloaderImage(source string) string {
	if img := os.Getenv("AIDASH_DOWNLOADER_IMAGE"); img != "" {
		return img
	}
	if source == "url" || source == "local" {
		return "alpine:3.20"
	}
	return "python:3.12-slim"
}

func pipIndex() string {
	if v := os.Getenv("PIP_INDEX_URL"); v != "" {
		return v
	}
	return "https://pypi.tuna.tsinghua.edu.cn/simple"
}

func downloadPlan(asset *model.ModelAsset, req CreateRequest) (image, cmd string, env []string) {
	env = append(env, "PIP_INDEX_URL="+pipIndex(), "PIP_DISABLE_PIP_VERSION_CHECK=1", "PYTHONUNBUFFERED=1", "HF_HUB_ENABLE_HF_TRANSFER=1")
	repo := asset.Repo
	if repo == "" || !safeRe.MatchString(repo) {
		repo = strings.ReplaceAll(repo, `"`, "")
	}
	rev := asset.Revision
	if rev == "" {
		rev = "master"
	}
	target := asset.LocalPath

	switch asset.Source {
	case "modelscope":
		script := fmt.Sprintf("set -e; pip install -q --no-cache-dir -U modelscope && modelscope download --model %s --revision %s --local_dir %s", repo, rev, target)
		if asset.File != "" && patternRe.MatchString(asset.File) {
			script += " --include " + asset.File
		}
		if req.Token != "" {
			env = append(env, "MODELSCOPE_API_TOKEN="+req.Token)
		}
		return downloaderImage("modelscope"), script, env

	case "huggingface":
		endpoint := firstNonEmpty(asset.Mirror, "https://hf-mirror.com")
		env = append(env, "HF_ENDPOINT="+endpoint)
		if req.Token != "" {
			env = append(env, "HF_TOKEN="+req.Token)
		}
		patterns := "None"
		if asset.File != "" && patternRe.MatchString(asset.File) {
			patterns = fmt.Sprintf("[%q]", asset.File)
		}
		script := fmt.Sprintf(
			"set -e; pip install -q --no-cache-dir -U 'huggingface_hub[hf_transfer]' && python -c \"from huggingface_hub import snapshot_download as s; s(repo_id='%s', revision='%s', local_dir='%s', allow_patterns=%s)\"",
			repo, rev, target, patterns)
		return downloaderImage("huggingface"), script, env

	case "url":
		u := strings.TrimSpace(req.URL)
		if u == "" || (!strings.HasPrefix(u, "http://") && !strings.HasPrefix(u, "https://")) {
			u = "http://invalid"
		}
		name := asset.File
		if name == "" || !safeRe.MatchString(name) {
			name = path.Base(strings.Split(u, "?")[0])
		}
		script := fmt.Sprintf("set -e; mkdir -p %s; cd %s; wget -q -O %s '%s'", target, target, name, u)
		if strings.HasSuffix(name, ".tar.gz") || strings.HasSuffix(name, ".tgz") || strings.HasSuffix(name, ".tar") {
			script += fmt.Sprintf("; tar -xf %s; rm -f %s", name, name)
		}
		return downloaderImage("url"), script, env
	}
	return downloaderImage("modelscope"), "echo unsupported source; exit 2", env
}

func validateRequest(req CreateRequest) error {
	src := req.Source
	switch src {
	case "modelscope", "huggingface":
		if strings.TrimSpace(req.Repo) == "" {
			return fmt.Errorf("请填写模型仓库，例如 Qwen/Qwen2.5-0.5B-Instruct-GGUF")
		}
		if strings.ContainsAny(req.Repo, " ;'\"$`\\") {
			return fmt.Errorf("模型仓库名包含非法字符")
		}
	case "ollama":
		if strings.TrimSpace(req.Repo) == "" {
			return fmt.Errorf("请填写 Ollama 模型，例如 qwen2.5:0.5b")
		}
	case "local":
		p := strings.TrimSpace(req.HostPath)
		if p == "" || !strings.HasPrefix(p, "/") {
			return fmt.Errorf("请填写宿主机绝对路径，例如 /opt/models/qwen2.5-7b")
		}
	case "url":
		u := strings.TrimSpace(req.URL)
		if !strings.HasPrefix(u, "http://") && !strings.HasPrefix(u, "https://") {
			return fmt.Errorf("请填写 http(s) 模型文件直链")
		}
	default:
		return fmt.Errorf("不支持的模型来源: %s", src)
	}
	return nil
}

func defaultRevision(source string) string {
	if source == "modelscope" {
		return "master"
	}
	if source == "huggingface" {
		return "main"
	}
	return ""
}

func normalizeRuntime(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "llamacpp", "llama.cpp", "llama_cpp":
		return "llamacpp"
	case "vllm":
		return "vllm"
	case "ollama":
		return "ollama"
	}
	return ""
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func tail(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return "…\n" + s[len(s)-n:]
}

// Logs returns recent downloader logs for the asset.
func (h *Hub) Logs(ctx context.Context, id string, tailLines int) (string, error) {
	asset, err := h.store.GetAsset(id)
	if err != nil {
		return "", err
	}
	if asset.ContainerID != "" {
		logs, err := h.docker.ContainerLogsText(ctx, asset.ContainerID, tailLines)
		if err == nil && strings.TrimSpace(logs) != "" {
			return logs, nil
		}
	}
	return asset.Log, nil
}
