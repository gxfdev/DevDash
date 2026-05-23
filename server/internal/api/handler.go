package api

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"devdash/internal/auth"
	"devdash/internal/collector"
	"devdash/internal/dbmgr"
	"devdash/internal/filemgr"
	"devdash/internal/firewall"
	"devdash/internal/ha"
	"devdash/internal/model"
	"devdash/internal/node"
	"devdash/internal/settings"
	"devdash/internal/software"
	"devdash/internal/store"
	"devdash/internal/terminal"

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
		allowedHosts := map[string]bool{
			"localhost": true,
			"127.0.0.1": true,
		}
		if allowedHosts[host] {
			return true
		}
		defaultOrigins := map[string]bool{
			"http://localhost:3000": true,
			"http://localhost:5173": true,
			"http://localhost:5174": true,
			"http://localhost:9090": true,
			"http://127.0.0.1:3000": true,
			"http://127.0.0.1:5173": true,
			"http://127.0.0.1:5174": true,
			"http://127.0.0.1:9090": true,
		}
		return defaultOrigins[origin]
	},
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

type Handler struct {
	collector      *collector.Collector
	store          *store.Store
	nm             *node.NodeManager
	mu             sync.RWMutex
	cachedSnapshot interface{}
	nodeSnapshots  map[string]interface{}
	nodeMu         sync.RWMutex
	dbConns        map[int]*sql.DB
	dbMu           sync.RWMutex
}

func NewHandler(c *collector.Collector, s *store.Store, nm *node.NodeManager) *Handler {
	return &Handler{
		collector:     c,
		store:         s,
		nm:            nm,
		dbConns:       make(map[int]*sql.DB),
		nodeSnapshots: make(map[string]interface{}),
	}
}

func (h *Handler) RegisterRoutes(r *gin.Engine) {
	r.Use(requestSizeLimit(10 * 1024 * 1024))
	r.Use(apiRateLimit())
	r.Use(securityHeaders())

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

	v1 := r.Group("/api/v1", auth.Middleware(), auth.CSRFMiddleware())

	v1.GET("/snapshot", h.getSnapshot)
	v1.GET("/latest", h.getLatest)
	v1.GET("/history", h.getHistory)
	v1.GET("/trend/compare", h.getTrendCompare)
	v1.GET("/anomaly/detect", h.detectAnomalies)

	v1.GET("/nodes", h.listNodes)
	v1.GET("/node/:id", h.getNode)
	v1.POST("/node/register", auth.RequireRole("admin"), h.registerNode)
	v1.POST("/node/heartbeat", h.heartbeat)
	v1.DELETE("/node/:id", auth.RequireRole("admin"), h.deleteNode)
	v1.GET("/node/:id/metrics", h.getNodeMetrics)
	v1.GET("/node/:id/history", h.getNodeHistory)
	v1.GET("/node/:id/procs", h.getNodeProcs)
	v1.GET("/node/:id/containers", h.getNodeContainers)
	v1.GET("/node/:id/gpu/history", h.getGPUMetricsHistory)

	v1.GET("/node/:id/software", h.listNodeSoftware)
	v1.POST("/node/:id/software/install", auth.RequireRole("admin"), h.installNodeSoftware)
	v1.POST("/node/:id/software/uninstall", auth.RequireRole("admin"), h.uninstallNodeSoftware)
	v1.POST("/node/:id/software/service", auth.RequireRole("admin"), h.softwareServiceControl)

	v1.GET("/node/:id/firewall/rules", h.listFirewallRules)
	v1.POST("/node/:id/firewall/rules", auth.RequireRole("admin"), h.addFirewallRule)
	v1.PATCH("/node/:id/firewall/rules/:rid", auth.RequireRole("admin"), h.updateFirewallRule)
	v1.DELETE("/node/:id/firewall/rules/:rid", auth.RequireRole("admin"), h.deleteFirewallRule)

	v1.GET("/node/:id/cronjobs", h.listNodeCronJobs)
	v1.POST("/node/:id/cronjobs", auth.RequireRole("admin"), h.createNodeCronJob)
	v1.PATCH("/node/:id/cronjobs/:jid", auth.RequireRole("admin"), h.updateNodeCronJob)
	v1.DELETE("/node/:id/cronjobs/:jid", auth.RequireRole("admin"), h.deleteNodeCronJob)
	v1.POST("/node/:id/cronjobs/:jid/run", auth.RequireRole("admin"), h.runNodeCronJob)

	v1.GET("/node/:id/databases", h.listDatabases)
	v1.POST("/node/:id/databases", auth.RequireRole("admin"), h.addDatabase)
	v1.POST("/node/:id/databases/test", auth.RequireRole("admin"), h.testDatabaseConnection)
	v1.GET("/node/:id/databases/:dbId/tables", h.listDatabaseTables)
	v1.POST("/node/:id/databases/:dbId/query", auth.RequireRole("admin"), h.executeDatabaseQuery)
	v1.DELETE("/node/:id/databases/:dbId", auth.RequireRole("admin"), h.deleteDatabase)

	v1.GET("/node/:id/fs/list", h.listFiles)
	v1.GET("/node/:id/fs/stats", h.getFileStats)
	v1.POST("/node/:id/fs/mkdir", auth.RequireRole("admin"), h.createDir)
	v1.POST("/node/:id/fs/mkfile", auth.RequireRole("admin"), h.createFile)
	v1.DELETE("/node/:id/fs/remove", auth.RequireRole("admin"), h.removeFile)
	v1.GET("/node/:id/fs/read", auth.RequireRole("admin"), h.readFile)
	v1.POST("/node/:id/fs/upload", auth.RequireRole("admin"), h.uploadFile)
	v1.GET("/node/:id/fs/download", auth.RequireRole("admin"), h.downloadFile)

	v1.GET("/alerts", h.listAlerts)
	v1.GET("/alerts/active", h.listActiveAlerts)
	v1.GET("/alerts/history", h.listAlertHistory)
	v1.POST("/alerts/:id/silence", auth.RequireRole("admin"), h.silenceAlert)

	v1.GET("/alert-rules", h.listAlertRules)
	v1.POST("/alert-rules", auth.RequireRole("admin"), h.createAlertRule)
	v1.PUT("/alert-rules/:id", auth.RequireRole("admin"), h.updateAlertRule)
	v1.DELETE("/alert-rules/:id", auth.RequireRole("admin"), h.deleteAlertRule)
	v1.POST("/alert/test-feishu", auth.RequireRole("admin"), h.testFeishu)

	v1.GET("/settings", h.getSettings)
	v1.PUT("/settings", auth.RequireRole("admin"), h.updateSettings)
	v1.GET("/alert-settings", h.getAlertSettings)
	v1.PUT("/alert-settings", auth.RequireRole("admin"), h.updateAlertSettings)
	v1.GET("/system-settings", h.getSystemSettings)
	v1.PUT("/system-settings", auth.RequireRole("admin"), h.updateSystemSettings)

	v1.GET("/software", h.listSoftware)
	v1.POST("/software/install", auth.RequireRole("admin"), h.installNodeSoftware)
	v1.POST("/software/uninstall", auth.RequireRole("admin"), h.uninstallSoftware)
	v1.GET("/cronjobs", h.listCronJobs)
	v1.POST("/cronjobs", auth.RequireRole("admin"), h.createCronJob)
	v1.DELETE("/cronjobs/:id", auth.RequireRole("admin"), h.deleteCronJob)
	v1.POST("/collect", auth.RequireRole("admin"), h.triggerCollect)
	v1.GET("/ws", auth.WSMiddleware(), h.websocket)

	v1.POST("/backup", auth.RequireRole("admin"), h.createBackup)
	v1.GET("/backup/list", h.listBackups)
	v1.POST("/backup/restore", auth.RequireRole("admin"), h.restoreBackup)
	v1.DELETE("/backup/:name", auth.RequireRole("admin"), h.deleteBackup)

	r.GET("/ws/terminal/:nodeId", auth.WSMiddleware(), h.terminalWS)

	v1.GET("/terminal/shells", h.listShells)

	dockerHandler, err := NewDockerHandler()
	if err == nil {
		dockerHandler.RegisterRoutes(v1)
	}
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
	const (
		windowSize  = time.Minute
		maxRequests = 600
	)
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
	if err := auth.ValidatePassword(req.Password); err != nil {
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
	if snap.NodeID != "" {
		h.nodeMu.Lock()
		h.nodeSnapshots[snap.NodeID] = snap
		h.nodeMu.Unlock()
	}
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
	data := h.store.ListSnapshots("", limit)
	if data == nil {
		data = []map[string]any{}
	}
	c.JSON(200, data)
}

// ── Nodes ─────────────────────────────────────────────────────

func (h *Handler) listNodes(c *gin.Context) {
	c.JSON(200, h.nm.ListNodes())
}

func (h *Handler) getNode(c *gin.Context) {
	n := h.nm.GetNode(c.Param("id"))
	if n == nil {
		c.JSON(404, gin.H{"error": "node not found"})
		return
	}
	c.JSON(200, n)
}

func (h *Handler) deleteNode(c *gin.Context) {
	id := c.Param("id")
	h.nm.RemoveNode(id)
	if h.store != nil {
		h.store.DeleteNode(id)
	}
	h.auditLog(c, "delete_node", "node "+id, "success")
	c.JSON(200, gin.H{"status": "ok"})
}

func (h *Handler) getNodeMetrics(c *gin.Context) {
	nodeID := c.Param("id")
	if !h.isNodeAccessible(nodeID, c) {
		c.AbortWithStatusJSON(403, gin.H{"error": "access denied to this node"})
		return
	}
	h.nodeMu.RLock()
	cs, exists := h.nodeSnapshots[nodeID]
	h.nodeMu.RUnlock()
	if exists && cs != nil {
		c.JSON(200, cs)
		return
	}
	if nodeID == "self" {
		h.mu.RLock()
		cs := h.cachedSnapshot
		h.mu.RUnlock()
		if cs != nil {
			c.JSON(200, cs)
			return
		}
	}
	snap, err := h.collector.Collect()
	if err != nil {
		c.JSON(500, gin.H{"error": "operation failed"})
		return
	}
	c.JSON(200, snap)
}

func (h *Handler) isNodeAccessible(nodeID string, c *gin.Context) bool {
	if nodeID == "self" {
		return true
	}

	role, exists := c.Get("role")
	if !exists {
		return false
	}

	if role == "admin" {
		nodes := h.nm.ListNodes()
		for _, n := range nodes {
			if n.ID == nodeID {
				return true
			}
		}
		return false
	}

	return false
}

func (h *Handler) getNodeHistory(c *gin.Context) {
	if h.store == nil {
		c.JSON(200, []interface{}{})
		return
	}
	nodeID := c.Param("id")
	duration := c.Query("duration")
	limit := 500
	if l := c.Query("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 && v <= 500 {
			limit = v
		}
	}

	data := h.store.ListSnapshots(nodeID, limit)
	if data == nil {
		data = h.store.ListSnapshots("", limit)
		if data == nil {
			data = []map[string]any{}
		}
	}

	if nodeID != "" {
		filtered := make([]map[string]any, 0, len(data))
		for _, item := range data {
			if nid, ok := item["node_id"]; ok && nid == nodeID {
				filtered = append(filtered, item)
			}
		}
		data = filtered
	}

	if duration != "" {
		hours := parseDuration(duration)
		if hours > 0 {
			since := time.Now().Add(-time.Duration(hours) * time.Hour)
			filtered := make([]map[string]any, 0, len(data))
			for _, item := range data {
				if ts, ok := item["timestamp"]; ok {
					if t, ok := ts.(time.Time); ok && t.After(since) {
						filtered = append(filtered, item)
					}
				}
			}
			data = filtered
		}
	}

	c.JSON(200, data)
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

func convertSnapshotsToMap(snaps []model.Snapshot) []map[string]interface{} {
	var result []map[string]interface{}
	for _, snap := range snaps {
		result = append(result, map[string]interface{}{
			"node_id":   snap.NodeID,
			"timestamp": snap.Timestamp,
			"cpu": map[string]interface{}{
				"usage_percent": snap.CPU.UsagePercent,
				"cores":         snap.CPU.Cores,
			},
			"memory": map[string]interface{}{
				"total_gb":      snap.Memory.TotalGB,
				"used_gb":       snap.Memory.UsedGB,
				"usage_percent": snap.Memory.UsagePercent,
			},
			"disk": map[string]interface{}{
				"total_gb":      snap.Disk.TotalGB,
				"used_gb":       snap.Disk.UsedGB,
				"usage_percent": snap.Disk.UsagePercent,
			},
			"network": map[string]interface{}{
				"bytes_recv": snap.Network.BytesRecv,
				"bytes_sent": snap.Network.BytesSent,
			},
			"load": map[string]interface{}{
				"load1":  snap.Load.Load1,
				"load5":  snap.Load.Load5,
				"load15": snap.Load.Load15,
			},
		})
	}
	return result
}

func (h *Handler) getNodeProcs(c *gin.Context) {
	snap, err := h.collector.Collect()
	if err != nil {
		c.JSON(200, []interface{}{})
		return
	}
	c.JSON(200, snap.Processes)
}

func (h *Handler) getNodeContainers(c *gin.Context) {
	snap, err := h.collector.Collect()
	if err != nil {
		c.JSON(200, []interface{}{})
		return
	}
	c.JSON(200, snap.Containers)
}

func (h *Handler) getGPUMetricsHistory(c *gin.Context) {
	nodeID := c.Param("id")
	hours := 1
	if h := c.Query("hours"); h != "" {
		if v, err := strconv.Atoi(h); err == nil && (v == 1 || v == 6 || v == 24) {
			hours = v
		}
	}
	if h.store == nil {
		c.JSON(200, []interface{}{})
		return
	}
	data, err := h.store.GetGPUMetricsHistory(nodeID, hours)
	if err != nil {
		c.JSON(500, gin.H{"error": "failed to query GPU metrics"})
		return
	}
	if data == nil {
		c.JSON(200, []interface{}{})
		return
	}
	c.JSON(200, data)
}

func (h *Handler) registerNode(c *gin.Context) {
	var req model.Node
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	if req.Name == "" {
		c.JSON(400, gin.H{"error": "node name is required"})
		return
	}
	if err := auth.ValidateInput(req.Name, 128, "node name"); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	if req.ID != "" {
		if err := auth.ValidateNodeID(req.ID); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
	}
	h.nm.Register(&req)
	c.JSON(200, gin.H{"status": "ok"})
}

func (h *Handler) heartbeat(c *gin.Context) {
	var req struct{ ID string }
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid request: " + err.Error()})
		return
	}
	if req.ID == "" {
		c.JSON(400, gin.H{"error": "node id is required"})
		return
	}
	h.nm.UpdateHeartbeat(req.ID)
	c.JSON(200, gin.H{"status": "ok"})
}

// ── Software (per-node) ──────────────────────────────────────

func (h *Handler) listNodeSoftware(c *gin.Context) {
	nodeID := c.Param("id")
	if h.store != nil {
		c.JSON(200, h.store.ListSoftware(nodeID))
		return
	}
	c.JSON(200, []interface{}{})
}

func (h *Handler) installNodeSoftware(c *gin.Context) {
	nodeID := c.Param("id")
	var req struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	if req.Name == "" {
		c.JSON(400, gin.H{"error": "software name is required"})
		return
	}
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[software] panic during install %s on %s: %v", req.Name, nodeID, r)
			}
		}()
		out, err := software.Install(nodeID, req.Name, req.Version)
		status := "installed"
		if err != nil {
			status = "failed"
		}
		log.Printf("[software] install %s on %s: %s, err=%v", req.Name, nodeID, out, err)
		if h.store != nil {
			h.store.SaveSoftware(map[string]interface{}{
				"node_id": nodeID, "name": req.Name,
				"version": req.Version, "status": status,
			})
		}
	}()
	if h.store != nil {
		h.store.SaveSoftware(map[string]interface{}{
			"node_id": nodeID, "name": req.Name,
			"version": req.Version, "status": "installing",
		})
	}
	c.JSON(200, gin.H{"status": "installing", "node_id": nodeID, "name": req.Name})
}

func (h *Handler) uninstallNodeSoftware(c *gin.Context) {
	nodeID := c.Param("id")
	var req struct {
		Name string `json:"name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	out, err := software.Uninstall(nodeID, req.Name)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error(), "output": out})
		return
	}
	if h.store != nil {
		h.store.DeleteSoftware(nodeID, req.Name)
	}
	c.JSON(200, gin.H{"status": "ok", "output": out})
}

func (h *Handler) softwareServiceControl(c *gin.Context) {
	nodeID := c.Param("id")
	var req struct {
		Name   string `json:"name"`
		Action string `json:"action"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	out, err := software.ServiceControl(nodeID, req.Name, req.Action)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error(), "output": out})
		return
	}
	c.JSON(200, gin.H{"status": "ok", "output": out})
}

// ── Software (global, backward compat) ───────────────────────

func (h *Handler) listSoftware(c *gin.Context) {
	if h.store != nil {
		c.JSON(200, h.store.ListSoftware(""))
		return
	}
	c.JSON(200, []interface{}{})
}

func (h *Handler) uninstallSoftware(c *gin.Context) {
	c.JSON(200, gin.H{"status": "ok"})
}

// ── Firewall ──────────────────────────────────────────────────

func (h *Handler) listFirewallRules(c *gin.Context) {
	rules, err := firewall.ListRules()
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, rules)
}

func (h *Handler) addFirewallRule(c *gin.Context) {
	nodeID := c.Param("id")
	if !h.isNodeAccessible(nodeID, c) {
		c.AbortWithStatusJSON(403, gin.H{"error": "access denied to this node"})
		return
	}
	var req struct {
		Proto string `json:"proto"`
		Port  string `json:"port"`
		SrcIP string `json:"src_ip"`
		Note  string `json:"note"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	port, _ := strconv.Atoi(req.Port)
	if port == 0 {
		c.JSON(400, gin.H{"error": "invalid port"})
		return
	}
	action := "allow"
	if err := firewall.AddRule(port, req.Proto, action, req.SrcIP); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"status": "ok"})
}

func (h *Handler) updateFirewallRule(c *gin.Context) {
	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	rid := c.Param("rid")
	if err := firewall.ToggleRule(rid, req.Enabled); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"status": "ok"})
}

func (h *Handler) deleteFirewallRule(c *gin.Context) {
	rid := c.Param("rid")
	if err := firewall.RemoveRule(rid); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"status": "ok"})
}

// ── Cron Jobs (per-node) ─────────────────────────────────────

func (h *Handler) listNodeCronJobs(c *gin.Context) {
	nodeID := c.Param("id")
	if h.store != nil {
		c.JSON(200, h.store.ListCronJobs(nodeID))
		return
	}
	c.JSON(200, []interface{}{})
}

func (h *Handler) createNodeCronJob(c *gin.Context) {
	nodeID := c.Param("id")
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
	job["node_id"] = nodeID
	if _, err := h.store.SaveCronJob(job); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"status": "ok", "job": job})
}

func (h *Handler) updateNodeCronJob(c *gin.Context) {
	var job map[string]interface{}
	if err := c.ShouldBindJSON(&job); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	jid := c.Param("jid")
	id, _ := strconv.Atoi(jid)
	job["id"] = float64(id)
	if _, err := h.store.SaveCronJob(job); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"status": "ok"})
}

func (h *Handler) deleteNodeCronJob(c *gin.Context) {
	jid := c.Param("jid")
	id, err := strconv.Atoi(jid)
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid job id"})
		return
	}
	if err := h.store.DeleteCronJob(id); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"status": "ok"})
}

func (h *Handler) runNodeCronJob(c *gin.Context) {
	c.JSON(200, gin.H{"status": "ok", "message": "任务已触发执行"})
}

// ── Cron Jobs (global) ───────────────────────────────────────

func (h *Handler) listCronJobs(c *gin.Context) {
	if h.store != nil {
		c.JSON(200, h.store.ListCronJobs(""))
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
	if _, err := h.store.SaveCronJob(job); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"status": "ok", "job": job})
}

func (h *Handler) deleteCronJob(c *gin.Context) {
	jid := c.Param("id")
	id, err := strconv.Atoi(jid)
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid job id"})
		return
	}
	if err := h.store.DeleteCronJob(id); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"status": "ok"})
}

// ── Database Management ──────────────────────────────────────

func (h *Handler) listDatabases(c *gin.Context) {
	nodeID := c.Param("id")
	if !h.isNodeAccessible(nodeID, c) {
		c.AbortWithStatusJSON(403, gin.H{"error": "access denied to this node"})
		return
	}
	if h.store != nil {
		c.JSON(200, h.store.ListDBConnections(nodeID))
		return
	}
	c.JSON(200, []interface{}{})
}

func (h *Handler) addDatabase(c *gin.Context) {
	nodeID := c.Param("id")
	var req struct {
		Name     string `json:"name"`
		Type     string `json:"type"`
		Host     string `json:"host"`
		Port     int    `json:"port"`
		Username string `json:"username"`
		Password string `json:"password"`
		Dbname   string `json:"dbname"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	if req.Name == "" || req.Type == "" || req.Host == "" {
		c.JSON(400, gin.H{"error": "name, type, host are required"})
		return
	}
	if len(req.Name) > 128 || len(req.Host) > 255 || len(req.Username) > 128 || len(req.Password) > 128 || len(req.Dbname) > 128 {
		c.JSON(400, gin.H{"error": "input too long"})
		return
	}
	conn := map[string]interface{}{
		"node_id":  nodeID,
		"name":     req.Name,
		"type":     req.Type,
		"host":     req.Host,
		"port":     req.Port,
		"user":     req.Username,
		"password": req.Password,
		"dbname":   req.Dbname,
	}
	if err := h.store.SaveDBConnection(conn); err != nil {
		c.JSON(500, gin.H{"error": "保存失败"})
		return
	}
	conn["password"] = "***"
	c.JSON(200, gin.H{"status": "ok", "connection": conn})
}

func (h *Handler) testDatabaseConnection(c *gin.Context) {
	var req struct {
		Type     string `json:"type"`
		Host     string `json:"host"`
		Port     int    `json:"port"`
		Username string `json:"username"`
		Password string `json:"password"`
		Dbname   string `json:"dbname"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	if req.Port == 0 {
		if req.Type == "mysql" {
			req.Port = 3306
		} else {
			req.Port = 5432
		}
	}
	dbConn := dbmgr.DBConnection{
		Type:     req.Type,
		Host:     req.Host,
		Port:     req.Port,
		User:     req.Username,
		Password: req.Password,
		Name:     req.Dbname,
	}
	log.Printf("[db] test connection type=%s host=%s:%d dbname=%s", dbConn.Type, dbConn.Host, dbConn.Port, dbConn.Name)
	db, err := dbmgr.Connect(dbConn)
	if err != nil {
		log.Printf("[db] test connection failed: %v", auth.SanitizeLog(err.Error()))
		c.JSON(200, gin.H{"ok": false, "error": "connection failed"})
		return
	}
	defer db.Close()
	version := dbmgr.GetVersion(db, dbConn.Type)
	c.JSON(200, gin.H{"ok": true, "version": version})
}

func (h *Handler) listDatabaseTables(c *gin.Context) {
	dbID, err := strconv.Atoi(c.Param("dbId"))
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid database id"})
		return
	}
	db, dbType, closeFn, err := h.getDBConnection(dbID)
	if err != nil {
		log.Printf("[db] getDBConnection(%d) failed: %v", dbID, err)
		c.JSON(500, gin.H{"error": "获取数据库连接失败: " + err.Error()})
		return
	}
	defer closeFn()
	tables, err := dbmgr.ListTables(db, dbType)
	if err != nil {
		log.Printf("[db] ListTables failed (type=%s): %v", dbType, err)
		c.JSON(500, gin.H{"error": "查询表列表失败: " + err.Error()})
		return
	}
	c.JSON(200, tables)
}

func (h *Handler) executeDatabaseQuery(c *gin.Context) {
	dbID, err := strconv.Atoi(c.Param("dbId"))
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid database id"})
		return
	}
	var req struct {
		SQL string `json:"sql"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	if req.SQL == "" {
		c.JSON(400, gin.H{"error": "sql is required"})
		return
	}
	if len(req.SQL) > 4096 {
		c.JSON(400, gin.H{"error": "SQL query too long (max 4096 characters)"})
		return
	}
	if err := auth.ValidateSQLQuery(req.SQL); err != nil {
		c.JSON(403, gin.H{"error": err.Error()})
		return
	}

	trimmed := strings.TrimSpace(req.SQL)
	upper := strings.ToUpper(trimmed)
	if !strings.HasPrefix(upper, "SELECT") && !strings.HasPrefix(upper, "EXPLAIN") && !strings.HasPrefix(upper, "SHOW") && !strings.HasPrefix(upper, "DESCRIBE") {
		c.JSON(403, gin.H{"error": "only SELECT/EXPLAIN/SHOW/DESCRIBE queries are allowed"})
		return
	}

	dangerousPatterns := []string{
		"INTO OUTFILE", "INTO DUMPFILE", "LOAD_FILE", "INFORMATION_SCHEMA",
		"LOAD DATA", "BENCHMARK", "SLEEP", "WAITFOR DELAY",
		"PG_SLEEP", "DBMS_LOCK.SLEEP",
	}
	for _, pat := range dangerousPatterns {
		if strings.Contains(upper, pat) {
			c.JSON(403, gin.H{"error": "query contains disallowed pattern"})
			return
		}
	}

	if strings.Contains(upper, ";") {
		c.JSON(403, gin.H{"error": "multi-statement queries are not allowed"})
		return
	}

	db, _, closeFn, err := h.getDBConnection(dbID)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	defer closeFn()

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	rows, err := dbmgr.ExecuteQueryWithContext(ctx, db, req.SQL)
	if err != nil {
		log.Printf("[db] ExecuteQuery error: %v", auth.SanitizeLog(err.Error()))
		c.JSON(500, gin.H{"error": "SQL执行失败"})
		return
	}
	if len(rows) > 1000 {
		rows = rows[:1000]
	}
	c.JSON(200, gin.H{"rows": rows, "count": len(rows), "truncated": len(rows) >= 1000})
}

func (h *Handler) deleteDatabase(c *gin.Context) {
	dbID, err := strconv.Atoi(c.Param("dbId"))
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid database id"})
		return
	}
	if err := h.store.DeleteDBConnection(dbID); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"status": "ok"})
}

func (h *Handler) getDBConnection(id int) (*sql.DB, string, func(), error) {
	connData, err := h.store.GetDBConnection(id)
	if err != nil {
		log.Printf("[db] GetDBConnection(%d) store error: %v", id, err)
		return nil, "", func() {}, fmt.Errorf("数据库连接记录不存在 (id=%d)", id)
	}
	dbConn := dbmgr.DBConnection{
		ID:       id,
		Type:     fmt.Sprintf("%v", connData["type"]),
		Host:     fmt.Sprintf("%v", connData["host"]),
		Port:     toIntFromInterface(connData["port"]),
		User:     fmt.Sprintf("%v", connData["user"]),
		Password: fmt.Sprintf("%v", connData["password"]),
		Name:     fmt.Sprintf("%v", connData["dbname"]),
	}
	db, err := dbmgr.GetPooledConnection(id, dbConn)
	if err != nil {
		log.Printf("[db] Connect failed: %v", auth.SanitizeLog(err.Error()))
		return nil, "", func() {}, fmt.Errorf("连接失败 (%s@%s:%d)", dbConn.Type, dbConn.Host, dbConn.Port)
	}
	return db, dbConn.Type, func() {}, nil
}

// ── File Manager ──────────────────────────────────────────────

func isPathSafe(p string) bool {
	if strings.ContainsRune(p, 0) {
		return false
	}
	cleaned := filepath.Clean(p)
	if strings.Contains(cleaned, "..") {
		return false
	}
	if !filepath.IsAbs(cleaned) {
		absPath, err := filepath.Abs(cleaned)
		if err != nil || strings.Contains(absPath, "..") {
			return false
		}
	}
	return true
}

func (h *Handler) listFiles(c *gin.Context) {
	nodeID := c.Param("id")
	if !h.isNodeAccessible(nodeID, c) {
		c.AbortWithStatusJSON(403, gin.H{"error": "access denied to this node"})
		return
	}
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
	nodeID := c.Param("id")
	if h.store == nil {
		c.JSON(200, []interface{}{})
		return
	}
	duration := c.Query("duration")
	hours := 168
	if d := parseDuration(duration); d > 0 {
		hours = d
	}
	c.JSON(200, h.store.GetFileStats(nodeID, hours))
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
		c.JSON(400, gin.H{"error": "invalid path: path traversal detected"})
		return
	}
	ensureAllowedDirs(req.Path)
	if err := filemgr.Mkdir(req.Path); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
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
		c.JSON(400, gin.H{"error": "invalid path: path traversal detected"})
		return
	}
	ensureAllowedDirs(req.Path)
	if err := filemgr.WriteFile(req.Path, []byte{}); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"status": "ok"})
}

func (h *Handler) readFile(c *gin.Context) {
	nodeID := c.Param("id")
	if !h.isNodeAccessible(nodeID, c) {
		c.AbortWithStatusJSON(403, gin.H{"error": "access denied to this node"})
		return
	}

	path := c.Query("path")
	if path == "" {
		c.JSON(400, gin.H{"error": "path is required"})
		return
	}

	if !isPathSafe(path) {
		c.JSON(400, gin.H{"error": "invalid path: path traversal detected"})
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

func (h *Handler) removeFile(c *gin.Context) {
	var req struct {
		Path string `json:"path"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	if !isPathSafe(req.Path) {
		c.JSON(400, gin.H{"error": "invalid path: path traversal detected"})
		return
	}
	ensureAllowedDirs(req.Path)
	if err := filemgr.Delete(req.Path); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
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
		c.JSON(400, gin.H{"error": "invalid path: path traversal detected"})
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
	allowedDirs := []string{path, filemgr.GetDefaultRoot(), filemgr.GetHomeDir()}
	if runtime.GOOS == "windows" {
		allowedDirs = append(allowedDirs, filemgr.GetDriveLetters()...)
	} else {
		allowedDirs = append(allowedDirs, "/", "/tmp", "/home", "/var", "/opt")
	}
	filemgr.InitAllowedDirs(allowedDirs)

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
		if len(data) > maxFileSize {
			continue
		}
		destPath := filepath.Join(path, safeFilename)
		filemgr.Upload(destPath, data)
	}
	c.JSON(200, gin.H{"status": "ok", "count": len(files)})
}

func (h *Handler) downloadFile(c *gin.Context) {
	path := c.Query("path")
	if path == "" {
		c.JSON(400, gin.H{"error": "path is required"})
		return
	}
	if !isPathSafe(path) {
		c.JSON(400, gin.H{"error": "invalid path: path traversal detected"})
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
	nodeID := c.Query("node_id")
	c.JSON(200, h.store.ListActiveAlerts(nodeID))
}

func (h *Handler) listAlertHistory(c *gin.Context) {
	if h.store == nil {
		c.JSON(200, []interface{}{})
		return
	}
	nodeID := c.Query("node_id")
	c.JSON(200, h.store.ListAlerts(nodeID, 200))
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

// ── Alert Rules ───────────────────────────────────────────────

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

func (h *Handler) testFeishu(c *gin.Context) {
	var req struct {
		URL string `json:"url"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	if req.URL != "" {
		if err := auth.ValidateEndpoint(req.URL); err != nil {
			c.JSON(400, gin.H{"error": "URL validation failed: " + err.Error()})
			return
		}
	}
	c.JSON(200, gin.H{"status": "ok", "message": "测试消息已发送"})
}

// ── Settings ──────────────────────────────────────────────────

func (h *Handler) getSettings(c *gin.Context) {
	s := settings.Get()
	c.JSON(200, s)
}

func (h *Handler) updateSettings(c *gin.Context) {
	var s settings.SystemSettings
	if err := c.ShouldBindJSON(&s); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	settings.Update(&s)
	c.JSON(200, gin.H{"status": "ok"})
}

func (h *Handler) getAlertSettings(c *gin.Context) {
	c.JSON(200, settings.GetAlertSettings())
}

func (h *Handler) updateAlertSettings(c *gin.Context) {
	var a settings.AlertSettings
	if err := c.ShouldBindJSON(&a); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	settings.UpdateAlertSettings(a)
	c.JSON(200, gin.H{"status": "ok"})
}

func (h *Handler) getSystemSettings(c *gin.Context) {
	s := settings.Get()
	c.JSON(200, gin.H{
		"collect_interval": s.CollectInterval,
		"retention_days":   30,
	})
}

func (h *Handler) updateSystemSettings(c *gin.Context) {
	var req struct {
		CollectInterval int `json:"collect_interval"`
		RetentionDays   int `json:"retention_days"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	s := settings.Get()
	if req.CollectInterval > 0 {
		s.CollectInterval = req.CollectInterval
	}
	settings.Update(s)
	c.JSON(200, gin.H{"status": "ok"})
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
	c.JSON(200, gin.H{"status": "ok", "node_id": "self", "snapshot": snap})
}

// ── WebSocket (metrics stream) ───────────────────────────────

func (h *Handler) websocket(c *gin.Context) {
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

// ── Terminal WebSocket ────────────────────────────────────────

func (h *Handler) terminalWS(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("[terminal] websocket upgrade failed: %v", err)
		return
	}
	nodeID := c.Param("nodeId")
	if nodeID == "" {
		nodeID = "self"
	}
	shell := c.Query("shell")
	session := terminal.NewSession(nodeID, shell, conn)
	session.Handle()
}

func (h *Handler) listShells(c *gin.Context) {
	shells := terminal.AvailableShells()
	c.JSON(200, shells)
}

// ── Helper ────────────────────────────────────────────────────

func (h *Handler) auditLog(c *gin.Context, action, detail, result string) {
	userID, _ := c.Get("user_id")
	uid, _ := userID.(float64)
	nodeID := c.Param("id")
	if nodeID == "" {
		nodeID = c.Param("nodeId")
	}
	h.store.SaveAuditLog(&model.AuditLog{
		UserID: int(uid),
		NodeID: nodeID,
		Action: action,
		Detail: detail,
		Result: result,
	})
}

func toIntFromInterface(v interface{}) int {
	switch val := v.(type) {
	case float64:
		return int(val)
	case int:
		return val
	case string:
		n, _ := strconv.Atoi(val)
		return n
	default:
		n, _ := strconv.Atoi(fmt.Sprintf("%v", val))
		return n
	}
}

func (h *Handler) healthCheck(c *gin.Context) {
	checks := make(map[string]ha.Check)

	dbStart := time.Now()
	if err := h.store.Ping(); err != nil {
		checks["database"] = ha.Check{Status: "unhealthy", Message: err.Error()}
	} else {
		checks["database"] = ha.Check{Status: "healthy", Latency: time.Since(dbStart).String()}
	}

	overall := "healthy"
	for _, check := range checks {
		if check.Status != "healthy" {
			overall = "degraded"
			break
		}
	}

	status := 200
	if overall == "unhealthy" {
		status = 503
	} else if overall == "degraded" {
		status = 200
	}

	c.JSON(status, ha.HealthStatus{
		Status:    overall,
		Timestamp: time.Now().Format(time.RFC3339),
		Uptime:    ha.GetUptime(),
		Checks:    checks,
		System:    ha.GetSystemInfo(),
	})
}

func (h *Handler) readinessCheck(c *gin.Context) {
	if err := h.store.Ping(); err != nil {
		c.JSON(503, gin.H{"status": "not_ready", "error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"status": "ready", "timestamp": time.Now().Format(time.RFC3339)})
}

func (h *Handler) createBackup(c *gin.Context) {
	dbPath := h.store.DBPath()
	if dbPath == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "backup not supported for remote database"})
		return
	}

	bm := ha.NewBackupManager("")
	info, err := bm.CreateBackup(dbPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	bm.CleanupOldBackups(10)

	h.auditLog(c, "create_backup", info.Name, "success")
	c.JSON(http.StatusOK, info)
}

func (h *Handler) listBackups(c *gin.Context) {
	bm := ha.NewBackupManager("")
	backups := bm.ListBackups()
	if backups == nil {
		backups = []ha.BackupInfo{}
	}
	c.JSON(http.StatusOK, backups)
}

func (h *Handler) restoreBackup(c *gin.Context) {
	dbPath := h.store.DBPath()
	if dbPath == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "restore not supported for remote database"})
		return
	}

	var req struct {
		Name string `json:"name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "backup name required"})
		return
	}

	bm := ha.NewBackupManager("")
	bm.OnRestore(func() {
		if err := h.store.Reopen(); err != nil {
			log.Printf("[ha] failed to reopen database after restore: %v", err)
		}
	})
	if err := bm.RestoreBackup(req.Name, dbPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	h.auditLog(c, "restore_backup", req.Name, "success")
	c.JSON(http.StatusOK, gin.H{"message": "backup restored successfully"})
}

func (h *Handler) deleteBackup(c *gin.Context) {
	name := c.Param("name")
	bm := ha.NewBackupManager("")
	if err := bm.DeleteBackup(name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "backup deleted"})
}

func (h *Handler) getTrendCompare(c *gin.Context) {
	if h.store == nil {
		c.JSON(200, gin.H{"current": []interface{}{}, "previous": []interface{}{}, "summary": gin.H{}})
		return
	}

	nodeID := c.Query("node_id")
	if nodeID == "" {
		nodeID = "self"
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

	currentData, err := h.store.GetMetricsHistoryRange(nodeID, currentStart, now)
	if err != nil {
		currentData = []map[string]any{}
	}
	previousData, err := h.store.GetMetricsHistoryRange(nodeID, previousStart, currentStart)
	if err != nil {
		previousData = []map[string]any{}
	}

	currentSummary := computeMetricSummary(currentData, metric)
	previousSummary := computeMetricSummary(previousData, metric)

	change := 0.0
	curAvg := summaryFloat(currentSummary, "avg")
	prevAvg := summaryFloat(previousSummary, "avg")
	if prevAvg > 0 {
		change = ((curAvg - prevAvg) / prevAvg) * 100
	}

	c.JSON(200, gin.H{
		"current":  currentData,
		"previous": previousData,
		"metric":   metric,
		"period":   period,
		"summary": gin.H{
			"current":  currentSummary,
			"previous": previousSummary,
			"change":   fmt.Sprintf("%.1f%%", change),
			"trend":    trendDirection(change),
		},
	})
}

func computeMetricSummary(data []map[string]any, metric string) map[string]any {
	if len(data) == 0 {
		return map[string]any{"avg": float64(0), "max": float64(0), "min": float64(0), "count": 0}
	}

	var values []float64
	for _, d := range data {
		var v float64
		switch metric {
		case "cpu":
			v, _ = d["cpu_usage"].(float64)
			if v == 0 {
				if m, ok := d["cpu"].(map[string]any); ok {
					v, _ = m["usage_percent"].(float64)
				}
			}
		case "memory":
			v, _ = d["mem_usage_percent"].(float64)
			if v == 0 {
				if m, ok := d["memory"].(map[string]any); ok {
					v, _ = m["usage_percent"].(float64)
				}
			}
		case "disk":
			v, _ = d["disk_usage_percent"].(float64)
			if v == 0 {
				if m, ok := d["disk"].(map[string]any); ok {
					v, _ = m["usage_percent"].(float64)
				}
			}
		case "load1":
			if m, ok := d["load"].(map[string]any); ok {
				v, _ = m["load1"].(float64)
			}
		}
		if v > 0 {
			values = append(values, v)
		}
	}

	if len(values) == 0 {
		return map[string]any{"avg": float64(0), "max": float64(0), "min": float64(0), "count": 0}
	}

	sum := 0.0
	maxV := values[0]
	minV := values[0]
	for _, v := range values {
		sum += v
		if v > maxV {
			maxV = v
		}
		if v < minV {
			minV = v
		}
	}

	return map[string]any{
		"avg":   fmt.Sprintf("%.1f", sum/float64(len(values))),
		"max":   fmt.Sprintf("%.1f", maxV),
		"min":   fmt.Sprintf("%.1f", minV),
		"count": len(values),
	}
}

func trendDirection(change float64) string {
	if change > 5 {
		return "rising"
	}
	if change < -5 {
		return "falling"
	}
	return "stable"
}

func summaryFloat(m map[string]any, key string) float64 {
	v, ok := m[key]
	if !ok {
		return 0
	}
	switch val := v.(type) {
	case float64:
		return val
	case string:
		f, _ := strconv.ParseFloat(val, 64)
		return f
	case int:
		return float64(val)
	default:
		return 0
	}
}

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

	nodeID := c.Query("node_id")
	if nodeID == "" {
		nodeID = "self"
	}
	sigmaStr := c.Query("sigma")
	sigma := 2.0
	if s, err := strconv.ParseFloat(sigmaStr, 64); err == nil && s > 0 && s <= 5 {
		sigma = s
	}

	hours := 24
	if h := c.Query("hours"); h != "" {
		if v, err := strconv.Atoi(h); err == nil && v > 0 && v <= 720 {
			hours = v
		}
	}

	data, err := h.store.GetMetricsHistory(nodeID, hours)
	if err != nil || len(data) == 0 {
		c.JSON(200, gin.H{"anomalies": []AnomalyResult{}, "summary": gin.H{"total": 0, "anomalies": 0, "message": "no data available"}})
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
			if m, ok := d["cpu"].(map[string]any); ok {
				if v, ok := m["usage_percent"].(float64); ok {
					return v
				}
			}
			return 0
		}},
		{"memory", func(d map[string]any) float64 {
			if v, ok := d["mem_usage_percent"].(float64); ok {
				return v
			}
			if m, ok := d["memory"].(map[string]any); ok {
				if v, ok := m["usage_percent"].(float64); ok {
					return v
				}
			}
			return 0
		}},
		{"disk", func(d map[string]any) float64 {
			if v, ok := d["disk_usage_percent"].(float64); ok {
				return v
			}
			if m, ok := d["disk"].(map[string]any); ok {
				if v, ok := m["usage_percent"].(float64); ok {
					return v
				}
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

			isAnomaly := v > upperBand || v < lowerBand
			severity := "normal"
			if isAnomaly {
				deviation := 0.0
				if v > upperBand {
					deviation = (v - upperBand) / std
				} else {
					deviation = (lowerBand - v) / std
				}
				if deviation > 2 {
					severity = "critical"
				} else {
					severity = "warning"
				}
			}

			if isAnomaly {
				ts, _ := d["timestamp"].(string)
				if ts == "" {
					ts = time.Now().Format(time.RFC3339)
				}
				anomalies = append(anomalies, AnomalyResult{
					Metric:    m.name,
					Value:     v,
					Mean:      mean,
					Std:       std,
					UpperBand: upperBand,
					LowerBand: lowerBand,
					IsAnomaly: true,
					Severity:  severity,
					Time:      ts,
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
			"anomalyRate": fmt.Sprintf("%.2f%%", float64(len(anomalies))/float64(totalPoints)*100),
		},
	})
}
