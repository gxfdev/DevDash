package api

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gxfdev/aidash/server/internal/auth"
	"github.com/gxfdev/aidash/server/internal/catalog"
	"github.com/gxfdev/aidash/server/internal/config"
	"github.com/gxfdev/aidash/server/internal/gateway"
	"github.com/gxfdev/aidash/server/internal/model"
	"github.com/gxfdev/aidash/server/internal/modelhub"
	"github.com/gxfdev/aidash/server/internal/orchestrator"
	"github.com/gxfdev/aidash/server/internal/store"
)

type Handler struct {
	cfg     *config.Config
	store   *store.Store
	orch    *orchestrator.Orchestrator
	auth    *auth.Manager
	cat     *catalog.Catalog
	gateway *gateway.Gateway
	copilot *gateway.Copilot
	hub     *modelhub.Hub

	mu        sync.RWMutex
	snapshot  *model.HostSnapshot
	startedAt time.Time
}

func NewHandler(cfg *config.Config, st *store.Store, orch *orchestrator.Orchestrator, mgr *auth.Manager, cat *catalog.Catalog, gw *gateway.Gateway, cp *gateway.Copilot, hub *modelhub.Hub) *Handler {
	return &Handler{
		cfg: cfg, store: st, orch: orch, auth: mgr, cat: cat,
		gateway: gw, copilot: cp, hub: hub, startedAt: time.Now(),
	}
}

func (h *Handler) SetSnapshot(s *model.HostSnapshot) {
	h.mu.Lock()
	h.snapshot = s
	h.mu.Unlock()
}

func (h *Handler) Snapshot() *model.HostSnapshot {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.snapshot
}

func ok(c *gin.Context, data any) {
	c.JSON(http.StatusOK, gin.H{"data": data})
}

func fail(c *gin.Context, code int, msg string) {
	c.AbortWithStatusJSON(code, gin.H{"error": msg})
}

func bad(c *gin.Context, err error) {
	fail(c, http.StatusBadRequest, err.Error())
}

func jsonEncoder(c *gin.Context) *json.Encoder {
	return json.NewEncoder(c.Writer)
}
