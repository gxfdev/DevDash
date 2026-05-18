package api

import (
	"database/sql"
	"fmt"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"strconv"
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
		allowedOrigins := map[string]bool{
			"http://localhost:3000":  true,
			"http://localhost:5173":  true,
			"http://localhost:9090":  true,
			"http://127.0.0.1:3000": true,
			"http://127.0.0.1:5173": true,
			"http://127.0.0.1:9090": true,
		}
		origin := r.Header.Get("Origin")
		if origin == "" || origin == "null" {
			return false
		}
		return allowedOrigins[origin]
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
	api := r.Group("/api")

	api.GET("/v1/health", func(c *gin.Context) { c.JSON(200, gin.H{"status": "ok", "timestamp": time.Now().Format(time.RFC3339)}) })

	api.POST("/auth/login", h.login)
	api.GET("/auth/me", auth.Middleware(), h.authMe)
	api.PUT("/auth/password", auth.Middleware(), h.changePassword)
	api.POST("/auth/logout", auth.Middleware(), h.authLogout)

	v1 := r.Group("/api/v1", auth.Middleware(), auth.CSRFMiddleware())

	v1.GET("/snapshot", h.getSnapshot)
	v1.GET("/latest", h.getLatest)
	v1.GET("/history", h.getHistory)

	v1.GET("/nodes", h.listNodes)
	v1.GET("/node/:id", h.getNode)
	v1.POST("/node/register", h.registerNode)
	v1.POST("/node/heartbeat", h.heartbeat)
	v1.DELETE("/node/:id", h.deleteNode)
	v1.GET("/node/:id/metrics", h.getNodeMetrics)
	v1.GET("/node/:id/history", h.getNodeHistory)
	v1.GET("/node/:id/procs", h.getNodeProcs)
	v1.GET("/node/:id/containers", h.getNodeContainers)

	v1.GET("/node/:id/software", h.listNodeSoftware)
	v1.POST("/node/:id/software/install", h.installNodeSoftware)
	v1.POST("/node/:id/software/uninstall", h.uninstallNodeSoftware)
	v1.POST("/node/:id/software/service", h.softwareServiceControl)

	v1.GET("/node/:id/firewall/rules", h.listFirewallRules)
	v1.POST("/node/:id/firewall/rules", h.addFirewallRule)
	v1.PATCH("/node/:id/firewall/rules/:rid", h.updateFirewallRule)
	v1.DELETE("/node/:id/firewall/rules/:rid", h.deleteFirewallRule)

	v1.GET("/node/:id/cronjobs", h.listNodeCronJobs)
	v1.POST("/node/:id/cronjobs", h.createNodeCronJob)
	v1.PATCH("/node/:id/cronjobs/:jid", h.updateNodeCronJob)
	v1.DELETE("/node/:id/cronjobs/:jid", h.deleteNodeCronJob)
	v1.POST("/node/:id/cronjobs/:jid/run", h.runNodeCronJob)

	v1.GET("/node/:id/databases", h.listDatabases)
	v1.POST("/node/:id/databases", h.addDatabase)
	v1.POST("/node/:id/databases/test", h.testDatabaseConnection)
	v1.GET("/node/:id/databases/:dbId/tables", h.listDatabaseTables)
	v1.POST("/node/:id/databases/:dbId/query", h.executeDatabaseQuery)
	v1.DELETE("/node/:id/databases/:dbId", h.deleteDatabase)

	v1.GET("/node/:id/fs/list", h.listFiles)
	v1.POST("/node/:id/fs/mkdir", h.createDir)
	v1.POST("/node/:id/fs/mkfile", h.createFile)
	v1.DELETE("/node/:id/fs/remove", h.removeFile)
	v1.POST("/node/:id/fs/upload", h.uploadFile)
	v1.GET("/node/:id/fs/download", h.downloadFile)

	v1.GET("/alerts", h.listAlerts)
	v1.GET("/alerts/active", h.listActiveAlerts)
	v1.GET("/alerts/history", h.listAlertHistory)
	v1.POST("/alerts/:id/silence", h.silenceAlert)

	v1.GET("/alert-rules", h.listAlertRules)
	v1.POST("/alert-rules", h.createAlertRule)
	v1.PUT("/alert-rules/:id", h.updateAlertRule)
	v1.DELETE("/alert-rules/:id", h.deleteAlertRule)
	v1.POST("/alert/test-feishu", h.testFeishu)

	v1.GET("/settings", h.getSettings)
	v1.PUT("/settings", h.updateSettings)
	v1.GET("/alert-settings", h.getAlertSettings)
	v1.PUT("/alert-settings", h.updateAlertSettings)

	v1.GET("/software", h.listSoftware)
	v1.POST("/software/install", h.installSoftware)
	v1.POST("/software/uninstall", h.uninstallSoftware)
	v1.GET("/cronjobs", h.listCronJobs)
	v1.POST("/cronjobs", h.createCronJob)
	v1.DELETE("/cronjobs/:id", h.deleteCronJob)
	v1.POST("/collect", h.triggerCollect)
	v1.GET("/ws", h.websocket)

	r.GET("/ws/terminal/:nodeId", auth.Middleware(), h.terminalWS)
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

	clientIP := c.ClientIP()
	if err := auth.CheckLoginRateLimit(clientIP); err != nil {
		c.JSON(429, gin.H{"error": "too many attempts, please try again later"})
		return
	}

	hash, err := h.store.GetUser(req.Username)
	if err != nil || !auth.CheckPassword(hash, req.Password) {
		c.JSON(401, gin.H{"error": "invalid credentials"})
		return
	}
	auth.ResetLoginAttempts(clientIP)

	role := "user"
	if req.Username == "admin" {
		role = "admin"
	}
	token, _ := auth.GenerateToken(1, req.Username, role)
	
	csrfToken, _ := auth.GenerateCSRFToken()
	c.SetCookie("csrf_token", csrfToken, 86400, "/", "", false, true)
	
	c.JSON(200, gin.H{"token": token})
}

func (h *Handler) authMe(c *gin.Context) {
	username, _ := c.Get("username")
	role, _ := c.Get("role")
	c.JSON(200, gin.H{"username": username, "role": role})
}

func (h *Handler) changePassword(c *gin.Context) {
	var req struct {
		Old   string `json:"old"`
		New   string `json:"new"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid request"})
		return
	}
	username, _ := c.Get("username")
	uname, _ := username.(string)
	hash, err := h.store.GetUser(uname)
	if err != nil || !auth.CheckPassword(hash, req.Old) {
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
		h.store.SaveSnapshot(snap)
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
		if m, ok := r.(map[string]interface{}); ok {
			enabled, _ := m["enabled"].(bool)
			if !enabled {
				continue
			}
			metric, _ := m["metric"].(string)
			op, _ := m["op"].(string)
			threshold, _ := m["threshold"].(float64)
			level, _ := m["level"].(string)
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
	defaultThresholds := []struct {
		metricName string
		value      float64
		threshold  float64
		level      string
	}{
		{"cpu", snap.CPU.UsagePercent, 80, "warning"},
		{"memory", snap.Memory.UsagePercent, 85, "warning"},
		{"disk", snap.Disk.UsagePercent, 90, "critical"},
	}
	for _, t := range defaultThresholds {
		if t.value > t.threshold {
			h.store.SaveAlert(map[string]interface{}{
				"node_id":   snap.NodeID,
				"node_name": hostName,
				"metric":    t.metricName,
				"message":   fmt.Sprintf("%s %.1f%% exceeds %d%%", t.metricName, t.value, int(t.threshold)),
				"level":     t.level,
				"value":     t.value,
				"threshold": t.threshold,
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
		if v, err := strconv.Atoi(l); err == nil && v > 0 && v <= 2000 {
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
		nodes := h.nm.List()
		for _, n := range nodes {
			if n.ID == nodeID {
				return true
			}
		}
	}

	username, _ := c.Get("username")
	uname, _ := username.(string)
	nodes := h.nm.ListByOwner(uname)
	for _, n := range nodes {
		if n.ID == nodeID {
			return true
		}
	}

	return false
}

func (h *Handler) getNodeHistory(c *gin.Context) {
	if h.store == nil {
		c.JSON(200, []interface{}{})
		return
	}
	nodeID := c.Param("id")
	limit := 200
	if l := c.Query("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 {
			limit = v
		}
	}
	c.JSON(200, h.store.ListSnapshots(nodeID, limit))
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

func (h *Handler) registerNode(c *gin.Context) {
	var req model.Node
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
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

func (h *Handler) installNodeSoftware(c *gin.Context) {
	nodeID := c.Param("id")
	if !h.isNodeAccessible(nodeID, c) {
		c.AbortWithStatusJSON(403, gin.H{"error": "access denied to this node"})
		return
	}
	var req struct {
		NodeID  string `json:"node_id"`
		Name    string `json:"name"`
		Version string `json:"version"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	nodeID := req.NodeID
	if nodeID == "" {
		nodeID = "self"
	}
	if h.store != nil {
		h.store.SaveSoftware(map[string]interface{}{
			"node_id": nodeID, "name": req.Name,
			"version": req.Version, "status": "installing",
		})
	}
	c.JSON(200, gin.H{"status": "installing", "node_id": nodeID, "name": req.Name})
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
	if err := h.store.SaveCronJob(job); err != nil {
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
	if err := h.store.SaveCronJob(job); err != nil {
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
	if err := h.store.SaveCronJob(job); err != nil {
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
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
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
		if req.Type == "mysql" { req.Port = 3306 } else { req.Port = 5432 }
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
		log.Printf("[db] test connection failed: %v", err)
		c.JSON(200, gin.H{"ok": false, "error": err.Error()})
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
	db, _, closeFn, err := h.getDBConnection(dbID)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	defer closeFn()
	rows, err := dbmgr.ExecuteQuery(db, req.SQL)
	if err != nil {
		log.Printf("[db] ExecuteQuery error: %v", err)
		c.JSON(500, gin.H{"error": "SQL执行失败: " + err.Error()})
		return
	}
	c.JSON(200, gin.H{"rows": rows, "count": len(rows)})
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
		Type:     fmt.Sprintf("%v", connData["type"]),
		Host:     fmt.Sprintf("%v", connData["host"]),
		Port:     toIntFromInterface(connData["port"]),
		User:     fmt.Sprintf("%v", connData["user"]),
		Password: fmt.Sprintf("%v", connData["password"]),
		Name:     fmt.Sprintf("%v", connData["dbname"]),
	}
	log.Printf("[db] connecting %s@%s:%d/%s ...", dbConn.Type, dbConn.Host, dbConn.Port, dbConn.Name)
	db, err := dbmgr.Connect(dbConn)
	if err != nil {
		log.Printf("[db] Connect failed: %v", err)
		return nil, "", func() {}, fmt.Errorf("连接失败 (%s@%s:%d): %v", dbConn.Type, dbConn.Host, dbConn.Port, err)
	}
	return db, dbConn.Type, func() { db.Close() }, nil
}

// ── File Manager ──────────────────────────────────────────────

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

func (h *Handler) createDir(c *gin.Context) {
	var req struct {
		Path string `json:"path"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
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
	if err := filemgr.WriteFile(req.Path, []byte{}); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
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
	if err := filemgr.Delete(req.Path); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"status": "ok"})
}

func (h *Handler) uploadFile(c *gin.Context) {
	path := c.Query("path")
	if path == "" {
		path = "./"
	}
	form, err := c.MultipartForm()
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	files := form.File["files"]
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
	data, err := filemgr.Download(path)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	name := filepath.Base(path)
	c.Header("Content-Disposition", "attachment; filename="+name)
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
	token := c.Query("token")
	if ok, _ := auth.WebsocketAuth(token); !ok {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
		return
	}
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
		for {
			select {
			case <-ticker.C:
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
		}
	}()

	wg.Wait()
}

// ── Terminal WebSocket ────────────────────────────────────────

func (h *Handler) terminalWS(c *gin.Context) {
	token := c.Query("token")
	if token == "" {
		token = c.Request.Header.Get("Sec-WebSocket-Protocol")
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
