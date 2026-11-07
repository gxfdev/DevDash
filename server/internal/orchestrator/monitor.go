package orchestrator

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/gxfdev/aidash/server/internal/engine"
	"github.com/gxfdev/aidash/server/internal/model"
)

// Monitor reconciles managed services with the Docker engine twice per
// interval and keeps health/status honest even when containers are
// changed outside the panel.
func (o *Orchestrator) Monitor(ctx context.Context, every time.Duration) {
	if every <= 0 {
		every = 10 * time.Second
	}
	ticker := time.NewTicker(every)
	defer ticker.Stop()
	o.RefreshAll(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			o.RefreshAll(ctx)
		}
	}
}

func (o *Orchestrator) RefreshAll(ctx context.Context) {
	services, err := o.store.ListServices()
	if err != nil {
		return
	}
	for _, svc := range services {
		if o.IsBusy(svc.ID) {
			continue
		}
		o.refreshService(ctx, &svc)
	}
}

func (o *Orchestrator) refreshOne(ctx context.Context, id string) {
	svc, err := o.store.GetService(id)
	if err != nil || o.IsBusy(id) {
		return
	}
	o.refreshService(ctx, svc)
}

func (o *Orchestrator) refreshService(ctx context.Context, svc *model.Service) {
	if svc.ContainerID == "" {
		if svc.Status == StatusPending || svc.Status == StatusPulling {
			return
		}
		return
	}
	ci, err := o.docker.InspectContainer(ctx, svc.ContainerID)
	if err != nil {
		msg := err.Error()
		if strings.Contains(strings.ToLower(msg), "no such container") {
			if svc.Status != StatusError {
				_ = o.store.PatchService(svc.ID, serviceStatus(StatusError, "unhealthy", svc.ContainerID, svc.Endpoints, "容器已被移除（可能在面板之外删除）"))
				o.invalidateEndpoint(svc.ID)
			}
			return
		}
		return
	}
	status := svc.Status
	health := svc.Health
	errMsg := svc.Error
	switch {
	case ci.State.Running:
		if status != StatusRunning {
			status = StatusRunning
		}
		health = "healthy"
		if ci.State.Health != nil && ci.State.Health.Status != "healthy" {
			health = ci.State.Health.Status
		}
		if ci.State.OOMKilled {
			health = "unhealthy"
			errMsg = "容器曾因内存不足被 OOM 终止"
		}
	case ci.State.Restarting:
		status = StatusStarting
	case ci.State.Paused:
		status = StatusStopped
		health = "unknown"
	case ci.State.ExitCode != 0 && !ci.State.Running:
		status = StatusError
		health = "unhealthy"
		errMsg = strings.TrimSpace(ci.State.Error)
		if errMsg == "" {
			errMsg = "容器退出码非 0"
		}
	default:
		if status == StatusRunning || status == StatusStarting {
			status = StatusStopped
		}
		health = "unknown"
	}

	if status == StatusRunning && svc.Kind == "model" {
		health = o.probeModel(ctx, svc)
	}
	if status == svc.Status && health == svc.Health && errMsg == svc.Error {
		return
	}
	_ = o.store.PatchService(svc.ID, serviceStatus(status, health, svc.ContainerID, svc.Endpoints, errMsg))
}

// serviceStatus builds the patch without importing the store type here.
func serviceStatus(status, health, containerID string, endpoints []string, msg string) servicePatch {
	return servicePatch{Status: status, Health: health, ContainerID: containerID, Endpoints: endpoints, Error: msg}
}

type servicePatch = struct {
	Status, Health, ContainerID string
	Endpoints                   []string
	Error                       string
	MarkStarted                 bool
}

func (o *Orchestrator) probeModel(ctx context.Context, svc *model.Service) string {
	base, err := o.ModelBaseURL(ctx, svc)
	if err != nil {
		return "unhealthy"
	}
	probeCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(probeCtx, http.MethodGet, base+"/v1/models", nil)
	if err != nil {
		return "unknown"
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "unhealthy"
	}
	defer resp.Body.Close()
	if resp.StatusCode < 500 {
		return "healthy"
	}
	return "unhealthy"
}

// ContainerCounts summarizes engine containers for the dashboard.
func (o *Orchestrator) ContainerCounts(ctx context.Context) model.ContainerSummary {
	out := model.ContainerSummary{}
	containers, err := o.docker.ListContainers(ctx, true)
	if err != nil {
		return out
	}
	out.Total = len(containers)
	for _, c := range containers {
		if c.State == "running" {
			out.Running++
		} else {
			out.Stopped++
		}
		if c.Managed {
			out.AIServices++
		}
	}
	return out
}

// EngineSnapshot returns a small, stable view of the Docker engine used by
// the UI header and the "系统状态" panel.
type EngineSnapshot struct {
	Connected  bool    `json:"connected"`
	Host       string  `json:"host"`
	Version    string  `json:"version"`
	APIVersion string  `json:"api_version"`
	OS         string  `json:"os"`
	Arch       string  `json:"arch"`
	CPUs       int     `json:"cpus"`
	MemoryGB   float64 `json:"memory_gb"`
	Images     int     `json:"images"`
	Containers int     `json:"containers"`
	Running    int     `json:"running"`
	Driver     string  `json:"driver"`
	Error      string  `json:"error,omitempty"`
}

func (o *Orchestrator) EngineSnapshot(ctx context.Context) EngineSnapshot {
	snap := EngineSnapshot{Host: o.docker.Host()}
	version, err := o.docker.Version(ctx)
	if err != nil {
		snap.Error = err.Error()
		return snap
	}
	info, err := o.docker.Info(ctx)
	if err != nil {
		snap.Error = err.Error()
		return snap
	}
	snap.Connected = true
	snap.Version = version.Version
	snap.APIVersion = version.APIVersion
	snap.OS = version.Os
	snap.Arch = version.Arch
	snap.CPUs = info.NCPU
	snap.MemoryGB = round1(float64(info.MemTotal) / 1e9)
	snap.Images = info.Images
	snap.Containers = info.Containers
	snap.Running = info.ContainersRunning
	snap.Driver = info.Driver
	return snap
}

func (o *Orchestrator) EngineInfo(ctx context.Context) (*engine.EngineInfo, error) {
	return o.docker.Info(ctx)
}

func round1(v float64) float64 {
	iv := int64(v*10 + 0.5)
	return float64(iv) / 10
}
