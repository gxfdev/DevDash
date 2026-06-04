package store

import (
	"database/sql"
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"time"

	"github.com/gxfdev/DevDash/server/internal/config"
	"github.com/gxfdev/DevDash/server/internal/model"
	"golang.org/x/crypto/bcrypt"

	_ "github.com/lib/pq"
	_ "modernc.org/sqlite"
)

type Store struct {
	db     *sql.DB
	dbType config.DBType
	dbPath string
}

func NewStore(cfg *config.Config) *Store {
	s := &Store{dbType: cfg.DBType}
	var db *sql.DB
	var err error

	switch cfg.DBType {
	case config.DBPostgreSQL:
		dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
			cfg.DBHost, cfg.DBPort, cfg.DBUser, cfg.DBPassword, cfg.DBName, cfg.DBSSLMode)
		db, err = sql.Open("postgres", dsn)
		if err != nil {
			log.Fatalf("[store] failed to open PostgreSQL: %v", err)
		}
		db.SetMaxOpenConns(25)
		db.SetMaxIdleConns(10)
		db.SetConnMaxLifetime(30 * time.Minute)
		db.SetConnMaxIdleTime(5 * time.Minute)
		if err = db.Ping(); err != nil {
			log.Fatalf("[store] failed to ping PostgreSQL: %v", err)
		}
		log.Println("[store] connected to PostgreSQL")

	default:
		dbPath := cfg.DBPath
		if dbPath == "" {
			dbPath = "devdash.db"
		}
		s.dbPath = dbPath
		db, err = sql.Open("sqlite", dbPath+"?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=on")
		if err != nil {
			log.Fatalf("[store] failed to open SQLite: %v", err)
		}
		db.SetMaxOpenConns(1)
		db.SetMaxIdleConns(1)
		db.SetConnMaxLifetime(0)
		if err = db.Ping(); err != nil {
			log.Fatalf("[store] failed to ping SQLite: %v", err)
		}
		if _, err = db.Exec("PRAGMA journal_mode=WAL"); err != nil {
			log.Printf("[store] warning: failed to set WAL mode: %v", err)
		}
		if _, err = db.Exec("PRAGMA busy_timeout=5000"); err != nil {
			log.Printf("[store] warning: failed to set busy_timeout: %v", err)
		}
		if _, err = db.Exec("PRAGMA synchronous=NORMAL"); err != nil {
			log.Printf("[store] warning: failed to set synchronous: %v", err)
		}
		log.Println("[store] connected to SQLite with WAL mode")
	}

	s.db = db
	s.runMigrations()
	return s
}

func (s *Store) Close() {
	if s.db != nil {
		s.db.Close()
	}
}

func (s *Store) Reopen() error {
	if s.dbType == config.DBPostgreSQL || s.dbPath == "" {
		return fmt.Errorf("reopen only supported for SQLite")
	}
	if s.db != nil {
		s.db.Close()
	}
	db, err := sql.Open("sqlite", s.dbPath+"?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=on")
	if err != nil {
		return fmt.Errorf("reopen sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)
	if err = db.Ping(); err != nil {
		return fmt.Errorf("ping after reopen: %w", err)
	}
	db.Exec("PRAGMA journal_mode=WAL")
	db.Exec("PRAGMA busy_timeout=5000")
	db.Exec("PRAGMA synchronous=NORMAL")
	s.db = db
	log.Println("[store] database reopened after restore")
	return nil
}

func (s *Store) DB() *sql.DB { return s.db }

func (s *Store) IsPostgreSQL() bool { return s.dbType == config.DBPostgreSQL }

func (s *Store) runMigrations() {
	migrations := []struct {
		name string
		sql  string
		pg   string
	}{
		{
			name: "metrics",
			sql: `CREATE TABLE IF NOT EXISTS metrics (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				node_id TEXT NOT NULL,
				timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
				cpu_usage REAL DEFAULT 0,
				cpu_cores INTEGER DEFAULT 0,
				mem_total_gb REAL DEFAULT 0,
				mem_used_gb REAL DEFAULT 0,
				mem_usage_percent REAL DEFAULT 0,
				disk_total_gb REAL DEFAULT 0,
				disk_used_gb REAL DEFAULT 0,
				disk_usage_percent REAL DEFAULT 0,
				disk_read_mb REAL DEFAULT 0,
				disk_write_mb REAL DEFAULT 0,
				disk_iops REAL DEFAULT 0,
				net_bytes_recv INTEGER DEFAULT 0,
				net_bytes_sent INTEGER DEFAULT 0,
				load1 REAL DEFAULT 0,
				load5 REAL DEFAULT 0,
				load15 REAL DEFAULT 0
			)`,
			pg: `CREATE TABLE IF NOT EXISTS metrics (
				id SERIAL PRIMARY KEY,
				node_id TEXT NOT NULL,
				timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
				cpu_usage REAL DEFAULT 0,
				cpu_cores INTEGER DEFAULT 0,
				mem_total_gb REAL DEFAULT 0,
				mem_used_gb REAL DEFAULT 0,
				mem_usage_percent REAL DEFAULT 0,
				disk_total_gb REAL DEFAULT 0,
				disk_used_gb REAL DEFAULT 0,
				disk_usage_percent REAL DEFAULT 0,
				disk_read_mb REAL DEFAULT 0,
				disk_write_mb REAL DEFAULT 0,
				disk_iops REAL DEFAULT 0,
				net_bytes_recv BIGINT DEFAULT 0,
				net_bytes_sent BIGINT DEFAULT 0,
				load1 REAL DEFAULT 0,
				load5 REAL DEFAULT 0,
				load15 REAL DEFAULT 0
			)`,
		},
		{
			name: "metrics_gpu",
			sql: `CREATE TABLE IF NOT EXISTS metrics_gpu (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				node_id TEXT NOT NULL,
				timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
				gpu_index INTEGER DEFAULT 0,
				usage_percent REAL DEFAULT 0,
				mem_used_mb REAL DEFAULT 0,
				mem_total_mb REAL DEFAULT 0,
				temperature_celsius REAL DEFAULT 0
			)`,
			pg: `CREATE TABLE IF NOT EXISTS metrics_gpu (
				id SERIAL PRIMARY KEY,
				node_id TEXT NOT NULL,
				timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
				gpu_index INTEGER DEFAULT 0,
				usage_percent REAL DEFAULT 0,
				mem_used_mb REAL DEFAULT 0,
				mem_total_mb REAL DEFAULT 0,
				temperature_celsius REAL DEFAULT 0
			)`,
		},
		{
			name: "alerts",
			sql: `CREATE TABLE IF NOT EXISTS alerts (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				node_id TEXT NOT NULL,
				node_name TEXT DEFAULT '',
				type TEXT NOT NULL,
				level TEXT DEFAULT 'warning',
				value REAL DEFAULT 0,
				threshold REAL DEFAULT 0,
				time DATETIME DEFAULT CURRENT_TIMESTAMP,
				status TEXT DEFAULT 'active',
				message TEXT DEFAULT ''
			)`,
			pg: `CREATE TABLE IF NOT EXISTS alerts (
				id SERIAL PRIMARY KEY,
				node_id TEXT NOT NULL,
				node_name TEXT DEFAULT '',
				type TEXT NOT NULL,
				level TEXT DEFAULT 'warning',
				value REAL DEFAULT 0,
				threshold REAL DEFAULT 0,
				time TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
				status TEXT DEFAULT 'active',
				message TEXT DEFAULT ''
			)`,
		},
		{
			name: "users",
			sql: `CREATE TABLE IF NOT EXISTS users (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				username TEXT UNIQUE NOT NULL,
				password_hash TEXT NOT NULL,
				role TEXT DEFAULT 'viewer',
				otp_enabled INTEGER DEFAULT 0,
				must_change_pwd INTEGER DEFAULT 0
			)`,
			pg: `CREATE TABLE IF NOT EXISTS users (
				id SERIAL PRIMARY KEY,
				username TEXT UNIQUE NOT NULL,
				password_hash TEXT NOT NULL,
				role TEXT DEFAULT 'viewer',
				otp_enabled BOOLEAN DEFAULT FALSE,
				must_change_pwd BOOLEAN DEFAULT FALSE
			)`,
		},
		{
			name: "audit_logs",
			sql: `CREATE TABLE IF NOT EXISTS audit_logs (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				user_id INTEGER DEFAULT 0,
				node_id TEXT DEFAULT '',
				action TEXT NOT NULL,
				detail TEXT DEFAULT '',
				result TEXT DEFAULT '',
				time DATETIME DEFAULT CURRENT_TIMESTAMP
			)`,
			pg: `CREATE TABLE IF NOT EXISTS audit_logs (
				id SERIAL PRIMARY KEY,
				user_id INTEGER DEFAULT 0,
				node_id TEXT DEFAULT '',
				action TEXT NOT NULL,
				detail TEXT DEFAULT '',
				result TEXT DEFAULT '',
				time TIMESTAMP DEFAULT CURRENT_TIMESTAMP
			)`,
		},
		{
			name: "cron_jobs",
			sql: `CREATE TABLE IF NOT EXISTS cron_jobs (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				node_id TEXT NOT NULL,
				name TEXT NOT NULL,
				expression TEXT NOT NULL,
				command TEXT NOT NULL,
				enabled INTEGER DEFAULT 1
			)`,
			pg: `CREATE TABLE IF NOT EXISTS cron_jobs (
				id SERIAL PRIMARY KEY,
				node_id TEXT NOT NULL,
				name TEXT NOT NULL,
				expression TEXT NOT NULL,
				command TEXT NOT NULL,
				enabled BOOLEAN DEFAULT TRUE
			)`,
		},
		{
			name: "file_operations",
			sql: `CREATE TABLE IF NOT EXISTS file_operations (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				node_id TEXT NOT NULL DEFAULT 'self',
				operation TEXT NOT NULL,
				path TEXT NOT NULL,
				name TEXT DEFAULT '',
				ext TEXT DEFAULT '',
				size INTEGER DEFAULT 0,
				is_dir INTEGER DEFAULT 0,
				timestamp DATETIME DEFAULT CURRENT_TIMESTAMP
			)`,
			pg: `CREATE TABLE IF NOT EXISTS file_operations (
				id SERIAL PRIMARY KEY,
				node_id TEXT NOT NULL DEFAULT 'self',
				operation TEXT NOT NULL,
				path TEXT NOT NULL,
				name TEXT DEFAULT '',
				ext TEXT DEFAULT '',
				size BIGINT DEFAULT 0,
				is_dir BOOLEAN DEFAULT FALSE,
				timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP
			)`,
		},
	}

	for _, m := range migrations {
		sqlStr := m.sql
		if s.IsPostgreSQL() && m.pg != "" {
			sqlStr = m.pg
		}
		if _, err := s.db.Exec(sqlStr); err != nil {
			log.Fatalf("[store] migration %s failed: %v", m.name, err)
		}
	}

	alterCols := []string{
		"ALTER TABLE metrics ADD COLUMN disk_read_mb REAL DEFAULT 0",
		"ALTER TABLE metrics ADD COLUMN disk_write_mb REAL DEFAULT 0",
		"ALTER TABLE metrics ADD COLUMN disk_iops REAL DEFAULT 0",
		"ALTER TABLE metrics ADD COLUMN net_recv_rate_mb REAL DEFAULT 0",
		"ALTER TABLE metrics ADD COLUMN net_sent_rate_mb REAL DEFAULT 0",
		"ALTER TABLE metrics ADD COLUMN disk_read_rate_mb REAL DEFAULT 0",
		"ALTER TABLE metrics ADD COLUMN disk_write_rate_mb REAL DEFAULT 0",
		"ALTER TABLE cron_jobs ADD COLUMN type TEXT DEFAULT 'shell'",
		"ALTER TABLE cron_jobs ADD COLUMN last_run INTEGER DEFAULT 0",
	}
	for _, col := range alterCols {
		s.db.Exec(col)
	}

	s.createIndexes()
	s.seedDefaultUser()
}

func (s *Store) createIndexes() {
	indexes := []string{
		"CREATE INDEX IF NOT EXISTS idx_metrics_node_id ON metrics(node_id)",
		"CREATE INDEX IF NOT EXISTS idx_metrics_timestamp ON metrics(timestamp)",
		"CREATE INDEX IF NOT EXISTS idx_metrics_gpu_node_id ON metrics_gpu(node_id)",
		"CREATE INDEX IF NOT EXISTS idx_metrics_gpu_timestamp ON metrics_gpu(timestamp)",
		"CREATE INDEX IF NOT EXISTS idx_alerts_node_id ON alerts(node_id)",
		"CREATE INDEX IF NOT EXISTS idx_alerts_status ON alerts(status)",
		"CREATE INDEX IF NOT EXISTS idx_audit_logs_user_id ON audit_logs(user_id)",
		"CREATE INDEX IF NOT EXISTS idx_audit_logs_time ON audit_logs(time)",
	}
	for _, idx := range indexes {
		if _, err := s.db.Exec(idx); err != nil {
			log.Printf("[store] warning: index creation failed: %v", err)
		}
	}
}

func (s *Store) seedDefaultUser() {
	var count int
	s.db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
	if count == 0 {
		hash := hashPassword("admin123")
		var err error
		if s.IsPostgreSQL() {
			_, err = s.db.Exec("INSERT INTO users (username, password_hash, role, must_change_pwd) VALUES ($1, $2, $3, $4)", "admin", hash, "admin", true)
		} else {
			_, err = s.db.Exec("INSERT INTO users (username, password_hash, role, must_change_pwd) VALUES (?, ?, ?, ?)", "admin", hash, "admin", true)
		}
		if err != nil {
			log.Printf("[store] warning: failed to seed default user: %v", err)
		} else {
			log.Println("[store] seeded default admin user with must_change_pwd=true - password must be changed on first login")
		}
	}
	s.ensureColumn("users", "must_change_pwd", "INTEGER DEFAULT 0", "BOOLEAN DEFAULT FALSE")
	s.ensureColumn("alerts", "node_name", "TEXT DEFAULT ''", "TEXT DEFAULT ''")
	s.ensureColumn("alerts", "message", "TEXT DEFAULT ''", "TEXT DEFAULT ''")
}

func (s *Store) placeholder(n int) string {
	if s.IsPostgreSQL() {
		return fmt.Sprintf("$%d", n)
	}
	return "?"
}

func (s *Store) placeholders(n int) string {
	if s.IsPostgreSQL() {
		parts := make([]string, n)
		for i := 0; i < n; i++ {
			parts[i] = fmt.Sprintf("$%d", i+1)
		}
		return strings.Join(parts, ", ")
	}
	return strings.Repeat("?, ", n-1) + "?"
}

func (s *Store) SaveSnapshot(nodeID string, snap *model.Snapshot) error {
	now := time.Now()
	if !snap.Timestamp.IsZero() {
		now = snap.Timestamp
	}
	var diskReadMB, diskWriteMB, diskIOPS float64
	var diskReadRateMB, diskWriteRateMB float64
	if snap.DiskIO != nil {
		diskReadMB = snap.DiskIO.ReadMB
		diskWriteMB = snap.DiskIO.WriteMB
		diskIOPS = snap.DiskIO.IOPS
		diskReadRateMB = snap.DiskIO.ReadRateMB
		diskWriteRateMB = snap.DiskIO.WriteRateMB
	}
	_, err := s.db.Exec(
		fmt.Sprintf("INSERT INTO metrics (node_id, timestamp, cpu_usage, cpu_cores, mem_total_gb, mem_used_gb, mem_usage_percent, disk_total_gb, disk_used_gb, disk_usage_percent, disk_read_mb, disk_write_mb, disk_iops, net_bytes_recv, net_bytes_sent, net_recv_rate_mb, net_sent_rate_mb, disk_read_rate_mb, disk_write_rate_mb, load1, load5, load15) VALUES (%s)", s.placeholders(22)),
		nodeID, now, snap.CPU.UsagePercent, snap.CPU.Cores,
		snap.Memory.TotalGB, snap.Memory.UsedGB, snap.Memory.UsagePercent,
		snap.Disk.TotalGB, snap.Disk.UsedGB, snap.Disk.UsagePercent,
		diskReadMB, diskWriteMB, diskIOPS,
		snap.Network.BytesRecv, snap.Network.BytesSent,
		snap.Network.RecvRateMB, snap.Network.SentRateMB,
		diskReadRateMB, diskWriteRateMB,
		snap.Load.Load1, snap.Load.Load5, snap.Load.Load15,
	)
	if err != nil {
		return fmt.Errorf("save metrics: %w", err)
	}

	if snap.GPU != nil {
		for _, dev := range snap.GPU.Devices {
			_, gErr := s.db.Exec(
				fmt.Sprintf("INSERT INTO metrics_gpu (node_id, timestamp, gpu_index, usage_percent, mem_used_mb, mem_total_mb, temperature_celsius) VALUES (%s)", s.placeholders(7)),
				nodeID, now, dev.Index, dev.UsagePercent, dev.MemUsedMB, dev.MemTotalMB, dev.Temperature,
			)
			if gErr != nil {
				log.Printf("[store] warning: failed to save GPU metric for device %d: %v", dev.Index, gErr)
			}
		}
	}
	return nil
}

func (s *Store) SaveSnapshotBatch(nodeID string, snaps []*model.Snapshot) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(
		fmt.Sprintf("INSERT INTO metrics (node_id, timestamp, cpu_usage, cpu_cores, mem_total_gb, mem_used_gb, mem_usage_percent, disk_total_gb, disk_used_gb, disk_usage_percent, net_bytes_recv, net_bytes_sent, load1, load5, load15) VALUES (%s)", s.placeholders(15)),
	)
	if err != nil {
		return fmt.Errorf("prepare stmt: %w", err)
	}
	defer stmt.Close()

	var gpuStmt *sql.Stmt
	gpuStmt, err = tx.Prepare(
		fmt.Sprintf("INSERT INTO metrics_gpu (node_id, timestamp, gpu_index, usage_percent, mem_used_mb, mem_total_mb, temperature_celsius) VALUES (%s)", s.placeholders(7)),
	)
	if err != nil {
		log.Printf("[store] warning: prepare GPU stmt: %v", err)
	} else {
		defer gpuStmt.Close()
	}

	for _, snap := range snaps {
		now := snap.Timestamp
		if now.IsZero() {
			now = time.Now()
		}
		_, err = stmt.Exec(
			nodeID, now, snap.CPU.UsagePercent, snap.CPU.Cores,
			snap.Memory.TotalGB, snap.Memory.UsedGB, snap.Memory.UsagePercent,
			snap.Disk.TotalGB, snap.Disk.UsedGB, snap.Disk.UsagePercent,
			snap.Network.BytesRecv, snap.Network.BytesSent,
			snap.Load.Load1, snap.Load.Load5, snap.Load.Load15,
		)
		if err != nil {
			return fmt.Errorf("batch insert metrics: %w", err)
		}
		if snap.GPU != nil && gpuStmt != nil {
			for _, dev := range snap.GPU.Devices {
				_, gErr := gpuStmt.Exec(nodeID, now, dev.Index, dev.UsagePercent, dev.MemUsedMB, dev.MemTotalMB, dev.Temperature)
				if gErr != nil {
					log.Printf("[store] warning: batch GPU insert device %d: %v", dev.Index, gErr)
				}
			}
		}
	}
	return tx.Commit()
}

func (s *Store) GetMetricsHistory(nodeID string, hours int) ([]map[string]any, error) {
	since := time.Now().Add(-time.Duration(hours) * time.Hour)
	rows, err := s.db.Query(
		fmt.Sprintf("SELECT node_id, timestamp, cpu_usage, cpu_cores, mem_total_gb, mem_used_gb, mem_usage_percent, disk_total_gb, disk_used_gb, disk_usage_percent, disk_read_mb, disk_write_mb, disk_iops, disk_read_rate_mb, disk_write_rate_mb, net_bytes_recv, net_bytes_sent, net_recv_rate_mb, net_sent_rate_mb, load1, load5, load15 FROM metrics WHERE node_id = %s AND timestamp >= %s ORDER BY timestamp ASC", s.placeholder(1), s.placeholder(2)),
		nodeID, since,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []map[string]any
	for rows.Next() {
		var nid string
		var ts time.Time
		var cpuUsage, cpuCores, memTotal, memUsed, memUsage float64
		var diskTotal, diskUsed, diskUsage float64
		var diskReadMB, diskWriteMB, diskIOPS float64
		var diskReadRateMB, diskWriteRateMB float64
		var netRecv, netSent int64
		var netRecvRate, netSentRate float64
		var load1, load5, load15 float64
		if err := rows.Scan(&nid, &ts, &cpuUsage, &cpuCores, &memTotal, &memUsed, &memUsage, &diskTotal, &diskUsed, &diskUsage, &diskReadMB, &diskWriteMB, &diskIOPS, &diskReadRateMB, &diskWriteRateMB, &netRecv, &netSent, &netRecvRate, &netSentRate, &load1, &load5, &load15); err != nil {
			continue
		}
		result = append(result, map[string]any{
			"node_id":   nid,
			"timestamp": ts,
			"cpu": map[string]any{
				"usage_percent": cpuUsage,
				"cores":         cpuCores,
			},
			"memory": map[string]any{
				"total_gb":      memTotal,
				"used_gb":       memUsed,
				"usage_percent": memUsage,
			},
			"disk": map[string]any{
				"total_gb":      diskTotal,
				"used_gb":       diskUsed,
				"usage_percent": diskUsage,
			},
			"disk_io": map[string]any{
				"read_mb":       diskReadMB,
				"write_mb":      diskWriteMB,
				"iops":          diskIOPS,
				"read_rate_mb":  diskReadRateMB,
				"write_rate_mb": diskWriteRateMB,
			},
			"network": map[string]any{
				"bytes_recv":   netRecv,
				"bytes_sent":   netSent,
				"recv_rate_mb": netRecvRate,
				"sent_rate_mb": netSentRate,
			},
			"load": map[string]any{
				"load1":  load1,
				"load5":  load5,
				"load15": load15,
			},
		})
	}
	return result, nil
}

func (s *Store) GetMetricsHistoryRange(nodeID string, start, end time.Time) ([]map[string]any, error) {
	rows, err := s.db.Query(
		fmt.Sprintf("SELECT node_id, timestamp, cpu_usage, cpu_cores, mem_total_gb, mem_used_gb, mem_usage_percent, disk_total_gb, disk_used_gb, disk_usage_percent, disk_read_mb, disk_write_mb, disk_iops, disk_read_rate_mb, disk_write_rate_mb, net_bytes_recv, net_bytes_sent, net_recv_rate_mb, net_sent_rate_mb, load1, load5, load15 FROM metrics WHERE node_id = %s AND timestamp >= %s AND timestamp < %s ORDER BY timestamp ASC", s.placeholder(1), s.placeholder(2), s.placeholder(3)),
		nodeID, start, end,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []map[string]any
	for rows.Next() {
		var nid string
		var ts time.Time
		var cpuUsage, cpuCores, memTotal, memUsed, memUsage float64
		var diskTotal, diskUsed, diskUsage float64
		var diskReadMB, diskWriteMB, diskIOPS float64
		var diskReadRateMB, diskWriteRateMB float64
		var netRecv, netSent int64
		var netRecvRate, netSentRate float64
		var load1, load5, load15 float64
		if err := rows.Scan(&nid, &ts, &cpuUsage, &cpuCores, &memTotal, &memUsed, &memUsage, &diskTotal, &diskUsed, &diskUsage, &diskReadMB, &diskWriteMB, &diskIOPS, &diskReadRateMB, &diskWriteRateMB, &netRecv, &netSent, &netRecvRate, &netSentRate, &load1, &load5, &load15); err != nil {
			continue
		}
		result = append(result, map[string]any{
			"node_id":   nid,
			"timestamp": ts,
			"cpu": map[string]any{
				"usage_percent": cpuUsage,
				"cores":         cpuCores,
			},
			"memory": map[string]any{
				"total_gb":      memTotal,
				"used_gb":       memUsed,
				"usage_percent": memUsage,
			},
			"disk": map[string]any{
				"total_gb":      diskTotal,
				"used_gb":       diskUsed,
				"usage_percent": diskUsage,
			},
			"disk_io": map[string]any{
				"read_mb":       diskReadMB,
				"write_mb":      diskWriteMB,
				"iops":          diskIOPS,
				"read_rate_mb":  diskReadRateMB,
				"write_rate_mb": diskWriteRateMB,
			},
			"network": map[string]any{
				"bytes_recv":   netRecv,
				"bytes_sent":   netSent,
				"recv_rate_mb": netRecvRate,
				"sent_rate_mb": netSentRate,
			},
			"load": map[string]any{
				"load1":  load1,
				"load5":  load5,
				"load15": load15,
			},
		})
	}
	return fillGapsWithNull(result), nil
}

func (s *Store) GetGPUMetricsHistory(nodeID string, hours int) ([]model.GPUMetricRow, error) {
	since := time.Now().Add(-time.Duration(hours) * time.Hour)
	rows, err := s.db.Query(
		fmt.Sprintf("SELECT id, node_id, timestamp, gpu_index, usage_percent, mem_used_mb, mem_total_mb, temperature_celsius FROM metrics_gpu WHERE node_id = %s AND timestamp >= %s ORDER BY timestamp ASC", s.placeholder(1), s.placeholder(2)),
		nodeID, since,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []model.GPUMetricRow
	for rows.Next() {
		var r model.GPUMetricRow
		if err := rows.Scan(&r.ID, &r.NodeID, &r.Timestamp, &r.GPUIndex, &r.UsagePercent, &r.MemUsedMB, &r.MemTotalMB, &r.TemperatureC); err != nil {
			continue
		}
		result = append(result, r)
	}
	return result, nil
}

func (s *Store) GetUserByUsername(username string) (*model.User, error) {
	row := s.db.QueryRow(fmt.Sprintf("SELECT id, username, password_hash, role, otp_enabled, must_change_pwd FROM users WHERE username = %s", s.placeholder(1)), username)
	var u model.User
	var otpEnabled int
	var mustChangePwd int
	if err := row.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &otpEnabled, &mustChangePwd); err != nil {
		return nil, err
	}
	u.OTPEnabled = otpEnabled != 0
	u.MustChangePwd = mustChangePwd != 0
	return &u, nil
}

func (s *Store) GetAlerts(nodeID string, limit int) ([]model.Alert, error) {
	rows, err := s.db.Query(
		fmt.Sprintf("SELECT id, node_id, node_name, type, level, value, threshold, time, status, message FROM alerts WHERE node_id = %s ORDER BY time DESC LIMIT %s", s.placeholder(1), s.placeholder(2)),
		nodeID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var alerts []model.Alert
	for rows.Next() {
		var a model.Alert
		if err := rows.Scan(&a.ID, &a.NodeID, &a.NodeName, &a.Type, &a.Level, &a.Value, &a.Threshold, &a.Time, &a.Status, &a.Message); err != nil {
			continue
		}
		alerts = append(alerts, a)
	}
	return alerts, nil
}

func (s *Store) SaveAuditLog(a *model.AuditLog) error {
	_, err := s.db.Exec(
		fmt.Sprintf("INSERT INTO audit_logs (user_id, node_id, action, detail, result, time) VALUES (%s)", s.placeholders(6)),
		a.UserID, a.NodeID, a.Action, a.Detail, a.Result, a.Time,
	)
	return err
}

func (s *Store) CleanupOldMetrics(days int) error {
	cutoff := time.Now().AddDate(0, 0, -days)
	if _, err := s.db.Exec(fmt.Sprintf("DELETE FROM metrics WHERE timestamp < %s", s.placeholder(1)), cutoff); err != nil {
		return err
	}
	if _, err := s.db.Exec(fmt.Sprintf("DELETE FROM metrics_gpu WHERE timestamp < %s", s.placeholder(1)), cutoff); err != nil {
		return err
	}
	if _, err := s.db.Exec(fmt.Sprintf("DELETE FROM alerts WHERE time < %s", s.placeholder(1)), cutoff); err != nil {
		return err
	}
	return nil
}

func hashPassword(password string) string {
	h, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		log.Fatalf("[store] failed to hash password: %v", err)
	}
	return string(h)
}

func (s *Store) UpdatePassword(username, newHash string) error {
	_, err := s.db.Exec(
		fmt.Sprintf("UPDATE users SET password_hash = %s, must_change_pwd = 0 WHERE username = %s", s.placeholder(1), s.placeholder(1)),
		newHash, username,
	)
	return err
}

func (s *Store) GetUser(username string) (string, error) {
	row := s.db.QueryRow(fmt.Sprintf("SELECT password_hash FROM users WHERE username = %s", s.placeholder(1)), username)
	var hash string
	if err := row.Scan(&hash); err != nil {
		return "", err
	}
	return hash, nil
}

func (s *Store) ListSnapshots(nodeID string, limit int) []map[string]any {
	var rows *sql.Rows
	var err error
	cols := "node_id, timestamp, cpu_usage, cpu_cores, mem_total_gb, mem_used_gb, mem_usage_percent, disk_total_gb, disk_used_gb, disk_usage_percent, disk_read_mb, disk_write_mb, disk_iops, disk_read_rate_mb, disk_write_rate_mb, net_bytes_recv, net_bytes_sent, net_recv_rate_mb, net_sent_rate_mb, load1, load5, load15"
	if nodeID == "" {
		rows, err = s.db.Query(fmt.Sprintf("SELECT %s FROM (SELECT %s FROM metrics ORDER BY timestamp DESC LIMIT %s) sub ORDER BY timestamp ASC", cols, cols, s.placeholder(1)), limit)
	} else {
		rows, err = s.db.Query(fmt.Sprintf("SELECT %s FROM (SELECT %s FROM metrics WHERE node_id = %s ORDER BY timestamp DESC LIMIT %s) sub ORDER BY timestamp ASC", cols, cols, s.placeholder(1), s.placeholder(2)), nodeID, limit)
	}
	if err != nil {
		return nil
	}
	defer rows.Close()
	var result []map[string]any
	for rows.Next() {
		var nid string
		var ts time.Time
		var cpuUsage, cpuCores, memTotal, memUsed, memUsage float64
		var diskTotal, diskUsed, diskUsage float64
		var diskReadMB, diskWriteMB, diskIOPS float64
		var diskReadRateMB, diskWriteRateMB float64
		var netRecv, netSent int64
		var netRecvRate, netSentRate float64
		var load1, load5, load15 float64
		if err := rows.Scan(&nid, &ts, &cpuUsage, &cpuCores, &memTotal, &memUsed, &memUsage, &diskTotal, &diskUsed, &diskUsage, &diskReadMB, &diskWriteMB, &diskIOPS, &diskReadRateMB, &diskWriteRateMB, &netRecv, &netSent, &netRecvRate, &netSentRate, &load1, &load5, &load15); err != nil {
			continue
		}
		result = append(result, map[string]any{
			"node_id":   nid,
			"timestamp": ts,
			"cpu": map[string]any{
				"usage_percent": cpuUsage,
				"cores":         cpuCores,
			},
			"memory": map[string]any{
				"total_gb":      memTotal,
				"used_gb":       memUsed,
				"usage_percent": memUsage,
			},
			"disk": map[string]any{
				"total_gb":      diskTotal,
				"used_gb":       diskUsed,
				"usage_percent": diskUsage,
			},
			"disk_io": map[string]any{
				"read_mb":       diskReadMB,
				"write_mb":      diskWriteMB,
				"iops":          diskIOPS,
				"read_rate_mb":  diskReadRateMB,
				"write_rate_mb": diskWriteRateMB,
			},
			"network": map[string]any{
				"bytes_recv":   netRecv,
				"bytes_sent":   netSent,
				"recv_rate_mb": netRecvRate,
				"sent_rate_mb": netSentRate,
			},
			"load": map[string]any{
				"load1":  load1,
				"load5":  load5,
				"load15": load15,
			},
		})
	}
	return fillGapsWithNull(result)
}

// parseDurationStr parses a duration string like "1h", "6h", "1d", "7d", "30d" into hours.
func parseDurationStr(d string) int {
	if d == "" {
		return 0
	}
	num := 0
	unit := ""
	if n, err := fmt.Sscanf(d, "%d%s", &num, &unit); err == nil && n >= 1 && num > 0 {
		switch unit {
		case "h":
			return num
		case "d":
			return num * 24
		default:
			return 0
		}
	}
	return 0
}

func (s *Store) ListSnapshotsWithDuration(nodeID string, limit int, duration string) []map[string]any {
	hours := parseDurationStr(duration)
	if hours <= 0 {
		return s.ListSnapshots(nodeID, limit)
	}
	since := time.Now().Add(-time.Duration(hours) * time.Hour)
	cols := "node_id, timestamp, cpu_usage, cpu_cores, mem_total_gb, mem_used_gb, mem_usage_percent, disk_total_gb, disk_used_gb, disk_usage_percent, disk_read_mb, disk_write_mb, disk_iops, disk_read_rate_mb, disk_write_rate_mb, net_bytes_recv, net_bytes_sent, net_recv_rate_mb, net_sent_rate_mb, load1, load5, load15"
	var rows *sql.Rows
	var err error
	if nodeID == "" {
		rows, err = s.db.Query(fmt.Sprintf("SELECT %s FROM (SELECT %s FROM metrics WHERE timestamp >= %s ORDER BY timestamp DESC LIMIT %s) sub ORDER BY timestamp ASC", cols, cols, s.placeholder(1), s.placeholder(2)), since, limit)
	} else {
		rows, err = s.db.Query(fmt.Sprintf("SELECT %s FROM (SELECT %s FROM metrics WHERE node_id = %s AND timestamp >= %s ORDER BY timestamp DESC LIMIT %s) sub ORDER BY timestamp ASC", cols, cols, s.placeholder(1), s.placeholder(2), s.placeholder(3)), nodeID, since, limit)
	}
	if err != nil {
		return nil
	}
	defer rows.Close()
	var result []map[string]any
	for rows.Next() {
		var nid string
		var ts time.Time
		var cpuUsage, cpuCores, memTotal, memUsed, memUsage float64
		var diskTotal, diskUsed, diskUsage float64
		var diskReadMB, diskWriteMB, diskIOPS float64
		var diskReadRateMB, diskWriteRateMB float64
		var netRecv, netSent int64
		var netRecvRate, netSentRate float64
		var load1, load5, load15 float64
		if err := rows.Scan(&nid, &ts, &cpuUsage, &cpuCores, &memTotal, &memUsed, &memUsage, &diskTotal, &diskUsed, &diskUsage, &diskReadMB, &diskWriteMB, &diskIOPS, &diskReadRateMB, &diskWriteRateMB, &netRecv, &netSent, &netRecvRate, &netSentRate, &load1, &load5, &load15); err != nil {
			continue
		}
		result = append(result, map[string]any{
			"node_id":   nid,
			"timestamp": ts,
			"cpu": map[string]any{
				"usage_percent": cpuUsage,
				"cores":         cpuCores,
			},
			"memory": map[string]any{
				"total_gb":      memTotal,
				"used_gb":       memUsed,
				"usage_percent": memUsage,
			},
			"disk": map[string]any{
				"total_gb":      diskTotal,
				"used_gb":       diskUsed,
				"usage_percent": diskUsage,
			},
			"disk_io": map[string]any{
				"read_mb":       diskReadMB,
				"write_mb":      diskWriteMB,
				"iops":          diskIOPS,
				"read_rate_mb":  diskReadRateMB,
				"write_rate_mb": diskWriteRateMB,
			},
			"network": map[string]any{
				"bytes_recv":   netRecv,
				"bytes_sent":   netSent,
				"recv_rate_mb": netRecvRate,
				"sent_rate_mb": netSentRate,
			},
			"load": map[string]any{
				"load1":  load1,
				"load5":  load5,
				"load15": load15,
			},
		})
	}
	return fillGapsWithNull(result)
}

const gapThresholdSeconds = 120

// fillGapsWithNull inserts null markers at data gaps (system off / not monitored).
// Frontend charts use connectNulls=false so null values create line breaks.
func fillGapsWithNull(data []map[string]any) []map[string]any {
	if len(data) < 2 {
		return data
	}

	var filled []map[string]any
	filled = append(filled, data[0])

	for i := 1; i < len(data); i++ {
		prevTs, ok1 := data[i-1]["timestamp"].(time.Time)
		curTs, ok2 := data[i]["timestamp"].(time.Time)
		if !ok1 || !ok2 {
			filled = append(filled, data[i])
			continue
		}

		gapSeconds := curTs.Sub(prevTs).Seconds()
		if gapSeconds > gapThresholdSeconds {
			nid, _ := data[i]["node_id"].(string)
			nullPoint := map[string]any{
				"node_id":   nid,
				"timestamp": prevTs.Add(time.Duration(gapThresholdSeconds/2) * time.Second),
				"_gap":      true,
				"cpu": map[string]any{
					"usage_percent": nil,
					"cores":         nil,
				},
				"memory": map[string]any{
					"total_gb":      nil,
					"used_gb":       nil,
					"usage_percent": nil,
				},
				"disk": map[string]any{
					"total_gb":      nil,
					"used_gb":       nil,
					"usage_percent": nil,
				},
				"disk_io": map[string]any{
					"read_mb":       nil,
					"write_mb":      nil,
					"iops":          nil,
					"read_rate_mb":  nil,
					"write_rate_mb": nil,
				},
				"network": map[string]any{
					"bytes_recv":   nil,
					"bytes_sent":   nil,
					"recv_rate_mb": nil,
					"sent_rate_mb": nil,
				},
				"load": map[string]any{
					"load1":  nil,
					"load5":  nil,
					"load15": nil,
				},
			}
			filled = append(filled, nullPoint)
		}

		filled = append(filled, data[i])
	}

	return filled
}

func (s *Store) Ping() error {
	return s.db.Ping()
}

func (s *Store) DBPath() string {
	if s.dbType == "sqlite" {
		return s.dbPath
	}
	return ""
}

func (s *Store) UpdateUsername(oldName, newName string) error {
	_, err := s.db.Exec(
		fmt.Sprintf("UPDATE users SET username = %s WHERE username = %s", s.placeholder(1), s.placeholder(2)),
		newName, oldName,
	)
	return err
}

func (s *Store) ListCronJobs(nodeID string) []map[string]any {
	var rows *sql.Rows
	var err error
	if nodeID == "" {
		rows, err = s.db.Query("SELECT id, node_id, name, expression, command, type, enabled, last_run FROM cron_jobs ORDER BY name")
	} else {
		rows, err = s.db.Query(fmt.Sprintf("SELECT id, node_id, name, expression, command, type, enabled, last_run FROM cron_jobs WHERE node_id = %s ORDER BY name", s.placeholder(1)), nodeID)
	}
	if err != nil {
		return nil
	}
	defer rows.Close()
	var result []map[string]any
	for rows.Next() {
		var id int
		var nid, name, expr, cmd, jobType string
		var enabled int
		var lastRun int64
		if err := rows.Scan(&id, &nid, &name, &expr, &cmd, &jobType, &enabled, &lastRun); err != nil {
			continue
		}
		result = append(result, map[string]any{
			"id": id, "node_id": nid, "name": name, "expression": expr, "command": cmd, "type": jobType, "enabled": enabled != 0, "last_run": lastRun,
		})
	}
	return result
}

func (s *Store) SaveCronJob(job map[string]any) (int64, error) {
	id, _ := job["id"].(float64)
	nid, _ := job["node_id"].(string)
	name, _ := job["name"].(string)
	expr, _ := job["expression"].(string)
	cmd, _ := job["command"].(string)
	jobType, _ := job["type"].(string)
	if jobType == "" {
		jobType = "shell"
	}
	enabled := 1
	if e, ok := job["enabled"].(bool); ok && !e {
		enabled = 0
	}
	if id > 0 {
		_, err := s.db.Exec(
			fmt.Sprintf("UPDATE cron_jobs SET name=%s, expression=%s, command=%s, type=%s, enabled=%s WHERE id=%s",
				s.placeholder(1), s.placeholder(2), s.placeholder(3), s.placeholder(4), s.placeholder(5), s.placeholder(6)),
			name, expr, cmd, jobType, enabled, int(id),
		)
		return int64(int(id)), err
	}
	result, err := s.db.Exec(
		fmt.Sprintf("INSERT INTO cron_jobs (node_id, name, expression, command, type, enabled) VALUES (%s, %s, %s, %s, %s, %s)",
			s.placeholder(1), s.placeholder(2), s.placeholder(3), s.placeholder(4), s.placeholder(5), s.placeholder(6)),
		nid, name, expr, cmd, jobType, enabled,
	)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func (s *Store) DeleteCronJob(id int) error {
	_, err := s.db.Exec(fmt.Sprintf("DELETE FROM cron_jobs WHERE id = %s", s.placeholder(1)), id)
	return err
}

func (s *Store) tableExists(name string) bool {
	var count int
	if s.IsPostgreSQL() {
		s.db.QueryRow("SELECT COUNT(*) FROM information_schema.tables WHERE table_name = $1", name).Scan(&count)
	} else {
		s.db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?", name).Scan(&count)
	}
	return count > 0
}

func (s *Store) ListAlerts(nodeID string, limit int) []map[string]any {
	var rows *sql.Rows
	var err error
	if nodeID == "" {
		rows, err = s.db.Query(fmt.Sprintf("SELECT id, node_id, node_name, type, level, value, threshold, time, status, message FROM alerts ORDER BY time DESC LIMIT %s", s.placeholder(1)), limit)
	} else {
		rows, err = s.db.Query(fmt.Sprintf("SELECT id, node_id, node_name, type, level, value, threshold, time, status, message FROM alerts WHERE node_id = %s ORDER BY time DESC LIMIT %s", s.placeholder(1), s.placeholder(2)), nodeID, limit)
	}
	if err != nil {
		return nil
	}
	defer rows.Close()
	var result []map[string]any
	for rows.Next() {
		var id int
		var nid, nodeName, alertType, level, status, message string
		var value, threshold float64
		var t time.Time
		if err := rows.Scan(&id, &nid, &nodeName, &alertType, &level, &value, &threshold, &t, &status, &message); err != nil {
			continue
		}
		result = append(result, map[string]any{
			"id": id, "node_id": nid, "node_name": nodeName, "type": alertType, "level": level,
			"value": value, "threshold": threshold, "time": t, "status": status, "message": message,
		})
	}
	return result
}

func (s *Store) ListActiveAlerts(nodeID string) []map[string]any {
	var rows *sql.Rows
	var err error
	if nodeID == "" {
		rows, err = s.db.Query("SELECT id, node_id, node_name, type, level, value, threshold, time, status, message FROM alerts WHERE status = 'firing' OR status = 'active' ORDER BY time DESC")
	} else {
		rows, err = s.db.Query(fmt.Sprintf("SELECT id, node_id, node_name, type, level, value, threshold, time, status, message FROM alerts WHERE (status = 'firing' OR status = 'active') AND node_id = %s ORDER BY time DESC", s.placeholder(1)), nodeID)
	}
	if err != nil {
		return nil
	}
	defer rows.Close()
	var result []map[string]any
	for rows.Next() {
		var id int
		var nid, nodeName, alertType, level, status, message string
		var value, threshold float64
		var t time.Time
		if err := rows.Scan(&id, &nid, &nodeName, &alertType, &level, &value, &threshold, &t, &status, &message); err != nil {
			continue
		}
		result = append(result, map[string]any{
			"id": id, "node_id": nid, "node_name": nodeName, "type": alertType, "level": level,
			"value": value, "threshold": threshold, "time": t, "status": status, "message": message,
		})
	}
	return result
}

func (s *Store) SilenceAlert(id int) error {
	_, err := s.db.Exec(fmt.Sprintf("UPDATE alerts SET status = 'silenced' WHERE id = %s", s.placeholder(1)), id)
	return err
}

func (s *Store) SaveAlert(data map[string]any) {
	nodeID, _ := data["node_id"].(string)
	nodeName, _ := data["node_name"].(string)
	alertType, _ := data["metric"].(string)
	if alertType == "" {
		alertType, _ = data["type"].(string)
	}
	level, _ := data["level"].(string)
	if level == "" {
		level = "warning"
	}
	value, _ := data["value"].(float64)
	threshold, _ := data["threshold"].(float64)
	status, _ := data["status"].(string)
	if status == "" {
		status = "active"
	}
	message, _ := data["message"].(string)
	now := time.Now()
	if s.IsPostgreSQL() {
		_, err := s.db.Exec(
			"INSERT INTO alerts (node_id, node_name, type, level, value, threshold, time, status, message) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)",
			nodeID, nodeName, alertType, level, value, threshold, now, status, message,
		)
		if err != nil {
			log.Printf("[store] SaveAlert error: %v", err)
		}
	} else {
		_, err := s.db.Exec(
			"INSERT INTO alerts (node_id, node_name, type, level, value, threshold, time, status, message) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)",
			nodeID, nodeName, alertType, level, value, threshold, now, status, message,
		)
		if err != nil {
			log.Printf("[store] SaveAlert error: %v", err)
		}
	}
}

func (s *Store) ListAlertRules() []map[string]any {
	if !s.tableExists("alert_rules") {
		return nil
	}
	rows, err := s.db.Query("SELECT id, name, metric, op, threshold, level, enabled FROM alert_rules ORDER BY id")
	if err != nil {
		return nil
	}
	defer rows.Close()
	var result []map[string]any
	for rows.Next() {
		var id int
		var name, metric, op, level string
		var threshold float64
		var enabled int
		if err := rows.Scan(&id, &name, &metric, &op, &threshold, &level, &enabled); err != nil {
			continue
		}
		result = append(result, map[string]any{
			"id": id, "name": name, "metric": metric, "op": op,
			"threshold": threshold, "level": level, "enabled": enabled != 0,
		})
	}
	return result
}

func (s *Store) SaveAlertRule(rule map[string]any) error {
	s.ensureAlertRulesTable()
	id, _ := rule["id"].(float64)
	name, _ := rule["name"].(string)
	metric, _ := rule["metric"].(string)
	op, _ := rule["op"].(string)
	threshold, _ := rule["threshold"].(float64)
	level, _ := rule["level"].(string)
	if level == "" {
		level = "warning"
	}
	enabled := 1
	if e, ok := rule["enabled"].(bool); ok && !e {
		enabled = 0
	}
	if id > 0 {
		_, err := s.db.Exec(
			fmt.Sprintf("UPDATE alert_rules SET name=%s, metric=%s, op=%s, threshold=%s, level=%s, enabled=%s WHERE id=%s",
				s.placeholder(1), s.placeholder(2), s.placeholder(3), s.placeholder(4), s.placeholder(5), s.placeholder(6), s.placeholder(7)),
			name, metric, op, threshold, level, enabled, int(id),
		)
		return err
	}
	_, err := s.db.Exec(
		fmt.Sprintf("INSERT INTO alert_rules (name, metric, op, threshold, level, enabled) VALUES (%s)", s.placeholders(6)),
		name, metric, op, threshold, level, enabled,
	)
	return err
}

func (s *Store) DeleteAlertRule(id int) error {
	_, err := s.db.Exec(fmt.Sprintf("DELETE FROM alert_rules WHERE id = %s", s.placeholder(1)), id)
	return err
}

func (s *Store) ensureAlertRulesTable() {
	if s.tableExists("alert_rules") {
		return
	}
	var sql string
	if s.IsPostgreSQL() {
		sql = `CREATE TABLE alert_rules (
			id SERIAL PRIMARY KEY,
			name TEXT NOT NULL,
			metric TEXT NOT NULL,
			op TEXT DEFAULT '>',
			threshold REAL DEFAULT 0,
			level TEXT DEFAULT 'warning',
			enabled BOOLEAN DEFAULT TRUE
		)`
	} else {
		sql = `CREATE TABLE alert_rules (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			metric TEXT NOT NULL,
			op TEXT DEFAULT '>',
			threshold REAL DEFAULT 0,
			level TEXT DEFAULT 'warning',
			enabled INTEGER DEFAULT 1
		)`
	}
	if _, err := s.db.Exec(sql); err != nil {
		log.Printf("[store] warning: create alert_rules table: %v", err)
	}
}

func (s *Store) ensureColumn(table, column, sqliteDDL, pgDDL string) {
	if !isValidIdentifier(table) || !isValidIdentifier(column) {
		log.Printf("[store] warning: invalid table/column name in ensureColumn: %s.%s", table, column)
		return
	}
	var colExists bool
	if s.IsPostgreSQL() {
		var count int
		s.db.QueryRow("SELECT COUNT(*) FROM information_schema.columns WHERE table_name = $1 AND column_name = $2", table, column).Scan(&count)
		colExists = count > 0
	} else {
		var count int
		s.db.QueryRow("SELECT COUNT(*) FROM pragma_table_info(?) WHERE name=?", table, column).Scan(&count)
		colExists = count > 0
	}
	if !colExists {
		ddl := sqliteDDL
		if s.IsPostgreSQL() {
			ddl = pgDDL
		}
		_, err := s.db.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", s.quoteIdentifier(table), s.quoteIdentifier(column), ddl))
		if err != nil {
			log.Printf("[store] warning: add column %s.%s: %v", table, column, err)
		}
	}
}

func isValidIdentifier(name string) bool {
	if len(name) == 0 || len(name) > 64 {
		return false
	}
	for i, c := range name {
		if i == 0 && !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '_') {
			return false
		}
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_') {
			return false
		}
	}
	return true
}

func (s *Store) quoteIdentifier(name string) string {
	if s.IsPostgreSQL() {
		return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
	}
	return "`" + strings.ReplaceAll(name, "`", "``") + "`"
}

func (s *Store) RecordFileOp(op map[string]any) {
	nodeID, _ := op["node_id"].(string)
	if nodeID == "" {
		nodeID = "self"
	}
	operation, _ := op["operation"].(string)
	path, _ := op["path"].(string)
	name, _ := op["name"].(string)
	ext, _ := op["ext"].(string)
	size, _ := op["size"].(int64)
	isDir := 0
	if d, ok := op["is_dir"].(bool); ok && d {
		isDir = 1
	}
	_, err := s.db.Exec(
		fmt.Sprintf("INSERT INTO file_operations (node_id, operation, path, name, ext, size, is_dir) VALUES (%s)", s.placeholders(7)),
		nodeID, operation, path, name, ext, size, isDir,
	)
	if err != nil {
		log.Printf("[store] RecordFileOp error: %v", err)
	}
}

func (s *Store) GetFileStats(nodeID string, hours int) []map[string]any {
	since := time.Now().Add(-time.Duration(hours) * time.Hour)
	rows, err := s.db.Query(
		fmt.Sprintf(`SELECT operation, path, name, ext, size, is_dir, timestamp 
			FROM file_operations 
			WHERE node_id = %s AND timestamp >= %s 
			ORDER BY timestamp DESC`, s.placeholder(1), s.placeholder(2)),
		nodeID, since,
	)
	if err != nil {
		return nil
	}
	defer rows.Close()

	type dirStat struct {
		path         string
		total        int
		dirs         int
		files        int
		size         int64
		types        map[string]int
		todayOps     int
		todayCreated int
		todayDeleted int
	}
	dirMap := make(map[string]*dirStat)
	today := time.Now().Truncate(24 * time.Hour)

	for rows.Next() {
		var op, path, name, ext string
		var size int64
		var isDir int
		var ts time.Time
		if err := rows.Scan(&op, &path, &name, &ext, &size, &isDir, &ts); err != nil {
			continue
		}
		dir := filepath.Dir(path)
		if dir == "." {
			dir = "/"
		}
		if _, ok := dirMap[dir]; !ok {
			dirMap[dir] = &dirStat{path: dir, types: make(map[string]int)}
		}
		ds := dirMap[dir]
		if op == "create" || op == "upload" {
			ds.total++
			ds.size += size
			if isDir != 0 {
				ds.dirs++
			} else {
				ds.files++
				if ext != "" {
					ds.types[ext]++
				}
			}
		}
		if ts.After(today) {
			ds.todayOps++
			switch op {
			case "create", "upload":
				ds.todayCreated++
			case "delete":
				ds.todayDeleted++
			}
		}
	}

	var result []map[string]any
	if len(dirMap) == 0 {
		return result
	}
	for _, ds := range dirMap {
		result = append(result, map[string]any{
			"path":          ds.path,
			"total":         ds.total,
			"dirs":          ds.dirs,
			"files":         ds.files,
			"size":          ds.size,
			"types":         ds.types,
			"today_ops":     ds.todayOps,
			"today_created": ds.todayCreated,
			"today_deleted": ds.todayDeleted,
		})
	}
	return result
}

func (s *Store) GetRecentMetricValues(nodeID, metric string, limit int) []float64 {
	if s.db == nil {
		return nil
	}

	var col string
	switch metric {
	case "cpu":
		col = "cpu_usage"
	case "memory", "mem":
		col = "mem_usage_percent"
	case "disk":
		col = "disk_usage_percent"
	case "load1":
		col = "load1"
	case "load5":
		col = "load5"
	case "load15":
		col = "load15"
	default:
		return nil
	}

	query := fmt.Sprintf("SELECT %s FROM metrics", col)
	var args []interface{}
	if nodeID != "" {
		query += fmt.Sprintf(" WHERE node_id = %s", s.placeholder(1))
		args = append(args, nodeID)
	}
	query += fmt.Sprintf(" ORDER BY timestamp DESC LIMIT %s", s.placeholder(len(args)+1))
	args = append(args, limit)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var values []float64
	for rows.Next() {
		var v float64
		if err := rows.Scan(&v); err == nil {
			values = append(values, v)
		}
	}
	return values
}

func (s *Store) ListUsers() []map[string]any {
	rows, err := s.db.Query("SELECT id, username, role, must_change_pwd FROM users ORDER BY id")
	if err != nil {
		return nil
	}
	defer rows.Close()
	var result []map[string]any
	for rows.Next() {
		var id int
		var username, role string
		var mustChangePwd int
		if err := rows.Scan(&id, &username, &role, &mustChangePwd); err != nil {
			continue
		}
		result = append(result, map[string]any{
			"id": id, "username": username, "role": role, "must_change_pwd": mustChangePwd != 0,
		})
	}
	return result
}

func (s *Store) CreateUser(username, passwordHash, role string) error {
	_, err := s.db.Exec(
		fmt.Sprintf("INSERT INTO users (username, password_hash, role, must_change_pwd) VALUES (%s)", s.placeholders(4)),
		username, passwordHash, role, 0,
	)
	return err
}

func (s *Store) GetUserByID(id int) (*model.User, error) {
	row := s.db.QueryRow(fmt.Sprintf("SELECT id, username, password_hash, role, otp_enabled, must_change_pwd FROM users WHERE id = %s", s.placeholder(1)), id)
	var u model.User
	var otpEnabled int
	var mustChangePwd int
	if err := row.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &otpEnabled, &mustChangePwd); err != nil {
		return nil, err
	}
	u.OTPEnabled = otpEnabled != 0
	u.MustChangePwd = mustChangePwd != 0
	return &u, nil
}

func (s *Store) DeleteUser(id int) error {
	_, err := s.db.Exec(fmt.Sprintf("DELETE FROM users WHERE id = %s", s.placeholder(1)), id)
	return err
}

func (s *Store) ListAuditLogs(limit int) []map[string]any {
	rows, err := s.db.Query(
		fmt.Sprintf("SELECT id, user_id, node_id, action, detail, result, time FROM audit_logs ORDER BY time DESC LIMIT %s", s.placeholder(1)),
		limit,
	)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var result []map[string]any
	for rows.Next() {
		var id, userID int
		var nodeID, action, detail, resultStr string
		var t time.Time
		if err := rows.Scan(&id, &userID, &nodeID, &action, &detail, &resultStr, &t); err != nil {
			continue
		}
		result = append(result, map[string]any{
			"id": id, "user_id": userID, "node_id": nodeID,
			"action": action, "detail": detail, "result": resultStr, "time": t,
		})
	}
	return result
}

func (s *Store) SaveCronJobLog(jobID int, command, output string, exitCode int, durationMs int64) {
	s.ensureCronJobLogsTable()
	_, err := s.db.Exec(
		fmt.Sprintf("INSERT INTO cron_job_logs (job_id, command, output, exit_code, duration_ms, timestamp) VALUES (%s)", s.placeholders(6)),
		jobID, command, output, exitCode, durationMs, time.Now(),
	)
	if err != nil {
		log.Printf("[store] SaveCronJobLog error: %v", err)
	}
}

func (s *Store) ListCronJobLogs(jobID int, limit int) []map[string]any {
	s.ensureCronJobLogsTable()
	rows, err := s.db.Query(
		fmt.Sprintf("SELECT id, job_id, command, output, exit_code, duration_ms, timestamp FROM cron_job_logs WHERE job_id = %s ORDER BY timestamp DESC LIMIT %s", s.placeholder(1), s.placeholder(2)),
		jobID, limit,
	)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var result []map[string]any
	for rows.Next() {
		var id, jid, exitCode int
		var command, output string
		var durationMs int64
		var ts time.Time
		if err := rows.Scan(&id, &jid, &command, &output, &exitCode, &durationMs, &ts); err != nil {
			continue
		}
		result = append(result, map[string]any{
			"id": id, "job_id": jid, "command": command, "output": output,
			"exit_code": exitCode, "duration_ms": durationMs, "timestamp": ts,
		})
	}
	return result
}

func (s *Store) SearchCronJobLogs(keyword, startTime, endTime string, limit int) []map[string]any {
	s.ensureCronJobLogsTable()
	query := "SELECT id, job_id, command, output, exit_code, duration_ms, timestamp FROM cron_job_logs WHERE 1=1"
	var args []interface{}
	argN := 1
	if keyword != "" {
		argN++
		query += fmt.Sprintf(" AND (command LIKE %s OR output LIKE %s)", s.placeholder(argN), s.placeholder(argN+1))
		args = append(args, "%"+keyword+"%", "%"+keyword+"%")
		argN++
	}
	if startTime != "" {
		argN++
		query += fmt.Sprintf(" AND timestamp >= %s", s.placeholder(argN))
		args = append(args, startTime)
	}
	if endTime != "" {
		argN++
		query += fmt.Sprintf(" AND timestamp <= %s", s.placeholder(argN))
		args = append(args, endTime)
	}
	argN++
	query += fmt.Sprintf(" ORDER BY timestamp DESC LIMIT %s", s.placeholder(argN))
	args = append(args, limit)
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var result []map[string]any
	for rows.Next() {
		var id, jid, exitCode int
		var command, output string
		var durationMs int64
		var ts time.Time
		if err := rows.Scan(&id, &jid, &command, &output, &exitCode, &durationMs, &ts); err != nil {
			continue
		}
		result = append(result, map[string]any{
			"id": id, "job_id": jid, "command": command, "output": output,
			"exit_code": exitCode, "duration_ms": durationMs, "timestamp": ts,
		})
	}
	return result
}

func (s *Store) ensureCronJobLogsTable() {
	if s.tableExists("cron_job_logs") {
		return
	}
	var sql string
	if s.IsPostgreSQL() {
		sql = `CREATE TABLE cron_job_logs (
			id SERIAL PRIMARY KEY,
			job_id INTEGER NOT NULL,
			command TEXT DEFAULT '',
			output TEXT DEFAULT '',
			exit_code INTEGER DEFAULT 0,
			duration_ms BIGINT DEFAULT 0,
			timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`
	} else {
		sql = `CREATE TABLE cron_job_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			job_id INTEGER NOT NULL,
			command TEXT DEFAULT '',
			output TEXT DEFAULT '',
			exit_code INTEGER DEFAULT 0,
			duration_ms INTEGER DEFAULT 0,
			timestamp DATETIME DEFAULT CURRENT_TIMESTAMP
		)`
	}
	if _, err := s.db.Exec(sql); err != nil {
		log.Printf("[store] warning: create cron_job_logs table: %v", err)
	}
}

func (s *Store) ListScripts() []map[string]any {
	s.ensureScriptsTable()
	rows, err := s.db.Query("SELECT id, name, interpreter, description, content, created_at, updated_at FROM scripts ORDER BY name")
	if err != nil {
		return nil
	}
	defer rows.Close()
	var result []map[string]any
	for rows.Next() {
		var id int
		var name, interpreter, description, content string
		var createdAt, updatedAt sql.NullTime
		if err := rows.Scan(&id, &name, &interpreter, &description, &content, &createdAt, &updatedAt); err != nil {
			continue
		}
		item := map[string]any{
			"id": id, "name": name, "interpreter": interpreter,
			"description": description, "content": content,
		}
		if createdAt.Valid {
			item["created_at"] = createdAt.Time
		}
		if updatedAt.Valid {
			item["updated_at"] = updatedAt.Time
		}
		result = append(result, item)
	}
	return result
}

func (s *Store) GetScript(id int) (map[string]any, error) {
	s.ensureScriptsTable()
	row := s.db.QueryRow(fmt.Sprintf("SELECT id, name, interpreter, description, content, created_at, updated_at FROM scripts WHERE id = %s", s.placeholder(1)), id)
	var sid int
	var name, interpreter, description, content string
	var createdAt, updatedAt sql.NullTime
	if err := row.Scan(&sid, &name, &interpreter, &description, &content, &createdAt, &updatedAt); err != nil {
		return nil, err
	}
	item := map[string]any{
		"id": sid, "name": name, "interpreter": interpreter,
		"description": description, "content": content,
	}
	if createdAt.Valid {
		item["created_at"] = createdAt.Time
	}
	if updatedAt.Valid {
		item["updated_at"] = updatedAt.Time
	}
	return item, nil
}

func (s *Store) SaveScript(data map[string]any) (int64, error) {
	s.ensureScriptsTable()
	id, _ := data["id"].(float64)
	name, _ := data["name"].(string)
	interpreter, _ := data["interpreter"].(string)
	description, _ := data["description"].(string)
	content, _ := data["content"].(string)
	now := time.Now()
	if id > 0 {
		_, err := s.db.Exec(
			fmt.Sprintf("UPDATE scripts SET name=%s, interpreter=%s, description=%s, content=%s, updated_at=%s WHERE id=%s",
				s.placeholder(1), s.placeholder(2), s.placeholder(3), s.placeholder(4), s.placeholder(5), s.placeholder(6)),
			name, interpreter, description, content, now, int(id),
		)
		return int64(int(id)), err
	}
	result, err := s.db.Exec(
		fmt.Sprintf("INSERT INTO scripts (name, interpreter, description, content, created_at, updated_at) VALUES (%s)",
			s.placeholders(6)),
		name, interpreter, description, content, now, now,
	)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func (s *Store) DeleteScript(id int) error {
	_, err := s.db.Exec(fmt.Sprintf("DELETE FROM scripts WHERE id = %s", s.placeholder(1)), id)
	return err
}

func (s *Store) ensureScriptsTable() {
	if s.tableExists("scripts") {
		return
	}
	var sql string
	if s.IsPostgreSQL() {
		sql = `CREATE TABLE scripts (
			id SERIAL PRIMARY KEY,
			name TEXT NOT NULL,
			interpreter TEXT DEFAULT '/bin/bash',
			description TEXT DEFAULT '',
			content TEXT NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`
	} else {
		sql = `CREATE TABLE scripts (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			interpreter TEXT DEFAULT '/bin/bash',
			description TEXT DEFAULT '',
			content TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`
	}
	if _, err := s.db.Exec(sql); err != nil {
		log.Printf("[store] warning: create scripts table: %v", err)
	}
}

func (s *Store) GetCommandHistory(limit int) []map[string]any {
	s.ensureCommandHistoryTable()
	rows, err := s.db.Query(
		fmt.Sprintf("SELECT id, command, timestamp FROM command_history ORDER BY timestamp DESC LIMIT %s", s.placeholder(1)),
		limit,
	)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var result []map[string]any
	for rows.Next() {
		var id int
		var command string
		var ts time.Time
		if err := rows.Scan(&id, &command, &ts); err != nil {
			continue
		}
		result = append(result, map[string]any{
			"id": id, "command": command, "timestamp": ts,
		})
	}
	return result
}

func (s *Store) SaveCommandHistory(command string) {
	s.ensureCommandHistoryTable()
	_, err := s.db.Exec(
		fmt.Sprintf("INSERT INTO command_history (command, timestamp) VALUES (%s, %s)", s.placeholder(1), s.placeholder(2)),
		command, time.Now(),
	)
	if err != nil {
		log.Printf("[store] SaveCommandHistory error: %v", err)
	}
}

func (s *Store) ClearCommandHistory() {
	s.ensureCommandHistoryTable()
	_, _ = s.db.Exec("DELETE FROM command_history")
}

func (s *Store) ensureCommandHistoryTable() {
	if s.tableExists("command_history") {
		return
	}
	var sql string
	if s.IsPostgreSQL() {
		sql = `CREATE TABLE command_history (
			id SERIAL PRIMARY KEY,
			command TEXT NOT NULL,
			timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`
	} else {
		sql = `CREATE TABLE command_history (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			command TEXT NOT NULL,
			timestamp DATETIME DEFAULT CURRENT_TIMESTAMP
		)`
	}
	if _, err := s.db.Exec(sql); err != nil {
		log.Printf("[store] warning: create command_history table: %v", err)
	}
}
