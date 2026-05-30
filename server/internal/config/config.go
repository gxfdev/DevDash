package config

import (
	"log"
	"os"
	"strconv"
)

type DBType string

const (
	DBSQLite     DBType = "sqlite"
	DBPostgreSQL DBType = "postgres"
)

type Config struct {
	ServerPort      string
	CollectInterval int
	DBType          DBType
	DBPath          string
	DBHost          string
	DBPort          int
	DBUser          string
	DBPassword      string
	DBName          string
	DBSSLMode       string
	JWTSecret       string
	AgentToken      string
}

type AgentConfig struct {
	NodeID          string
	NodeToken       string
	ServerURL       string
	CollectInterval int
}

func Load() *Config {
	port := os.Getenv("PORT")
	if port == "" {
		port = "9090"
	}
	interval := 5
	if i := os.Getenv("INTERVAL"); i != "" {
		if v, err := strconv.Atoi(i); err == nil {
			interval = v
		}
	}

	dbType := DBType(os.Getenv("DB_TYPE"))
	if dbType == "" {
		dbType = DBSQLite
	}

	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "./devdash.db"
	}

	dbHost := os.Getenv("DB_HOST")
	dbPort := 5432
	if p := os.Getenv("DB_PORT"); p != "" {
		if v, err := strconv.Atoi(p); err == nil {
			dbPort = v
		}
	}
	dbUser := os.Getenv("DB_USER")
	if dbUser == "" && dbType == DBPostgreSQL {
		dbUser = "devdash"
	}
	dbPassword := os.Getenv("DB_PASSWORD")
	dbName := os.Getenv("DB_NAME")
	if dbName == "" {
		if dbType == DBPostgreSQL {
			dbName = "devdash"
		}
	}

	dbSSLMode := os.Getenv("DB_SSLMODE")
	if dbSSLMode == "" {
		if dbType == DBPostgreSQL {
			dbSSLMode = "disable"
		}
	}

	secret := os.Getenv("JWT_SECRET")
	if secret != "" && len(secret) < 32 {
		log.Printf("[config] WARNING: JWT_SECRET should be at least 32 characters (current: %d)", len(secret))
	}

	agentToken := os.Getenv("AGENT_TOKEN")

	return &Config{
		ServerPort:      port,
		CollectInterval: interval,
		DBType:          dbType,
		DBPath:          dbPath,
		DBHost:          dbHost,
		DBPort:          dbPort,
		DBUser:          dbUser,
		DBPassword:      dbPassword,
		DBName:          dbName,
		DBSSLMode:       dbSSLMode,
		JWTSecret:       secret,
		AgentToken:      agentToken,
	}
}

func LoadAgent() *AgentConfig {
	interval := 30
	if i := os.Getenv("INTERVAL"); i != "" {
		if v, err := strconv.Atoi(i); err == nil {
			interval = v
		}
	}
	return &AgentConfig{
		NodeID:          os.Getenv("NODE_ID"),
		NodeToken:       os.Getenv("NODE_TOKEN"),
		ServerURL:       os.Getenv("SERVER_URL"),
		CollectInterval: interval,
	}
}
