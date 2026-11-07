package store

import (
	"database/sql"
	"strconv"
	"strings"

	"github.com/gxfdev/aidash/server/internal/model"
)

func (s *Store) InsertAudit(a *model.AuditLog) error {
	if a.CreatedAt.IsZero() {
		a.CreatedAt = parseTime(nowStr())
	}
	res, err := s.db.Exec(`INSERT INTO audit_logs(username,role,action,target,detail,ip,result,created_at) VALUES(?,?,?,?,?,?,?,?)`,
		a.User, a.Role, a.Action, a.Target, a.Detail, a.IP, a.Result, a.CreatedAt.UTC().Format(timeLayout))
	if err != nil {
		return err
	}
	a.ID, _ = res.LastInsertId()
	return nil
}

func (s *Store) ListAudit(q string, limit int) ([]model.AuditLog, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	args := []any{}
	where := ""
	if q = strings.TrimSpace(q); q != "" {
		where = ` WHERE username LIKE ? OR action LIKE ? OR target LIKE ? OR detail LIKE ?`
		like := "%" + q + "%"
		args = append(args, like, like, like, like)
	}
	args = append(args, limit)
	rows, err := s.db.Query(`SELECT id,username,role,action,target,detail,ip,result,created_at FROM audit_logs`+where+` ORDER BY id DESC LIMIT ?`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.AuditLog{}
	for rows.Next() {
		var a model.AuditLog
		var created string
		if err := rows.Scan(&a.ID, &a.User, &a.Role, &a.Action, &a.Target, &a.Detail, &a.IP, &a.Result, &created); err != nil {
			return nil, err
		}
		a.CreatedAt = parseTime(created)
		out = append(out, a)
	}
	return out, rows.Err()
}

// ---------- settings ----------

func (s *Store) GetSetting(key, def string) string {
	var v string
	err := s.db.QueryRow(`SELECT value FROM settings WHERE key=?`, key).Scan(&v)
	if err != nil {
		return def
	}
	return v
}

func (s *Store) SetSetting(key, value string) error {
	_, err := s.db.Exec(`INSERT INTO settings(key,value) VALUES(?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value`, key, value)
	return err
}

func (s *Store) GetSettingInt(key string, def int64) int64 {
	v := s.GetSetting(key, "")
	if v == "" {
		return def
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return def
	}
	return n
}

func (s *Store) GetSettingBool(key string, def bool) bool {
	v := s.GetSetting(key, "")
	if v == "" {
		return def
	}
	return v == "true" || v == "1"
}

// ---------- metrics ----------

func (s *Store) InsertMetric(m model.MetricPoint) {
	_, _ = s.db.Exec(`INSERT OR REPLACE INTO host_metrics(ts,cpu,memory,disk,net_rx,net_tx,gpu,gpu_mem) VALUES(?,?,?,?,?,?,?,?)`,
		m.TS, m.CPU, m.Memory, m.Disk, m.NetRX, m.NetTX, m.GPU, m.GPUMem)
}

func (s *Store) QueryMetrics(sinceTS int64, maxPoints int) ([]model.MetricPoint, error) {
	if maxPoints <= 0 {
		maxPoints = 240
	}
	rows, err := s.db.Query(`SELECT ts,cpu,memory,disk,net_rx,net_tx,gpu,gpu_mem FROM host_metrics WHERE ts>=? ORDER BY ts ASC`, sinceTS)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	all := []model.MetricPoint{}
	for rows.Next() {
		var p model.MetricPoint
		if err := rows.Scan(&p.TS, &p.CPU, &p.Memory, &p.Disk, &p.NetRX, &p.NetTX, &p.GPU, &p.GPUMem); err != nil {
			return nil, err
		}
		all = append(all, p)
	}
	if len(all) <= maxPoints {
		return all, nil
	}
	// down-sample evenly
	out := make([]model.MetricPoint, 0, maxPoints)
	step := float64(len(all)-1) / float64(maxPoints-1)
	for i := 0; i < maxPoints; i++ {
		out = append(out, all[int(float64(i)*step)])
	}
	return out, rows.Err()
}

// ---------- API keys ----------

func (s *Store) CreateAPIKey(k *model.APIKey) error {
	_, err := s.db.Exec(`INSERT INTO api_keys(id,name,prefix,key_hash,role,created_at,revoked,token_limit) VALUES(?,?,?,?,?,?,0,?)`,
		k.ID, k.Name, k.Prefix, k.KeyHash, k.Role, k.CreatedAt.UTC().Format(timeLayout), k.TokenLimit)
	return err
}

func (s *Store) ListAPIKeys() ([]model.APIKey, error) {
	rows, err := s.db.Query(`SELECT k.id,k.name,k.prefix,k.role,k.created_at,k.last_used_at,k.revoked,k.token_limit,
		COALESCE((SELECT SUM(total_tokens) FROM token_usage u WHERE u.key_id=k.id),0),
		COALESCE((SELECT COUNT(*) FROM token_usage u WHERE u.key_id=k.id),0)
		FROM api_keys k ORDER BY k.created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.APIKey{}
	for rows.Next() {
		var k model.APIKey
		var created string
		var last sql.NullString
		var revoked int
		if err := rows.Scan(&k.ID, &k.Name, &k.Prefix, &k.Role, &created, &last, &revoked, &k.TokenLimit, &k.TokensUsed, &k.RequestsUsed); err != nil {
			return nil, err
		}
		k.CreatedAt = parseTime(created)
		k.LastUsedAt = parseNullTime(last)
		k.Revoked = revoked == 1
		out = append(out, k)
	}
	return out, rows.Err()
}

func (s *Store) FindAPIKeyByPrefix(prefix string) (*model.APIKey, error) {
	row := s.db.QueryRow(`SELECT id,name,prefix,key_hash,role,created_at,last_used_at,revoked,token_limit FROM api_keys WHERE prefix=?`, prefix)
	var k model.APIKey
	var created string
	var last sql.NullString
	var revoked int
	err := row.Scan(&k.ID, &k.Name, &k.Prefix, &k.KeyHash, &k.Role, &created, &last, &revoked, &k.TokenLimit)
	if err != nil {
		return nil, err
	}
	k.CreatedAt = parseTime(created)
	k.LastUsedAt = parseNullTime(last)
	k.Revoked = revoked == 1
	return &k, nil
}

func (s *Store) RevokeAPIKey(id string) error {
	_, err := s.db.Exec(`DELETE FROM api_keys WHERE id=?`, id)
	return err
}

func (s *Store) TouchAPIKey(id string) {
	_, _ = s.db.Exec(`UPDATE api_keys SET last_used_at=? WHERE id=?`, nowStr(), id)
}

func (s *Store) InsertUsage(u *model.TokenUsage) error {
	if u.TS.IsZero() {
		u.TS = parseTime(nowStr())
	}
	res, err := s.db.Exec(`INSERT INTO token_usage(ts,key_id,key_name,service_id,model,prompt_tokens,completion_tokens,total_tokens,status,latency_ms) VALUES(?,?,?,?,?,?,?,?,?,?)`,
		u.TS.Unix(), u.KeyID, u.KeyName, u.ServiceID, u.Model, u.PromptTokens, u.CompletionTokens, u.TotalTokens, u.Status, u.LatencyMS)
	if err != nil {
		return err
	}
	u.ID, _ = res.LastInsertId()
	return nil
}

type UsageTotals struct {
	Requests         int64 `json:"requests"`
	PromptTokens     int64 `json:"prompt_tokens"`
	CompletionTokens int64 `json:"completion_tokens"`
	TotalTokens      int64 `json:"total_tokens"`
}

func (s *Store) UsageTotalsSince(ts int64) (UsageTotals, error) {
	var t UsageTotals
	row := s.db.QueryRow(`SELECT COUNT(*),COALESCE(SUM(prompt_tokens),0),COALESCE(SUM(completion_tokens),0),COALESCE(SUM(total_tokens),0) FROM token_usage WHERE ts>=?`, ts)
	err := row.Scan(&t.Requests, &t.PromptTokens, &t.CompletionTokens, &t.TotalTokens)
	return t, err
}

type UsageSeriesPoint struct {
	TS         int64 `json:"ts"`
	Requests   int64 `json:"requests"`
	Tokens     int64 `json:"tokens"`
	Prompt     int64 `json:"prompt_tokens"`
	Completion int64 `json:"completion_tokens"`
}

func (s *Store) UsageSeries(sinceTS int64) ([]UsageSeriesPoint, error) {
	rows, err := s.db.Query(`SELECT ts,prompt_tokens,completion_tokens,total_tokens FROM token_usage WHERE ts>=? ORDER BY ts ASC`, sinceTS)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	buckets := map[int64]*UsageSeriesPoint{}
	order := []int64{}
	for rows.Next() {
		var ts int64
		var p, c, t int64
		if err := rows.Scan(&ts, &p, &c, &t); err != nil {
			return nil, err
		}
		b := (ts / 3600) * 3600
		bp, ok := buckets[b]
		if !ok {
			bp = &UsageSeriesPoint{TS: b}
			buckets[b] = bp
			order = append(order, b)
		}
		bp.Requests++
		bp.Tokens += t
		bp.Prompt += p
		bp.Completion += c
	}
	out := make([]UsageSeriesPoint, 0, len(order))
	for _, b := range order {
		out = append(out, *buckets[b])
	}
	return out, rows.Err()
}

func (s *Store) ModelUsage() ([]map[string]any, error) {
	rows, err := s.db.Query(`SELECT model,COUNT(*),COALESCE(SUM(total_tokens),0) FROM token_usage GROUP BY model ORDER BY 3 DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var m string
		var req, tok int64
		if err := rows.Scan(&m, &req, &tok); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{"model": m, "requests": req, "tokens": tok})
	}
	return out, rows.Err()
}

func (s *Store) UsageForKeySince(keyID string, ts int64) (UsageTotals, error) {
	var t UsageTotals
	row := s.db.QueryRow(`SELECT COUNT(*),COALESCE(SUM(prompt_tokens),0),COALESCE(SUM(completion_tokens),0),COALESCE(SUM(total_tokens),0) FROM token_usage WHERE key_id=? AND ts>=?`, keyID, ts)
	err := row.Scan(&t.Requests, &t.PromptTokens, &t.CompletionTokens, &t.TotalTokens)
	return t, err
}
