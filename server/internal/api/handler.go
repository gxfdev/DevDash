package api

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"math"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/gxfdev/DevDash/server/internal/alert"
	"github.com/gxfdev/DevDash/server/internal/auth"
	"github.com/gxfdev/DevDash/server/internal/collector"
	"github.com/gxfdev/DevDash/server/internal/config"
	"github.com/gxfdev/DevDash/server/internal/filemgr"
	"github.com/gxfdev/DevDash/server/internal/model"
	"github.com/gxfdev/DevDash/server/internal/store"
	"github.com/gxfdev/DevDash/server/internal/terminal"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if origin == "" || origin == "null" {
			return true
		}
		allowedOrigins := os.Getenv("CORS_ORIGINS")
		if allowedOrigins != "" {
			for _, o := range strings.Split(allowedOrigins, ",") {
				if strings.TrimSpace(o) == origin {
					return true
				}
			}
		}
		u, err := url.Parse(origin)
		if err != nil {
			return false
		}
		host := u.Hostname()
		if host == "localhost" || host == "127.0.0.1" || host == "::1" {
			return true
		}
		if ip := net.ParseIP(host); ip != nil {
			if ip.IsLoopback() || ip.IsPrivate() {
				return true
			}
		}
		return false
	},
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

type Handler struct {
	collector      *collector.Collector
	store          *store.Store
	cfg            *config.Config
	alertEngine    *alert.Engine
	mu             sync.RWMutex
	cachedSnapshot interface{}
}

func NewHandler(c *collector.Collector, s *store.Store, cfg *config.Config) *Handler {
	return &Handler{
		collector: c,
		store:     s,
		cfg:       cfg,
	}
}

func (h *Handler) SetAlertEngine(e *alert.Engine) {
	h.alertEngine = e
}

func (h *Handler) RegisterRoutes(r *gin.Engine) {
	r.Use(requestSizeLimit(10 * 1024 * 1024))
	r.Use(apiRateLimit())
	r.Use(securityHeaders())

	ws := r.Group("/ws")
	ws.Use(auth.WSMiddleware())
	ws.GET("/terminal", h.terminalWS)
	ws.GET("/terminal/:nodeID", h.terminalWS)

	api := r.Group("/api")

	api.GET("/v1/health", h.healthCheck)
	api.GET("/v1/readiness", h.readinessCheck)

	api.POST("/auth/login", h.login)
	api.POST("/auth/refresh", h.refreshToken)
	authGroup := api.Group("", auth.Middleware(), auth.CSRFMiddleware())
	authGroup.GET("/auth/me", h.authMe)
	authGroup.PUT("/auth/password", h.changePassword)
	authGroup.PUT("/auth/username", h.changeUsername)
	authGroup.POST("/auth/logout", h.authLogout)

	adminUsers := api.Group("", auth.Middleware(), auth.CSRFMiddleware(), auth.RequireRole("admin"))
	adminUsers.GET("/users", h.listUsers)
	adminUsers.POST("/users", h.createUser)
	adminUsers.DELETE("/users/:id", h.deleteUser)

	v1 := r.Group("/api/v1", auth.Middleware(), auth.CSRFMiddleware())

	v1.GET("/snapshot", h.getSnapshot)
	v1.GET("/latest", h.getLatest)
	v1.GET("/history", h.getHistory)
	v1.GET("/trend/compare", h.getTrendCompare)
	v1.GET("/anomaly/detect", h.detectAnomalies)
	v1.GET("/monitor/stream", h.monitorStream)
	v1.GET("/settings/collect-interval", h.getCollectInterval)
	v1.PUT("/settings/collect-interval", auth.RequireRole("admin"), h.updateCollectInterval)

	v1.GET("/cronjobs", h.listCronJobs)
	v1.POST("/cronjobs", auth.RequireRole("admin"), h.createCronJob)
	v1.PUT("/cronjobs/:id", auth.RequireRole("admin"), h.updateCronJob)
	v1.DELETE("/cronjobs/:id", auth.RequireRole("admin"), h.deleteCronJob)
	v1.POST("/cronjobs/:id/run", auth.RequireRole("admin"), h.runCronJob)
	v1.GET("/cronjobs/:id/logs", h.listCronJobLogs)
	v1.GET("/cronjob-logs", h.listAllCronJobLogs)

	v1.GET("/fs/list", h.listFiles)
	v1.GET("/fs/stats", h.getFileStats)
	v1.POST("/fs/mkdir", auth.RequireRole("admin"), h.createDir)
	v1.POST("/fs/mkfile", auth.RequireRole("admin"), h.createFile)
	v1.DELETE("/fs/remove", auth.RequireRole("admin"), h.removeFile)
	v1.GET("/fs/read", h.readFile)
	v1.PUT("/fs/write", auth.RequireRole("admin"), h.writeFile)
	v1.PUT("/fs/chmod", auth.RequireRole("admin"), h.chmodFile)
	v1.POST("/fs/upload", auth.RequireRole("admin"), h.uploadFile)
	v1.GET("/fs/download", h.downloadFile)

	v1.GET("/scripts", h.listScripts)
	v1.GET("/scripts/:id", h.getScript)
	v1.POST("/scripts", auth.RequireRole("admin"), h.createScript)
	v1.PUT("/scripts/:id", auth.RequireRole("admin"), h.updateScript)
	v1.DELETE("/scripts/:id", auth.RequireRole("admin"), h.deleteScript)
	v1.POST("/scripts/:id/run", h.runScript)
	v1.POST("/scripts/check", h.checkScriptSyntax)

	v1.GET("/terminal/history", h.getCommandHistory)
	v1.DELETE("/terminal/history", h.clearCommandHistory)
	v1.GET("/terminal/shells", h.listShells)

	v1.GET("/alerts", h.listAlerts)
	v1.GET("/alerts/active", h.listActiveAlerts)
	v1.GET("/alerts/history", h.listAlertHistory)
	v1.POST("/alerts/:id/silence", auth.RequireRole("admin"), h.silenceAlert)

	v1.GET("/alert-rules", h.listAlertRules)
	v1.POST("/alert-rules", auth.RequireRole("admin"), h.createAlertRule)
	v1.PUT("/alert-rules/:id", auth.RequireRole("admin"), h.updateAlertRule)
	v1.DELETE("/alert-rules/:id", auth.RequireRole("admin"), h.deleteAlertRule)

	v1.GET("/alert-notify/config", h.getAlertNotifyConfig)
	v1.PUT("/alert-notify/config", auth.RequireRole("admin"), h.updateAlertNotifyConfig)
	v1.POST("/alert-notify/test", auth.RequireRole("admin"), h.testAlertNotify)

	v1.GET("/audit-logs", h.listAuditLogs)
	v1.POST("/collect", auth.RequireRole("admin"), h.triggerCollect)
}

func requestSizeLimit(maxBytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
		c.Next()
	}
}

func securityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("X-XSS-Protection", "1; mode=block")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Header("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline' 'unsafe-eval'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; connect-src 'self' ws: wss:; font-src 'self' data:")
		c.Header("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		c.Next()
	}
}

func apiRateLimit() gin.HandlerFunc {
	type visitor struct {
		count       int
		windowStart time.Time
		lastSeen    time.Time
	}
	var (
		visitors = make(map[string]*visitor)
		mu       sync.Mutex
	)
	const windowSize = time.Minute
	const maxRequests = 600
	go func() {
		for {
			time.Sleep(2 * time.Minute)
			mu.Lock()
			now := time.Now()
			for ip, v := range visitors {
				if now.Sub(v.lastSeen) > 2*windowSize {
					delete(visitors, ip)
				}
			}
			mu.Unlock()
		}
	}()
	return func(c *gin.Context) {
		ip := c.ClientIP()
		mu.Lock()
		now := time.Now()
		v, exists := visitors[ip]
		if !exists {
			visitors[ip] = &visitor{count: 1, windowStart: now, lastSeen: now}
			mu.Unlock()
			c.Next()
			return
		}
		v.lastSeen = now
		if now.Sub(v.windowStart) > windowSize {
			v.count = 1
			v.windowStart = now
		} else {
			v.count++
		}
		if v.count > maxRequests {
			mu.Unlock()
			c.Header("Retry-After", fmt.Sprintf("%d", int(windowSize.Seconds())))
			c.AbortWithStatusJSON(429, gin.H{"error": "rate limit exceeded"})
			return
		}
		mu.Unlock()
		c.Next()
	}
}

// ── Auth ──────────────────────────────────────────────────────

func (h *Handler) login(c *gin.Context) {
	var req struct{ Username, Password string }
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid request"})
		return
	}
	if req.Username == "" || req.Password == "" {
		c.JSON(400, gin.H{"error": "username and password are required"})
		return
	}
	if err := auth.ValidateUsername(req.Username); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	clientIP := c.ClientIP()
	if err := auth.CheckLoginRateLimit(clientIP); err != nil {
		c.JSON(429, gin.H{"error": "too many attempts, please try again later"})
		return
	}

	user, err := h.store.GetUserByUsername(req.Username)
	if err != nil || user == nil {
		auth.CheckPassword("", req.Password)
		log.Printf("[auth] login failed for user %s from %s", auth.SanitizeLog(req.Username), c.ClientIP())
		c.JSON(401, gin.H{"error": "invalid credentials"})
		return
	}
	if !auth.CheckPassword(user.PasswordHash, req.Password) {
		log.Printf("[auth] login failed for user %s from %s", auth.SanitizeLog(req.Username), c.ClientIP())
		c.JSON(401, gin.H{"error": "invalid credentials"})
		return
	}
	auth.ResetLoginAttempts(clientIP)
	log.Printf("[auth] login success for user %s from %s", auth.SanitizeLog(req.Username), c.ClientIP())

	tokenPair, err := auth.GenerateTokenPair(user.ID, user.Username, user.Role)
	if err != nil {
		c.JSON(500, gin.H{"error": "failed to generate token"})
		return
	}

	csrfToken, _ := auth.GenerateCSRFToken()
	secure := os.Getenv("GIN_MODE") == "release"
	c.SetCookie("csrf_token", csrfToken, 86400, "/", "", secure, true)
	c.SetCookie("refresh_token", tokenPair.RefreshToken, 7*86400, "/", "", secure, true)

	resp := gin.H{
		"access_token": tokenPair.AccessToken,
		"expires_in":   tokenPair.ExpiresIn,
		"token_type":   "Bearer",
	}
	if user.MustChangePwd {
		resp["must_change_pwd"] = true
	}
	c.JSON(200, resp)
}

func (h *Handler) authMe(c *gin.Context) {
	username, _ := c.Get("username")
	role, _ := c.Get("role")
	uname, _ := username.(string)
	user, err := h.store.GetUserByUsername(uname)
	mustChangePwd := false
	if err == nil && user != nil {
		mustChangePwd = user.MustChangePwd
	}
	c.JSON(200, gin.H{"username": username, "role": role, "must_change_pwd": mustChangePwd})
}

func (h *Handler) changePassword(c *gin.Context) {
	var req struct {
		Old string `json:"old"`
		New string `json:"new"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid request"})
		return
	}
	if err := validatePasswordComplexity(req.New); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	username, _ := c.Get("username")
	uname, _ := username.(string)
	user, err := h.store.GetUserByUsername(uname)
	if err != nil {
		c.JSON(401, gin.H{"error": "user not found"})
		return
	}
	if !user.MustChangePwd {
		if !auth.CheckPassword(user.PasswordHash, req.Old) {
			c.JSON(401, gin.H{"error": "当前密码错误"})
			return
		}
	}
	newHash := auth.HashPassword(req.New)
	if err := h.store.UpdatePassword(uname, newHash); err != nil {
		c.JSON(500, gin.H{"error": "更新失败"})
		return
	}
	h.auditLog(c, "change_password", "user "+uname, "success")
	c.JSON(200, gin.H{"status": "ok"})
}

func validatePasswordComplexity(pwd string) error {
	if len(pwd) < 8 {
		return fmt.Errorf("密码长度至少8位")
	}
	if len(pwd) > 128 {
		return fmt.Errorf("密码长度不能超过128位")
	}
	var hasUpper, hasLower, hasDigit, hasSpecial bool
	for _, ch := range pwd {
		switch {
		case unicode.IsUpper(ch):
			hasUpper = true
		case unicode.IsLower(ch):
			hasLower = true
		case unicode.IsDigit(ch):
			hasDigit = true
		case unicode.IsPunct(ch) || unicode.IsSymbol(ch):
			hasSpecial = true
		}
	}
	var missing []string
	if !hasUpper {
		missing = append(missing, "大写字母")
	}
	if !hasLower {
		missing = append(missing, "小写字母")
	}
	if !hasDigit {
		missing = append(missing, "数字")
	}
	if !hasSpecial {
		missing = append(missing, "特殊符号")
	}
	if len(missing) > 0 {
		return fmt.Errorf("密码必须包含：%s", strings.Join(missing, "、"))
	}
	return nil
}

func (h *Handler) authLogout(c *gin.Context) {
	c.JSON(200, gin.H{"status": "ok"})
}

func (h *Handler) changeUsername(c *gin.Context) {
	var req struct {
		NewUsername string `json:"new_username"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid request"})
		return
	}
	if len(req.NewUsername) < 2 || len(req.NewUsername) > 32 {
		c.JSON(400, gin.H{"error": "用户名长度需在2-32位之间"})
		return
	}
	username, _ := c.Get("username")
	uname, _ := username.(string)
	if err := h.store.UpdateUsername(uname, req.NewUsername); err != nil {
		c.JSON(500, gin.H{"error": "修改失败，用户名可能已存在"})
		return
	}
	c.JSON(200, gin.H{"status": "ok", "username": req.NewUsername})
}

func (h *Handler) refreshToken(c *gin.Context) {
	refreshToken, err := c.Cookie("refresh_token")
	if err != nil || refreshToken == "" {
		c.JSON(400, gin.H{"error": "refresh_token is required"})
		return
	}
	claims, err := auth.ValidateRefreshToken(refreshToken)
	if err != nil {
		c.JSON(401, gin.H{"error": "invalid or expired refresh token"})
		return
	}
	userID, _ := claims["user_id"].(float64)
	username, _ := claims["username"].(string)
	role, _ := claims["role"].(string)
	tokenPair, err := auth.GenerateTokenPair(int(userID), username, role)
	if err != nil {
		c.JSON(500, gin.H{"error": "failed to generate token"})
		return
	}
	secure := os.Getenv("GIN_MODE") == "release"
	c.SetCookie("refresh_token", tokenPair.RefreshToken, 7*86400, "/", "", secure, true)
	c.JSON(200, gin.H{
		"access_token": tokenPair.AccessToken,
		"expires_in":   tokenPair.ExpiresIn,
		"token_type":   "Bearer",
	})
}

// ── User Management (RBAC) ────────────────────────────────────

func (h *Handler) listUsers(c *gin.Context) {
	users := h.store.ListUsers()
	c.JSON(200, users)
}

func (h *Handler) createUser(c *gin.Context) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Role     string `json:"role"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	if req.Username == "" || req.Password == "" {
		c.JSON(400, gin.H{"error": "username and password are required"})
		return
	}
	if err := validatePasswordComplexity(req.Password); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	validRoles := map[string]bool{"admin": true, "operator": true, "viewer": true}
	if !validRoles[req.Role] {
		req.Role = "viewer"
	}
	hash := auth.HashPassword(req.Password)
	if err := h.store.CreateUser(req.Username, hash, req.Role); err != nil {
		c.JSON(500, gin.H{"error": "创建用户失败，用户名可能已存在"})
		return
	}
	h.auditLog(c, "create_user", "user "+req.Username+" role="+req.Role, "success")
	c.JSON(200, gin.H{"status": "ok"})
}

func (h *Handler) deleteUser(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid user id"})
		return
	}
	username, _ := c.Get("username")
	if u, err := h.store.GetUserByID(id); err == nil && u != nil && u.Username == username {
		c.JSON(400, gin.H{"error": "不能删除自己"})
		return
	}
	if err := h.store.DeleteUser(id); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	h.auditLog(c, "delete_user", fmt.Sprintf("user_id=%d", id), "success")
	c.JSON(200, gin.H{"status": "ok"})
}

// ── Snapshot / Metrics ────────────────────────────────────────

func (h *Handler) getSnapshot(c *gin.Context) {
	snap, err := h.collector.Collect()
	if err != nil {
		c.JSON(500, gin.H{"error": "collection failed"})
		return
	}
	snap.NodeID = "self"
	h.persistAndCache(snap)
	c.JSON(200, snap)
}

func (h *Handler) getLatest(c *gin.Context) {
	h.mu.RLock()
	cs := h.cachedSnapshot
	h.mu.RUnlock()
	if cs != nil {
		c.JSON(200, cs)
		return
	}
	snap, err := h.collector.Collect()
	if err != nil {
		c.JSON(500, gin.H{"error": "collection failed"})
		return
	}
	snap.NodeID = "self"
	h.persistAndCache(snap)
	c.JSON(200, snap)
}

func (h *Handler) persistAndCache(snap *model.Snapshot) {
	if h.store != nil {
		h.store.SaveSnapshot(snap.NodeID, snap)
		h.checkThresholds(snap)
	}
	h.mu.Lock()
	h.cachedSnapshot = snap
	h.mu.Unlock()
}

func (h *Handler) UpdateCache(snap *model.Snapshot) {
	h.mu.Lock()
	h.cachedSnapshot = snap
	h.mu.Unlock()
}

func (h *Handler) checkThresholds(snap *model.Snapshot) {
	rules := h.store.ListAlertRules()
	hostName := "self"
	if snap.Host.Hostname != "" {
		hostName = snap.Host.Hostname
	}
	for _, r := range rules {
		enabled, _ := r["enabled"].(bool)
		if !enabled {
			continue
		}
		metric, _ := r["metric"].(string)
		op, _ := r["op"].(string)
		threshold, _ := r["threshold"].(float64)
		level, _ := r["level"].(string)
		var value float64
		switch metric {
		case "cpu":
			value = snap.CPU.UsagePercent
		case "mem":
			value = snap.Memory.UsagePercent
		case "disk":
			value = snap.Disk.UsagePercent
		case "load":
			value = snap.Load.Load1
		}
		if h.evaluateCondition(value, op, threshold) {
			h.store.SaveAlert(map[string]interface{}{
				"node_id":   snap.NodeID,
				"node_name": hostName,
				"metric":    metric,
				"message":   fmt.Sprintf("%s %.1f%% %s %.0f%%", metric, value, op, threshold),
				"level":     level,
				"value":     value,
				"threshold": threshold,
				"status":    "firing",
			})
		}
	}
}

func (h *Handler) evaluateCondition(value float64, op string, threshold float64) bool {
	switch op {
	case ">":
		return value > threshold
	case ">=":
		return value >= threshold
	case "<":
		return value < threshold
	case "<=":
		return value <= threshold
	}
	return false
}

func (h *Handler) getHistory(c *gin.Context) {
	if h.store == nil {
		c.JSON(200, []interface{}{})
		return
	}
	limit := 200
	if l := c.Query("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 && v <= 500 {
			limit = v
		}
	}
	duration := c.Query("duration")
	data := h.store.ListSnapshotsWithDuration("", limit, duration)
	if data == nil {
		data = []map[string]any{}
	}
	c.JSON(200, data)
}

func (h *Handler) monitorStream(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()

	go func() {
		defer wg.Done()
		for range ticker.C {
			conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			h.mu.RLock()
			cs := h.cachedSnapshot
			h.mu.RUnlock()
			if cs == nil {
				continue
			}
			if err := conn.WriteJSON(cs); err != nil {
				return
			}
		}
	}()

	wg.Wait()
}

// ── Cron Jobs ─────────────────────────────────────────────────

func (h *Handler) listCronJobs(c *gin.Context) {
	if h.store != nil {
		c.JSON(200, h.store.ListCronJobs("self"))
		return
	}
	c.JSON(200, []interface{}{})
}

func (h *Handler) createCronJob(c *gin.Context) {
	var job map[string]interface{}
	if err := c.ShouldBindJSON(&job); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	if name, ok := job["name"].(string); !ok || strings.TrimSpace(name) == "" {
		c.JSON(400, gin.H{"error": "name is required"})
		return
	}
	if expr, ok := job["expression"].(string); !ok || strings.TrimSpace(expr) == "" {
		c.JSON(400, gin.H{"error": "expression is required"})
		return
	}
	if cmd, ok := job["command"].(string); !ok || strings.TrimSpace(cmd) == "" {
		c.JSON(400, gin.H{"error": "command is required"})
		return
	}
	job["node_id"] = "self"
	if _, ok := job["enabled"]; !ok {
		job["enabled"] = true
	}
	if _, ok := job["type"]; !ok {
		job["type"] = "shell"
	}
	jobID, err := h.store.SaveCronJob(job)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	h.auditLog(c, "create_cronjob", fmt.Sprintf("job_id=%d name=%s", jobID, job["name"]), "success")
	c.JSON(200, gin.H{"status": "ok", "id": jobID})
}

func (h *Handler) updateCronJob(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid job id"})
		return
	}
	var job map[string]interface{}
	if err := c.ShouldBindJSON(&job); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	job["id"] = float64(id)
	job["node_id"] = "self"
	if _, err := h.store.SaveCronJob(job); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	h.auditLog(c, "update_cronjob", fmt.Sprintf("job_id=%d", id), "success")
	c.JSON(200, gin.H{"status": "ok"})
}

func (h *Handler) deleteCronJob(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid job id"})
		return
	}
	if err := h.store.DeleteCronJob(id); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	h.auditLog(c, "delete_cronjob", fmt.Sprintf("job_id=%d", id), "success")
	c.JSON(200, gin.H{"status": "ok"})
}

func (h *Handler) runCronJob(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid job id"})
		return
	}
	jobs := h.store.ListCronJobs("self")
	var targetJob map[string]any
	for _, j := range jobs {
		if jid, ok := j["id"].(float64); ok && int(jid) == id {
			targetJob = j
			break
		}
	}
	if targetJob == nil {
		c.JSON(404, gin.H{"error": "job not found"})
		return
	}
	cmd, _ := targetJob["command"].(string)
	result := h.executeCommand(cmd, 60*time.Second)
	h.store.SaveCronJobLog(id, cmd, result.Output, result.ExitCode, result.DurationMs)
	h.auditLog(c, "run_cronjob", fmt.Sprintf("job_id=%d", id), "success")
	c.JSON(200, gin.H{"status": "ok", "result": result})
}

func (h *Handler) listCronJobLogs(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid job id"})
		return
	}
	logs := h.store.ListCronJobLogs(id, 100)
	c.JSON(200, logs)
}

func (h *Handler) listAllCronJobLogs(c *gin.Context) {
	keyword := c.Query("keyword")
	startTime := c.Query("start")
	endTime := c.Query("end")
	limit := 200
	if l := c.Query("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 && v <= 500 {
			limit = v
		}
	}
	logs := h.store.SearchCronJobLogs(keyword, startTime, endTime, limit)
	c.JSON(200, logs)
}

// ── File Manager ──────────────────────────────────────────────

func isPathSafe(p string) bool {
	if strings.ContainsRune(p, 0) {
		return false
	}
	if strings.TrimSpace(p) == "" && p != "" {
		return false
	}
	if runtime.GOOS != "windows" {
		if len(p) >= 2 && p[1] == ':' && (p[0] >= 'A' && p[0] <= 'Z' || p[0] >= 'a' && p[0] <= 'z') {
			return false
		}
		if strings.ContainsRune(p, '\\') {
			return false
		}
	}
	cleaned := filepath.Clean(p)
	if strings.Contains(cleaned, "..") {
		return false
	}
	return true
}

func (h *Handler) listFiles(c *gin.Context) {
	path := c.Query("path")
	if path == "" {
		path = filemgr.GetDefaultRoot()
	}
	if !isPathSafe(path) {
		c.JSON(400, gin.H{"error": "invalid path: path traversal detected"})
		return
	}
	ensureAllowedDirs(path)
	entries, err := filemgr.ListDir(path)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	var result []map[string]interface{}
	for _, e := range entries {
		item := map[string]interface{}{
			"name":  e.Name,
			"size":  e.Size,
			"mode":  e.Mode,
			"mtime": e.Modified,
			"type":  "file",
			"path":  filepath.Join(path, e.Name),
			"perm":  e.Perm,
		}
		if e.IsDir {
			item["type"] = "dir"
		}
		result = append(result, item)
	}
	c.JSON(200, result)
}

func ensureAllowedDirs(path string) {
	absPath := filepath.Clean(path)
	if !filepath.IsAbs(absPath) {
		absPath, _ = filepath.Abs(absPath)
	}
	allowedDirs := []string{absPath, filemgr.GetDefaultRoot(), filemgr.GetHomeDir()}
	if runtime.GOOS == "windows" {
		allowedDirs = append(allowedDirs, filemgr.GetDriveLetters()...)
	} else {
		allowedDirs = append(allowedDirs, "/", "/tmp", "/home", "/var", "/opt")
	}
	filemgr.InitAllowedDirs(allowedDirs)
}

func (h *Handler) getFileStats(c *gin.Context) {
	if h.store == nil {
		c.JSON(200, []interface{}{})
		return
	}
	duration := c.Query("duration")
	hours := 168
	if d := parseDuration(duration); d > 0 {
		hours = d
	}
	c.JSON(200, h.store.GetFileStats("self", hours))
}

func (h *Handler) createDir(c *gin.Context) {
	var req struct {
		Path string `json:"path"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	if !isPathSafe(req.Path) {
		c.JSON(400, gin.H{"error": "invalid path"})
		return
	}
	ensureAllowedDirs(req.Path)
	if err := filemgr.Mkdir(req.Path); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	h.auditLog(c, "mkdir", req.Path, "success")
	c.JSON(200, gin.H{"status": "ok"})
}

func (h *Handler) createFile(c *gin.Context) {
	var req struct {
		Path string `json:"path"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	if !isPathSafe(req.Path) {
		c.JSON(400, gin.H{"error": "invalid path"})
		return
	}
	ensureAllowedDirs(req.Path)
	if err := filemgr.WriteFile(req.Path, []byte{}); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	h.auditLog(c, "create_file", req.Path, "success")
	c.JSON(200, gin.H{"status": "ok"})
}

func (h *Handler) readFile(c *gin.Context) {
	path := c.Query("path")
	if path == "" {
		c.JSON(400, gin.H{"error": "path is required"})
		return
	}
	if !isPathSafe(path) {
		c.JSON(400, gin.H{"error": "invalid path"})
		return
	}
	data, err := filemgr.ReadFile(path)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	const maxPreviewSize = 2 * 1024 * 1024
	if len(data) > maxPreviewSize {
		data = data[:maxPreviewSize]
		c.Header("X-Truncated", "true")
	}
	c.Data(200, "text/plain; charset=utf-8", data)
}

func (h *Handler) writeFile(c *gin.Context) {
	var req struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	if !isPathSafe(req.Path) {
		c.JSON(400, gin.H{"error": "invalid path"})
		return
	}
	ensureAllowedDirs(req.Path)
	if err := filemgr.WriteFile(req.Path, []byte(req.Content)); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	h.auditLog(c, "write_file", req.Path, "success")
	c.JSON(200, gin.H{"status": "ok"})
}

func (h *Handler) chmodFile(c *gin.Context) {
	var req struct {
		Path string `json:"path"`
		Mode string `json:"mode"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	if !isPathSafe(req.Path) {
		c.JSON(400, gin.H{"error": "invalid path"})
		return
	}
	if runtime.GOOS == "windows" {
		c.JSON(400, gin.H{"error": "chmod not supported on Windows"})
		return
	}
	mode, err := strconv.ParseUint(req.Mode, 8, 32)
	if err != nil || mode < 1 || mode > 07777 {
		c.JSON(400, gin.H{"error": "invalid mode, use octal notation (e.g. 755, 644)"})
		return
	}
	if err := os.Chmod(req.Path, os.FileMode(mode)); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	h.auditLog(c, "chmod", fmt.Sprintf("%s -> %s", req.Path, req.Mode), "success")
	c.JSON(200, gin.H{"status": "ok"})
}

func (h *Handler) removeFile(c *gin.Context) {
	var req struct {
		Path string `json:"path"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	if !isPathSafe(req.Path) {
		c.JSON(400, gin.H{"error": "invalid path"})
		return
	}
	ensureAllowedDirs(req.Path)
	if err := filemgr.Delete(req.Path); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	h.auditLog(c, "delete_file", req.Path, "success")
	c.JSON(200, gin.H{"status": "ok"})
}

func (h *Handler) uploadFile(c *gin.Context) {
	path := c.PostForm("path")
	if path == "" {
		path = c.Query("path")
	}
	if path == "" {
		path = "./"
	}
	if !isPathSafe(path) {
		c.JSON(400, gin.H{"error": "invalid path"})
		return
	}
	form, err := c.MultipartForm()
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	files := form.File["files"]
	if len(files) == 0 {
		c.JSON(400, gin.H{"error": "no files uploaded"})
		return
	}
	if len(files) > 10 {
		c.JSON(400, gin.H{"error": "too many files (max 10)"})
		return
	}
	const maxFileSize = 50 * 1024 * 1024
	for _, f := range files {
		if f.Size > maxFileSize {
			c.JSON(400, gin.H{"error": fmt.Sprintf("file %s exceeds 50MB limit", f.Filename)})
			return
		}
	}
	for _, f := range files {
		safeFilename := filemgr.SanitizeFileName(f.Filename)
		if safeFilename == "" {
			continue
		}
		src, err := f.Open()
		if err != nil {
			continue
		}
		data, err := io.ReadAll(io.LimitReader(src, maxFileSize+1))
		src.Close()
		if err != nil {
			continue
		}
		destPath := filepath.Join(path, safeFilename)
		filemgr.Upload(destPath, data)
	}
	h.auditLog(c, "upload_files", path, "success")
	c.JSON(200, gin.H{"status": "ok", "count": len(files)})
}

func (h *Handler) downloadFile(c *gin.Context) {
	path := c.Query("path")
	if path == "" {
		c.JSON(400, gin.H{"error": "path is required"})
		return
	}
	if !isPathSafe(path) {
		c.JSON(400, gin.H{"error": "invalid path"})
		return
	}
	data, err := filemgr.Download(path)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	name := filepath.Base(path)
	safeName := strings.NewReplacer("\"", "", "\\", "", "\n", "", "\r", "").Replace(name)
	c.Header("Content-Disposition", `attachment; filename="`+safeName+`"`)
	c.Data(200, "application/octet-stream", data)
}

// ── Script Management ─────────────────────────────────────────

type Script struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Interpreter string `json:"interpreter"`
	Description string `json:"description"`
	Content     string `json:"content"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

type ExecResult struct {
	ExitCode   int    `json:"exitCode"`
	Output     string `json:"output"`
	Error      string `json:"error"`
	DurationMs int64  `json:"durationMs"`
	StartTime  string `json:"startTime"`
	EndTime    string `json:"endTime"`
}

func (h *Handler) listScripts(c *gin.Context) {
	scripts := h.store.ListScripts()
	c.JSON(200, scripts)
}

func (h *Handler) getScript(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid script id"})
		return
	}
	script, err := h.store.GetScript(id)
	if err != nil {
		c.JSON(404, gin.H{"error": "script not found"})
		return
	}
	c.JSON(200, script)
}

func (h *Handler) createScript(c *gin.Context) {
	var req struct {
		Name        string `json:"name"`
		Interpreter string `json:"interpreter"`
		Description string `json:"description"`
		Content     string `json:"content"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	if req.Name == "" || req.Content == "" {
		c.JSON(400, gin.H{"error": "name and content are required"})
		return
	}
	if req.Interpreter == "" {
		req.Interpreter = "/bin/bash"
	}
	if !isAllowedInterpreter(req.Interpreter) {
		c.JSON(400, gin.H{"error": "interpreter not allowed"})
		return
	}
	if warnings := checkScriptSecurity(req.Content); len(warnings) > 0 {
		c.JSON(400, gin.H{"error": "security check failed", "warnings": warnings})
		return
	}
	id, err := h.store.SaveScript(map[string]interface{}{
		"name": req.Name, "interpreter": req.Interpreter,
		"description": req.Description, "content": req.Content,
	})
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	h.auditLog(c, "create_script", fmt.Sprintf("id=%d name=%s", id, req.Name), "success")
	c.JSON(200, gin.H{"status": "ok", "id": id})
}

func (h *Handler) updateScript(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid script id"})
		return
	}
	var req struct {
		Name        string `json:"name"`
		Interpreter string `json:"interpreter"`
		Description string `json:"description"`
		Content     string `json:"content"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	if req.Interpreter != "" && !isAllowedInterpreter(req.Interpreter) {
		c.JSON(400, gin.H{"error": "interpreter not allowed"})
		return
	}
	if req.Content != "" {
		if warnings := checkScriptSecurity(req.Content); len(warnings) > 0 {
			c.JSON(400, gin.H{"error": "security check failed", "warnings": warnings})
			return
		}
	}
	_, err = h.store.SaveScript(map[string]interface{}{
		"id": float64(id), "name": req.Name, "interpreter": req.Interpreter,
		"description": req.Description, "content": req.Content,
	})
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	h.auditLog(c, "update_script", fmt.Sprintf("id=%d", id), "success")
	c.JSON(200, gin.H{"status": "ok"})
}

func (h *Handler) deleteScript(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid script id"})
		return
	}
	if err := h.store.DeleteScript(id); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	h.auditLog(c, "delete_script", fmt.Sprintf("id=%d", id), "success")
	c.JSON(200, gin.H{"status": "ok"})
}

func (h *Handler) runScript(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid script id"})
		return
	}
	scriptData, err := h.store.GetScript(id)
	if err != nil {
		c.JSON(404, gin.H{"error": "script not found"})
		return
	}
	content, _ := scriptData["content"].(string)
	interpreter := "/bin/bash"
	if v, ok := scriptData["interpreter"].(string); ok && v != "" {
		interpreter = v
	}
	result := h.executeScript(interpreter, content, 120*time.Second)
	h.auditLog(c, "run_script", fmt.Sprintf("id=%d exitCode=%d", id, result.ExitCode), "success")
	c.JSON(200, result)
}

func (h *Handler) checkScriptSyntax(c *gin.Context) {
	var req struct {
		Content     string `json:"content"`
		Interpreter string `json:"interpreter"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	if req.Interpreter == "" {
		req.Interpreter = "/bin/bash"
	}
	warnings := checkScriptSecurity(req.Content)
	syntaxOk := true
	syntaxMsg := ""
	if strings.Contains(req.Interpreter, "bash") || strings.Contains(req.Interpreter, "sh") {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, req.Interpreter, "-n", "-c", req.Content)
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			syntaxOk = false
			syntaxMsg = strings.TrimSpace(stderr.String())
			if syntaxMsg == "" {
				syntaxMsg = err.Error()
			}
		}
	} else if strings.Contains(req.Interpreter, "python") {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		tmpFile, err := os.CreateTemp("", "syntax-check-*.py")
		if err == nil {
			tmpFile.WriteString(req.Content)
			tmpFile.Close()
			defer os.Remove(tmpFile.Name())
			cmd := exec.CommandContext(ctx, req.Interpreter, "-m", "py_compile", tmpFile.Name())
			var stderr bytes.Buffer
			cmd.Stderr = &stderr
			if err := cmd.Run(); err != nil {
				syntaxOk = false
				syntaxMsg = strings.TrimSpace(stderr.String())
			}
		}
	}
	c.JSON(200, gin.H{"syntax_ok": syntaxOk, "syntax_message": syntaxMsg, "security_warnings": warnings})
}

func isAllowedInterpreter(interp string) bool {
	allowed := []string{
		"/bin/bash", "/bin/sh", "/bin/dash", "/bin/zsh",
		"/usr/bin/bash", "/usr/bin/sh", "/usr/bin/python3", "/usr/bin/python",
		"/usr/bin/perl", "/usr/bin/ruby", "/usr/bin/node",
	}
	for _, a := range allowed {
		if interp == a {
			return true
		}
	}
	return false
}

func checkScriptSecurity(content string) []string {
	var warnings []string
	dangerousPatterns := []struct {
		pattern string
		message string
	}{
		{"rm -rf /", "dangerous: rm -rf / detected"},
		{":(){ :|:& };:", "dangerous: fork bomb detected"},
		{"mkfs.", "dangerous: filesystem format command detected"},
		{"dd if=", "warning: dd command detected, verify intent"},
		{"> /dev/sd", "dangerous: direct disk write detected"},
		{"chmod -R 777 /", "dangerous: recursive world-writable permission detected"},
		{"curl.*|.*sh", "warning: piping curl to shell detected"},
		{"wget.*|.*sh", "warning: piping wget to shell detected"},
	}
	lower := strings.ToLower(content)
	for _, p := range dangerousPatterns {
		if strings.Contains(lower, p.pattern) {
			warnings = append(warnings, p.message)
		}
	}
	return warnings
}

func (h *Handler) executeScript(interpreter, content string, timeout time.Duration) *ExecResult {
	start := time.Now()
	result := &ExecResult{
		StartTime: start.Format("2006-01-02 15:04:05"),
	}
	tmpFile, err := os.CreateTemp("", "devdash-script-*")
	if err != nil {
		result.Error = fmt.Sprintf("create temp file: %v", err)
		result.ExitCode = -1
		result.EndTime = time.Now().Format("2006-01-02 15:04:05")
		return result
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.WriteString(content)
	tmpFile.Close()
	os.Chmod(tmpFile.Name(), 0755)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, interpreter, tmpFile.Name())
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = cmd.Run()
	end := time.Now()
	result.EndTime = end.Format("2006-01-02 15:04:05")
	result.DurationMs = end.Sub(start).Milliseconds()
	result.Output = stdout.String()
	result.Error = stderr.String()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else if ctx.Err() == context.DeadlineExceeded {
			result.ExitCode = -1
			result.Error = "execution timeout"
		} else {
			result.ExitCode = -1
			if result.Error == "" {
				result.Error = err.Error()
			}
		}
	}
	return result
}

func (h *Handler) executeCommand(cmdStr string, timeout time.Duration) *ExecResult {
	start := time.Now()
	result := &ExecResult{
		StartTime: start.Format("2006-01-02 15:04:05"),
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "cmd", "/c", cmdStr)
	} else {
		cmd = exec.CommandContext(ctx, "/bin/sh", "-c", cmdStr)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	end := time.Now()
	result.EndTime = end.Format("2006-01-02 15:04:05")
	result.DurationMs = end.Sub(start).Milliseconds()
	result.Output = stdout.String()
	result.Error = stderr.String()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else {
			result.ExitCode = -1
		}
	}
	return result
}

// ── Terminal ──────────────────────────────────────────────────

func (h *Handler) terminalWS(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("[terminal] websocket upgrade failed: %v", err)
		return
	}
	nodeID := c.Param("nodeID")
	if nodeID == "" {
		nodeID = "self"
	}
	shell := c.Query("shell")
	session := terminal.NewSession(nodeID, shell, conn)
	session.CommandSaver = func(command string) {
		if h.store != nil {
			h.store.SaveCommandHistory(command)
		}
	}
	session.Handle()
}

func (h *Handler) listShells(c *gin.Context) {
	shells := terminal.AvailableShells()
	c.JSON(200, shells)
}

func (h *Handler) getCommandHistory(c *gin.Context) {
	limit := 100
	if l := c.Query("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 && v <= 500 {
			limit = v
		}
	}
	history := h.store.GetCommandHistory(limit)
	c.JSON(200, history)
}

func (h *Handler) clearCommandHistory(c *gin.Context) {
	h.store.ClearCommandHistory()
	h.auditLog(c, "clear_history", "command history", "success")
	c.JSON(200, gin.H{"status": "ok"})
}

// ── Alerts ────────────────────────────────────────────────────

func (h *Handler) listAlerts(c *gin.Context) {
	if h.store == nil {
		c.JSON(200, []interface{}{})
		return
	}
	c.JSON(200, h.store.ListAlerts("", 100))
}

func (h *Handler) listActiveAlerts(c *gin.Context) {
	if h.store == nil {
		c.JSON(200, []interface{}{})
		return
	}
	c.JSON(200, h.store.ListActiveAlerts(""))
}

func (h *Handler) listAlertHistory(c *gin.Context) {
	if h.store == nil {
		c.JSON(200, []interface{}{})
		return
	}
	c.JSON(200, h.store.ListAlerts("", 200))
}

func (h *Handler) silenceAlert(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid alert id"})
		return
	}
	if err := h.store.SilenceAlert(id); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"status": "ok"})
}

func (h *Handler) listAlertRules(c *gin.Context) {
	if h.store == nil {
		c.JSON(200, []interface{}{})
		return
	}
	c.JSON(200, h.store.ListAlertRules())
}

func (h *Handler) createAlertRule(c *gin.Context) {
	var rule map[string]interface{}
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	if err := validateAlertRule(rule); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	if err := h.store.SaveAlertRule(rule); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	h.auditLog(c, "create_alert_rule", fmt.Sprintf("metric=%s", rule["metric"]), "success")
	c.JSON(200, gin.H{"status": "ok", "rule": rule})
}

func (h *Handler) updateAlertRule(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid rule id"})
		return
	}
	var rule map[string]interface{}
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	if err := validateAlertRule(rule); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	rule["id"] = float64(id)
	if err := h.store.SaveAlertRule(rule); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"status": "ok"})
}

func validateAlertRule(rule map[string]interface{}) error {
	validMetrics := map[string]bool{"cpu": true, "mem": true, "disk": true, "load": true}
	validOps := map[string]bool{">": true, ">=": true, "<": true, "<=": true}
	validLevels := map[string]bool{"info": true, "warning": true, "critical": true}
	metric, _ := rule["metric"].(string)
	if !validMetrics[metric] {
		return fmt.Errorf("metric must be one of: cpu, mem, disk, load")
	}
	op, _ := rule["op"].(string)
	if !validOps[op] {
		return fmt.Errorf("op must be one of: >, >=, <, <=")
	}
	threshold, _ := rule["threshold"].(float64)
	if threshold <= 0 {
		return fmt.Errorf("threshold must be a positive number")
	}
	level, _ := rule["level"].(string)
	if level != "" && !validLevels[level] {
		return fmt.Errorf("level must be one of: info, warning, critical")
	}
	return nil
}

func (h *Handler) deleteAlertRule(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid rule id"})
		return
	}
	if err := h.store.DeleteAlertRule(id); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"status": "ok"})
}

// ── Alert Notification Config ─────────────────────────────────

func (h *Handler) getAlertNotifyConfig(c *gin.Context) {
	if h.alertEngine == nil {
		c.JSON(200, alert.AlertConfig{Browser: true})
		return
	}
	c.JSON(200, h.alertEngine.GetConfig())
}

func (h *Handler) updateAlertNotifyConfig(c *gin.Context) {
	if h.alertEngine == nil {
		c.JSON(500, gin.H{"error": "alert engine not initialized"})
		return
	}
	var cfg alert.AlertConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		c.JSON(400, gin.H{"error": "invalid request"})
		return
	}
	h.alertEngine.SetConfig(cfg)
	h.auditLog(c, "update_alert_notify_config", "alert notification config updated", "success")
	c.JSON(200, gin.H{"status": "ok", "config": cfg})
}

func (h *Handler) testAlertNotify(c *gin.Context) {
	if h.alertEngine == nil {
		c.JSON(500, gin.H{"error": "alert engine not initialized"})
		return
	}
	testAlert := map[string]interface{}{
		"node_id":   "test",
		"node_name": "测试主机",
		"metric":    "test",
		"level":     "warning",
		"message":   "这是一条测试告警消息 - 验证通知渠道是否正常工作",
		"value":     99.9,
		"threshold": 90.0,
		"time":      time.Now(),
		"status":    "firing",
	}
	go h.alertEngine.SendNotifications(testAlert)
	h.auditLog(c, "test_alert_notify", "sent test alert notification", "success")
	c.JSON(200, gin.H{"status": "ok", "message": "测试告警已发送，请检查各通知渠道"})
}

// ── Audit Logs ────────────────────────────────────────────────

func (h *Handler) listAuditLogs(c *gin.Context) {
	limit := 100
	if l := c.Query("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 && v <= 500 {
			limit = v
		}
	}
	logs := h.store.ListAuditLogs(limit)
	c.JSON(200, logs)
}

// ── Trigger Collect ───────────────────────────────────────────

func (h *Handler) triggerCollect(c *gin.Context) {
	snap, err := h.collector.Collect()
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	snap.NodeID = "self"
	h.persistAndCache(snap)
	c.JSON(200, gin.H{"status": "ok"})
}

// ── Collect Interval Settings ────────────────────────────────

func (h *Handler) getCollectInterval(c *gin.Context) {
	h.mu.RLock()
	interval := h.cfg.CollectInterval
	h.mu.RUnlock()
	if interval < 3 {
		interval = 5
	}
	c.JSON(200, gin.H{"interval_seconds": interval})
}

func (h *Handler) updateCollectInterval(c *gin.Context) {
	var req struct {
		IntervalSeconds int `json:"interval_seconds"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid request"})
		return
	}
	if req.IntervalSeconds < 3 {
		c.JSON(400, gin.H{"error": "minimum interval is 3 seconds"})
		return
	}
	if req.IntervalSeconds > 300 {
		c.JSON(400, gin.H{"error": "maximum interval is 300 seconds"})
		return
	}
	h.mu.Lock()
	h.cfg.CollectInterval = req.IntervalSeconds
	h.mu.Unlock()
	log.Printf("[settings] collect interval updated to %ds", req.IntervalSeconds)
	c.JSON(200, gin.H{"status": "ok", "interval_seconds": req.IntervalSeconds})
}

// ── Health ────────────────────────────────────────────────────

func (h *Handler) healthCheck(c *gin.Context) {
	if err := h.store.Ping(); err != nil {
		c.JSON(503, gin.H{"status": "unhealthy", "error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"status": "healthy", "timestamp": time.Now().Format(time.RFC3339)})
}

func (h *Handler) readinessCheck(c *gin.Context) {
	if err := h.store.Ping(); err != nil {
		c.JSON(503, gin.H{"status": "not_ready", "error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"status": "ready", "timestamp": time.Now().Format(time.RFC3339)})
}

// ── Trend Compare ─────────────────────────────────────────────

func (h *Handler) getTrendCompare(c *gin.Context) {
	if h.store == nil {
		c.JSON(200, gin.H{"current": []interface{}{}, "previous": []interface{}{}, "summary": gin.H{}})
		return
	}
	metric := c.Query("metric")
	if metric == "" {
		metric = "cpu"
	}
	period := c.Query("period")
	if period == "" {
		period = "7d"
	}
	var duration time.Duration
	switch period {
	case "1h":
		duration = 1 * time.Hour
	case "6h":
		duration = 6 * time.Hour
	case "1d":
		duration = 24 * time.Hour
	case "7d":
		duration = 168 * time.Hour
	case "30d":
		duration = 720 * time.Hour
	default:
		duration = 168 * time.Hour
	}
	now := time.Now()
	currentStart := now.Add(-duration)
	previousStart := now.Add(-2 * duration)
	currentData, _ := h.store.GetMetricsHistoryRange("self", currentStart, now)
	previousData, _ := h.store.GetMetricsHistoryRange("self", previousStart, currentStart)
	if currentData == nil {
		currentData = []map[string]any{}
	}
	if previousData == nil {
		previousData = []map[string]any{}
	}
	c.JSON(200, gin.H{
		"current":  currentData,
		"previous": previousData,
		"metric":   metric,
		"period":   period,
	})
}

// ── Anomaly Detection ─────────────────────────────────────────

type AnomalyResult struct {
	Metric    string  `json:"metric"`
	Value     float64 `json:"value"`
	Mean      float64 `json:"mean"`
	Std       float64 `json:"std"`
	UpperBand float64 `json:"upper_band"`
	LowerBand float64 `json:"lower_band"`
	IsAnomaly bool    `json:"is_anomaly"`
	Severity  string  `json:"severity"`
	Time      string  `json:"time"`
}

func (h *Handler) detectAnomalies(c *gin.Context) {
	if h.store == nil {
		c.JSON(200, gin.H{"anomalies": []AnomalyResult{}, "summary": gin.H{"total": 0, "anomalies": 0}})
		return
	}
	sigma := 2.0
	if s, err := strconv.ParseFloat(c.Query("sigma"), 64); err == nil && s > 0 && s <= 5 {
		sigma = s
	}
	hours := 24
	if v, err := strconv.Atoi(c.Query("hours")); err == nil && v > 0 && v <= 720 {
		hours = v
	}
	data, err := h.store.GetMetricsHistory("self", hours)
	if err != nil || len(data) == 0 {
		c.JSON(200, gin.H{"anomalies": []AnomalyResult{}, "summary": gin.H{"total": 0, "anomalies": 0}})
		return
	}
	metrics := []struct {
		name    string
		extract func(map[string]any) float64
	}{
		{"cpu", func(d map[string]any) float64 {
			if v, ok := d["cpu_usage"].(float64); ok {
				return v
			}
			return 0
		}},
		{"memory", func(d map[string]any) float64 {
			if v, ok := d["mem_usage_percent"].(float64); ok {
				return v
			}
			return 0
		}},
		{"disk", func(d map[string]any) float64 {
			if v, ok := d["disk_usage_percent"].(float64); ok {
				return v
			}
			return 0
		}},
	}
	var anomalies []AnomalyResult
	totalPoints := 0
	for _, m := range metrics {
		var values []float64
		for _, d := range data {
			v := m.extract(d)
			if v > 0 {
				values = append(values, v)
			}
		}
		if len(values) < 5 {
			continue
		}
		totalPoints += len(values)
		mean := 0.0
		for _, v := range values {
			mean += v
		}
		mean /= float64(len(values))
		variance := 0.0
		for _, v := range values {
			variance += (v - mean) * (v - mean)
		}
		variance /= float64(len(values))
		std := math.Sqrt(variance)
		upperBand := mean + sigma*std
		lowerBand := mean - sigma*std
		if lowerBand < 0 {
			lowerBand = 0
		}
		for _, d := range data {
			v := m.extract(d)
			if v <= 0 {
				continue
			}
			if v > upperBand || v < lowerBand {
				severity := "warning"
				deviation := 0.0
				if v > upperBand {
					deviation = (v - upperBand) / std
				} else {
					deviation = (lowerBand - v) / std
				}
				if deviation > 2 {
					severity = "critical"
				}
				ts, _ := d["timestamp"].(string)
				if ts == "" {
					ts = time.Now().Format(time.RFC3339)
				}
				anomalies = append(anomalies, AnomalyResult{
					Metric: m.name, Value: v, Mean: mean, Std: std,
					UpperBand: upperBand, LowerBand: lowerBand,
					IsAnomaly: true, Severity: severity, Time: ts,
				})
			}
		}
	}
	c.JSON(200, gin.H{
		"anomalies": anomalies,
		"summary": gin.H{
			"total":       totalPoints,
			"anomalies":   len(anomalies),
			"sigma":       sigma,
			"hours":       hours,
			"anomalyRate": fmt.Sprintf("%.2f%%", float64(len(anomalies))/float64(max(totalPoints, 1))*100),
		},
	})
}

// ── Helper ────────────────────────────────────────────────────

func (h *Handler) auditLog(c *gin.Context, action, detail, result string) {
	userID, _ := c.Get("user_id")
	uid, _ := userID.(float64)
	h.store.SaveAuditLog(&model.AuditLog{
		UserID: int(uid),
		NodeID: "self",
		Action: action,
		Detail: detail,
		Result: result,
	})
}

func parseDuration(d string) int {
	d = strings.TrimSpace(d)
	switch d {
	case "1h":
		return 1
	case "6h":
		return 6
	case "1d":
		return 24
	case "7d":
		return 168
	case "30d":
		return 720
	default:
		if h, err := strconv.Atoi(strings.TrimSuffix(d, "h")); err == nil && h > 0 && h <= 720 {
			return h
		}
		return 0
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
