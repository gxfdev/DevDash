package api

import (
	"github.com/gin-gonic/gin"
	"github.com/gxfdev/aidash/server/internal/auth"
)

func (h *Handler) Register(r *gin.Engine) {
	api := r.Group("/api")

	api.POST("/auth/login", h.login)
	api.GET("/health", func(c *gin.Context) { ok(c, gin.H{"status": "ok"}) })

	authed := api.Group("", h.auth.Middleware())
	{
		authed.GET("/auth/me", h.me)
		authed.POST("/auth/password", h.changePassword)

		authed.GET("/system/overview", h.overview)
		authed.GET("/system/host", h.hostSnapshot)
		authed.GET("/system/engine", h.engineInfo)
		authed.GET("/system/metrics", h.metricsHistory)
		authed.GET("/system/disk", h.diskUsage)
		authed.GET("/system/events", h.containerEvents)

		authed.GET("/model-assets/sources", h.assetSources)
		authed.GET("/model-assets", h.listAssets)
		authed.GET("/model-assets/:id", h.getAsset)
		authed.GET("/model-assets/:id/logs", h.assetLogs)

		authed.GET("/catalog", h.catalogList)
		authed.GET("/catalog/:id", h.catalogItem)

		authed.GET("/services", h.listServices)
		authed.GET("/services/:id", h.getService)
		authed.GET("/services/:id/logs", h.serviceLogs)
		authed.GET("/services/:id/events", h.serviceEvents)
		authed.GET("/services/:id/stats", h.serviceStats)
		authed.POST("/services/:id/exec", h.serviceExec)

		authed.GET("/docker/containers", h.listContainers)
		authed.GET("/docker/containers/:id", h.inspectContainer)
		authed.GET("/docker/containers/:id/logs", h.containerLogs)
		authed.POST("/docker/containers/:id/action", h.containerAction)
		authed.GET("/docker/images", h.listImages)
		authed.POST("/docker/images/pull", h.pullImage)
		authed.DELETE("/docker/images/:id", h.removeImage)
		authed.GET("/docker/networks", h.listNetworks)
		authed.POST("/docker/networks", h.createNetwork)
		authed.DELETE("/docker/networks/:id", h.removeNetwork)
		authed.GET("/docker/volumes", h.listVolumes)
		authed.GET("/docker/df", h.diskUsage)
		authed.DELETE("/docker/volumes/:name", h.removeVolume)

		authed.GET("/gateway", h.gatewayInfo)
		authed.GET("/gateway/keys", h.listKeys)
		authed.POST("/gateway/keys", h.createKey)
		authed.DELETE("/gateway/keys/:id", h.deleteKey)
		authed.POST("/gateway/test", h.gatewayTest)
		authed.POST("/playground/chat", h.playground)
		authed.POST("/copilot/chat", h.copilotChat)

		authed.GET("/users", auth.RequireRoles("admin"), h.listUsers)
		authed.POST("/users", auth.RequireRoles("admin"), h.createUser)
		authed.PATCH("/users/:id", auth.RequireRoles("admin"), h.updateUser)
		authed.DELETE("/users/:id", auth.RequireRoles("admin"), h.deleteUser)
		authed.GET("/audit", auth.RequireRoles("admin", "operator"), h.listAudit)
		authed.GET("/settings", auth.RequireRoles("admin", "operator"), h.getSettings)
		authed.PUT("/settings", auth.RequireRoles("admin"), h.updateSettings)

		write := authed.Group("", auth.WriteGuard())
		{
			write.POST("/model-assets", h.createAsset)
			write.POST("/model-assets/:id/cancel", h.cancelAsset)
			write.POST("/model-assets/:id/scan", h.scanAsset)
			write.DELETE("/model-assets/:id", h.deleteAsset)

			write.POST("/services", h.deployService)
			write.DELETE("/services/:id", h.removeService)
			write.POST("/services/:id/action", h.serviceAction)
		}
	}

	r.GET("/ws/terminal", h.hostTerminal)
	r.GET("/ws/services/:id/terminal", h.serviceTerminal)

	v1 := r.Group("/v1")
	v1.GET("/models", h.v1Models)
	v1.POST("/chat/completions", h.v1Chat)
	v1.POST("/completions", h.v1Completions)
	v1.POST("/embeddings", h.v1Embeddings)
}
