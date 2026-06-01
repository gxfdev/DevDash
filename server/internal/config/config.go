package config

import "os"

type Config struct {
	Port        string
	JWTSecret   string
	DBPath      string
	GinMode     string
	CORSOrigins string
	DataDir     string
}

func Load() *Config {
	dataDir := envOr("DATA_DIR", "/data")
	return &Config{
		Port:        envOr("PORT", "9090"),
		JWTSecret:   envOr("JWT_SECRET", ""),
		DBPath:      envOr("DB_PATH", dataDir+"/webshell.db"),
		GinMode:     envOr("GIN_MODE", "release"),
		CORSOrigins: envOr("CORS_ORIGINS", "*"),
		DataDir:     dataDir,
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
