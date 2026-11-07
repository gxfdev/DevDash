package config

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Config is the single-host AI-OS runtime configuration.
type Config struct {
	Port         string
	JWTSecret    string
	DataDir      string
	DBPath       string
	StaticDir    string
	DockerHost   string
	HostRoot     string
	HostProc     string
	HostSys      string
	HostDev      string
	CollectEvery time.Duration
	AdminUser    string
	AdminPass    string
	Edition      string
	Version      string
	PublicURL    string
}

func env(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

func Load() *Config {
	dataDir := env("AIDASH_DATA_DIR", "./data")
	_ = os.MkdirAll(dataDir, 0o755)

	dockerHost := env("DOCKER_HOST", defaultDockerHost())
	interval := 5
	if v, err := strconv.Atoi(env("COLLECT_INTERVAL", "5")); err == nil && v >= 2 {
		interval = v
	}

	secret := strings.TrimSpace(os.Getenv("JWT_SECRET"))
	if secret == "" {
		secret = readOrCreateSecret(filepath.Join(dataDir, ".jwt_secret"))
	}
	if len(secret) < 16 {
		secret = secret + "-aidash-" + hex.EncodeToString(randomBytes(8))
	}

	return &Config{
		Port:         env("PORT", "9090"),
		JWTSecret:    secret,
		DataDir:      dataDir,
		DBPath:       env("DB_PATH", filepath.Join(dataDir, "aidash.db")),
		StaticDir:    env("STATIC_DIR", "../web/dist"),
		DockerHost:   dockerHost,
		HostRoot:     env("HOST_ROOT", ""),
		HostProc:     env("HOST_PROC", ""),
		HostSys:      env("HOST_SYS", ""),
		HostDev:      env("HOST_DEV", ""),
		CollectEvery: time.Duration(interval) * time.Second,
		AdminUser:    env("ADMIN_USER", "admin"),
		AdminPass:    env("ADMIN_PASS", "admin123"),
		Edition:      "AIDash 单机 AI 操作系统",
		Version:      "2.0.0",
		PublicURL:    env("PUBLIC_URL", ""),
	}
}

func defaultDockerHost() string {
	if os.PathSeparator == '\\' {
		return "npipe:////./pipe/docker_engine"
	}
	return "unix:///var/run/docker.sock"
}

func randomBytes(n int) []byte {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		for i := range b {
			b[i] = byte(i * 37)
		}
	}
	return b
}

func readOrCreateSecret(path string) string {
	if b, err := os.ReadFile(path); err == nil && len(strings.TrimSpace(string(b))) >= 16 {
		return strings.TrimSpace(string(b))
	}
	s := hex.EncodeToString(randomBytes(32))
	_ = os.WriteFile(path, []byte(s), 0o600)
	return s
}
