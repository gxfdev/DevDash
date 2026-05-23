package store

import (
	"database/sql"
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"time"

	"devdash/internal/auth"
	"devdash/internal/config"
	"devdash/internal/model"

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

func (s *Store) DB() *sql.DB { return s.db }

func (s *Store) IsPostgreSQL() bool { return s.dbType == config.DBPostgreSQL }

func (s *Store) runMigrations() {
	migrations := []struct {
		name string
		sql  string
		pg   string
	}{
		{
			name: "nodes",
			sql: `CREATE TABLE IF NOT EXISTS nodes (
				id TEXT PRIMARY KEY,
				name TEXT NOT NULL,
				os TEXT DEFAULT '',
				arch TEXT DEFAULT '',
				ip TEXT DEFAULT '',
				role TEXT DEFAULT 'agent',
				token TEXT DEFAULT '',
				status TEXT DEFAULT 'online',
				last_heartbeat DATETIME DEFAULT CURRENT_TIMESTAMP,
				created_at DATETIME DEFAULT CURRENT_TIMESTAMP
			)`,
			pg: `CREATE TABLE IF NOT EXISTS nodes (
				id TEXT PRIMARY KEY,
				name TEXT NOT NULL,
				os TEXT DEFAULT '',
				arch TEXT DEFAULT '',
				ip TEXT DEFAULT '',
				role TEXT DEFAULT 'agent',
				token TEXT DEFAULT '',
				status TEXT DEFAULT 'online',
				last_heartbeat TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
			)`,
		},
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
				load15 REAL DEFAULT 0,
				FOREIGN KEY (node_id) REFERENCES nodes(id)
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
				load15 REAL DEFAULT 0,
				FOREIGN KEY (node_id) REFERENCES nodes(id)
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
				temperature_celsius REAL DEFAULT 0,
				FOREIGN KEY (node_id) REFERENCES nodes(id)
			)`,
			pg: `CREATE TABLE IF NOT EXISTS metrics_gpu (
				id SERIAL PRIMARY KEY,
				node_id TEXT NOT NULL,
				timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
				gpu_index INTEGER DEFAULT 0,
				usage_percent REAL DEFAULT 0,
				mem_used_mb REAL DEFAULT 0,
				mem_total_mb REAL DEFAULT 0,
				temperature_celsius REAL DEFAULT 0,
				FOREIGN KEY (node_id) REFERENCES nodes(id)
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
			name: "software",
			sql: `CREATE TABLE IF NOT EXISTS software (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				node_id TEXT NOT NULL,
				name TEXT NOT NULL,
				version TEXT DEFAULT '',
				status TEXT DEFAULT 'installed',
				FOREIGN KEY (node_id) REFERENCES nodes(id)
			)`,
			pg: `CREATE TABLE IF NOT EXISTS software (
				id SERIAL PRIMARY KEY,
				node_id TEXT NOT NULL,
				name TEXT NOT NULL,
				version TEXT DEFAULT '',
				status TEXT DEFAULT 'installed',
				FOREIGN KEY (node_id) REFERENCES nodes(id)
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
				enabled INTEGER DEFAULT 1,
				FOREIGN KEY (node_id) REFERENCES nodes(id)
			)`,
			pg: `CREATE TABLE IF NOT EXISTS cron_jobs (
				id SERIAL PRIMARY KEY,
				node_id TEXT NOT NULL,
				name TEXT NOT NULL,
				expression TEXT NOT NULL,
				command TEXT NOT NULL,
				enabled BOOLEAN DEFAULT TRUE,
				FOREIGN KEY (node_id) REFERENCES nodes(id)
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
				timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
				FOREIGN KEY (node_id) REFERENCES nodes(id)
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
				timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
				FOREIGN KEY (node_id) REFERENCES nodes(id)
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
	return result, nil
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

func (s *Store) GetNode(id string) (*model.Node, error) {
	row := s.db.QueryRow(fmt.Sprintf("SELECT id, name, os, arch, ip, role, token, status, last_heartbeat, created_at FROM nodes WHERE id = %s", s.placeholder(1)), id)
	var n model.Node
	if err := row.Scan(&n.ID, &n.Name, &n.OS, &n.Arch, &n.IP, &n.Role, &n.Token, &n.Status, &n.LastHeartbeat, &n.CreatedAt); err != nil {
		return nil, err
	}
	if n.Token != "" {
		n.Token = "***"
	}
	return &n, nil
}

func (s *Store) ListNodes() ([]model.Node, error) {
	rows, err := s.db.Query("SELECT id, name, os, arch, ip, role, token, status, last_heartbeat, created_at FROM nodes ORDER BY created_at DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var nodes []model.Node
	for rows.Next() {
		var n model.Node
		if err := rows.Scan(&n.ID, &n.Name, &n.OS, &n.Arch, &n.IP, &n.Role, &n.Token, &n.Status, &n.LastHeartbeat, &n.CreatedAt); err != nil {
			continue
		}
		if n.Token != "" {
			n.Token = "***"
		}
		nodes = append(nodes, n)
	}
	return nodes, nil
}

func (s *Store) SaveNode(n *model.Node) error {
	var err error
	if s.IsPostgreSQL() {
		_, err = s.db.Exec(
			`INSERT INTO nodes (id, name, os, arch, ip, role, token, status, last_heartbeat, created_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
			 ON CONFLICT (id) DO UPDATE SET name=$2, os=$3, arch=$4, ip=$5, role=$6, token=$7, status=$8, last_heartbeat=$9`,
			n.ID, n.Name, n.OS, n.Arch, n.IP, n.Role, n.Token, n.Status, n.LastHeartbeat, n.CreatedAt,
		)
	} else {
		_, err = s.db.Exec(
			"INSERT OR REPLACE INTO nodes (id, name, os, arch, ip, role, token, status, last_heartbeat, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
			n.ID, n.Name, n.OS, n.Arch, n.IP, n.Role, n.Token, n.Status, n.LastHeartbeat, n.CreatedAt,
		)
	}
	return err
}

func (s *Store) UpdateNodeHeartbeat(id string) error {
	_, err := s.db.Exec(fmt.Sprintf("UPDATE nodes SET last_heartbeat = CURRENT_TIMESTAMP, status = 'online' WHERE id = %s", s.placeholder(1)), id)
	return err
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

func (s *Store) DeleteNode(id string) error {
	_, err := s.db.Exec(fmt.Sprintf("DELETE FROM nodes WHERE id = %s", s.placeholder(1)), id)
	return err
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
	return result
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

func (s *Store) ListSoftware(nodeID string) []map[string]any {
	var rows *sql.Rows
	var err error
	if nodeID == "" {
		rows, err = s.db.Query("SELECT id, node_id, name, version, status FROM software ORDER BY name")
	} else {
		rows, err = s.db.Query(fmt.Sprintf("SELECT id, node_id, name, version, status FROM software WHERE node_id = %s ORDER BY name", s.placeholder(1)), nodeID)
	}
	if err != nil {
		return nil
	}
	defer rows.Close()
	var result []map[string]any
	for rows.Next() {
		var id int
		var nid, name, version, status string
		if err := rows.Scan(&id, &nid, &name, &version, &status); err != nil {
			continue
		}
		result = append(result, map[string]any{
			"id": id, "node_id": nid, "name": name, "version": version, "status": status,
		})
	}
	return result
}

func (s *Store) SaveSoftware(data map[string]any) {
	_, err := s.db.Exec(
		fmt.Sprintf("INSERT INTO software (node_id, name, version, status) VALUES (%s)", s.placeholders(4)),
		data["node_id"], data["name"], data["version"], data["status"],
	)
	if err != nil {
		log.Printf("[store] SaveSoftware error: %v", err)
	}
}

func (s *Store) DeleteSoftware(nodeID, name string) {
	_, _ = s.db.Exec(
		fmt.Sprintf("DELETE FROM software WHERE node_id = %s AND name = %s", s.placeholder(1), s.placeholder(2)),
		nodeID, name,
	)
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

func (s *Store) ListDBConnections(nodeID string) []map[string]any {
	tableExists := s.tableExists("db_connections")
	if !tableExists {
		return nil
	}
	var rows *sql.Rows
	var err error
	if nodeID == "" {
		rows, err = s.db.Query("SELECT id, node_id, name, type, host, port, user, dbname FROM db_connections ORDER BY name")
	} else {
		rows, err = s.db.Query(fmt.Sprintf("SELECT id, node_id, name, type, host, port, user, dbname FROM db_connections WHERE node_id = %s ORDER BY name", s.placeholder(1)), nodeID)
	}
	if err != nil {
		return nil
	}
	defer rows.Close()
	var result []map[string]any
	for rows.Next() {
		var id int
		var nid, name, dbType, host, user, dbname string
		var port int
		if err := rows.Scan(&id, &nid, &name, &dbType, &host, &port, &user, &dbname); err != nil {
			continue
		}
		result = append(result, map[string]any{
			"id": id, "node_id": nid, "name": name, "type": dbType,
			"host": host, "port": port, "user": user, "dbname": dbname,
		})
	}
	return result
}

func (s *Store) SaveDBConnection(conn map[string]any) error {
	s.ensureDBConnectionsTable()
	pw, _ := conn["password"].(string)
	encPW, err := auth.EncryptField(pw)
	if err != nil {
		return fmt.Errorf("encrypt password: %w", err)
	}
	_, err = s.db.Exec(
		fmt.Sprintf("INSERT INTO db_connections (node_id, name, type, host, port, user, password, dbname) VALUES (%s)", s.placeholders(8)),
		conn["node_id"], conn["name"], conn["type"], conn["host"], conn["port"], conn["user"], encPW, conn["dbname"],
	)
	return err
}

func (s *Store) GetDBConnection(id int) (map[string]any, error) {
	s.ensureDBConnectionsTable()
	row := s.db.QueryRow(fmt.Sprintf("SELECT id, node_id, name, type, host, port, user, password, dbname FROM db_connections WHERE id = %s", s.placeholder(1)), id)
	var dbID int
	var nid, name, dbType, host, user, encPassword, dbname string
	var port int
	if err := row.Scan(&dbID, &nid, &name, &dbType, &host, &port, &user, &encPassword, &dbname); err != nil {
		return nil, err
	}
	decPassword, err := auth.DecryptField(encPassword)
	if err != nil {
		decPassword = encPassword
	}
	return map[string]any{
		"id": dbID, "node_id": nid, "name": name, "type": dbType,
		"host": host, "port": port, "user": user, "password": decPassword, "dbname": dbname,
	}, nil
}

func (s *Store) DeleteDBConnection(id int) error {
	_, err := s.db.Exec(fmt.Sprintf("DELETE FROM db_connections WHERE id = %s", s.placeholder(1)), id)
	return err
}

func (s *Store) ensureDBConnectionsTable() {
	if s.tableExists("db_connections") {
		return
	}
	var sql string
	if s.IsPostgreSQL() {
		sql = `CREATE TABLE db_connections (
			id SERIAL PRIMARY KEY,
			node_id TEXT DEFAULT '',
			name TEXT NOT NULL,
			type TEXT DEFAULT '',
			host TEXT DEFAULT '',
			port INTEGER DEFAULT 0,
			user TEXT DEFAULT '',
			password TEXT DEFAULT '',
			dbname TEXT DEFAULT ''
		)`
	} else {
		sql = `CREATE TABLE db_connections (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			node_id TEXT DEFAULT '',
			name TEXT NOT NULL,
			type TEXT DEFAULT '',
			host TEXT DEFAULT '',
			port INTEGER DEFAULT 0,
			user TEXT DEFAULT '',
			password TEXT DEFAULT '',
			dbname TEXT DEFAULT ''
		)`
	}
	if _, err := s.db.Exec(sql); err != nil {
		log.Printf("[store] warning: create db_connections table: %v", err)
	}
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
		_, err := s.db.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, column, ddl))
		if err != nil {
			log.Printf("[store] warning: add column %s.%s: %v", table, column, err)
		}
	}
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
	for _, ds := range dirMap {
		result = append(result, map[string]any{
			"path":          ds.path,
			"total":         ds.total,
			"dirs":          ds.dirs,
			"files":         ds.files - ds.dirs,
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
