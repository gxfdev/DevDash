package api

import (
	"net/http"
	"time"

	"github.com/gxfdev/DevDash/server/internal/agent"
	"github.com/gxfdev/DevDash/server/internal/auth"
	"github.com/gxfdev/DevDash/server/internal/logger"

	"github.com/gin-gonic/gin"
)

type AgentHandler struct {
	agentMgr *agent.AgentManager
}

func NewAgentHandler(agentMgr *agent.AgentManager) *AgentHandler {
	return &AgentHandler{
		agentMgr: agentMgr,
	}
}

func (h *AgentHandler) RegisterRoutes(r *gin.Engine) {
	agentGroup := r.Group("/api/v1/hosts", auth.Middleware())
	{
		agentGroup.GET("", h.ListHosts)
		agentGroup.GET("/overview", h.GetOverview)
		agentGroup.GET("/:id", h.GetHost)
		agentGroup.GET("/:id/metrics", h.GetHostMetrics)
		agentGroup.GET("/:id/containers", h.GetHostContainers)
		agentGroup.GET("/:id/containers/:containerId/stats", h.GetHostContainerStats)
		agentGroup.POST("", auth.RequireRole("admin"), h.RegisterHost)
		agentGroup.DELETE("/:id", auth.RequireRole("admin"), h.RemoveHost)
		agentGroup.POST("/:id/collect", auth.RequireRole("admin"), h.CollectFromHost)
		agentGroup.POST("/collect-all", auth.RequireRole("admin"), h.CollectFromAllHosts)
	}
}

func (h *AgentHandler) ListHosts(c *gin.Context) {
	hosts := h.agentMgr.ListHosts()
	for _, h := range hosts {
		if h.Token != "" {
			h.Token = "***"
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    hosts,
	})
}

func (h *AgentHandler) RegisterHost(c *gin.Context) {
	var req struct {
		Name     string `json:"name" binding:"required"`
		Endpoint string `json:"endpoint" binding:"required"`
		Token    string `json:"token"`
		OS       string `json:"os"`
		Arch     string `json:"arch"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid request: " + err.Error(),
		})
		return
	}

	if len(req.Name) > 128 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "name too long (max 128)"})
		return
	}

	if err := auth.ValidateEndpoint(req.Endpoint); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Endpoint validation failed: " + err.Error(),
		})
		return
	}

	host := &agent.RemoteHost{
		ID:       generateHostID(req.Name, req.Endpoint),
		Name:     req.Name,
		Endpoint: req.Endpoint,
		Token:    req.Token,
		Status:   "online",
		OS:       req.OS,
		Arch:     req.Arch,
	}

	if err := h.agentMgr.RegisterHost(host); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	safeHost := *host
	safeHost.Token = "***"
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    safeHost,
	})
}

func (h *AgentHandler) GetHost(c *gin.Context) {
	hostID := c.Param("id")
	host, ok := h.agentMgr.GetHost(hostID)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "Host not found",
		})
		return
	}
	if host.Token != "" {
		host.Token = "***"
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    host,
	})
}

func (h *AgentHandler) RemoveHost(c *gin.Context) {
	hostID := c.Param("id")
	h.agentMgr.RemoveHost(hostID)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Host removed",
	})
}

func (h *AgentHandler) CollectFromHost(c *gin.Context) {
	hostID := c.Param("id")
	metrics, err := h.agentMgr.CollectFromHost(hostID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    metrics,
	})
}

func (h *AgentHandler) GetHostMetrics(c *gin.Context) {
	hostID := c.Param("id")
	metrics, ok := h.agentMgr.GetHostMetrics(hostID)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "No metrics available for this host",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    metrics,
	})
}

func (h *AgentHandler) GetHostContainers(c *gin.Context) {
	hostID := c.Param("id")
	containers, err := h.agentMgr.GetHostContainers(hostID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    containers,
	})
}

func (h *AgentHandler) GetHostContainerStats(c *gin.Context) {
	hostID := c.Param("id")
	containerID := c.Param("containerId")

	stats, err := h.agentMgr.GetHostContainerStats(hostID, containerID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    stats,
	})
}

func (h *AgentHandler) CollectFromAllHosts(c *gin.Context) {
	allMetrics := h.agentMgr.CollectFromAllHosts()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    allMetrics,
	})
}

func (h *AgentHandler) GetOverview(c *gin.Context) {
	hosts := h.agentMgr.ListHosts()
	allMetrics := h.agentMgr.GetAllMetrics()

	totalHosts := len(hosts)
	onlineHosts := 0
	offlineHosts := 0
	totalContainers := 0
	totalCPUPercent := 0.0
	totalMemoryUsedGB := 0.0
	totalMemoryTotalGB := 0.0

	for _, h := range hosts {
		if h.Status == "online" {
			onlineHosts++
		} else {
			offlineHosts++
		}
	}

	for _, m := range allMetrics {
		if m.Snapshot != nil {
			totalCPUPercent += m.Snapshot.CPU.UsagePercent
			totalMemoryUsedGB += m.Snapshot.Memory.UsedGB
			totalMemoryTotalGB += m.Snapshot.Memory.TotalGB
		}
		totalContainers += len(m.Containers)
	}

	avgCPU := 0.0
	if totalHosts > 0 {
		avgCPU = totalCPUPercent / float64(totalHosts)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"total_hosts":     totalHosts,
			"online_hosts":    onlineHosts,
			"offline_hosts":   offlineHosts,
			"total_containers": totalContainers,
			"avg_cpu_percent":    avgCPU,
			"total_memory_used_gb":  totalMemoryUsedGB,
			"total_memory_total_gb": totalMemoryTotalGB,
			"hosts":            hosts,
		},
	})
}

func generateHostID(name, endpoint string) string {
	return name + "-" + endpoint + "-" + time.Now().Format("20060102150405")
}

func init() {
	_ = logger.InfoLogger
}
