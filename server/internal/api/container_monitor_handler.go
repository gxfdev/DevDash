package api

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gxfdev/DevDash/server/internal/docker"
	"github.com/gxfdev/DevDash/server/internal/kubernetes"
	"github.com/gxfdev/DevDash/server/internal/logger"

	"github.com/gin-gonic/gin"
)

type ContainerMonitorHandler struct {
	dockerHandler *DockerHandler
	k8sMonitor    *kubernetes.K8sMonitor
	dockerMonitor *docker.ContainerMonitor
	historyStorage *docker.HistoryStorage
	streamService *docker.ContainerStreamService
}

func NewContainerMonitorHandler(dockerHandler *DockerHandler) (*ContainerMonitorHandler, error) {
	handler := &ContainerMonitorHandler{
		dockerHandler: dockerHandler,
	}

	if dm, err := docker.NewDockerManager(); err == nil {
		handler.dockerMonitor = docker.NewContainerMonitor(dm)
		go handler.dockerMonitor.Start()
		logger.InfoLogger("Docker container monitor started")
	}

	return handler, nil
}

func (h *ContainerMonitorHandler) SetHistoryStorage(storage *docker.HistoryStorage) {
	h.historyStorage = storage
}

func (h *ContainerMonitorHandler) SetStreamService(streamSvc *docker.ContainerStreamService) {
	h.streamService = streamSvc
}

func (h *ContainerMonitorHandler) SetK8sMonitor(k8sMon *kubernetes.K8sMonitor) {
	h.k8sMonitor = k8sMon
}

func (h *ContainerMonitorHandler) RegisterRoutes(r *gin.RouterGroup) {
	monitor := r.Group("/monitor")
	{
		monitor.GET("/overview", h.getOverview)
		
		docker := monitor.Group("/docker")
		{
			docker.GET("/containers/realtime", h.getDockerRealtimeMetrics)
			docker.GET("/containers/:id/metrics", h.getContainerMetrics)
			docker.GET("/containers/:id/history", h.getContainerHistory)
			docker.GET("/top/cpu", h.getTopContainersByCPU)
			docker.GET("/top/memory", h.getTopContainersByMemory)
			docker.GET("/summary", h.getDockerSummary)
			docker.GET("/ws", h.handleWebSocket)
		}
		
		kubernetes := monitor.Group("/kubernetes")
		{
			kubernetes.GET("/clusters", h.listK8sClusters)
			kubernetes.POST("/clusters", h.addK8sCluster)
			kubernetes.DELETE("/clusters/:id", h.removeK8sCluster)
			kubernetes.GET("/clusters/:id/nodes", h.listK8sNodes)
			kubernetes.GET("/clusters/:id/pods", h.listK8sPods)
			kubernetes.GET("/clusters/:id/pods/:namespace/:name/metrics", h.getPodMetrics)
			kubernetes.GET("/clusters/:id/namespaces", h.listK8sNamespaces)
			kubernetes.GET("/clusters/:id/deployments", h.listK8sDeployments)
			kubernetes.GET("/clusters/:id/services", h.listK8sServices)
		}
	}
}

func (h *ContainerMonitorHandler) getOverview(c *gin.Context) {
	overview := make(map[string]interface{})

	if h.dockerMonitor != nil {
		allMetrics := h.dockerMonitor.GetAllCachedMetrics()
		containerCount := len(allMetrics)

		var totalCPU, totalMemory uint64
		var runningCount int

		for _, metrics := range allMetrics {
			totalCPU += uint64(metrics.CPU.UsagePercent * 100)
			totalMemory += metrics.Memory.Usage
			runningCount++
		}

		overview["docker"] = map[string]interface{}{
			"total_containers": containerCount,
			"running":          runningCount,
			"avg_cpu_percent":  float64(totalCPU) / float64(containerCount+1),
			"total_memory":     totalMemory,
		}
	}

	if h.k8sMonitor != nil {
		clusters := h.k8sMonitor.ListClusters()
		var totalPods, totalNodes int

		for _, cluster := range clusters {
			totalPods += cluster.PodCount
			totalNodes += cluster.NodeCount
		}

		overview["kubernetes"] = map[string]interface{}{
			"cluster_count": len(clusters),
			"total_pods":    totalPods,
			"total_nodes":   totalNodes,
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    overview,
	})
}

func (h *ContainerMonitorHandler) getDockerRealtimeMetrics(c *gin.Context) {
	if h.dockerMonitor == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"success": false,
			"error":   "Docker monitor not available",
		})
		return
	}

	metrics := h.dockerMonitor.GetAllCachedMetrics()

	c.JSON(http.StatusOK, gin.H{
		"success":      true,
		"data":         metrics,
		"total":        len(metrics),
		"collected_at": time.Now(),
	})
}

func (h *ContainerMonitorHandler) getContainerMetrics(c *gin.Context) {
	containerID := c.Param("id")

	if h.dockerMonitor == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"success": false,
			"error":   "Docker monitor not available",
		})
		return
	}

	metrics, ok := h.dockerMonitor.GetCachedMetrics(containerID)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "Container metrics not found",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    metrics,
	})
}

func (h *ContainerMonitorHandler) getContainerHistory(c *gin.Context) {
	containerID := c.Param("id")

	if h.historyStorage == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"success": false,
			"error":   "History storage not available",
		})
		return
	}

	durationStr := c.DefaultQuery("duration", "1h")
	duration, err := time.ParseDuration(durationStr)
	if err != nil {
		duration = 1 * time.Hour
	}

	interval := c.DefaultQuery("interval", "5m")
	limitStr := c.DefaultQuery("limit", "100")
	limit, _ := strconv.Atoi(limitStr)

	records, err := h.historyStorage.GetRecentHistory(containerID, duration)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to get container history",
			"details": err.Error(),
		})
		return
	}

	aggregated, _ := h.historyStorage.GetAggregatedMetrics(containerID, interval, limit)

	c.JSON(http.StatusOK, gin.H{
		"success":       true,
		"raw_data":      records,
		"aggregated":    aggregated,
		"duration":      durationStr,
		"record_count":  len(records),
	})
}

func (h *ContainerMonitorHandler) getTopContainersByCPU(c *gin.Context) {
	if h.historyStorage == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"success": false,
			"error":   "History storage not available",
		})
		return
	}

	limitStr := c.DefaultQuery("limit", "10")
	limit, _ := strconv.Atoi(limitStr)
	if limit <= 0 || limit > 50 {
		limit = 10
	}

	durationStr := c.DefaultQuery("duration", "24h")
	duration, err := time.ParseDuration(durationStr)
	if err != nil {
		duration = 24 * time.Hour
	}

	topContainers, err := h.historyStorage.GetTopContainersByCPU(limit, duration)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to get top containers by CPU",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    topContainers,
	})
}

func (h *ContainerMonitorHandler) getTopContainersByMemory(c *gin.Context) {
	if h.historyStorage == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"success": false,
			"error":   "History storage not available",
		})
		return
	}

	limitStr := c.DefaultQuery("limit", "10")
	limit, _ := strconv.Atoi(limitStr)
	if limit <= 0 || limit > 50 {
		limit = 10
	}

	durationStr := c.DefaultQuery("duration", "24h")
	duration, err := time.ParseDuration(durationStr)
	if err != nil {
		duration = 24 * time.Hour
	}

	topContainers, err := h.historyStorage.GetTopContainersByMemory(limit, duration)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to get top containers by memory",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    topContainers,
	})
}

func (h *ContainerMonitorHandler) getDockerSummary(c *gin.Context) {
	if h.dockerMonitor == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"success": false,
			"error":   "Docker monitor not available",
		})
		return
	}

	summary := h.dockerMonitor.GetContainerSummary()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    summary,
		"total":   len(summary),
	})
}

func (h *ContainerMonitorHandler) handleWebSocket(c *gin.Context) {
	if h.streamService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"success": false,
			"error":   "WebSocket service not available",
		})
		return
	}

	h.streamService.HandleWebSocket(c)
}

func (h *ContainerMonitorHandler) listK8sClusters(c *gin.Context) {
	if h.k8sMonitor == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"success": false,
			"error":   "Kubernetes monitor not available",
		})
		return
	}

	clusters := h.k8sMonitor.ListClusters()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    clusters,
		"total":   len(clusters),
	})
}

type AddClusterRequest struct {
	Name       string `json:"name"`
	Kubeconfig string `json:"kubeconfig,omitempty"`
	APIEndpoint string `json:"api_endpoint,omitempty"`
	CACert     string `json:"ca_cert,omitempty"`
	Token      string `json:"token,omitempty"`
}

func (h *ContainerMonitorHandler) addK8sCluster(c *gin.Context) {
	if h.k8sMonitor == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"success": false,
			"error":   "Kubernetes monitor not available",
		})
		return
	}

	var req AddClusterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid request body",
			"details": err.Error(),
		})
		return
	}

	var addErr error
	if req.Kubeconfig != "" {
		addErr = h.k8sMonitor.AddCluster(req.Kubeconfig, req.Name)
	} else if req.APIEndpoint != "" && req.Token != "" {
		addErr = h.k8sMonitor.AddClusterWithConfig(req.Name, req.APIEndpoint, req.CACert, req.Token)
	} else {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Either kubeconfig or API endpoint + token must be provided",
		})
		return
	}

	if addErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to add cluster",
			"details": addErr.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"message": fmt.Sprintf("Kubernetes cluster '%s' added successfully", req.Name),
	})
}

func (h *ContainerMonitorHandler) removeK8sCluster(c *gin.Context) {
	clusterID := c.Param("id")

	if h.k8sMonitor == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"success": false,
			"error":   "Kubernetes monitor not available",
		})
		return
	}

	err := h.k8sMonitor.RemoveCluster(clusterID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to remove cluster",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Cluster removed successfully",
	})
}

func (h *ContainerMonitorHandler) listK8sNodes(c *gin.Context) {
	clusterID := c.Param("id")

	nodes, err := h.k8sMonitor.ListNodes(clusterID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to list nodes",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    nodes,
		"total":   len(nodes),
	})
}

func (h *ContainerMonitorHandler) listK8sPods(c *gin.Context) {
	clusterID := c.Param("id")
	namespace := c.Query("namespace")
	labelSelector := c.Query("label_selector")

	pods, err := h.k8sMonitor.ListPods(clusterID, namespace, labelSelector)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to list pods",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    pods,
		"total":   len(pods),
	})
}

func (h *ContainerMonitorHandler) getPodMetrics(c *gin.Context) {
	clusterID := c.Param("id")
	namespace := c.Param("namespace")
	podName := c.Param("name")

	metrics, err := h.k8sMonitor.GetPodMetrics(clusterID, namespace, podName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to get pod metrics",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    metrics,
	})
}

func (h *ContainerMonitorHandler) listK8sNamespaces(c *gin.Context) {
	clusterID := c.Param("id")

	namespaces, err := h.k8sMonitor.ListNamespaces(clusterID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to list namespaces",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    namespaces,
		"total":   len(namespaces),
	})
}

func (h *ContainerMonitorHandler) listK8sDeployments(c *gin.Context) {
	clusterID := c.Param("id")
	namespace := c.Query("namespace")

	deployments, err := h.k8sMonitor.ListDeployments(clusterID, namespace)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to list deployments",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    deployments,
		"total":   len(deployments),
	})
}

func (h *ContainerMonitorHandler) listK8sServices(c *gin.Context) {
	clusterID := c.Param("id")
	namespace := c.Query("namespace")

	services, err := h.k8sMonitor.ListServices(clusterID, namespace)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to list services",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    services,
		"total":   len(services),
	})
}
