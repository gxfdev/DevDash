package api

import (
	"github.com/gxfdev/DevDash/server/internal/docker"
	"io"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type DockerHandler struct {
	dm       *docker.DockerManager
	cm       *docker.ComposeManager
	avail    bool
	availMsg string
}

func NewDockerHandler() (*DockerHandler, error) {
	dm, err := docker.NewDockerManager()
	if err != nil {
		return nil, err
	}

	h := &DockerHandler{
		dm: dm,
		cm: docker.NewComposeManager(dm),
	}

	if pingErr := dm.Ping(); pingErr != nil {
		h.avail = false
		h.availMsg = pingErr.Error()
	} else {
		h.avail = true
	}

	return h, nil
}

func (h *DockerHandler) DockerManager() *docker.DockerManager {
	return h.dm
}

func (h *DockerHandler) requireDocker() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !h.avail {
			c.JSON(http.StatusOK, gin.H{
				"success":         false,
				"error":           "Docker daemon is not running",
				"details":         h.availMsg,
				"docker_available": false,
			})
			c.Abort()
			return
		}
		c.Next()
	}
}

func (h *DockerHandler) RegisterRoutes(r *gin.RouterGroup) {
	docker := r.Group("/docker")
	{
		docker.GET("/ping", h.ping)

		ops := docker.Group("", h.requireDocker())
		ops.GET("/info", h.info)
		ops.GET("/usage", h.usage)

		containers := ops.Group("/containers")
		{
			containers.GET("", h.listContainers)
			containers.GET("/:id", h.getContainer)
			containers.POST("/:id/start", h.startContainer)
			containers.POST("/:id/stop", h.stopContainer)
			containers.POST("/:id/restart", h.restartContainer)
			containers.DELETE("/:id", h.removeContainer)
			containers.GET("/:id/logs", h.getContainerLogs)
			containers.GET("/:id/stats", h.getContainerStats)
		}

		images := ops.Group("/images")
		{
			images.GET("", h.listImages)
			images.POST("/pull", h.pullImage)
			images.DELETE("/:id", h.removeImage)
		}

		networks := ops.Group("/networks")
		{
			networks.GET("", h.listNetworks)
		}

		volumes := ops.Group("/volumes")
		{
			volumes.GET("", h.listVolumes)
		}

		compose := ops.Group("/compose")
		{
			compose.GET("/projects", h.listComposeProjects)
			compose.POST("/start", h.startComposeProject)
			compose.POST("/stop", h.stopComposeProject)
			compose.POST("/restart", h.restartComposeService)
			compose.GET("/logs", h.getComposeLogs)
			compose.POST("/validate", h.validateCompose)
			compose.POST("/deploy", h.deployFromTemplate)
		}
	}
}

func (h *DockerHandler) ping(c *gin.Context) {
	err := h.dm.Ping()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"error":   "Docker daemon is not running",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Docker daemon is running",
	})
}

func (h *DockerHandler) info(c *gin.Context) {
	info, err := h.dm.SystemInfo()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to get Docker info",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    info,
	})
}

func (h *DockerHandler) usage(c *gin.Context) {
	usage, err := h.dm.DiskUsage()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to get disk usage",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    usage,
	})
}

func (h *DockerHandler) listContainers(c *gin.Context) {
	all := c.DefaultQuery("all", "false") == "true"

	containers, err := h.dm.ListContainers(all)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to list containers",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    containers,
		"total":   len(containers),
	})
}

func (h *DockerHandler) getContainer(c *gin.Context) {
	id := c.Param("id")

	container, err := h.dm.GetContainer(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "Container not found",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    container,
	})
}

func (h *DockerHandler) startContainer(c *gin.Context) {
	id := c.Param("id")

	err := h.dm.StartContainer(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to start container",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Container started successfully",
	})
}

func (h *DockerHandler) stopContainer(c *gin.Context) {
	id := c.Param("id")
	timeoutStr := c.DefaultQuery("timeout", "10")
	timeout, _ := strconv.Atoi(timeoutStr)

	err := h.dm.StopContainer(id, &timeout)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to stop container",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Container stopped successfully",
	})
}

func (h *DockerHandler) restartContainer(c *gin.Context) {
	id := c.Param("id")
	timeoutStr := c.DefaultQuery("timeout", "10")
	timeout, _ := strconv.Atoi(timeoutStr)

	err := h.dm.RestartContainer(id, &timeout)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to restart container",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Container restarted successfully",
	})
}

func (h *DockerHandler) removeContainer(c *gin.Context) {
	id := c.Param("id")
	force := c.DefaultQuery("force", "false") == "true"
	volumes := c.DefaultQuery("volumes", "false") == "true"

	err := h.dm.RemoveContainer(id, force, volumes)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to remove container",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Container removed successfully",
	})
}

func (h *DockerHandler) getContainerLogs(c *gin.Context) {
	id := c.Param("id")
	tail := c.DefaultQuery("tail", "100")
	follow := c.DefaultQuery("follow", "false") == "true"

	reader, err := h.dm.GetContainerLogs(id, tail, follow)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to get logs",
			"details": err.Error(),
		})
		return
	}
	defer reader.Close()

	c.Header("Content-Type", "text/plain; charset=utf-8")
	c.Stream(func(w io.Writer) bool {
		buf := make([]byte, 1024)
		n, err := reader.Read(buf)
		if n > 0 {
			w.Write(buf[:n])
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		}
		if err != nil {
			return false
		}
		return true
	})
}

func (h *DockerHandler) getContainerStats(c *gin.Context) {
	id := c.Param("id")

	stats, err := h.dm.GetContainerStats(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to get stats",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    stats,
	})
}

func (h *DockerHandler) listImages(c *gin.Context) {
	images, err := h.dm.ListImages()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to list images",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    images,
		"total":   len(images),
	})
}

func (h *DockerHandler) pullImage(c *gin.Context) {
	var req struct {
		Image string `json:"image" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Image name is required",
		})
		return
	}

	reader, err := h.dm.PullImage(req.Image)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to pull image",
			"details": err.Error(),
		})
		return
	}
	defer reader.Close()

	c.Header("Content-Type", "application/json; charset=utf-8")
	c.Stream(func(w io.Writer) bool {
		buf := make([]byte, 1024)
		n, err := reader.Read(buf)
		if n > 0 {
			w.Write(buf[:n])
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		}
		if err != nil {
			return false
		}
		return true
	})
}

func (h *DockerHandler) removeImage(c *gin.Context) {
	id := c.Param("id")
	force := c.DefaultQuery("force", "false") == "true"

	items, err := h.dm.RemoveImage(id, force)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to remove image",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Image removed successfully",
		"data":    items,
	})
}

func (h *DockerHandler) listNetworks(c *gin.Context) {
	networks, err := h.dm.ListNetworks()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to list networks",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    networks,
		"total":   len(networks),
	})
}

func (h *DockerHandler) listVolumes(c *gin.Context) {
	volumes, err := h.dm.ListVolumes()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to list volumes",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    volumes,
		"total":   len(volumes),
	})
}

func (h *DockerHandler) listComposeProjects(c *gin.Context) {
	projects, err := h.cm.ListProjects()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to list compose projects",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    projects,
		"total":   len(projects),
	})
}

func (h *DockerHandler) startComposeProject(c *gin.Context) {
	var req struct {
		Path string `json:"path" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Compose file path is required",
		})
		return
	}

	reader, err := h.cm.StartProject(req.Path)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to start compose project",
			"details": err.Error(),
		})
		return
	}
	defer reader.Close()

	c.Header("Content-Type", "text/plain; charset=utf-8")
	c.Stream(func(w io.Writer) bool {
		buf := make([]byte, 1024)
		n, err := reader.Read(buf)
		if n > 0 {
			w.Write(buf[:n])
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		}
		if err != nil {
			return false
		}
		return true
	})
}

func (h *DockerHandler) stopComposeProject(c *gin.Context) {
	var req struct {
		Path string `json:"path" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Compose file path is required",
		})
		return
	}

	err := h.cm.StopProject(req.Path)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to stop compose project",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Compose project stopped successfully",
	})
}

func (h *DockerHandler) restartComposeService(c *gin.Context) {
	var req struct {
		Path        string `json:"path" binding:"required"`
		ServiceName string `json:"service_name" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Path and service_name are required",
		})
		return
	}

	err := h.cm.RestartService(req.Path, req.ServiceName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to restart service",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Service restarted successfully",
	})
}

func (h *DockerHandler) getComposeLogs(c *gin.Context) {
	path := c.Query("path")
	serviceName := c.Query("service_name")
	tail := c.DefaultQuery("tail", "100")
	follow := c.DefaultQuery("follow", "false") == "true"

	reader, err := h.cm.GetServiceLogs(path, serviceName, tail, follow)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to get compose logs",
			"details": err.Error(),
		})
		return
	}
	defer reader.Close()

	c.Header("Content-Type", "text/plain; charset=utf-8")
	c.Stream(func(w io.Writer) bool {
		buf := make([]byte, 1024)
		n, err := reader.Read(buf)
		if n > 0 {
			w.Write(buf[:n])
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		}
		if err != nil {
			return false
		}
		return true
	})
}

func (h *DockerHandler) validateCompose(c *gin.Context) {
	var req struct {
		Content string `json:"content" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Compose content is required",
		})
		return
	}

	err := h.cm.ValidateCompose([]byte(req.Content))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid compose file",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Compose file is valid",
	})
}

func (h *DockerHandler) deployFromTemplate(c *gin.Context) {
	var req struct {
		TemplateType string            `json:"template_type" binding:"required"`
		Config       map[string]string `json:"config"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Template type is required",
		})
		return
	}

	project, err := h.cm.DeployFromTemplate(req.TemplateType, req.Config)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to deploy from template",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Compose project created successfully",
		"data":    project,
	})
}
