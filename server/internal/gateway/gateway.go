package gateway

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/gxfdev/aidash/server/internal/model"
	"github.com/gxfdev/aidash/server/internal/orchestrator"
	"github.com/gxfdev/aidash/server/internal/store"
)

// Gateway exposes every running model service as an OpenAI-compatible
// endpoint with API-key auth, model routing and token metering.
type Gateway struct {
	store  *store.Store
	orch   *orchestrator.Orchestrator
	client *http.Client

	mu    sync.Mutex
	cache map[string]resolved
}

type resolved struct {
	base  string
	svcID string
	model string
	at    time.Time
}

func New(st *store.Store, orch *orchestrator.Orchestrator) *Gateway {
	return &Gateway{
		store:  st,
		orch:   orch,
		client: &http.Client{Timeout: 0},
		cache:  map[string]resolved{},
	}
}

// ---------- API keys ----------

func GenerateKey() (plain, prefix, hash, id string) {
	buf := make([]byte, 16)
	for i := range buf {
		buf[i] = keyAlphabet()[int(time.Now().UnixNano()>>uint(i%7))%len(keyAlphabet())]
	}
	plain = "sk-aidash-" + string(buf)
	id = "key_" + shortHash(plain)[:10]
	prefix = plain[:20]
	hash = HashKey(plain)
	return
}

func keyAlphabet() string {
	return "abcdefghijkmnopqrstuvwxyz23456789"
}

func HashKey(k string) string {
	sum := sha256.Sum256([]byte(k))
	return hex.EncodeToString(sum[:])
}

func shortHash(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

var (
	ErrNoKey      = errors.New("缺少 API Key（Authorization: Bearer sk-aidash-…）")
	ErrBadKey     = errors.New("API Key 无效或已吊销")
	ErrQuota      = errors.New("该 API Key 的 Token 配额已用尽")
	ErrNoModel    = errors.New("没有可用的模型服务，请先在「模型广场」部署模型")
	ErrModelNotOn = errors.New("请求的模型未在运行")
)

func (g *Gateway) Authenticate(r *http.Request) (*model.APIKey, error) {
	raw := ""
	h := r.Header.Get("Authorization")
	if strings.HasPrefix(strings.ToLower(h), "bearer ") {
		raw = strings.TrimSpace(h[7:])
	}
	if raw == "" {
		raw = strings.TrimSpace(r.Header.Get("X-API-Key"))
	}
	if raw == "" {
		return nil, ErrNoKey
	}
	if !strings.HasPrefix(raw, "sk-aidash-") || len(raw) < 20 {
		return nil, ErrBadKey
	}
	prefix := raw[:20]
	key, err := g.store.FindAPIKeyByPrefix(prefix)
	if err != nil {
		return nil, ErrBadKey
	}
	if key.Revoked || key.KeyHash != HashKey(raw) {
		return nil, ErrBadKey
	}
	if key.TokenLimit > 0 {
		start := time.Now().Truncate(24*time.Hour).AddDate(0, 0, -30).Unix()
		totals, err := g.store.UsageForKeySince(key.ID, start)
		if err == nil && totals.TotalTokens >= key.TokenLimit {
			return nil, ErrQuota
		}
	}
	return key, nil
}

// ---------- model routing ----------

func (g *Gateway) resolveModel(ctx context.Context, name string) (*resolved, error) {
	services, err := g.store.ListServices()
	if err != nil {
		return nil, err
	}
	candidates := []model.Service{}
	for _, svc := range services {
		if svc.Kind != "model" || svc.Status != orchestrator.StatusRunning || svc.ContainerID == "" {
			continue
		}
		candidates = append(candidates, svc)
	}
	if len(candidates) == 0 {
		return nil, ErrNoModel
	}
	want := strings.ToLower(strings.TrimSpace(name))
	if want != "" {
		for _, svc := range candidates {
			if strings.EqualFold(svc.ModelName, want) || strings.EqualFold(svc.Name, want) ||
				strings.EqualFold(svc.CatalogID, want) {
				return g.resolveService(ctx, svc)
			}
		}
		// accept "provider/model"
		if i := strings.LastIndex(want, "/"); i >= 0 {
			suffix := want[i+1:]
			for _, svc := range candidates {
				if strings.EqualFold(svc.ModelName, suffix) || strings.EqualFold(svc.Name, suffix) {
					return g.resolveService(ctx, svc)
				}
			}
		}
		return nil, fmt.Errorf("%w: %s（可用模型：%s）", ErrModelNotOn, name, g.modelNames(candidates))
	}
	return g.resolveService(ctx, candidates[0])
}

func (g *Gateway) modelNames(list []model.Service) string {
	names := make([]string, 0, len(list))
	for _, svc := range list {
		if svc.ModelName != "" {
			names = append(names, svc.ModelName)
		} else {
			names = append(names, svc.Name)
		}
	}
	return strings.Join(names, ", ")
}

func (g *Gateway) resolveService(ctx context.Context, svc model.Service) (*resolved, error) {
	g.mu.Lock()
	c, ok := g.cache[svc.ID]
	g.mu.Unlock()
	if ok && time.Since(c.at) < 20*time.Second {
		return &c, nil
	}
	base, err := g.orch.ModelBaseURL(ctx, &svc)
	if err != nil {
		return nil, err
	}
	display := svc.ModelName
	if display == "" {
		display = svc.Name
	}
	r := resolved{base: base, svcID: svc.ID, model: display, at: time.Now()}
	g.mu.Lock()
	g.cache[svc.ID] = r
	g.mu.Unlock()
	return &r, nil
}

// ListModels returns the OpenAI /v1/models payload.
func (g *Gateway) ListModels(ctx context.Context) []map[string]any {
	services, _ := g.store.ListServices()
	out := []map[string]any{}
	for _, svc := range services {
		if svc.Kind != "model" || svc.Status != orchestrator.StatusRunning {
			continue
		}
		name := svc.ModelName
		if name == "" {
			name = svc.Name
		}
		out = append(out, map[string]any{
			"id":       name,
			"object":   "model",
			"created":  svc.CreatedAt.Unix(),
			"owned_by": "aidash",
			"status":   svc.Status,
		})
	}
	return out
}

// ---------- proxy ----------

// Proxy forwards an OpenAI-compatible request to the target model and
// meters token usage. key may be nil for panel-originated traffic.
func (g *Gateway) Proxy(w http.ResponseWriter, r *http.Request, path string, key *model.APIKey, actor string) {
	ctx := r.Context()
	body, err := io.ReadAll(io.LimitReader(r.Body, 16<<20))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "读取请求失败: "+err.Error())
		return
	}
	var payload map[string]any
	if len(body) > 0 {
		if err := json.Unmarshal(body, &payload); err != nil {
			writeErr(w, http.StatusBadRequest, "请求体必须是 JSON")
			return
		}
	}

	requested, _ := payload["model"].(string)
	target, err := g.resolveModel(ctx, requested)
	if err != nil {
		code := http.StatusBadGateway
		if errors.Is(err, ErrModelNotOn) || errors.Is(err, ErrNoModel) {
			code = http.StatusNotFound
		}
		writeErr(w, code, err.Error())
		return
	}

	upstream := strings.TrimSuffix(target.base, "/") + path
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, upstream, bytes.NewReader(body))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", r.Header.Get("Accept"))
	if key != nil && key.Name != "" {
		req.Header.Set("X-Aidash-Key", key.Name)
	}

	start := time.Now()
	resp, err := g.client.Do(req)
	if err != nil {
		g.record(key, actor, target, 0, 0, http.StatusBadGateway, start)
		writeErr(w, http.StatusBadGateway, "调用模型失败: "+err.Error())
		return
	}
	defer resp.Body.Close()

	stream, _ := payload["stream"].(bool)
	if resp.StatusCode >= 400 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
		g.record(key, actor, target, 0, 0, resp.StatusCode, start)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(resp.StatusCode)
		_, _ = w.Write(raw)
		return
	}

	if stream {
		flusher, _ := w.(http.Flusher)
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-Accel-Buffering", "no")
		w.WriteHeader(http.StatusOK)
		var completionChars int
		buf := make([]byte, 4096)
		for {
			n, readErr := resp.Body.Read(buf)
			if n > 0 {
				_, _ = w.Write(buf[:n])
				if flusher != nil {
					flusher.Flush()
				}
				completionChars += countContentChars(buf[:n])
			}
			if readErr != nil {
				break
			}
		}
		promptTokens := estimateTokens(string(body))
		completionTokens := int64(float64(completionChars)/1.6) + 1
		g.record(key, actor, target, promptTokens, completionTokens, http.StatusOK, start)
		return
	}

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		writeErr(w, http.StatusBadGateway, "读取模型响应失败: "+err.Error())
		return
	}
	promptTokens, completionTokens := int64(0), int64(0)
	var parsed struct {
		Usage *struct {
			PromptTokens     int64 `json:"prompt_tokens"`
			CompletionTokens int64 `json:"completion_tokens"`
		} `json:"usage"`
	}
	if json.Unmarshal(raw, &parsed) == nil && parsed.Usage != nil {
		promptTokens = parsed.Usage.PromptTokens
		completionTokens = parsed.Usage.CompletionTokens
	}
	if promptTokens == 0 {
		promptTokens = estimateTokens(string(body))
		completionTokens = estimateTokens(string(raw))
	}
	g.record(key, actor, target, promptTokens, completionTokens, resp.StatusCode, start)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	_, _ = w.Write(raw)
}

func (g *Gateway) record(key *model.APIKey, actor string, target *resolved, prompt, completion int64, status int, start time.Time) {
	u := &model.TokenUsage{
		ServiceID:        target.svcID,
		Model:            target.model,
		PromptTokens:     prompt,
		CompletionTokens: completion,
		TotalTokens:      prompt + completion,
		Status:           status,
		LatencyMS:        time.Since(start).Milliseconds(),
	}
	if key != nil {
		u.KeyID = key.ID
		u.KeyName = key.Name
		g.store.TouchAPIKey(key.ID)
	} else {
		u.KeyName = actor
		if u.KeyName == "" {
			u.KeyName = "控制台"
		}
	}
	_ = g.store.InsertUsage(u)
}

// ---------- raw model call (used by the Copilot agent loop) ----------

// ChatRaw performs a non-streaming chat completion against a resolved
// model service and returns the decoded response body.
func (g *Gateway) ChatRaw(ctx context.Context, modelName string, body map[string]any) (map[string]any, error) {
	target, err := g.resolveModel(ctx, modelName)
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimSuffix(target.base, "/")+"/v1/chat/completions", bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := g.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		msg := strings.TrimSpace(string(data))
		if len(msg) > 500 {
			msg = msg[:500]
		}
		return nil, fmt.Errorf("模型返回错误 %d: %s", resp.StatusCode, msg)
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	// meter console traffic
	u := &model.TokenUsage{ServiceID: target.svcID, Model: target.model, KeyName: "Copilot", Status: resp.StatusCode}
	if usage, ok := out["usage"].(map[string]any); ok {
		u.PromptTokens = toInt64(usage["prompt_tokens"])
		u.CompletionTokens = toInt64(usage["completion_tokens"])
		u.TotalTokens = u.PromptTokens + u.CompletionTokens
	}
	if u.TotalTokens == 0 {
		u.PromptTokens = estimateTokens(string(raw))
	}
	_ = g.store.InsertUsage(u)
	return out, nil
}

func toInt64(v any) int64 {
	switch n := v.(type) {
	case float64:
		return int64(n)
	case int64:
		return n
	case int:
		return int64(n)
	case json.Number:
		i, _ := n.Int64()
		return i
	}
	return 0
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]any{"message": msg, "type": "aidash_error", "code": code},
	})
}

// ---------- token estimation ----------

func estimateTokens(text string) int64 {
	var cjk, other int64
	for _, r := range text {
		if r < 128 {
			other++
		} else if unicode.Is(unicode.Han, r) || unicode.Is(unicode.Hiragana, r) || unicode.Is(unicode.Katakana, r) || unicode.Is(unicode.Hangul, r) {
			cjk++
		} else {
			other += 2
		}
	}
	tokens := cjk + other/4
	if tokens < 1 {
		tokens = 1
	}
	return tokens
}

func countContentChars(chunk []byte) int {
	total := 0
	for _, line := range strings.Split(string(chunk), "\n") {
		line = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "data:"))
		if line == "" || line == "[DONE]" {
			continue
		}
		var payload struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
				Text string `json:"text"`
			} `json:"choices"`
		}
		if json.Unmarshal([]byte(line), &payload) == nil {
			for _, ch := range payload.Choices {
				total += len(ch.Delta.Content) + len(ch.Text)
			}
		}
	}
	return total
}
