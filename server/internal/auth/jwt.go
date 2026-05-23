package auth

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

var Secret []byte

func InitSecret(secret string) error {
	if secret != "" {
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
	if isProduction() {
		return fmt.Errorf("JWT_SECRET environment variable is required in production")
	}
	Secret = []byte("devdash-dev-secret-key-min-32ch!!")
	fmt.Println("WARNING: Using default dev JWT secret. Set JWT_SECRET env var for production!")
	return nil
}

func isProduction() bool {
	return os.Getenv("GIN_MODE") == "release"
}

const (
	AccessTokenTTL  = 24 * time.Hour
	RefreshTokenTTL = 7 * 24 * time.Hour
)

type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
}

func GenerateToken(userID int, username, role string) (string, error) {
	if len(Secret) == 0 {
		return "", errors.New("auth: Secret not initialized, call InitSecret first")
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id":  userID,
		"username": username,
		"role":     role,
		"type":     "access",
		"exp":      time.Now().Add(AccessTokenTTL).Unix(),
		"iat":      time.Now().Unix(),
	}).SignedString(Secret)
}

func GenerateRefreshToken(userID int, username, role string) (string, error) {
	if len(Secret) == 0 {
		return "", errors.New("auth: Secret not initialized")
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id":  userID,
		"username": username,
		"role":     role,
		"type":     "refresh",
		"exp":      time.Now().Add(RefreshTokenTTL).Unix(),
		"iat":      time.Now().Unix(),
	}).SignedString(Secret)
}

func GenerateTokenPair(userID int, username, role string) (*TokenPair, error) {
	accessToken, err := GenerateToken(userID, username, role)
	if err != nil {
		return nil, err
	}
	refreshToken, err := GenerateRefreshToken(userID, username, role)
	if err != nil {
		return nil, err
	}
	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int64(AccessTokenTTL.Seconds()),
	}, nil
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

func ValidateAccessToken(token string) (jwt.MapClaims, error) {
	claims, err := ValidateToken(token)
	if err != nil {
		return nil, err
	}
	if tokenType, ok := claims["type"].(string); !ok || tokenType != "access" {
		return nil, errors.New("not an access token")
	}
	return claims, nil
}

func ValidateRefreshToken(token string) (jwt.MapClaims, error) {
	claims, err := ValidateToken(token)
	if err != nil {
		return nil, err
	}
	if tokenType, ok := claims["type"].(string); !ok || tokenType != "refresh" {
		return nil, errors.New("not a refresh token")
	}
	return claims, nil
}

func CheckPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

func HashPassword(password string) string {
	h, _ := bcrypt.GenerateFromPassword([]byte(password), 12)
	return string(h)
}
