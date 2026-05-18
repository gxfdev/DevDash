-- nodes
CREATE TABLE IF NOT EXISTS nodes (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    os TEXT,
    arch TEXT,
    ip TEXT,
    role TEXT DEFAULT 'full',
    token TEXT UNIQUE,
    status TEXT DEFAULT 'offline',
    last_heartbeat DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- users
CREATE TABLE IF NOT EXISTS users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    role TEXT DEFAULT 'user',
    otp_enabled INTEGER DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- snapshots
CREATE TABLE IF NOT EXISTS snapshots (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    node_id TEXT,
    timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
    cpu REAL, mem REAL, disk REAL,
    net_recv REAL, net_sent REAL,
    load1 REAL, load5 REAL, load15 REAL
);

-- alerts
CREATE TABLE IF NOT EXISTS alerts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    node_id TEXT,
    type TEXT,
    level TEXT,
    value REAL,
    threshold REAL,
    time DATETIME DEFAULT CURRENT_TIMESTAMP,
    status TEXT DEFAULT 'active'
);

-- software
CREATE TABLE IF NOT EXISTS software (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    node_id TEXT,
    name TEXT,
    version TEXT,
    status TEXT,
    installed_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- cron_jobs
CREATE TABLE IF NOT EXISTS cron_jobs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    node_id TEXT,
    name TEXT,
    expression TEXT,
    command TEXT,
    enabled INTEGER DEFAULT 1,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- audit_logs
CREATE TABLE IF NOT EXISTS audit_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER,
    node_id TEXT,
    action TEXT,
    detail TEXT,
    result TEXT,
    time DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- default admin
INSERT OR IGNORE INTO users (username, password_hash, role)
VALUES ('admin', '$2a$10$N9qo8uLOickgx2ZMRZoMye.IqOOZwlHGvBH1Jgz4E/3w5VxJ5zKKS', 'admin');

CREATE INDEX IF NOT EXISTS idx_snapshots_node_time ON snapshots(node_id, timestamp);
CREATE INDEX IF NOT EXISTS idx_alerts_node_time ON alerts(node_id, time);
CREATE INDEX IF NOT EXISTS idx_software_node ON software(node_id);