package api

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gxfdev/aidash/server/internal/orchestrator"
)

func (h *Handler) deployService(c *gin.Context) {
	var req orchestrator.DeployRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		bad(c, err)
		return
	}
	req.Owner = actor(c)
	svc, err := h.orch.Deploy(c.Request.Context(), req)
	if err != nil {
		h.audit(c, "部署服务", req.Name, err.Error(), "failed")
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	detail := svc.Image
	if svc.Spec.HostAccess.Enabled {
		detail += "（已授权宿主机权限）"
	}
	h.audit(c, "部署服务", svc.Name, detail, "ok")
	c.JSON(http.StatusAccepted, gin.H{"data": svc})
}

func (h *Handler) listServices(c *gin.Context) {
	services, err := h.store.ListServices()
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	ok(c, gin.H{"services": services})
}

func (h *Handler) getService(c *gin.Context) {
	svc, err := h.store.GetService(c.Param("id"))
	if err != nil {
		fail(c, http.StatusNotFound, "服务不存在")
		return
	}
	events, _ := h.store.ListServiceEvents(svc.ID, 60)
	stats, statsErr := h.orch.Stats(c.Request.Context(), svc.ID)
	resp := gin.H{"service": svc, "events": events, "busy": h.orch.IsBusy(svc.ID)}
	if statsErr == nil {
		resp["stats"] = stats
	}
	ok(c, resp)
}

func (h *Handler) serviceLogs(c *gin.Context) {
	tail, _ := strconv.Atoi(c.DefaultQuery("tail", "200"))
	logs, err := h.orch.Logs(c.Request.Context(), c.Param("id"), tail)
	if err != nil {
		fail(c, http.StatusBadGateway, err.Error())
		return
	}
	ok(c, gin.H{"logs": logs, "tail": tail})
}

func (h *Handler) serviceEvents(c *gin.Context) {
	svc, err := h.store.GetService(c.Param("id"))
	if err != nil {
		fail(c, http.StatusNotFound, "服务不存在")
		return
	}
	events, err := h.store.ListServiceEvents(svc.ID, 200)
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	ok(c, gin.H{"events": events})
}

func (h *Handler) serviceStats(c *gin.Context) {
	stats, err := h.orch.Stats(c.Request.Context(), c.Param("id"))
	if err != nil {
		fail(c, http.StatusBadGateway, err.Error())
		return
	}
	ok(c, stats)
}

type serviceActionRequest struct {
	Action string `json:"action"`
}

func (h *Handler) serviceAction(c *gin.Context) {
	svc, err := h.store.GetService(c.Param("id"))
	if err != nil {
		fail(c, http.StatusNotFound, "服务不存在")
		return
	}
	var req serviceActionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		bad(c, err)
		return
	}
	ctx, cancel := contextWithTimeout(c, 90*time.Second)
	defer cancel()
	switch strings.ToLower(req.Action) {
	case "start":
		err = h.orch.Start(ctx, svc.ID)
	case "stop":
		err = h.orch.Stop(ctx, svc.ID)
	case "restart":
		err = h.orch.Restart(ctx, svc.ID)
	case "upgrade":
		err = h.orch.Upgrade(ctx, svc.ID)
	default:
		fail(c, http.StatusBadRequest, "不支持的操作："+req.Action)
		return
	}
	if err != nil {
		h.audit(c, "服务操作", svc.Name, req.Action+": "+err.Error(), "failed")
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	h.audit(c, "服务操作", svc.Name, req.Action, "ok")
	updated, _ := h.store.GetService(svc.ID)
	ok(c, gin.H{"service": updated})
}

func (h *Handler) removeService(c *gin.Context) {
	svc, err := h.store.GetService(c.Param("id"))
	if err != nil {
		fail(c, http.StatusNotFound, "服务不存在")
		return
	}
	keepVolumes := c.DefaultQuery("keep_volumes", "0") == "1"
	ctx, cancel := contextWithTimeout(c, 60*time.Second)
	defer cancel()
	if err := h.orch.Remove(ctx, svc.ID, keepVolumes); err != nil {
		h.audit(c, "删除服务", svc.Name, err.Error(), "failed")
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	h.audit(c, "删除服务", svc.Name, "keep_volumes="+strconv.FormatBool(keepVolumes), "ok")
	ok(c, gin.H{"message": "服务已移除"})
}

type execRequest struct {
	Command string `json:"command"`
	Timeout int    `json:"timeout"`
}

func (h *Handler) serviceExec(c *gin.Context) {
	svc, err := h.store.GetService(c.Param("id"))
	if err != nil {
		fail(c, http.StatusNotFound, "服务不存在")
		return
	}
	var req execRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		bad(c, err)
		return
	}
	if strings.TrimSpace(req.Command) == "" {
		fail(c, http.StatusBadRequest, "命令不能为空")
		return
	}
	timeout := 30 * time.Second
	if req.Timeout > 0 && req.Timeout <= 120 {
		timeout = time.Duration(req.Timeout) * time.Second
	}
	ctx, cancel := contextWithTimeout(c, timeout+2*time.Second)
	defer cancel()
	out, err := h.orch.RunCommand(ctx, svc.ID, req.Command, timeout)
	h.audit(c, "容器命令", svc.Name, req.Command, map[bool]string{true: "ok", false: "failed"}[err == nil])
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"data": gin.H{"output": out, "error": err.Error()}})
		return
	}
	ok(c, gin.H{"output": out})
}
