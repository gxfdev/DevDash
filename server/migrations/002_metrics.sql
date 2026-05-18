-- metrics tables for long-term storage
CREATE TABLE IF NOT EXISTS metrics_cpu (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    node_id TEXT,
    timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
    usage_percent REAL,
    per_core TEXT -- JSON array
);

CREATE TABLE IF NOT EXISTS metrics_mem (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    node_id TEXT,
    timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
    total_gb REAL, used_gb REAL, available_gb REAL,
    usage_percent REAL, swap_total_gb REAL, swap_used_gb REAL
);

CREATE TABLE IF NOT EXISTS metrics_disk (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    node_id TEXT,
    timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
    total_gb REAL, used_gb REAL, free_gb REAL, usage_percent REAL
);

CREATE TABLE IF NOT EXISTS metrics_net (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    node_id TEXT,
    timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
    bytes_recv INTEGER, bytes_sent INTEGER,
    recv_rate_mb REAL, sent_rate_mb REAL
);

CREATE TABLE IF NOT EXISTS metrics_container (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    node_id TEXT,
    timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
    container_id TEXT, name TEXT, image TEXT, status TEXT,
    cpu_percent REAL, mem_usage_mb REAL, mem_limit_mb REAL
);

CREATE INDEX IF NOT EXISTS idx_metrics_cpu_node_time ON metrics_cpu(node_id, timestamp);
CREATE INDEX IF NOT EXISTS idx_metrics_mem_node_time ON metrics_mem(node_id, timestamp);
CREATE INDEX IF NOT EXISTS idx_metrics_disk_node_time ON metrics_disk(node_id, timestamp);
CREATE INDEX IF NOT EXISTS idx_metrics_net_node_time ON metrics_net(node_id, timestamp);
CREATE INDEX IF NOT EXISTS idx_metrics_container_node_time ON metrics_container(node_id, timestamp);

-- retention policy: keep 30 days
DELETE FROM metrics_cpu WHERE timestamp < datetime('now', '-30 days');
DELETE FROM metrics_mem WHERE timestamp < datetime('now', '-30 days');
DELETE FROM metrics_disk WHERE timestamp < datetime('now', '-30 days');
DELETE FROM metrics_net WHERE timestamp < datetime('now', '-30 days');
DELETE FROM metrics_container WHERE timestamp < datetime('now', '-30 days');