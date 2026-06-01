package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"webshell/internal/auth"
	"webshell/internal/cronmgr"
	"webshell/internal/filemgr"
	"webshell/internal/monitor"
	"webshell/internal/script"
	"webshell/internal/store"
	"webshell/internal/terminal"
)

type Handler struct {
	store   *store.Store
	secret  string
	dataDir string
}

func NewHandler(s *store.Store, secret, dataDir string) *Handler {
	return &Handler{store: s, secret: secret, dataDir: dataDir}
}

func (h *Handler) RegisterRoutes(r *gin.Engine) {
	ws := r.Group("/ws")
	ws.Use(auth.WSMiddleware(h.secret))
	ws.GET("/terminal", terminal.HandleTerminal)

	api := r.Group("/api")
	api.POST("/login", h.login)

	authed := api.Group("")
	authed.Use(auth.JWTMiddleware(h.secret))
	{
		authed.GET("/profile", h.profile)
		authed.PUT("/password", h.changePassword)

		admin := authed.Group("")
		admin.Use(auth.AdminRequired())
		{
			admin.GET("/users", h.listUsers)
			admin.POST("/users", h.createUser)
			admin.DELETE("/users/:id", h.deleteUser)
		}

		authed.GET("/monitor", h.getMonitor)
		authed.GET("/monitor/stream", h.monitorStream)

		authed.GET("/cron", h.listCronJobs)
		authed.POST("/cron", h.createCronJob)
		authed.PUT("/cron/:id", h.updateCronJob)
		authed.DELETE("/cron/:id", h.deleteCronJob)
		authed.POST("/cron/sync", h.syncCronJobs)
		authed.GET("/cron/system", h.listSystemCrontab)

		authed.GET("/files", h.listFiles)
		authed.GET("/files/content", h.readFile)
		authed.PUT("/files/content", h.writeFile)
		authed.DELETE("/files", h.deleteFile)
		authed.POST("/files/mkdir", h.mkdir)
		authed.GET("/files/tree", h.fileTree)

		authed.GET("/scripts", h.listScripts)
		authed.GET("/scripts/:id", h.getScript)
		authed.POST("/scripts", h.createScript)
		authed.PUT("/scripts/:id", h.updateScript)
		authed.DELETE("/scripts/:id", h.deleteScript)
		authed.POST("/scripts/:id/run", h.runScript)

		authed.GET("/audit-logs", h.listAuditLogs)
	}
}

func (h *Handler) login(c *gin.Context) {
	var req struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "username and password required"})
		return
	}

	user, err := h.store.GetUserByUsername(req.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "server error"})
		return
	}
	if user == nil || !auth.CheckPassword(req.Password, user.Password) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	token, err := auth.GenerateToken(h.secret, user.ID, user.Username, user.Role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "token generation failed"})
		return
	}

	h.store.AddAuditLog(user.Username, "login", "user logged in")
	c.JSON(http.StatusOK, gin.H{
		"token": token,
		"user": gin.H{
			"id":       user.ID,
			"username": user.Username,
			"role":     user.Role,
		},
	})
}

func (h *Handler) profile(c *gin.Context) {
	userID := c.GetInt64("user_id")
	user, err := h.store.GetUserByID(userID)
	if err != nil || user == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"id":       user.ID,
		"username": user.Username,
		"role":     user.Role,
	})
}

func (h *Handler) changePassword(c *gin.Context) {
	var req struct {
		OldPassword string `json:"oldPassword" binding:"required"`
		NewPassword string `json:"newPassword" binding:"required,min=6"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "old and new password required (min 6 chars)"})
		return
	}

	userID := c.GetInt64("user_id")
	user, err := h.store.GetUserByID(userID)
	if err != nil || user == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}
	if !auth.CheckPassword(req.OldPassword, user.Password) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "old password incorrect"})
		return
	}

	if err := h.store.UpdatePassword(userID, req.NewPassword); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "update failed"})
		return
	}
	h.store.AddAuditLog(user.Username, "change_password", "password changed")
	c.JSON(http.StatusOK, gin.H{"message": "password updated"})
}

func (h *Handler) listUsers(c *gin.Context) {
	users, err := h.store.ListUsers()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "fetch failed"})
		return
	}
	c.JSON(http.StatusOK, users)
}

func (h *Handler) createUser(c *gin.Context) {
	var req struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required,min=6"`
		Role     string `json:"role"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "username and password required (min 6 chars)"})
		return
	}
	if req.Role == "" {
		req.Role = "user"
	}
	if err := h.store.CreateUser(req.Username, req.Password, req.Role); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "username already exists"})
		return
	}
	h.store.AddAuditLog(c.GetString("username"), "create_user", "created user: "+req.Username)
	c.JSON(http.StatusCreated, gin.H{"message": "user created"})
}

func (h *Handler) deleteUser(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	if err := h.store.DeleteUser(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "delete failed"})
		return
	}
	h.store.AddAuditLog(c.GetString("username"), "delete_user", "deleted user id: "+c.Param("id"))
	c.JSON(http.StatusOK, gin.H{"message": "user deleted"})
}

func (h *Handler) getMonitor(c *gin.Context) {
	status, err := monitor.Collect()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, status)
}

func (h *Handler) monitorStream(c *gin.Context) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.Request.Context().Done():
			return
		case <-ticker.C:
			status, err := monitor.Collect()
			if err != nil {
				continue
			}
			c.SSEvent("monitor", status)
			c.Writer.Flush()
		}
	}
}

func (h *Handler) listCronJobs(c *gin.Context) {
	jobs, err := h.store.ListCronJobs()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "fetch failed"})
		return
	}
	c.JSON(http.StatusOK, jobs)
}

func (h *Handler) createCronJob(c *gin.Context) {
	var req struct {
		Name     string `json:"name" binding:"required"`
		Schedule string `json:"schedule" binding:"required"`
		Command  string `json:"command" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name, schedule and command required"})
		return
	}
	if err := h.store.CreateCronJob(req.Name, req.Schedule, req.Command); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "create failed"})
		return
	}
	h.store.AddAuditLog(c.GetString("username"), "create_cron", req.Name+": "+req.Schedule+" "+req.Command)
	c.JSON(http.StatusCreated, gin.H{"message": "cron job created"})
}

func (h *Handler) updateCronJob(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var req struct {
		Name     string `json:"name" binding:"required"`
		Schedule string `json:"schedule" binding:"required"`
		Command  string `json:"command" binding:"required"`
		Enabled  bool   `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name, schedule and command required"})
		return
	}
	if err := h.store.UpdateCronJob(id, req.Name, req.Schedule, req.Command, req.Enabled); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "update failed"})
		return
	}
	h.store.AddAuditLog(c.GetString("username"), "update_cron", req.Name)
	c.JSON(http.StatusOK, gin.H{"message": "cron job updated"})
}

func (h *Handler) deleteCronJob(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	job, _ := h.store.GetCronJob(id)
	if err := h.store.DeleteCronJob(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "delete failed"})
		return
	}
	name := ""
	if job != nil {
		name = job.Name
	}
	h.store.AddAuditLog(c.GetString("username"), "delete_cron", name)
	c.JSON(http.StatusOK, gin.H{"message": "cron job deleted"})
}

func (h *Handler) syncCronJobs(c *gin.Context) {
	jobs, err := h.store.ListCronJobs()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "fetch failed"})
		return
	}
	var items []cronmgr.SyncJob
	for _, j := range jobs {
		items = append(items, cronmgr.SyncJob{Schedule: j.Schedule, Command: j.Command, Enabled: j.Enabled})
	}
	if err := cronmgr.SyncCrontab(items); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "sync failed: " + err.Error()})
		return
	}
	h.store.AddAuditLog(c.GetString("username"), "sync_cron", "synced crontab")
	c.JSON(http.StatusOK, gin.H{"message": "crontab synced"})
}

func (h *Handler) listSystemCrontab(c *gin.Context) {
	entries, err := cronmgr.ListCrontab()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"entries": []cronmgr.CrontabEntry{}, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"entries": entries, "available": cronmgr.IsCrontabAvailable()})
}

func (h *Handler) listFiles(c *gin.Context) {
	path := c.Query("path")
	if path == "" {
		path = "/"
	}
	files, err := filemgr.ListDir(path)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"path": path, "files": files})
}

func (h *Handler) readFile(c *gin.Context) {
	path := c.Query("path")
	if path == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "path required"})
		return
	}
	content, err := filemgr.ReadFile(path)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, content)
}

func (h *Handler) writeFile(c *gin.Context) {
	var req struct {
		Path    string `json:"path" binding:"required"`
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "path required"})
		return
	}
	if err := filemgr.WriteFile(req.Path, req.Content); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	h.store.AddAuditLog(c.GetString("username"), "write_file", req.Path)
	c.JSON(http.StatusOK, gin.H{"message": "file saved"})
}

func (h *Handler) deleteFile(c *gin.Context) {
	path := c.Query("path")
	if path == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "path required"})
		return
	}
	if err := filemgr.DeletePath(path); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	h.store.AddAuditLog(c.GetString("username"), "delete_file", path)
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

func (h *Handler) mkdir(c *gin.Context) {
	var req struct {
		Path string `json:"path" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "path required"})
		return
	}
	if err := filemgr.CreateDir(req.Path); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	h.store.AddAuditLog(c.GetString("username"), "mkdir", req.Path)
	c.JSON(http.StatusOK, gin.H{"message": "directory created"})
}

func (h *Handler) fileTree(c *gin.Context) {
	root := c.Query("root")
	if root == "" {
		root = "/"
	}
	depth, _ := strconv.Atoi(c.Query("depth"))
	if depth <= 0 || depth > 5 {
		depth = 3
	}
	files, err := filemgr.WalkDir(root, depth)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"root": root, "files": files})
}

func (h *Handler) listScripts(c *gin.Context) {
	scripts, err := h.store.ListScripts()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "fetch failed"})
		return
	}
	c.JSON(http.StatusOK, scripts)
}

func (h *Handler) getScript(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	sc, err := h.store.GetScript(id)
	if err != nil || sc == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "script not found"})
		return
	}
	c.JSON(http.StatusOK, sc)
}

func (h *Handler) createScript(c *gin.Context) {
	var req struct {
		Name        string `json:"name" binding:"required"`
		Description string `json:"description"`
		Content     string `json:"content" binding:"required"`
		Interpreter string `json:"interpreter"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name and content required"})
		return
	}
	if req.Interpreter == "" {
		req.Interpreter = "/bin/bash"
	}
	if err := h.store.CreateScript(req.Name, req.Description, req.Content, req.Interpreter); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "create failed"})
		return
	}
	h.store.AddAuditLog(c.GetString("username"), "create_script", req.Name)
	c.JSON(http.StatusCreated, gin.H{"message": "script created"})
}

func (h *Handler) updateScript(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var req struct {
		Name        string `json:"name" binding:"required"`
		Description string `json:"description"`
		Content     string `json:"content" binding:"required"`
		Interpreter string `json:"interpreter"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name and content required"})
		return
	}
	if req.Interpreter == "" {
		req.Interpreter = "/bin/bash"
	}
	if err := h.store.UpdateScript(id, req.Name, req.Description, req.Content, req.Interpreter); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "update failed"})
		return
	}
	h.store.AddAuditLog(c.GetString("username"), "update_script", req.Name)
	c.JSON(http.StatusOK, gin.H{"message": "script updated"})
}

func (h *Handler) deleteScript(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	sc, _ := h.store.GetScript(id)
	if err := h.store.DeleteScript(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "delete failed"})
		return
	}
	name := ""
	if sc != nil {
		name = sc.Name
	}
	h.store.AddAuditLog(c.GetString("username"), "delete_script", name)
	c.JSON(http.StatusOK, gin.H{"message": "script deleted"})
}

func (h *Handler) runScript(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	sc, err := h.store.GetScript(id)
	if err != nil || sc == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "script not found"})
		return
	}
	h.store.AddAuditLog(c.GetString("username"), "run_script", sc.Name)
	result := script.Execute(sc.Interpreter, sc.Content, 30*time.Second)
	c.JSON(http.StatusOK, result)
}

func (h *Handler) listAuditLogs(c *gin.Context) {
	limit, _ := strconv.Atoi(c.Query("limit"))
	logs, err := h.store.ListAuditLogs(limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "fetch failed"})
		return
	}
	c.JSON(http.StatusOK, logs)
}
