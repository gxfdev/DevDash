package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"

	"webshell/internal/auth"
)

type Store struct {
	db *sql.DB
}

type User struct {
	ID        int64  `json:"id"`
	Username  string `json:"username"`
	Password  string `json:"-"`
	Role      string `json:"role"`
	CreatedAt string `json:"created_at"`
}

type CronJob struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Schedule  string `json:"schedule"`
	Command   string `json:"command"`
	Enabled   bool   `json:"enabled"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type Script struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Content     string `json:"content"`
	Interpreter string `json:"interpreter"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

type AuditLog struct {
	ID        int64  `json:"id"`
	Username  string `json:"username"`
	Action    string `json:"action"`
	Detail    string `json:"detail"`
	CreatedAt string `json:"created_at"`
}

func New(dbPath string) (*Store, error) {
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create db dir: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	db.SetMaxOpenConns(1)

	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return s, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		username TEXT UNIQUE NOT NULL,
		password TEXT NOT NULL,
		role TEXT DEFAULT 'user',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE TABLE IF NOT EXISTS cron_jobs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		schedule TEXT NOT NULL,
		command TEXT NOT NULL,
		enabled INTEGER DEFAULT 1,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE TABLE IF NOT EXISTS scripts (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		description TEXT DEFAULT '',
		content TEXT NOT NULL,
		interpreter TEXT DEFAULT '/bin/bash',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE TABLE IF NOT EXISTS audit_logs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		username TEXT NOT NULL,
		action TEXT NOT NULL,
		detail TEXT DEFAULT '',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	`
	_, err := s.db.Exec(schema)
	if err != nil {
		return err
	}
	return s.ensureAdmin()
}

func (s *Store) ensureAdmin() error {
	var count int
	s.db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
	if count > 0 {
		return nil
	}
	hash, err := auth.HashPassword("admin123")
	if err != nil {
		return err
	}
	_, err = s.db.Exec("INSERT INTO users (username, password, role) VALUES (?, ?, 'admin')", "admin", hash)
	return err
}

func (s *Store) GetUserByUsername(username string) (*User, error) {
	u := &User{}
	err := s.db.QueryRow("SELECT id, username, password, role, created_at FROM users WHERE username=?", username).
		Scan(&u.ID, &u.Username, &u.Password, &u.Role, &u.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return u, err
}

func (s *Store) GetUserByID(id int64) (*User, error) {
	u := &User{}
	err := s.db.QueryRow("SELECT id, username, password, role, created_at FROM users WHERE id=?", id).
		Scan(&u.ID, &u.Username, &u.Password, &u.Role, &u.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return u, err
}

func (s *Store) ListUsers() ([]User, error) {
	rows, err := s.db.Query("SELECT id, username, role, created_at FROM users ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var users []User
	for rows.Next() {
		var u User
		rows.Scan(&u.ID, &u.Username, &u.Role, &u.CreatedAt)
		users = append(users, u)
	}
	return users, nil
}

func (s *Store) UpdatePassword(userID int64, newPassword string) error {
	hash, err := auth.HashPassword(newPassword)
	if err != nil {
		return err
	}
	_, err = s.db.Exec("UPDATE users SET password=? WHERE id=?", hash, userID)
	return err
}

func (s *Store) CreateUser(username, password, role string) error {
	hash, err := auth.HashPassword(password)
	if err != nil {
		return err
	}
	_, err = s.db.Exec("INSERT INTO users (username, password, role) VALUES (?, ?, ?)", username, hash, role)
	return err
}

func (s *Store) DeleteUser(userID int64) error {
	_, err := s.db.Exec("DELETE FROM users WHERE id=? AND role != 'admin'", userID)
	return err
}

func (s *Store) ListCronJobs() ([]CronJob, error) {
	rows, err := s.db.Query("SELECT id, name, schedule, command, enabled, created_at, updated_at FROM cron_jobs ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var jobs []CronJob
	for rows.Next() {
		var j CronJob
		var enabled int
		rows.Scan(&j.ID, &j.Name, &j.Schedule, &j.Command, &enabled, &j.CreatedAt, &j.UpdatedAt)
		j.Enabled = enabled == 1
		jobs = append(jobs, j)
	}
	return jobs, nil
}

func (s *Store) CreateCronJob(name, schedule, command string) error {
	_, err := s.db.Exec("INSERT INTO cron_jobs (name, schedule, command) VALUES (?, ?, ?)", name, schedule, command)
	return err
}

func (s *Store) UpdateCronJob(id int64, name, schedule, command string, enabled bool) error {
	e := 0
	if enabled {
		e = 1
	}
	_, err := s.db.Exec("UPDATE cron_jobs SET name=?, schedule=?, command=?, enabled=?, updated_at=? WHERE id=?",
		name, schedule, command, e, time.Now().Format("2006-01-02 15:04:05"), id)
	return err
}

func (s *Store) DeleteCronJob(id int64) error {
	_, err := s.db.Exec("DELETE FROM cron_jobs WHERE id=?", id)
	return err
}

func (s *Store) GetCronJob(id int64) (*CronJob, error) {
	j := &CronJob{}
	var enabled int
	err := s.db.QueryRow("SELECT id, name, schedule, command, enabled, created_at, updated_at FROM cron_jobs WHERE id=?", id).
		Scan(&j.ID, &j.Name, &j.Schedule, &j.Command, &enabled, &j.CreatedAt, &j.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	j.Enabled = enabled == 1
	return j, err
}

func (s *Store) ListScripts() ([]Script, error) {
	rows, err := s.db.Query("SELECT id, name, description, content, interpreter, created_at, updated_at FROM scripts ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var scripts []Script
	for rows.Next() {
		var sc Script
		rows.Scan(&sc.ID, &sc.Name, &sc.Description, &sc.Content, &sc.Interpreter, &sc.CreatedAt, &sc.UpdatedAt)
		scripts = append(scripts, sc)
	}
	return scripts, nil
}

func (s *Store) GetScript(id int64) (*Script, error) {
	sc := &Script{}
	err := s.db.QueryRow("SELECT id, name, description, content, interpreter, created_at, updated_at FROM scripts WHERE id=?", id).
		Scan(&sc.ID, &sc.Name, &sc.Description, &sc.Content, &sc.Interpreter, &sc.CreatedAt, &sc.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return sc, err
}

func (s *Store) CreateScript(name, description, content, interpreter string) error {
	_, err := s.db.Exec("INSERT INTO scripts (name, description, content, interpreter) VALUES (?, ?, ?, ?)",
		name, description, content, interpreter)
	return err
}

func (s *Store) UpdateScript(id int64, name, description, content, interpreter string) error {
	_, err := s.db.Exec("UPDATE scripts SET name=?, description=?, content=?, interpreter=?, updated_at=? WHERE id=?",
		name, description, content, interpreter, time.Now().Format("2006-01-02 15:04:05"), id)
	return err
}

func (s *Store) DeleteScript(id int64) error {
	_, err := s.db.Exec("DELETE FROM scripts WHERE id=?", id)
	return err
}

func (s *Store) AddAuditLog(username, action, detail string) error {
	_, err := s.db.Exec("INSERT INTO audit_logs (username, action, detail) VALUES (?, ?, ?)", username, action, detail)
	return err
}

func (s *Store) ListAuditLogs(limit int) ([]AuditLog, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.db.Query("SELECT id, username, action, detail, created_at FROM audit_logs ORDER BY id DESC LIMIT ?", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var logs []AuditLog
	for rows.Next() {
		var l AuditLog
		rows.Scan(&l.ID, &l.Username, &l.Action, &l.Detail, &l.CreatedAt)
		logs = append(logs, l)
	}
	return logs, nil
}
