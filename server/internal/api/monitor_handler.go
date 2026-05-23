package api

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	"devdash/internal/auth"
	"devdash/internal/collector"
	"devdash/internal/docker"
	"devdash/internal/model"

	"github.com/gin-gonic/gin"
)

type MonitorHandler struct {
	dockerMgr    *docker.DockerManager
	containerMon *docker.ContainerMonitor
	collector    *collector.Collector
	mu           sync.RWMutex
	k8sClusters  map[string]*model.KubernetesCluster
}

func NewMonitorHandler(dockerMgr *docker.DockerManager, containerMon *docker.ContainerMonitor, c *collector.Collector) *MonitorHandler {
	return &MonitorHandler{
		dockerMgr:    dockerMgr,
		containerMon: containerMon,
		collector:    c,
		k8sClusters:  make(map[string]*model.KubernetesCluster),
	}
}

func (h *MonitorHandler) RegisterRoutes(r *gin.Engine) {
	g := r.Group("/api/v1/monitor", auth.Middleware())
	{
		g.GET("/overview", h.getOverview)
		g.GET("/docker/containers/realtime", h.getDockerRealtime)
		g.GET("/docker/top/cpu", h.getTopCPU)
		g.GET("/docker/top/memory", h.getTopMemory)
		g.GET("/kubernetes/clusters", h.listK8sClusters)
		g.POST("/kubernetes/clusters", h.addK8sCluster)
		g.DELETE("/kubernetes/clusters/:id", h.removeK8sCluster)
	}
}

func (h *MonitorHandler) getOverview(c *gin.Context) {
	dockerInfo := gin.H{
		"total_containers": 0,
		"running":          0,
		"avg_cpu_percent":  0.0,
	}

	if h.dockerMgr != nil {
		containers, err := h.dockerMgr.ListContainers(false)
		if err == nil {
			running := 0
			var totalCPU float64
			for _, ct := range containers {
				if ct.State == "running" {
					running++
				}
			}
			if h.containerMon != nil {
				allMetrics := h.containerMon.GetAllCachedMetrics()
				for _, m := range allMetrics {
					totalCPU += m.CPU.UsagePercent
				}
				if len(allMetrics) > 0 {
					dockerInfo["avg_cpu_percent"] = totalCPU / float64(len(allMetrics))
				}
			}
			dockerInfo["total_containers"] = len(containers)
			dockerInfo["running"] = running
		}
	}

	h.mu.RLock()
	clusterCount := len(h.k8sClusters)
	totalNodes := 0
	totalPods := 0
	for _, cl := range h.k8sClusters {
		totalNodes += cl.NodeCount
		totalPods += cl.PodCount
	}
	h.mu.RUnlock()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"docker": dockerInfo,
			"kubernetes": gin.H{
				"cluster_count": clusterCount,
				"total_nodes":   totalNodes,
				"total_pods":    totalPods,
			},
		},
	})
}

func (h *MonitorHandler) getDockerRealtime(c *gin.Context) {
	if h.containerMon == nil {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data":    map[string]interface{}{},
		})
		return
	}

	allMetrics := h.containerMon.GetAllCachedMetrics()
	result := make(map[string]interface{}, len(allMetrics))
	for id, m := range allMetrics {
		result[id] = m
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    result,
	})
}

func (h *MonitorHandler) getTopCPU(c *gin.Context) {
	limit := 10
	if l, err := strconv.Atoi(c.Query("limit")); err == nil && l > 0 {
		limit = l
	}

	if h.containerMon == nil {
		c.JSON(http.StatusOK, gin.H{"success": true, "data": []interface{}{}})
		return
	}

	allMetrics := h.containerMon.GetAllCachedMetrics()
	type topItem struct {
		ContainerID   string  `json:"container_id"`
		ContainerName string  `json:"container_name"`
		AvgCPU        float64 `json:"avg_cpu"`
	}

	var items []topItem
	for _, m := range allMetrics {
		items = append(items, topItem{
			ContainerID:   m.ContainerID,
			ContainerName: m.ContainerName,
			AvgCPU:        m.CPU.UsagePercent,
		})
	}

	for i := 0; i < len(items); i++ {
		for j := i + 1; j < len(items); j++ {
			if items[j].AvgCPU > items[i].AvgCPU {
				items[i], items[j] = items[j], items[i]
			}
		}
	}

	if len(items) > limit {
		items = items[:limit]
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": items})
}

func (h *MonitorHandler) getTopMemory(c *gin.Context) {
	limit := 10
	if l, err := strconv.Atoi(c.Query("limit")); err == nil && l > 0 {
		limit = l
	}

	if h.containerMon == nil {
		c.JSON(http.StatusOK, gin.H{"success": true, "data": []interface{}{}})
		return
	}

	allMetrics := h.containerMon.GetAllCachedMetrics()
	type topItem struct {
		ContainerID   string `json:"container_id"`
		ContainerName string `json:"container_name"`
		AvgMemory     uint64 `json:"avg_memory"`
	}

	var items []topItem
	for _, m := range allMetrics {
		items = append(items, topItem{
			ContainerID:   m.ContainerID,
			ContainerName: m.ContainerName,
			AvgMemory:     m.Memory.Usage,
		})
	}

	for i := 0; i < len(items); i++ {
		for j := i + 1; j < len(items); j++ {
			if items[j].AvgMemory > items[i].AvgMemory {
				items[i], items[j] = items[j], items[i]
			}
		}
	}

	if len(items) > limit {
		items = items[:limit]
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": items})
}

func (h *MonitorHandler) listK8sClusters(c *gin.Context) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	clusters := make([]interface{}, 0, len(h.k8sClusters))
	for _, cl := range h.k8sClusters {
		clusters = append(clusters, cl)
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": clusters})
}

func (h *MonitorHandler) addK8sCluster(c *gin.Context) {
	var req struct {
		Name        string `json:"name" binding:"required"`
		Method      string `json:"method"`
		APIEndpoint string `json:"api_endpoint"`
		Token       string `json:"token"`
		CACert      string `json:"ca_cert"`
		Kubeconfig  string `json:"kubeconfig"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}

	cluster := &model.KubernetesCluster{
		ID:          "k8s-" + strconv.FormatInt(time.Now().UnixMilli(), 10),
		Name:        req.Name,
		APIEndpoint: req.APIEndpoint,
		Status:      "disconnected",
		Version:     "unknown",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	h.mu.Lock()
	h.k8sClusters[cluster.ID] = cluster
	h.mu.Unlock()

	c.JSON(http.StatusOK, gin.H{"success": true, "data": cluster})
}

func (h *MonitorHandler) removeK8sCluster(c *gin.Context) {
	id := c.Param("id")
	h.mu.Lock()
	delete(h.k8sClusters, id)
	h.mu.Unlock()

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "removed"})
}
