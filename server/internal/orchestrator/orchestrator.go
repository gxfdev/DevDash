package orchestrator

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gxfdev/aidash/server/internal/catalog"
	"github.com/gxfdev/aidash/server/internal/engine"
	"github.com/gxfdev/aidash/server/internal/model"
	"github.com/gxfdev/aidash/server/internal/store"
)

const (
	StatusPending    = "pending"
	StatusPulling    = "pulling"
	StatusInstalling = "installing"
	StatusStarting   = "starting"
	StatusRunning    = "running"
	StatusStopped    = "stopped"
	StatusError      = "error"
	StatusRemoving   = "removing"
)

type Orchestrator struct {
	store   *store.Store
	docker  *engine.Client
	catalog *catalog.Catalog
	hostIP  string

	mu     sync.Mutex
	busy   map[string]bool // serviceID -> operation in flight
	portMu sync.Mutex

	endpointMu sync.Mutex
	endpoints  map[string]cachedEndpoint
}

type cachedEndpoint struct {
	base    string
	expires time.Time
}

func New(st *store.Store, dc *engine.Client, cat *catalog.Catalog) *Orchestrator {
	return &Orchestrator{
		store:     st,
		docker:    dc,
		catalog:   cat,
		busy:      map[string]bool{},
		endpoints: map[string]cachedEndpoint{},
	}
}

func (o *Orchestrator) Docker() *engine.Client { return o.docker }
func (o *Orchestrator) Store() *store.Store    { return o.store }

func newID() string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func (o *Orchestrator) event(serviceID, level, msg string) {
	o.store.AddServiceEvent(serviceID, level, msg)
}

// lock marks a service as busy. Returns false when another operation runs.
func (o *Orchestrator) lock(id string) bool {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.busy[id] {
		return false
	}
	o.busy[id] = true
	return true
}

func (o *Orchestrator) unlock(id string) {
	o.mu.Lock()
	delete(o.busy, id)
	o.mu.Unlock()
}

func (o *Orchestrator) IsBusy(id string) bool {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.busy[id]
}

// Deploy validates the request, persists the service and starts the
// asynchronous provisioning pipeline.
func (o *Orchestrator) Deploy(ctx context.Context, req DeployRequest) (*model.Service, error) {
	spec, err := o.BuildSpec(req, o.catalog)
	if err != nil {
		return nil, err
	}
	if existing, err := o.store.GetServiceByName(spec.Name); err == nil && existing != nil {
		return nil, fmt.Errorf("服务名称 %q 已存在", spec.Name)
	}
	svc := &model.Service{
		ID:          newID(),
		Name:        spec.Name,
		Kind:        spec.Kind,
		CatalogID:   spec.CatalogID,
		Image:       spec.Image,
		ModelName:   spec.ModelName,
		Description: spec.Description,
		Spec:        *spec,
		Status:      StatusPending,
		Health:      "unknown",
		Endpoints:   []string{},
		Owner:       req.Owner,
		HostAccess:  spec.HostAccess.Enabled,
	}
	if svc.Owner == "" {
		svc.Owner = "system"
	}
	if err := o.store.CreateService(svc); err != nil {
		return nil, fmt.Errorf("保存服务失败: %w", err)
	}
	o.event(svc.ID, "info", fmt.Sprintf("创建部署任务：%s（%s）", svc.Name, svc.Image))
	if spec.HostAccess.Enabled {
		o.event(svc.ID, "warn", "已授予宿主机权限（特权容器 / host PID / 宿主机根目录 / Docker Socket）")
	}
	if !o.lock(svc.ID) {
		return nil, errors.New("该服务正在执行其他操作")
	}
	go func() {
		defer o.unlock(svc.ID)
		o.provision(svc.ID)
	}()
	return svc, nil
}

func (o *Orchestrator) provision(id string) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Minute)
	defer cancel()

	svc, err := o.store.GetService(id)
	if err != nil {
		return
	}
	if err := o.ensureNetwork(ctx); err != nil {
		o.fail(id, "创建容器网络失败: "+err.Error())
		return
	}

	imageRef, imageTag := splitImageRef(svc.Spec.Image)
	if exists, _ := o.docker.ImageExists(ctx, imageRef, imageTag); exists {
		o.event(id, "success", "使用本地镜像："+svc.Spec.Image)
	} else {
		o.setStatus(id, StatusPulling, "unknown", "", nil, "")
		o.event(id, "info", "开始拉取镜像 "+svc.Spec.Image)
		if err := o.pullImage(ctx, id, svc.Spec.Image); err != nil {
			o.fail(id, "镜像拉取失败: "+err.Error()+"（国内网络可先手动 docker pull 或配置镜像加速）")
			return
		}
		o.event(id, "success", "镜像就绪："+svc.Spec.Image)
	}

	req := o.BuildCreateRequest(svc)
	o.event(id, "info", "正在创建容器…")
	created, err := o.docker.CreateContainer(ctx, "aidash-"+svc.Name, req)
	if err != nil {
		o.fail(id, "创建容器失败: "+err.Error())
		return
	}
	o.setStatus(id, StatusStarting, "unknown", created.ID, nil, "")
	if err := o.docker.StartContainer(ctx, created.ID); err != nil {
		o.fail(id, "启动容器失败: "+err.Error())
		return
	}
	o.event(id, "success", "容器已启动 "+short(created.ID))

	endpoints := buildEndpoints(&svc.Spec, "127.0.0.1")
	o.setStatus(id, StatusStarting, "unknown", created.ID, endpoints, "")

	// Ollama runtimes need the model pulled into the container.
	runtimeKind := ""
	if svc.Spec.Labels != nil {
		runtimeKind = svc.Spec.Labels["aios.runtime"]
	}
	if runtimeKind == "ollama" {
		ref := ""
		if svc.Spec.Labels != nil {
			ref = svc.Spec.Labels["aios.model_ref"]
		}
		if ref == "" {
			ref = o.catalogModelRef(svc.CatalogID)
		}
		if ref == "" {
			ref = svc.ModelName
		}
		o.setStatus(id, StatusInstalling, "unknown", created.ID, endpoints, "")
		o.event(id, "info", "等待 Ollama 运行时就绪…")
		if err := o.waitOllamaUp(ctx, created.ID); err != nil {
			o.event(id, "error", "Ollama 未就绪: "+err.Error())
			o.setStatus(id, StatusRunning, "unhealthy", created.ID, endpoints, "Ollama 运行时未就绪")
			return
		}
		o.event(id, "info", "拉取模型 "+ref+"（首次下载可能较慢）")
		out, err := o.execCapture(ctx, created.ID, []string{"ollama", "pull", ref}, 45*time.Minute, func(line string) {
			trimmed := strings.TrimSpace(line)
			if trimmed != "" && !strings.HasPrefix(trimmed, "\x1b") {
				o.event(id, "info", "模型下载: "+truncate(trimmed, 300))
			}
		})
		if err != nil {
			o.event(id, "error", "模型下载失败: "+err.Error()+" "+truncate(out, 500))
			o.setStatus(id, StatusRunning, "unhealthy", created.ID, endpoints, "模型下载失败："+truncate(err.Error(), 200))
			return
		}
		o.event(id, "success", "模型 "+ref+" 就绪")
	}

	// Wait until the service answers, but do not block the deployment
	// forever for heavyweight images.
	base := o.internalBase(ctx, created.ID, &svc.Spec)
	if base != "" {
		o.event(id, "info", "等待服务健康检查："+base)
		_ = waitHTTP(ctx, strings.TrimSuffix(base, "/")+"/", healthTimeout())
	}
	o.setStatus(id, StatusRunning, "healthy", created.ID, endpoints, "")
	o.event(id, "success", "服务已就绪")
	o.invalidateEndpoint(id)
}

func (o *Orchestrator) catalogModelRef(catalogID string) string {
	if item, ok := o.catalog.Get(catalogID); ok {
		return item.ModelRef
	}
	return ""
}

func (o *Orchestrator) pullImage(ctx context.Context, serviceID, ref string) error {
	lastEmit := time.Time{}
	lastStatus := ""
	pullRef := ref
	if mirror := strings.TrimSpace(os.Getenv("AIDASH_IMAGE_MIRROR")); mirror != "" && strings.HasPrefix(ref, "ghcr.io/") {
		pullRef = strings.TrimSuffix(mirror, "/") + strings.TrimPrefix(ref, "ghcr.io")
		o.event(serviceID, "info", "使用镜像加速: "+pullRef)
	}
	err := o.docker.PullImage(ctx, pullRef, "", func(p engine.PullProgress) {
		msg := strings.TrimSpace(p.Status + " " + p.ID + " " + p.Progress)
		if msg == "" || msg == lastStatus {
			return
		}
		if time.Since(lastEmit) < 1200*time.Millisecond && !strings.Contains(p.Status, "Downloaded") {
			return
		}
		lastEmit = time.Now()
		lastStatus = msg
		o.event(serviceID, "info", "拉取镜像: "+truncate(msg, 200))
	})
	if err != nil {
		return err
	}
	if pullRef != ref {
		refName, refTag := splitImageRef(ref)
		if tagErr := o.docker.TagImage(ctx, pullRef, refName, refTag); tagErr != nil {
			o.event(serviceID, "warn", "镜像重命名失败: "+tagErr.Error())
		}
	}
	return nil
}

func (o *Orchestrator) ensureNetwork(ctx context.Context) error {
	nets, err := o.docker.ListNetworks(ctx)
	if err != nil {
		return err
	}
	for _, n := range nets {
		if n.Name == "aidash-net" {
			return nil
		}
	}
	_, err = o.docker.CreateNetwork(ctx, "aidash-net", "bridge")
	if err != nil && !strings.Contains(strings.ToLower(err.Error()), "exists") {
		return err
	}
	return nil
}

func (o *Orchestrator) waitOllamaUp(ctx context.Context, containerID string) error {
	deadline := time.Now().Add(3 * time.Minute)
	for time.Now().Before(deadline) {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if _, err := o.execCapture(ctx, containerID, []string{"ollama", "list"}, 30*time.Second, nil); err == nil {
			return nil
		}
		time.Sleep(3 * time.Second)
	}
	return errors.New("等待超时（3 分钟）")
}

func (o *Orchestrator) fail(id, msg string) {
	svc, err := o.store.GetService(id)
	if err != nil {
		return
	}
	o.event(id, "error", msg)
	o.setStatus(id, StatusError, "unhealthy", svc.ContainerID, svc.Endpoints, truncate(msg, 500))
}

func (o *Orchestrator) setStatus(id, status, health, containerID string, endpoints []string, errMsg string) {
	svc, err := o.store.GetService(id)
	if err != nil {
		return
	}
	if containerID == "" {
		containerID = svc.ContainerID
	}
	if endpoints == nil {
		endpoints = svc.Endpoints
	}
	_ = o.store.PatchService(id, store.ServiceStatusPatch{
		Status:      status,
		Health:      health,
		ContainerID: containerID,
		Endpoints:   endpoints,
		Error:       errMsg,
		MarkStarted: status == StatusRunning && svc.StartedAt == nil,
	})
	o.invalidateEndpoint(id)
}

// ---------- lifecycle ----------

func (o *Orchestrator) Start(ctx context.Context, id string) error {
	svc, err := o.store.GetService(id)
	if err != nil {
		return err
	}
	if !o.lock(id) {
		return errors.New("该服务正在执行其他操作")
	}
	defer o.unlock(id)
	if svc.ContainerID == "" {
		return errors.New("容器不存在，请重新部署")
	}
	if err := o.docker.StartContainer(ctx, svc.ContainerID); err != nil {
		o.fail(id, "启动失败: "+err.Error())
		return err
	}
	o.event(id, "success", "已启动容器")
	o.setStatus(id, StatusRunning, "unknown", svc.ContainerID, svc.Endpoints, "")
	go func() {
		time.Sleep(2 * time.Second)
		o.refreshOne(context.Background(), id)
	}()
	return nil
}

func (o *Orchestrator) Stop(ctx context.Context, id string) error {
	svc, err := o.store.GetService(id)
	if err != nil {
		return err
	}
	if !o.lock(id) {
		return errors.New("该服务正在执行其他操作")
	}
	defer o.unlock(id)
	if svc.ContainerID != "" {
		if err := o.docker.StopContainer(ctx, svc.ContainerID, 15); err != nil {
			return err
		}
	}
	o.event(id, "info", "已停止服务")
	o.setStatus(id, StatusStopped, "unknown", svc.ContainerID, svc.Endpoints, "")
	o.invalidateEndpoint(id)
	return nil
}

func (o *Orchestrator) Restart(ctx context.Context, id string) error {
	svc, err := o.store.GetService(id)
	if err != nil {
		return err
	}
	if !o.lock(id) {
		return errors.New("该服务正在执行其他操作")
	}
	defer o.unlock(id)
	if svc.ContainerID == "" {
		return errors.New("容器不存在，请重新部署")
	}
	if err := o.docker.RestartContainer(ctx, svc.ContainerID, 15); err != nil {
		return err
	}
	o.event(id, "success", "容器已重启")
	o.setStatus(id, StatusRunning, "unknown", svc.ContainerID, svc.Endpoints, "")
	o.invalidateEndpoint(id)
	go func() {
		time.Sleep(3 * time.Second)
		o.refreshOne(context.Background(), id)
	}()
	return nil
}

func (o *Orchestrator) Remove(ctx context.Context, id string, keepVolumes bool) error {
	svc, err := o.store.GetService(id)
	if err != nil {
		return err
	}
	if !o.lock(id) {
		return errors.New("该服务正在执行其他操作")
	}
	defer o.unlock(id)
	o.setStatus(id, StatusRemoving, "unknown", svc.ContainerID, svc.Endpoints, "")
	if svc.ContainerID != "" {
		if err := o.docker.RemoveContainer(ctx, svc.ContainerID, true, !keepVolumes); err != nil {
			o.event(id, "warn", "移除容器失败: "+err.Error())
		}
	}
	if err := o.store.DeleteService(id); err != nil {
		return err
	}
	o.invalidateEndpoint(id)
	return nil
}

// Upgrade pulls the newest image tag and recreates the container while
// keeping the persisted spec.
func (o *Orchestrator) Upgrade(ctx context.Context, id string) error {
	svc, err := o.store.GetService(id)
	if err != nil {
		return err
	}
	if !o.lock(id) {
		return errors.New("该服务正在执行其他操作")
	}
	defer o.unlock(id)
	o.event(id, "info", "开始升级：拉取最新镜像")
	if err := o.pullImage(ctx, id, svc.Spec.Image); err != nil {
		o.fail(id, "升级失败（镜像拉取）: "+err.Error())
		return err
	}
	if svc.ContainerID != "" {
		_ = o.docker.RemoveContainer(ctx, svc.ContainerID, true, false)
	}
	created, err := o.docker.CreateContainer(ctx, "aidash-"+svc.Name, o.BuildCreateRequest(svc))
	if err != nil {
		o.fail(id, "重建容器失败: "+err.Error())
		return err
	}
	if err := o.docker.StartContainer(ctx, created.ID); err != nil {
		o.fail(id, "启动新容器失败: "+err.Error())
		return err
	}
	o.setStatus(id, StatusRunning, "unknown", created.ID, buildEndpoints(&svc.Spec, "127.0.0.1"), "")
	o.event(id, "success", "升级完成")
	o.invalidateEndpoint(id)
	return nil
}

// ---------- inspection helpers ----------

func (o *Orchestrator) internalBase(ctx context.Context, containerID string, spec *model.ServiceSpec) string {
	ci, err := o.docker.InspectContainer(ctx, containerID)
	if err != nil {
		return ""
	}
	port := 0
	for _, p := range spec.Ports {
		port = p.ContPort
		if p.Accessible {
			break
		}
	}
	if port == 0 {
		return ""
	}
	ip := containerIP(ci)
	if ip == "" {
		return ""
	}
	return fmt.Sprintf("http://%s:%d", ip, port)
}

func containerIP(ci *engine.ContainerInspect) string {
	if ci == nil {
		return ""
	}
	if ci.NetworkSettings.IPAddress != "" {
		return ci.NetworkSettings.IPAddress
	}
	for _, n := range ci.NetworkSettings.Networks {
		if n.IPAddress != "" {
			return n.IPAddress
		}
	}
	return ""
}

func (o *Orchestrator) invalidateEndpoint(id string) {
	o.endpointMu.Lock()
	delete(o.endpoints, id)
	o.endpointMu.Unlock()
}

// ModelBaseURL resolves the OpenAI-compatible base URL (…/v1) for a
// running model service. Results are cached briefly.
func (o *Orchestrator) ModelBaseURL(ctx context.Context, svc *model.Service) (string, error) {
	o.endpointMu.Lock()
	if c, ok := o.endpoints[svc.ID]; ok && time.Now().Before(c.expires) {
		o.endpointMu.Unlock()
		return c.base, nil
	}
	o.endpointMu.Unlock()

	ci, err := o.docker.InspectContainer(ctx, svc.ContainerID)
	if err != nil {
		return "", fmt.Errorf("容器不可用: %w", err)
	}
	if !ci.State.Running {
		return "", errors.New("模型服务未运行")
	}
	port := 0
	for _, p := range svc.Spec.Ports {
		if p.Accessible {
			port = p.ContPort
			break
		}
		if port == 0 {
			port = p.ContPort
		}
	}
	if port == 0 {
		return "", errors.New("服务没有声明端口")
	}
	base := fmt.Sprintf("http://%s:%d", containerIP(ci), port)
	if containerIP(ci) == "" {
		base = fmt.Sprintf("http://127.0.0.1:%d", port)
	}
	o.endpointMu.Lock()
	o.endpoints[svc.ID] = cachedEndpoint{base: base, expires: time.Now().Add(30 * time.Second)}
	o.endpointMu.Unlock()
	return base, nil
}

func splitImageRef(image string) (string, string) {
	lastSlash := strings.LastIndex(image, "/")
	lastColon := strings.LastIndex(image, ":")
	if lastColon > lastSlash {
		return image[:lastColon], image[lastColon+1:]
	}
	return image, ""
}

func short(id string) string {
	if len(id) > 12 {
		return id[:12]
	}
	return id
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func healthTimeout() time.Duration {
	if v := os.Getenv("AIDASH_HEALTH_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			return d
		}
	}
	return 3 * time.Minute
}

func waitHTTP(ctx context.Context, url string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: 3 * time.Second}
	for time.Now().Before(deadline) {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err == nil {
			resp, err := client.Do(req)
			if err == nil {
				resp.Body.Close()
				return nil
			}
		}
		time.Sleep(2 * time.Second)
	}
	return errors.New("健康检查超时")
}

// dialCheck verifies that a published host port accepts connections.
func dialCheck(port int, timeout time.Duration) bool {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), timeout)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}
