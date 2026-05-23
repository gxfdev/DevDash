package dbmgr

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"sync"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
)

type DBConnection struct {
	ID       int    `json:"id"`
	Type     string `json:"type"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"`
	Name     string `json:"name"`
	Online   bool   `json:"online"`
	Version  string `json:"version"`
}

type poolEntry struct {
	db      *sql.DB
	lastUsed time.Time
}

var (
	connPool   = make(map[int]*poolEntry)
	poolMu     sync.RWMutex
	poolMaxAge = 10 * time.Minute
)

func init() {
	go func() {
		ticker := time.NewTicker(2 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			cleanupPool()
		}
	}()
}

func cleanupPool() {
	poolMu.Lock()
	defer poolMu.Unlock()
	now := time.Now()
	for id, entry := range connPool {
		if now.Sub(entry.lastUsed) > poolMaxAge {
			entry.db.Close()
			delete(connPool, id)
		}
	}
}

func GetPooledConnection(id int, conn DBConnection) (*sql.DB, error) {
	poolMu.RLock()
	entry, ok := connPool[id]
	if ok {
		if err := entry.db.Ping(); err == nil {
			entry.lastUsed = time.Now()
			poolMu.RUnlock()
			return entry.db, nil
		}
		entry.db.Close()
		delete(connPool, id)
	}
	poolMu.RUnlock()

	db, err := Connect(conn)
	if err != nil {
		return nil, err
	}

	poolMu.Lock()
	if old, exists := connPool[id]; exists {
		old.db.Close()
	}
	connPool[id] = &poolEntry{db: db, lastUsed: time.Now()}
	poolMu.Unlock()

	return db, nil
}

func ClosePooledConnection(id int) {
	poolMu.Lock()
	defer poolMu.Unlock()
	if entry, ok := connPool[id]; ok {
		entry.db.Close()
		delete(connPool, id)
	}
}

func Connect(conn DBConnection) (*sql.DB, error) {
	var dsn string
	switch conn.Type {
	case "mysql":
		dsn = conn.User + ":" + conn.Password + "@tcp(" + conn.Host + ":" + strconv.Itoa(conn.Port) + ")/" + conn.Name + "?parseTime=true&timeout=10s"
	case "postgres":
		dsn = fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable&connect_timeout=10",
			conn.Host, conn.Port, conn.User, conn.Password, conn.Name)
	default:
		return nil, fmt.Errorf("unsupported database type: %s", conn.Type)
	}

	db, err := sql.Open(conn.Type, dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(2)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(5 * time.Minute)
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("connection failed: %v", err)
	}
	return db, nil
}

func ListTables(db *sql.DB, dbType string) ([]string, error) {
	var query string
	switch dbType {
	case "mysql":
		query = "SHOW TABLES"
	case "postgres":
		query = "SELECT tablename FROM pg_tables WHERE schemaname='public'"
	default:
		return nil, fmt.Errorf("unsupported type: %s", dbType)
	}
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tables []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			continue
		}
		tables = append(tables, t)
	}
	return tables, nil
}

const maxQueryRows = 1000

func ExecuteQuery(db *sql.DB, query string) ([]map[string]interface{}, error) {
	return ExecuteQueryWithContext(context.Background(), db, query)
}

func ExecuteQueryWithContext(ctx context.Context, db *sql.DB, query string) ([]map[string]interface{}, error) {
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	cols, _ := rows.Columns()
	var result []map[string]interface{}
	count := 0
	for rows.Next() {
		if count >= maxQueryRows {
			break
		}
		row := make(map[string]interface{})
		cols2 := make([]interface{}, len(cols))
		for i := range cols2 {
			cols2[i] = new(interface{})
		}
		if err := rows.Scan(cols2...); err != nil {
			continue
		}
		for i, c := range cols {
			val := cols2[i].(*interface{})
			switch v := (*val).(type) {
			case []byte:
				row[c] = string(v)
			default:
				row[c] = v
			}
		}
		result = append(result, row)
		count++
	}
	return result, nil
}

func GetVersion(db *sql.DB, dbType string) string {
	var query string
	switch dbType {
	case "mysql":
		query = "SELECT VERSION()"
	case "postgres":
		query = "SELECT version()"
	default:
		return ""
	}
	var v string
	if err := db.QueryRow(query).Scan(&v); err != nil {
		return ""
	}
	return v
}

func Backup(db *sql.DB, dbType, name string) ([]byte, error) {
	return nil, fmt.Errorf("backup not implemented yet")
}
