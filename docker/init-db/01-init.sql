-- DevDash PostgreSQL Initialization Script
-- Compatible with PostgreSQL 14+
-- Runs automatically on first container start via /docker-entrypoint-initdb.d

BEGIN;

CREATE TABLE IF NOT EXISTS nodes (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    os TEXT,
    arch TEXT,
    ip TEXT,
    role TEXT DEFAULT 'full',
    token TEXT UNIQUE,
    status TEXT DEFAULT 'offline',
    last_heartbeat TIMESTAMP,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS users (
    id SERIAL PRIMARY KEY,
    username TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    role TEXT DEFAULT 'user',
    otp_enabled INTEGER DEFAULT 0,
    must_change_pwd INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS snapshots (
    id SERIAL PRIMARY KEY,
    node_id TEXT,
    timestamp TIMESTAMP DEFAULT NOW(),
    cpu DOUBLE PRECISION, mem DOUBLE PRECISION, disk DOUBLE PRECISION,
    net_recv DOUBLE PRECISION, net_sent DOUBLE PRECISION,
    load1 DOUBLE PRECISION, load5 DOUBLE PRECISION, load15 DOUBLE PRECISION
);

CREATE TABLE IF NOT EXISTS alerts (
    id SERIAL PRIMARY KEY,
    node_id TEXT,
    type TEXT,
    level TEXT,
    value DOUBLE PRECISION,
    threshold DOUBLE PRECISION,
    time TIMESTAMP DEFAULT NOW(),
    status TEXT DEFAULT 'active'
);

CREATE TABLE IF NOT EXISTS software (
    id SERIAL PRIMARY KEY,
    node_id TEXT,
    name TEXT,
    version TEXT,
    status TEXT,
    installed_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS cron_jobs (
    id SERIAL PRIMARY KEY,
    node_id TEXT,
    name TEXT,
    expression TEXT,
    command TEXT,
    type TEXT DEFAULT 'shell',
    enabled INTEGER DEFAULT 1,
    last_run BIGINT DEFAULT 0,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS audit_logs (
    id SERIAL PRIMARY KEY,
    user_id INTEGER,
    node_id TEXT,
    action TEXT,
    detail TEXT,
    result TEXT,
    time TIMESTAMP DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS db_connections (
    id SERIAL PRIMARY KEY,
    node_id TEXT,
    name TEXT,
    type TEXT,
    host TEXT,
    port INTEGER,
    database_name TEXT,
    username TEXT,
    password_encrypted TEXT,
    enabled INTEGER DEFAULT 1,
    created_at TIMESTAMP DEFAULT NOW()
);

-- metrics tables for long-term storage
CREATE TABLE IF NOT EXISTS metrics_cpu (
    id SERIAL PRIMARY KEY,
    node_id TEXT,
    timestamp TIMESTAMP DEFAULT NOW(),
    usage_percent DOUBLE PRECISION,
    per_core TEXT
);

CREATE TABLE IF NOT EXISTS metrics_mem (
    id SERIAL PRIMARY KEY,
    node_id TEXT,
    timestamp TIMESTAMP DEFAULT NOW(),
    total_gb DOUBLE PRECISION, used_gb DOUBLE PRECISION, available_gb DOUBLE PRECISION,
    usage_percent DOUBLE PRECISION, swap_total_gb DOUBLE PRECISION, swap_used_gb DOUBLE PRECISION
);

CREATE TABLE IF NOT EXISTS metrics_disk (
    id SERIAL PRIMARY KEY,
    node_id TEXT,
    timestamp TIMESTAMP DEFAULT NOW(),
    total_gb DOUBLE PRECISION, used_gb DOUBLE PRECISION, free_gb DOUBLE PRECISION, usage_percent DOUBLE PRECISION
);

CREATE TABLE IF NOT EXISTS metrics_net (
    id SERIAL PRIMARY KEY,
    node_id TEXT,
    timestamp TIMESTAMP DEFAULT NOW(),
    bytes_recv BIGINT, bytes_sent BIGINT,
    recv_rate_mb DOUBLE PRECISION, sent_rate_mb DOUBLE PRECISION
);

CREATE TABLE IF NOT EXISTS metrics_container (
    id SERIAL PRIMARY KEY,
    node_id TEXT,
    timestamp TIMESTAMP DEFAULT NOW(),
    container_id TEXT, name TEXT, image TEXT, status TEXT,
    cpu_percent DOUBLE PRECISION, mem_usage_mb DOUBLE PRECISION, mem_limit_mb DOUBLE PRECISION
);

-- indexes
CREATE INDEX IF NOT EXISTS idx_snapshots_node_time ON snapshots(node_id, timestamp);
CREATE INDEX IF NOT EXISTS idx_alerts_node_time ON alerts(node_id, time);
CREATE INDEX IF NOT EXISTS idx_software_node ON software(node_id);
CREATE INDEX IF NOT EXISTS idx_metrics_cpu_node_time ON metrics_cpu(node_id, timestamp);
CREATE INDEX IF NOT EXISTS idx_metrics_mem_node_time ON metrics_mem(node_id, timestamp);
CREATE INDEX IF NOT EXISTS idx_metrics_disk_node_time ON metrics_disk(node_id, timestamp);
CREATE INDEX IF NOT EXISTS idx_metrics_net_node_time ON metrics_net(node_id, timestamp);
CREATE INDEX IF NOT EXISTS idx_metrics_container_node_time ON metrics_container(node_id, timestamp);

-- default admin user (bcrypt hash for 'admin123' - MUST CHANGE ON FIRST LOGIN)
INSERT INTO users (username, password_hash, role, must_change_pwd)
VALUES ('admin', '$2a$10$N9qo8uLOickgx2ZMRZoMye.IqOOZwlHGvBH1Jgz4E/3w5VxJ5zKKS', 'admin', 1)
ON CONFLICT (username) DO NOTHING;

COMMIT;
