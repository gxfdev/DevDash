package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

var (
	loginAttempts = make(map[string]*loginRecord)
	loginMu      sync.RWMutex
)

type loginRecord struct {
	attempts  int
	lastAttempt time.Time
	blockedUntil time.Time
}

const (
	maxLoginAttempts    = 5
	lockoutDuration     = 15 * time.Minute
	cleanupInterval     = 5 * time.Minute
)

func init() {
	go cleanupOldRecords()
}

func cleanupOldRecords() {
	ticker := time.NewTicker(cleanupInterval)
	defer ticker.Stop()
	for range ticker.C {
		loginMu.Lock()
		now := time.Now()
		for ip, record := range loginAttempts {
			if now.Sub(record.lastAttempt) > lockoutDuration*2 {
				delete(loginAttempts, ip)
			}
		}
		loginMu.Unlock()
	}
}

func CheckLoginRateLimit(ip string) error {
	loginMu.Lock()
	defer loginMu.Unlock()

	record, exists := loginAttempts[ip]
	now := time.Now()

	if exists {
		if now.Before(record.blockedUntil) {
			return errors.New("too many attempts, please try again later")
		}
		if record.attempts >= maxLoginAttempts && now.Sub(record.lastAttempt) < lockoutDuration {
			record.blockedUntil = now.Add(lockoutDuration)
			return errors.New("account locked due to too many failed attempts")
		}
	}

	if !exists {
		loginAttempts[ip] = &loginRecord{
			attempts:      1,
			lastAttempt:   now,
		}
	} else {
		record.attempts++
		record.lastAttempt = now
		if record.attempts >= maxLoginAttempts {
			record.blockedUntil = now.Add(lockoutDuration)
			return errors.New("too many failed attempts, account temporarily locked")
		}
	}
	return nil
}

func ClearLoginAttempts(ip string) {
	loginMu.Lock()
	defer loginMu.Unlock()
	delete(loginAttempts, ip)
}

func ResetLoginAttempts(ip string) {
	loginMu.Lock()
	defer loginMu.Unlock()
	if record, exists := loginAttempts[ip]; exists {
		record.attempts = 0
		record.blockedUntil = time.Time{}
	}
}

func SecurityMiddleware(allowedIPs []string) gin.HandlerFunc {
	ipSet := make(map[string]bool)
	for _, ip := range allowedIPs {
		ipSet[ip] = true
	}

	return func(c *gin.Context) {
		clientIP := c.ClientIP()

		if len(ipSet) > 0 && !ipSet[clientIP] && !ipSet["127.0.0.1"] && !ipSet["::1"] {
			c.AbortWithStatusJSON(403, gin.H{"error": "IP not allowed"})
			return
		}

		c.Header("X-Request-ID", generateRequestID())
		c.Next()
	}
}

func CSRFMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method == "GET" || c.Request.Method == "HEAD" || c.Request.Method == "OPTIONS" {
			c.Next()
			return
		}

		token := c.GetHeader("X-CSRF-Token")
		cookie, err := c.Cookie("csrf_token")
		if err != nil || token == "" || token != cookie {
			c.AbortWithStatusJSON(403, gin.H{"error": "CSRF validation failed"})
			return
		}
		c.Next()
	}
}

func GenerateCSRFToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func generateRequestID() string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}