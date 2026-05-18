package config

import (
	"crypto/rand"
	"encoding/hex"
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
		if os.Getenv("GIN_MODE") == "release" {
			log.Fatal("FATAL: JWT_SECRET environment variable is required in production mode!")
		}
		b := make([]byte, 32)
		if _, err := rand.Read(b); err != nil {
			log.Fatalf("FATAL: Failed to generate random JWT secret: %v", err)
		}
		secret = hex.EncodeToString(b)
		log.Printf("[config] WARNING: Using auto-generated JWT secret. Set JWT_SECRET env var for production!")
		log.Printf("[config] Auto-generated secret (save this): %s", secret)
	} else if len(secret) < 32 {
		log.Printf("[config] WARNING: JWT_SECRET should be at least 32 characters (current: %d)", len(secret))
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