package config

import (
	"log"
	"os"
	"strconv"
)

type Config struct {
	ServerPort      string
	CollectInterval int
	DBPath          string
	JWTSecret       string
}

type AgentConfig struct {
	NodeID         string
	NodeToken      string
	ServerURL      string
	CollectInterval int
}

func Load() *Config {
	port := os.Getenv("PORT")
	if port == "" {
		port = "9090"
	}
	interval := 30
	if i := os.Getenv("INTERVAL"); i != "" {
		if v, err := strconv.Atoi(i); err == nil {
			interval = v
		}
	}
	db := os.Getenv("DB_PATH")
	if db == "" {
		db = "./devdash.db"
	}
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		secret = "devdash-default-secret-change-in-production-2024"
		log.Println("[config] WARNING: JWT_SECRET not set, using default. Set JWT_SECRET env var for production.")
	}
	return &Config{
		ServerPort:      port,
		CollectInterval: interval,
		DBPath:          db,
		JWTSecret:       secret,
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
		NodeID:         os.Getenv("NODE_ID"),
		NodeToken:      os.Getenv("NODE_TOKEN"),
		ServerURL:      os.Getenv("SERVER_URL"),
		CollectInterval: interval,
	}
}