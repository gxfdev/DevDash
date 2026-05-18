package dbmgr

import (
	"database/sql"
	"fmt"
	"strconv"

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

func Connect(conn DBConnection) (*sql.DB, error) {
	var dsn string
	switch conn.Type {
	case "mysql":
		dsn = conn.User + ":" + conn.Password + "@tcp(" + conn.Host + ":" + strconv.Itoa(conn.Port) + ")/" + conn.Name + "?parseTime=true"
	case "postgres":
		dsn = fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
			conn.Host, conn.Port, conn.User, conn.Password, conn.Name)
	default:
		return nil, fmt.Errorf("unsupported database type: %s", conn.Type)
	}

	db, err := sql.Open(conn.Type, dsn)
	if err != nil {
		return nil, err
	}
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

func ExecuteQuery(db *sql.DB, query string) ([]map[string]interface{}, error) {
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	cols, _ := rows.Columns()
	var result []map[string]interface{}
	for rows.Next() {
		row := make(map[string]interface{})
		cols2 := make([]interface{}, len(cols))
		for i := range cols2 {
			cols2[i] = new(interface{})
		}
		rows.Scan(cols2...)
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
