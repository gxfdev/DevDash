package auth

import (
	"crypto/rand"
	"errors"
	"fmt"
	"runtime"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

// Secret is set at startup via config. Must be at least 32 bytes.
var Secret []byte

// InitSecret sets the JWT secret. Call once at startup.
// If secret is empty, generates a random 32-byte secret (useful for dev only).
func InitSecret(secret string) error {
	if secret == "" {
		if runtime.GOOS != "windows" && isProduction() {
			return fmt.Errorf("JWT_SECRET environment variable is required in production")
		}
		b := make([]byte, 32)
		rand.Read(b)
		Secret = b
		fmt.Println("⚠️  WARNING: Using auto-generated JWT secret. Set JWT_SECRET env var for production!")
		return nil
	}

	if len(secret) < 32 {
		return fmt.Errorf("JWT_SECRET must be at least 32 characters (current: %d)", len(secret))
	}

	weakSecrets := []string{
		"secret", "password", "123456", "admin", "devdash",
		"jwt_secret", "changeme", "default", "test",
	}
	for _, weak := range weakSecrets {
		if secret == weak {
			return fmt.Errorf("JWT_SECRET '%s' is too common and insecure", secret)
		}
	}

	Secret = []byte(secret)
	return nil
}

func isProduction() bool {
	return false
}

func GenerateToken(userID int, username, role string) (string, error) {
	if len(Secret) == 0 {
		return "", errors.New("auth: Secret not initialized, call InitSecret first")
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id":  userID,
		"username": username,
		"role":     role,
		"exp":      time.Now().Add(7 * 24 * time.Hour).Unix(),
	}).SignedString(Secret)
}

func ValidateToken(token string) (jwt.MapClaims, error) {
	if len(Secret) == 0 {
		return nil, errors.New("auth: Secret not initialized")
	}
	t, err := jwt.Parse(token, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("invalid signing method")
		}
		return Secret, nil
	})
	if err != nil {
		return nil, err
	}
	if claims, ok := t.Claims.(jwt.MapClaims); ok && t.Valid {
		return claims, nil
	}
	return nil, errors.New("invalid token")
}

func CheckPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

func HashPassword(password string) string {
	h, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(h)
}