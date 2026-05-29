package api

import (
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gxfdev/DevDash/server/internal/auth"
	"github.com/gxfdev/DevDash/server/internal/collector"
	"github.com/gxfdev/DevDash/server/internal/docker"
	"github.com/gxfdev/DevDash/server/internal/model"

	"github.com/gin-gonic/gin"
)

const maxTopLimit = 100

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
			for _, ct := range containers {
				if ct.State == "running" {
					running++
				}
			}
			if h.containerMon != nil {
				allMetrics := h.containerMon.GetAllCachedMetrics()
				var totalCPU float64
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
	if allMetrics == nil {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data":    map[string]interface{}{},
		})
		return
	}

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
	limit := parseLimit(c.Query("limit"), 10, maxTopLimit)

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

	items := make([]topItem, 0, len(allMetrics))
	for _, m := range allMetrics {
		items = append(items, topItem{
			ContainerID:   m.ContainerID,
			ContainerName: m.ContainerName,
			AvgCPU:        m.CPU.UsagePercent,
		})
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].AvgCPU > items[j].AvgCPU
	})

	if len(items) > limit {
		items = items[:limit]
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": items})
}

func (h *MonitorHandler) getTopMemory(c *gin.Context) {
	limit := parseLimit(c.Query("limit"), 10, maxTopLimit)

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

	items := make([]topItem, 0, len(allMetrics))
	for _, m := range allMetrics {
		items = append(items, topItem{
			ContainerID:   m.ContainerID,
			ContainerName: m.ContainerName,
			AvgMemory:     m.Memory.Usage,
		})
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].AvgMemory > items[j].AvgMemory
	})

	if len(items) > limit {
		items = items[:limit]
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": items})
}

func (h *MonitorHandler) listK8sClusters(c *gin.Context) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	clusters := make([]*model.KubernetesCluster, 0, len(h.k8sClusters))
	for _, cl := range h.k8sClusters {
		safe := *cl
		clusters = append(clusters, &safe)
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

	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "name is required"})
		return
	}

	if req.APIEndpoint != "" {
		if _, err := url.Parse(req.APIEndpoint); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid api_endpoint format"})
			return
		}
		if !strings.HasPrefix(req.APIEndpoint, "https://") {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "api_endpoint must use https"})
			return
		}
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

	resp := *cluster
	c.JSON(http.StatusOK, gin.H{"success": true, "data": resp})
}

func (h *MonitorHandler) removeK8sCluster(c *gin.Context) {
	id := c.Param("id")

	h.mu.Lock()
	_, exists := h.k8sClusters[id]
	if !exists {
		h.mu.Unlock()
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "cluster not found"})
		return
	}
	delete(h.k8sClusters, id)
	h.mu.Unlock()

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "removed"})
}

func parseLimit(raw string, defaultVal, maxVal int) int {
	limit := defaultVal
	if l, err := strconv.Atoi(raw); err == nil && l > 0 {
		limit = l
	}
	if limit > maxVal {
		limit = maxVal
	}
	return limit
}
