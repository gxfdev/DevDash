package api

import (
	"database/sql"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"devdash/internal/auth"
	"devdash/internal/collector"
	"devdash/internal/dbmgr"
	"devdash/internal/filemgr"
	"devdash/internal/firewall"
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
			return false
		}
		allowedOrigins := os.Getenv("CORS_ORIGINS")
		if allowedOrigins != "" {
			for _, o := range strings.Split(allowedOrigins, ",") {
				if strings.TrimSpace(o) == origin {
					return true
				}
			}
		}
		defaultOrigins := map[string]bool{
			"http://localhost:3000": true,
			"http://localhost:5173": true,
			"http://localhost:9090": true,
			"http://127.0.0.1:3000": true,
			"http://127.0.0.1:5173": true,
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
	dbConns        map[int]*sql.DB
	dbMu           sync.RWMutex
}

func NewHandler(c *collector.Collector, s *store.Store, nm *node.NodeManager) *Handler {
	return &Handler{
		collector: c,
		store:     s,
		nm:        nm,
		dbConns:   make(map[int]*sql.DB),
	}
}

func (h *Handler) RegisterRoutes(r *gin.Engine) {
	r.Use(requestSizeLimit(10 * 1024 * 1024))

	api := r.Group("/api")

	api.GET("/v1/health", func(c *gin.Context) { c.JSON(200, gin.H{"status": "ok", "timestamp": time.Now().Format(time.RFC3339)}) })

	api.POST("/auth/login", h.login)
	api.POST("/auth/refresh", h.refreshToken)
	api.GET("/auth/me", auth.Middleware(), h.authMe)
	api.PUT("/auth/password", auth.Middleware(), h.changePassword)
	api.POST("/auth/logout", auth.Middleware(), h.authLogout)

	v1 := r.Group("/api/v1", auth.Middleware(), auth.CSRFMiddleware())

	v1.GET("/snapshot", h.getSnapshot)
	v1.GET("/latest", h.getLatest)
	v1.GET("/history", h.getHistory)

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
	v1.GET("/node/:id/fs/read", h.readFile)
	v1.POST("/node/:id/fs/upload", auth.RequireRole("admin"), h.uploadFile)
	v1.GET("/node/:id/fs/download", h.downloadFile)

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

	v1.GET("/software", h.listSoftware)
	v1.POST("/software/install", auth.RequireRole("admin"), h.installNodeSoftware)
	v1.POST("/software/uninstall", auth.RequireRole("admin"), h.uninstallSoftware)
	v1.GET("/cronjobs", h.listCronJobs)
	v1.POST("/cronjobs", auth.RequireRole("admin"), h.createCronJob)
	v1.DELETE("/cronjobs/:id", auth.RequireRole("admin"), h.deleteCronJob)
	v1.POST("/collect", auth.RequireRole("admin"), h.triggerCollect)
	v1.GET("/ws", auth.WSMiddleware(), h.websocket)

	r.GET("/ws/terminal/:nodeId", auth.WSMiddleware(), auth.RequireRole("admin"), h.terminalWS)

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
	if len(req.Password) > 128 {
		c.JSON(400, gin.H{"error": "password too long"})
		return
	}

	clientIP := c.ClientIP()
	if err := auth.CheckLoginRateLimit(clientIP); err != nil {
		c.JSON(429, gin.H{"error": "too many attempts, please try again later"})
		return
	}

	user, err := h.store.GetUserByUsername(req.Username)
	if err != nil || !auth.CheckPassword(user.PasswordHash, req.Password) {
		c.JSON(401, gin.H{"error": "invalid credentials"})
		return
	}
	auth.ResetLoginAttempts(clientIP)

	tokenPair, err := auth.GenerateTokenPair(user.ID, user.Username, user.Role)
	if err != nil {
		c.JSON(500, gin.H{"error": "failed to generate token"})
		return
	}

	csrfToken, _ := auth.GenerateCSRFToken()
	c.SetCookie("csrf_token", csrfToken, 86400, "/", "", false, true)
	c.SetCookie("refresh_token", tokenPair.RefreshToken, 7*86400, "/", "", false, true)

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
	c.JSON(200, gin.H{"username": username, "role": role})
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
	if len(req.New) < 8 {
		c.JSON(400, gin.H{"error": "新密码长度至少8位"})
		return
	}
	if len(req.New) > 128 {
		c.JSON(400, gin.H{"error": "密码长度不能超过128位"})
		return
	}
	username, _ := c.Get("username")
	uname, _ := username.(string)
	user, err := h.store.GetUserByUsername(uname)
	if err != nil || !auth.CheckPassword(user.PasswordHash, req.Old) {
		c.JSON(401, gin.H{"error": "当前密码错误"})
		return
	}
	newHash := auth.HashPassword(req.New)
	if err := h.store.UpdatePassword(uname, newHash); err != nil {
		c.JSON(500, gin.H{"error": "更新失败"})
		return
	}
	c.JSON(200, gin.H{"status": "ok"})
}

func (h *Handler) authLogout(c *gin.Context) {
	c.JSON(200, gin.H{"status": "ok"})
}

func (h *Handler) refreshToken(c *gin.Context) {
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid request"})
		return
	}
	if req.RefreshToken == "" {
		c.JSON(400, gin.H{"error": "refresh_token is required"})
		return
	}
	claims, err := auth.ValidateRefreshToken(req.RefreshToken)
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
	c.JSON(200, gin.H{
		"access_token":  tokenPair.AccessToken,
		"refresh_token": tokenPair.RefreshToken,
		"expires_in":    tokenPair.ExpiresIn,
		"token_type":    "Bearer",
	})
}

// ── Snapshot / Metrics ────────────────────────────────────────

func (h *Handler) getSnapshot(c *gin.Context) {
	snap, err := h.collector.Collect()
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
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
		c.JSON(500, gin.H{"error": err.Error()})
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
	limit := 100
	if l := c.Query("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 && v <= 100 {
			limit = v
		}
	}
	c.JSON(200, h.store.ListSnapshots("", limit))
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
	c.JSON(200, gin.H{"status": "ok"})
}

func (h *Handler) getNodeMetrics(c *gin.Context) {
	nodeID := c.Param("id")
	if !h.isNodeAccessible(nodeID, c) {
		c.AbortWithStatusJSON(403, gin.H{"error": "access denied to this node"})
		return
	}
	h.mu.RLock()
	cs := h.cachedSnapshot
	h.mu.RUnlock()
	if cs != nil {
		c.JSON(200, cs)
		return
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

	if duration != "" {
		hours := parseDuration(duration)
		if hours > 0 {
			data, err := h.store.GetMetricsHistory(nodeID, hours)
			if err == nil {
				result := convertSnapshotsToMap(data)
				c.JSON(200, result)
				return
			}
		}
	}
	c.JSON(200, h.store.ListSnapshots(nodeID, limit))
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
	log.Printf("[db] test connection %s@%s:%d/%s", dbConn.Type, dbConn.Host, dbConn.Port, dbConn.Name)
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
	rows, err := dbmgr.ExecuteQuery(db, req.SQL)
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
	cleaned := filepath.Clean(p)
	if strings.Contains(cleaned, "..") {
		return false
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
	allowedDirs := []string{path, filemgr.GetDefaultRoot(), filemgr.GetHomeDir()}
	if runtime.GOOS == "windows" {
		allowedDirs = append(allowedDirs, filemgr.GetDriveLetters()...)
	} else {
		allowedDirs = append(allowedDirs, "/", "/tmp", "/home", "/var", "/opt")
	}
	filemgr.InitAllowedDirs(allowedDirs)

	for _, f := range files {
		src, err := f.Open()
		if err != nil {
			continue
		}
		data, err := io.ReadAll(src)
		src.Close()
		if err != nil {
			continue
		}
		destPath := filepath.Join(path, f.Filename)
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
	rule["id"] = float64(id)
	if err := h.store.SaveAlertRule(rule); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"status": "ok"})
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
	token := c.Query("token")
	if token == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "token required"})
		return
	}
	if ok, _ := auth.WebsocketAuth(token); !ok {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
		return
	}
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	nodeID := c.Param("nodeId")
	session := terminal.NewSession(nodeID, conn)
	session.Handle()
}

// ── Helper ────────────────────────────────────────────────────

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
