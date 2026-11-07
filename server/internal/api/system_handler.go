package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gxfdev/aidash/server/internal/engine"
	"github.com/gxfdev/aidash/server/internal/model"
	"github.com/gxfdev/aidash/server/internal/orchestrator"
)

type serviceSummary struct {
	Total         int `json:"total"`
	Running       int `json:"running"`
	Models        int `json:"models"`
	ModelsRunning int `json:"models_running"`
	Apps          int `json:"apps"`
	HostAccess    int `json:"host_access"`
	Unhealthy     int `json:"unhealthy"`
}

func (h *Handler) overview(c *gin.Context) {
	ctx := c.Request.Context()
	snap := h.Snapshot()
	if snap == nil {
		snap = &model.HostSnapshot{Timestamp: time.Now()}
	}
	services, _ := h.store.ListServices()
	sum := serviceSummary{Total: len(services)}
	for _, svc := range services {
		if svc.Status == orchestrator.StatusRunning {
			sum.Running++
		}
		if svc.Kind == "model" {
			sum.Models++
			if svc.Status == orchestrator.StatusRunning {
				sum.ModelsRunning++
			}
		} else {
			sum.Apps++
		}
		if svc.Spec.HostAccess.Enabled {
			sum.HostAccess++
		}
		if svc.Health == "unhealthy" {
			sum.Unhealthy++
		}
	}
	now := time.Now()
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	today, _ := h.store.UsageTotalsSince(dayStart.Unix())
	month, _ := h.store.UsageTotalsSince(monthStart.Unix())

	alerts := []map[string]any{}
	addAlert := func(level, title, detail string) {
		alerts = append(alerts, map[string]any{"level": level, "title": title, "detail": detail})
	}
	if snap.CPU.UsagePercent >= 85 {
		addAlert("warning", "CPU 使用率偏高", strconv.FormatFloat(snap.CPU.UsagePercent, 'f', 1, 64)+"%")
	}
	if snap.Memory.UsagePercent >= 88 {
		addAlert("warning", "内存使用率偏高", strconv.FormatFloat(snap.Memory.UsagePercent, 'f', 1, 64)+"%")
	}
	if snap.Disk.UsagePercent >= 90 {
		addAlert("critical", "磁盘空间不足", strconv.FormatFloat(snap.Disk.UsagePercent, 'f', 1, 64)+"%")
	}
	if sum.Unhealthy > 0 {
		addAlert("critical", "存在异常服务", strconv.Itoa(sum.Unhealthy)+" 个服务健康检查失败")
	}

	ok(c, gin.H{
		"edition":        h.cfg.Edition,
		"version":        h.cfg.Version,
		"server_time":    now,
		"uptime_seconds": int64(time.Since(h.startedAt).Seconds()),
		"host":           snap.Host,
		"metrics": gin.H{
			"cpu":     snap.CPU,
			"memory":  snap.Memory,
			"disk":    snap.Disk,
			"network": snap.Network,
			"load":    snap.Load,
			"gpu":     snap.GPU,
			"at":      snap.Timestamp,
		},
		"containers": h.orch.ContainerCounts(ctx),
		"services":   sum,
		"engine":     h.orch.EngineSnapshot(ctx),
		"usage":      gin.H{"today": today, "month": month},
		"alerts":     alerts,
	})
}

func (h *Handler) hostSnapshot(c *gin.Context) {
	snap := h.Snapshot()
	if snap == nil {
		fail(c, http.StatusServiceUnavailable, "正在采集主机信息，请稍后重试")
		return
	}
	ok(c, snap)
}

func (h *Handler) engineInfo(c *gin.Context) {
	ctx := c.Request.Context()
	snap := h.orch.EngineSnapshot(ctx)
	info, err := h.orch.EngineInfo(ctx)
	resp := gin.H{"engine": snap, "info": nil}
	if err == nil {
		resp["info"] = info
	}
	ok(c, resp)
}

func (h *Handler) metricsHistory(c *gin.Context) {
	rng := c.DefaultQuery("range", "1h")
	var window time.Duration
	points := 180
	switch rng {
	case "1h":
		window = time.Hour
	case "6h":
		window = 6 * time.Hour
		points = 200
	case "24h":
		window = 24 * time.Hour
		points = 240
	case "7d":
		window = 7 * 24 * time.Hour
		points = 336
	default:
		window = time.Hour
	}
	since := time.Now().Add(-window).Unix()
	data, err := h.store.QueryMetrics(since, points)
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	ok(c, gin.H{"range": rng, "points": data})
}

func (h *Handler) diskUsage(c *gin.Context) {
	du, err := h.orch.Docker().DiskUsage(c.Request.Context())
	if err != nil {
		fail(c, http.StatusBadGateway, "Docker 不可用: "+err.Error())
		return
	}
	var images, containers, volumes int64
	for _, im := range du.Images {
		images += im.Size
	}
	for _, ct := range du.Containers {
		containers += ct.SizeRw
	}
	for _, v := range du.Volumes {
		if v.UsageData != nil {
			volumes += v.UsageData.Size
		}
	}
	ok(c, gin.H{
		"layers_size":     du.LayersSize,
		"images_size":     images,
		"containers_size": containers,
		"volumes_size":    volumes,
		"images":          len(du.Images),
		"containers":      len(du.Containers),
		"volumes":         len(du.Volumes),
	})
}

// containerEvents streams docker engine events as SSE (used for live
// container state in the UI).
func (h *Handler) containerEvents(c *gin.Context) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	flusher, okf := c.Writer.(http.Flusher)
	if !okf {
		fail(c, http.StatusInternalServerError, "streaming unsupported")
		return
	}
	ctx := c.Request.Context()
	emit := func(ev engine.Event) {
		payload, _ := json.Marshal(ev)
		_, _ = c.Writer.Write([]byte("data: " + string(payload) + "\n\n"))
		flusher.Flush()
	}
	_ = h.orch.Docker().StreamEvents(ctx, emit)
}
