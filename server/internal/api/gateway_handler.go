package api

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gxfdev/aidash/server/internal/gateway"
	"github.com/gxfdev/aidash/server/internal/model"
)

func (h *Handler) gatewayInfo(c *gin.Context) {
	now := time.Now()
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	today, _ := h.store.UsageTotalsSince(dayStart.Unix())
	week, _ := h.store.UsageTotalsSince(now.AddDate(0, 0, -7).Unix())
	month, _ := h.store.UsageTotalsSince(time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()).Unix())
	series, _ := h.store.UsageSeries(now.Add(-24 * time.Hour).Unix())
	byModel, _ := h.store.ModelUsage()
	keys, _ := h.store.ListAPIKeys()
	models := h.gateway.ListModels(c.Request.Context())

	baseURL := strings.TrimSuffix(h.cfg.PublicURL, "/")
	if baseURL == "" {
		scheme := "http"
		if c.Request.TLS != nil {
			scheme = "https"
		}
		baseURL = scheme + "://" + c.Request.Host
	}
	ok(c, gin.H{
		"models":     models,
		"usage":      gin.H{"today": today, "week": week, "month": month},
		"series":     series,
		"by_model":   byModel,
		"keys":       len(keys),
		"base_url":   baseURL + "/v1",
		"models_url": baseURL + "/v1/models",
	})
}

func (h *Handler) listKeys(c *gin.Context) {
	keys, err := h.store.ListAPIKeys()
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	ok(c, gin.H{"keys": keys})
}

type createKeyRequest struct {
	Name       string `json:"name"`
	Role       string `json:"role"`
	TokenLimit int64  `json:"token_limit"`
}

func (h *Handler) createKey(c *gin.Context) {
	var req createKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		bad(c, err)
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		fail(c, http.StatusBadRequest, "请填写密钥名称")
		return
	}
	if req.Role == "" {
		req.Role = "operator"
	}
	plain, prefix, hash, id := gateway.GenerateKey()
	key := &model.APIKey{
		ID: id, Name: req.Name, Prefix: prefix, KeyHash: hash, Role: req.Role,
		CreatedAt: time.Now(), TokenLimit: req.TokenLimit,
	}
	if err := h.store.CreateAPIKey(key); err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	h.audit(c, "创建 API Key", req.Name, "", "ok")
	c.JSON(http.StatusCreated, gin.H{"data": gin.H{"key": key, "secret": plain}})
}

func (h *Handler) deleteKey(c *gin.Context) {
	if err := h.store.RevokeAPIKey(c.Param("id")); err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	h.audit(c, "删除 API Key", c.Param("id"), "", "ok")
	ok(c, gin.H{"message": "密钥已删除"})
}

func (h *Handler) gatewayTest(c *gin.Context) {
	var req struct {
		Model   string `json:"model"`
		Message string `json:"message"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		bad(c, err)
		return
	}
	if req.Message == "" {
		req.Message = "用一句话介绍你自己"
	}
	body := map[string]any{
		"model":    req.Model,
		"messages": []map[string]any{{"role": "user", "content": req.Message}},
		"stream":   false,
	}
	out, err := h.gateway.ChatRaw(c.Request.Context(), req.Model, body)
	if err != nil {
		fail(c, http.StatusBadGateway, err.Error())
		return
	}
	ok(c, out)
}

func (h *Handler) playground(c *gin.Context) {
	h.gateway.Proxy(c.Writer, c.Request, "/v1/chat/completions", nil, actor(c))
}

func (h *Handler) copilotChat(c *gin.Context) {
	var req gateway.CopilotRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		bad(c, err)
		return
	}
	ctx, cancel := contextWithTimeout(c, 180*time.Second)
	defer cancel()
	resp, err := h.copilot.Chat(ctx, req)
	if err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	h.audit(c, "Copilot 对话", req.Model, "exec="+map[bool]string{true: "on", false: "off"}[req.AllowExec], "ok")
	ok(c, resp)
}

// ---------- OpenAI compatible surface ----------

func (h *Handler) v1Models(c *gin.Context) {
	key, err := h.gateway.Authenticate(c.Request)
	if err != nil {
		gateway.WriteAuthError(c.Writer, err)
		return
	}
	_ = key
	ok(c, gin.H{"object": "list", "data": h.gateway.ListModels(c.Request.Context())})
}

func authProxy(h *Handler, c *gin.Context, path string) {
	key, err := h.gateway.Authenticate(c.Request)
	if err != nil {
		gateway.WriteAuthError(c.Writer, err)
		return
	}
	h.gateway.Proxy(c.Writer, c.Request, path, key, "")
}

func (h *Handler) v1Chat(c *gin.Context)        { authProxy(h, c, "/v1/chat/completions") }
func (h *Handler) v1Completions(c *gin.Context) { authProxy(h, c, "/v1/completions") }
func (h *Handler) v1Embeddings(c *gin.Context)  { authProxy(h, c, "/v1/embeddings") }
