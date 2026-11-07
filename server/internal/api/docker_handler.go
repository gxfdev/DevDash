package api

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gxfdev/aidash/server/internal/engine"
)

func actor(c *gin.Context) string {
	v, _ := c.Get("username")
	s, _ := v.(string)
	return s
}

func contextWithTimeout(c *gin.Context, d time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(c.Request.Context(), d)
}

func (h *Handler) listContainers(c *gin.Context) {
	all := c.DefaultQuery("all", "1") != "0"
	containers, err := h.orch.Docker().ListContainers(c.Request.Context(), all)
	if err != nil {
		fail(c, http.StatusBadGateway, "Docker 连接失败: "+err.Error())
		return
	}
	ok(c, gin.H{"containers": containers})
}

func (h *Handler) inspectContainer(c *gin.Context) {
	ci, err := h.orch.Docker().InspectContainer(c.Request.Context(), c.Param("id"))
	if err != nil {
		fail(c, http.StatusBadGateway, err.Error())
		return
	}
	ok(c, ci)
}

func (h *Handler) containerLogs(c *gin.Context) {
	tail, _ := strconv.Atoi(c.DefaultQuery("tail", "200"))
	logs, err := h.orch.Docker().ContainerLogsText(c.Request.Context(), c.Param("id"), tail)
	if err != nil {
		fail(c, http.StatusBadGateway, err.Error())
		return
	}
	ok(c, gin.H{"logs": logs, "tail": tail})
}

func (h *Handler) containerAction(c *gin.Context) {
	var req serviceActionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		bad(c, err)
		return
	}
	id := c.Param("id")
	ctx, cancel := contextWithTimeout(c, 60*time.Second)
	defer cancel()
	var err error
	switch strings.ToLower(req.Action) {
	case "start":
		err = h.orch.Docker().StartContainer(ctx, id)
	case "stop":
		err = h.orch.Docker().StopContainer(ctx, id, 15)
	case "restart":
		err = h.orch.Docker().RestartContainer(ctx, id, 15)
	case "remove":
		err = h.orch.Docker().RemoveContainer(ctx, id, true, false)
	default:
		fail(c, http.StatusBadRequest, "不支持的操作："+req.Action)
		return
	}
	if err != nil {
		fail(c, http.StatusBadGateway, err.Error())
		return
	}
	h.audit(c, "容器操作", id, req.Action, "ok")
	ok(c, gin.H{"message": "操作完成"})
}

func (h *Handler) listImages(c *gin.Context) {
	images, err := h.orch.Docker().ListImages(c.Request.Context())
	if err != nil {
		fail(c, http.StatusBadGateway, err.Error())
		return
	}
	ok(c, gin.H{"images": images})
}

type pullImageRequest struct {
	Image string `json:"image"`
	Tag   string `json:"tag"`
}

func (h *Handler) pullImage(c *gin.Context) {
	var req pullImageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		bad(c, err)
		return
	}
	if strings.TrimSpace(req.Image) == "" {
		fail(c, http.StatusBadRequest, "镜像名不能为空")
		return
	}
	c.Header("Content-Type", "application/x-ndjson")
	c.Header("Cache-Control", "no-cache")
	c.Header("X-Accel-Buffering", "no")
	flusher, _ := c.Writer.(http.Flusher)
	enc := jsonEncoder(c)
	writeLine := func(v any) {
		_ = enc.Encode(v)
		if flusher != nil {
			flusher.Flush()
		}
	}
	ctx := c.Request.Context()
	err := h.orch.Docker().PullImage(ctx, req.Image, req.Tag, func(p engine.PullProgress) {
		writeLine(gin.H{"status": p.Status, "id": p.ID, "progress": p.Progress, "current": p.ProgressDetail.Current, "total": p.ProgressDetail.Total})
	})
	if err != nil {
		writeLine(gin.H{"error": err.Error()})
		return
	}
	writeLine(gin.H{"done": true})
	h.audit(c, "拉取镜像", req.Image+":"+req.Tag, "", "ok")
}

func (h *Handler) removeImage(c *gin.Context) {
	force := c.DefaultQuery("force", "0") == "1"
	if err := h.orch.Docker().RemoveImage(c.Request.Context(), c.Param("id"), force); err != nil {
		fail(c, http.StatusBadGateway, err.Error())
		return
	}
	h.audit(c, "删除镜像", c.Param("id"), "", "ok")
	ok(c, gin.H{"message": "镜像已删除"})
}

func (h *Handler) listNetworks(c *gin.Context) {
	nets, err := h.orch.Docker().ListNetworks(c.Request.Context())
	if err != nil {
		fail(c, http.StatusBadGateway, err.Error())
		return
	}
	ok(c, gin.H{"networks": nets})
}

type createNetworkRequest struct {
	Name   string `json:"name"`
	Driver string `json:"driver"`
}

func (h *Handler) createNetwork(c *gin.Context) {
	var req createNetworkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		bad(c, err)
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		fail(c, http.StatusBadRequest, "网络名称不能为空")
		return
	}
	if req.Driver == "" {
		req.Driver = "bridge"
	}
	id, err := h.orch.Docker().CreateNetwork(c.Request.Context(), req.Name, req.Driver)
	if err != nil {
		fail(c, http.StatusBadGateway, err.Error())
		return
	}
	h.audit(c, "创建网络", req.Name, req.Driver, "ok")
	ok(c, gin.H{"id": id, "name": req.Name})
}

func (h *Handler) removeNetwork(c *gin.Context) {
	if err := h.orch.Docker().RemoveNetwork(c.Request.Context(), c.Param("id")); err != nil {
		fail(c, http.StatusBadGateway, err.Error())
		return
	}
	h.audit(c, "删除网络", c.Param("id"), "", "ok")
	ok(c, gin.H{"message": "网络已删除"})
}

func (h *Handler) listVolumes(c *gin.Context) {
	vols, err := h.orch.Docker().ListVolumes(c.Request.Context())
	if err != nil {
		fail(c, http.StatusBadGateway, err.Error())
		return
	}
	ok(c, gin.H{"volumes": vols})
}

func (h *Handler) removeVolume(c *gin.Context) {
	force := c.DefaultQuery("force", "0") == "1"
	if err := h.orch.Docker().RemoveVolume(c.Request.Context(), c.Param("name"), force); err != nil {
		fail(c, http.StatusBadGateway, err.Error())
		return
	}
	h.audit(c, "删除数据卷", c.Param("name"), "", "ok")
	ok(c, gin.H{"message": "数据卷已删除"})
}
