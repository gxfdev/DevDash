package store

import (
	"database/sql"
	"encoding/json"
	"log"
	"strconv"
	"time"

	"devdash/internal/config"
	"devdash/internal/model"
	"devdash/internal/software"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

func NewStore(cfg interface{}) *Store {
	dbPath := "./devdash.db"
	if c, ok := cfg.(*config.Config); ok && c != nil {
		dbPath = c.DBPath
	}
	
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		log.Fatal(err)
	}
	
	// ✅ 配置数据库连接池
	db.SetMaxOpenConns(10)           // 最大打开连接数
	db.SetMaxIdleConns(5)            // 最大空闲连接数
	db.SetConnMaxLifetime(30 * time.Minute)  // 连接最大存活时间
	
	// 启用WAL模式提升并发性能
	db.Exec(`PRAGMA journal_mode=WAL`)
	// 设置超时时间
	db.Exec(`PRAGMA busy_timeout=5000`)
	db.Exec(`CREATE TABLE IF NOT EXISTS nodes (
		id TEXT PRIMARY KEY, name TEXT, os TEXT, arch TEXT, ip TEXT,
		role TEXT, token TEXT, status TEXT, last_heartbeat DATETIME, created_at DATETIME)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_nodes_status ON nodes(status)`)
	
	db.Exec(`CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY, username TEXT UNIQUE, password_hash TEXT, role TEXT, otp_enabled INTEGER)`)
	
	db.Exec(`CREATE TABLE IF NOT EXISTS snapshots (
		id INTEGER PRIMARY KEY AUTOINCREMENT, node_id TEXT, timestamp DATETIME,
		cpu REAL, mem REAL, disk REAL, net_recv REAL, net_sent REAL, load1 REAL,
		snapshot_json TEXT)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_snapshots_node_time ON snapshots(node_id, timestamp DESC)`)
	
	db.Exec(`CREATE TABLE IF NOT EXISTS alerts (
		id INTEGER PRIMARY KEY AUTOINCREMENT, node_id TEXT, node_name TEXT,
		metric TEXT, level TEXT, message TEXT,
		value REAL, threshold REAL, time DATETIME, status TEXT)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_alerts_node_status ON alerts(node_id, status)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_alerts_metric ON alerts(metric)`)
	db.Exec(`CREATE TABLE IF NOT EXISTS audit_logs (
		id INTEGER PRIMARY KEY AUTOINCREMENT, user_id INT, node_id TEXT,
		action TEXT, detail TEXT, result TEXT, time DATETIME)`)
	db.Exec(`CREATE TABLE IF NOT EXISTS software (
		id INTEGER PRIMARY KEY AUTOINCREMENT, node_id TEXT, name TEXT, version TEXT, status TEXT)`)
	db.Exec(`CREATE TABLE IF NOT EXISTS cron_jobs (
		id INTEGER PRIMARY KEY AUTOINCREMENT, node_id TEXT, name TEXT, expression TEXT, command TEXT, enabled INTEGER)`)
	db.Exec(`CREATE TABLE IF NOT EXISTS alert_rules (
		id INTEGER PRIMARY KEY AUTOINCREMENT, metric TEXT, op TEXT, threshold REAL,
		level TEXT, channels TEXT, enabled INTEGER DEFAULT 1)`)
	db.Exec(`CREATE TABLE IF NOT EXISTS db_connections (
		id INTEGER PRIMARY KEY AUTOINCREMENT, node_id TEXT, name TEXT, type TEXT,
		host TEXT, port INTEGER, user TEXT, password TEXT, dbname TEXT)`)
	db.Exec(`CREATE TABLE IF NOT EXISTS settings (
		key TEXT PRIMARY KEY, value TEXT)`)
	db.Exec(`INSERT OR IGNORE INTO users (username, password_hash, role) VALUES ('admin', '$2a$10$iKWgfHOOvvXaekbLOn5wIeFWcv/LUDHnUmuRYaNiMD3FFAbOFYe5m', 'admin')`)
	return &Store{db: db}
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) SaveSnapshot(snap *model.Snapshot) {
	if snap == nil {
		return
	}
	jsonData, _ := json.Marshal(snap)
	s.db.Exec(`INSERT INTO snapshots (node_id, timestamp, cpu, mem, disk, net_recv, net_sent, load1, snapshot_json)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		snap.NodeID, snap.Timestamp,
		snap.CPU.UsagePercent, snap.Memory.UsagePercent, snap.Disk.UsagePercent,
		snap.Network.BytesRecv, snap.Network.BytesSent,
		snap.Load.Load1, string(jsonData))
}

func (s *Store) ListSnapshots(nodeID string, limit int) []interface{} {
	if limit <= 0 {
		limit = 100
	}
	var rows *sql.Rows
	var err error
	if nodeID != "" {
		rows, err = s.db.Query(`SELECT snapshot_json FROM snapshots WHERE node_id=? ORDER BY timestamp DESC LIMIT ?`, nodeID, limit)
	} else {
		rows, err = s.db.Query(`SELECT snapshot_json FROM snapshots ORDER BY timestamp DESC LIMIT ?`, limit)
	}
	if err != nil {
		log.Println("ListSnapshots error:", err)
		return []interface{}{}
	}
	defer rows.Close()
	var result []interface{}
	for rows.Next() {
		var jsonData string
		if err := rows.Scan(&jsonData); err != nil {
			continue
		}
		var m map[string]interface{}
		if err := json.Unmarshal([]byte(jsonData), &m); err == nil {
			result = append(result, m)
		}
	}
	return result
}

func (s *Store) GetUser(username string) (string, error) {
	var hash string
	err := s.db.QueryRow("SELECT password_hash FROM users WHERE username=?", username).Scan(&hash)
	return hash, err
}

func (s *Store) UpdatePassword(username, hash string) error {
	_, err := s.db.Exec("UPDATE users SET password_hash=? WHERE username=?", hash, username)
	return err
}

func (s *Store) GetUserByID(id int) (string, string, error) {
	var username, role string
	err := s.db.QueryRow("SELECT username, role FROM users WHERE id=?", id).Scan(&username, &role)
	return username, role, err
}

func (s *Store) CreateNode(node interface{}) error {
	m, ok := node.(map[string]interface{})
	if !ok {
		return nil
	}
	_, err := s.db.Exec(`INSERT OR REPLACE INTO nodes (id, name, os, arch, ip, role, token, status, last_heartbeat, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		m["id"], m["name"], m["os"], m["arch"], m["ip"], m["role"], m["token"],
		m["status"], timeNow(m["last_heartbeat"]), timeNow(m["created_at"]))
	return err
}

func (s *Store) ListNodes() []interface{} {
	rows, err := s.db.Query(`SELECT id, name, os, arch, ip, role, token, status, last_heartbeat, created_at FROM nodes`)
	if err != nil {
		log.Println("ListNodes error:", err)
		return []interface{}{}
	}
	defer rows.Close()
	var result []interface{}
	for rows.Next() {
		var id, name, os, arch, ip, role, token, status, lastHeartbeat, createdAt string
		if err := rows.Scan(&id, &name, &os, &arch, &ip, &role, &token, &status, &lastHeartbeat, &createdAt); err != nil {
			continue
		}
		result = append(result, map[string]interface{}{
			"id": id, "name": name, "os": os, "arch": arch, "ip": ip,
			"role": role, "token": token, "status": status,
			"last_heartbeat": lastHeartbeat, "created_at": createdAt,
		})
	}
	return result
}

func (s *Store) GetNode(id string) interface{} {
	row := s.db.QueryRow(`SELECT id, name, os, arch, ip, role, token, status, last_heartbeat, created_at FROM nodes WHERE id=?`, id)
	var nodeID, name, os, arch, ip, role, token, status, lastHeartbeat, createdAt string
	if err := row.Scan(&nodeID, &name, &os, &arch, &ip, &role, &token, &status, &lastHeartbeat, &createdAt); err != nil {
		return nil
	}
	return map[string]interface{}{
		"id": nodeID, "name": name, "os": os, "arch": arch, "ip": ip,
		"role": role, "token": token, "status": status,
		"last_heartbeat": lastHeartbeat, "created_at": createdAt,
	}
}

func (s *Store) DeleteNode(id string) error {
	_, err := s.db.Exec(`DELETE FROM nodes WHERE id=?`, id)
	return err
}

func (s *Store) UpdateNodeStatus(id, status string) {
	s.db.Exec(`UPDATE nodes SET status=? WHERE id=?`, status, id)
}

func (s *Store) SaveAlert(alert interface{}) error {
	m, ok := alert.(map[string]interface{})
	if !ok {
		return nil
	}
	_, err := s.db.Exec(`INSERT INTO alerts (node_id, node_name, metric, level, message, value, threshold, time, status)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		m["node_id"], m["node_name"], m["metric"], m["level"], m["message"],
		m["value"], m["threshold"], timeNow(m["time"]), m["status"])
	return err
}

func (s *Store) ListAlerts(nodeID string, limit int) []interface{} {
	if limit <= 0 {
		limit = 100
	}
	var rows *sql.Rows
	var err error
	if nodeID != "" {
		rows, err = s.db.Query(`SELECT id, node_id, node_name, metric, level, message, value, threshold, time, status FROM alerts WHERE node_id=? ORDER BY time DESC LIMIT ?`, nodeID, limit)
	} else {
		rows, err = s.db.Query(`SELECT id, node_id, node_name, metric, level, message, value, threshold, time, status FROM alerts ORDER BY time DESC LIMIT ?`, limit)
	}
	if err != nil {
		log.Println("ListAlerts error:", err)
		return []interface{}{}
	}
	defer rows.Close()
	var result []interface{}
	for rows.Next() {
		var id int
		var nid, nodeName, metric, level, message, status string
		var value, threshold float64
		var t string
		if err := rows.Scan(&id, &nid, &nodeName, &metric, &level, &message, &value, &threshold, &t, &status); err != nil {
			continue
		}
		ts := parseTime(t)
		result = append(result, map[string]interface{}{
			"id": id, "node_id": nid, "node_name": nodeName,
			"metric": metric, "level": level, "message": message,
			"value": value, "threshold": threshold,
			"created_at": ts, "resolved_at": nil, "status": status,
		})
	}
	return result
}

func (s *Store) ListActiveAlerts(nodeID string) []interface{} {
	var rows *sql.Rows
	var err error
	if nodeID != "" {
		rows, err = s.db.Query(`SELECT id, node_id, node_name, metric, level, message, value, threshold, time, status FROM alerts WHERE node_id=? AND status='firing' ORDER BY time DESC`, nodeID)
	} else {
		rows, err = s.db.Query(`SELECT id, node_id, node_name, metric, level, message, value, threshold, time, status FROM alerts WHERE status='firing' ORDER BY time DESC`)
	}
	if err != nil {
		return []interface{}{}
	}
	defer rows.Close()
	var result []interface{}
	for rows.Next() {
		var id int
		var nid, nodeName, metric, level, message, status string
		var value, threshold float64
		var t string
		if err := rows.Scan(&id, &nid, &nodeName, &metric, &level, &message, &value, &threshold, &t, &status); err != nil {
			continue
		}
		ts := parseTime(t)
		result = append(result, map[string]interface{}{
			"id": id, "node_id": nid, "node_name": nodeName,
			"metric": metric, "level": level, "message": message,
			"value": value, "threshold": threshold,
			"created_at": ts, "status": status,
		})
	}
	return result
}

func (s *Store) SilenceAlert(id int) error {
	_, err := s.db.Exec(`UPDATE alerts SET status='silenced' WHERE id=?`, id)
	return err
}

func (s *Store) SaveAlertRule(rule interface{}) error {
	m, ok := rule.(map[string]interface{})
	if !ok {
		return nil
	}
	channels, _ := json.Marshal(m["channels"])
	id, _ := m["id"].(float64)
	if id > 0 {
		_, err := s.db.Exec(`UPDATE alert_rules SET metric=?, op=?, threshold=?, level=?, channels=?, enabled=? WHERE id=?`,
			m["metric"], m["op"], m["threshold"], m["level"], string(channels), 1, int(id))
		return err
	}
	res, err := s.db.Exec(`INSERT INTO alert_rules (metric, op, threshold, level, channels, enabled) VALUES (?, ?, ?, ?, ?, ?)`,
		m["metric"], m["op"], m["threshold"], m["level"], string(channels), 1)
	if err != nil {
		return err
	}
	newID, _ := res.LastInsertId()
	m["id"] = float64(newID)
	return nil
}

func (s *Store) ListAlertRules() []interface{} {
	rows, err := s.db.Query(`SELECT id, metric, op, threshold, level, channels, enabled FROM alert_rules`)
	if err != nil {
		return []interface{}{}
	}
	defer rows.Close()
	var result []interface{}
	for rows.Next() {
		var id int
		var metric, op, level, channelsStr string
		var threshold float64
		var enabled int
		if err := rows.Scan(&id, &metric, &op, &threshold, &level, &channelsStr, &enabled); err != nil {
			continue
		}
		var channels []string
		json.Unmarshal([]byte(channelsStr), &channels)
		result = append(result, map[string]interface{}{
			"id": id, "metric": metric, "op": op,
			"threshold": threshold, "level": level,
			"channels": channels, "enabled": enabled == 1,
		})
	}
	return result
}

func (s *Store) DeleteAlertRule(id int) error {
	_, err := s.db.Exec(`DELETE FROM alert_rules WHERE id=?`, id)
	return err
}

func (s *Store) SaveAuditLog(log interface{}) error {
	m, ok := log.(map[string]interface{})
	if !ok {
		return nil
	}
	_, err := s.db.Exec(`INSERT INTO audit_logs (user_id, node_id, action, detail, result, time)
		VALUES (?, ?, ?, ?, ?, ?)`,
		m["user_id"], m["node_id"], m["action"], m["detail"], m["result"],
		timeNow(m["time"]))
	return err
}

func (s *Store) SaveSoftware(sw interface{}) error {
	m, ok := sw.(map[string]interface{})
	if !ok {
		return nil
	}
	_, err := s.db.Exec(`INSERT OR REPLACE INTO software (node_id, name, version, status)
		VALUES (?, ?, ?, ?)`,
		m["node_id"], m["name"], m["version"], m["status"])
	return err
}

func (s *Store) ListSoftware(nodeID string) []interface{} {
	var rows *sql.Rows
	var err error
	if nodeID != "" {
		rows, err = s.db.Query(`SELECT id, node_id, name, version, status FROM software WHERE node_id=?`, nodeID)
	} else {
		rows, err = s.db.Query(`SELECT id, node_id, name, version, status FROM software`)
	}
	if err != nil {
		log.Println("ListSoftware error:", err)
		return []interface{}{}
	}
	defer rows.Close()
	var result []interface{}
	for rows.Next() {
		var id int
		var nid, name, version, status string
		if err := rows.Scan(&id, &nid, &name, &version, &status); err != nil {
			continue
		}
		running := false
		if status == "installed" || status == "running" {
			running = software.IsServiceRunning(nid, name)
		}
		nodeName := nid
		if n := s.GetNode(nid); n != nil {
			if nm, ok := n.(map[string]interface{}); ok {
				if nn, ok := nm["name"].(string); ok && nn != "" {
					nodeName = nn
				}
			}
		}
		result = append(result, map[string]interface{}{
			"id": id, "node_id": nid, "node_name": nodeName,
			"name": name, "version": version, "status": status, "running": running,
		})
	}
	return result
}

func (s *Store) DeleteSoftware(nodeID, name string) error {
	_, err := s.db.Exec(`DELETE FROM software WHERE node_id=? AND name=?`, nodeID, name)
	return err
}

func (s *Store) SaveCronJob(job interface{}) error {
	m, ok := job.(map[string]interface{})
	if !ok {
		return nil
	}
	id, _ := m["id"].(float64)
	if id > 0 {
		_, err := s.db.Exec(`UPDATE cron_jobs SET name=?, expression=?, command=?, enabled=? WHERE id=?`,
			m["name"], m["expression"], m["command"], m["enabled"], int(id))
		return err
	}
	res, err := s.db.Exec(`INSERT INTO cron_jobs (node_id, name, expression, command, enabled) VALUES (?, ?, ?, ?, ?)`,
		m["node_id"], m["name"], m["expression"], m["command"], 1)
	if err != nil {
		return err
	}
	newID, _ := res.LastInsertId()
	m["id"] = float64(newID)
	return nil
}

func (s *Store) ListCronJobs(nodeID string) []interface{} {
	var rows *sql.Rows
	var err error
	if nodeID != "" {
		rows, err = s.db.Query(`SELECT id, node_id, name, expression, command, enabled FROM cron_jobs WHERE node_id=?`, nodeID)
	} else {
		rows, err = s.db.Query(`SELECT id, node_id, name, expression, command, enabled FROM cron_jobs`)
	}
	if err != nil {
		log.Println("ListCronJobs error:", err)
		return []interface{}{}
	}
	defer rows.Close()
	var result []interface{}
	for rows.Next() {
		var id int
		var nid, name, expr, cmd string
		var enabled int
		if err := rows.Scan(&id, &nid, &name, &expr, &cmd, &enabled); err != nil {
			continue
		}
		result = append(result, map[string]interface{}{
			"id": id, "node_id": nid, "name": name,
			"expression": expr, "command": cmd, "enabled": enabled == 1,
		})
	}
	return result
}

func (s *Store) DeleteCronJob(id int) error {
	_, err := s.db.Exec(`DELETE FROM cron_jobs WHERE id=?`, id)
	return err
}

func (s *Store) SaveDBConnection(conn interface{}) error {
	m, ok := conn.(map[string]interface{})
	if !ok {
		return nil
	}
	id, _ := m["id"].(float64)
	if id > 0 {
		_, err := s.db.Exec(`UPDATE db_connections SET name=?, type=?, host=?, port=?, user=?, password=?, dbname=? WHERE id=?`,
			m["name"], m["type"], m["host"], toInt(m["port"]), m["user"], m["password"], m["dbname"], int(id))
		return err
	}
	res, err := s.db.Exec(`INSERT INTO db_connections (node_id, name, type, host, port, user, password, dbname) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		m["node_id"], m["name"], m["type"], m["host"], toInt(m["port"]), m["user"], m["password"], m["dbname"])
	if err != nil {
		return err
	}
	newID, _ := res.LastInsertId()
	m["id"] = float64(newID)
	return nil
}

func (s *Store) ListDBConnections(nodeID string) []interface{} {
	var rows *sql.Rows
	var err error
	if nodeID != "" {
		rows, err = s.db.Query(`SELECT id, node_id, name, type, host, port, user, password, dbname FROM db_connections WHERE node_id=?`, nodeID)
	} else {
		rows, err = s.db.Query(`SELECT id, node_id, name, type, host, port, user, password, dbname FROM db_connections`)
	}
	if err != nil {
		return []interface{}{}
	}
	defer rows.Close()
	var result []interface{}
	for rows.Next() {
		var id, port int
		var nid, name, dbType, host, user, password, dbname string
		if err := rows.Scan(&id, &nid, &name, &dbType, &host, &port, &user, &password, &dbname); err != nil {
			continue
		}
		result = append(result, map[string]interface{}{
			"id": id, "node_id": nid, "name": name, "type": dbType,
			"host": host, "port": port, "user": user, "password": "******", "dbname": dbname,
		})
	}
	return result
}

func (s *Store) GetDBConnection(id int) (map[string]interface{}, error) {
	row := s.db.QueryRow(`SELECT id, node_id, name, type, host, port, user, password, dbname FROM db_connections WHERE id=?`, id)
	var dbID, port int
	var nid, name, dbType, host, user, password, dbname string
	if err := row.Scan(&dbID, &nid, &name, &dbType, &host, &port, &user, &password, &dbname); err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"id": dbID, "node_id": nid, "name": name, "type": dbType,
		"host": host, "port": port, "user": user, "password": password, "dbname": dbname,
	}, nil
}

func (s *Store) DeleteDBConnection(id int) error {
	_, err := s.db.Exec(`DELETE FROM db_connections WHERE id=?`, id)
	return err
}

func (s *Store) GetSetting(key string) string {
	var val string
	err := s.db.QueryRow("SELECT value FROM settings WHERE key=?", key).Scan(&val)
	if err != nil {
		return ""
	}
	return val
}

func (s *Store) SetSetting(key, value string) error {
	_, err := s.db.Exec(`INSERT OR REPLACE INTO settings (key, value) VALUES (?, ?)`, key, value)
	return err
}

func timeNow(v interface{}) time.Time {
	if t, ok := v.(time.Time); ok {
		return t
	}
	return time.Now()
}

func toInt(v interface{}) int {
	switch val := v.(type) {
	case float64:
		return int(val)
	case int:
		return val
	case string:
		n, _ := strconv.Atoi(val)
		return n
	}
	return 0
}

func toFloat(v interface{}) float64 {
	switch val := v.(type) {
	case float64:
		return val
	case float32:
		return float64(val)
	case int:
		return float64(val)
	case int64:
		return float64(val)
	default:
		if m, ok := v.(map[string]interface{}); ok {
			if u, ok := m["usage_percent"].(float64); ok {
				return u
			}
		}
		return 0
	}
}

func toUint(v interface{}) uint64 {
	switch val := v.(type) {
	case uint64:
		return val
	case uint:
		return uint64(val)
	case int:
		if val >= 0 {
			return uint64(val)
		}
	case int64:
		if val >= 0 {
			return uint64(val)
		}
	case float64:
		if val >= 0 {
			return uint64(val)
		}
	}
	return 0
}

func parseTime(t string) int64 {
	if t == "" {
		return time.Now().Unix()
	}
	if ts, err := time.Parse(time.RFC3339, t); err == nil {
		return ts.Unix()
	}
	if ts, err := time.Parse("2006-01-02 15:04:05", t); err == nil {
		return ts.Unix()
	}
	if ts, err := time.Parse(time.UnixDate, t); err == nil {
		return ts.Unix()
	}
	return time.Now().Unix()
}
