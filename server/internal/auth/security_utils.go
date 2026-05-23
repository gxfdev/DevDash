package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
	"log"
	"net"
	"net/url"
	"os"
	"strings"
	"sync"
)

var (
	ErrInvalidEndpoint = errors.New("invalid endpoint URL")
	ErrSSRFBlocked     = errors.New("endpoint resolves to private/restricted IP address")
)

var privateIPRanges = []struct {
	network string
}{
	{"10.0.0.0/8"},
	{"172.16.0.0/12"},
	{"192.168.0.0/16"},
	{"127.0.0.0/8"},
	{"169.254.0.0/16"},
	{"0.0.0.0/8"},
	{"100.64.0.0/10"},
	{"198.18.0.0/15"},
	{"::1/128"},
	{"fc00::/7"},
	{"fe80::/10"},
	{"ff00::/8"},
}

func IsPrivateIP(ipStr string) bool {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}
	for _, r := range privateIPRanges {
		_, cidr, err := net.ParseCIDR(r.network)
		if err != nil {
			continue
		}
		if cidr.Contains(ip) {
			return true
		}
	}
	return ip.IsUnspecified() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast()
}

func ValidateEndpoint(rawURL string) error {
	if rawURL == "" {
		return ErrInvalidEndpoint
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return ErrInvalidEndpoint
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return ErrInvalidEndpoint
	}
	host := u.Hostname()
	if host == "" {
		return ErrInvalidEndpoint
	}
	if host == "localhost" || host == "localhost.localdomain" {
		return ErrSSRFBlocked
	}
	ip := net.ParseIP(host)
	if ip != nil && IsPrivateIP(host) {
		return ErrSSRFBlocked
	}
	ips, err := net.LookupIP(host)
	if err == nil {
		for _, resolvedIP := range ips {
			if IsPrivateIP(resolvedIP.String()) {
				return ErrSSRFBlocked
			}
		}
	}
	return nil
}

func SanitizeLog(s string) string {
	s = strings.ReplaceAll(s, "password=", "password=***")
	s = strings.ReplaceAll(s, "Password=", "Password=***")
	s = strings.ReplaceAll(s, "token=", "token=***")
	s = strings.ReplaceAll(s, "Token=", "Token=***")
	s = strings.ReplaceAll(s, "secret=", "secret=***")
	s = strings.ReplaceAll(s, "SECRET=", "SECRET=***")
	return s
}

var (
	encKey     []byte
	encKeyOnce sync.Once
)

func getEncryptionKey() []byte {
	encKeyOnce.Do(func() {
		key := os.Getenv("ENCRYPTION_KEY")
		if key == "" {
			key = os.Getenv("JWT_SECRET")
		}
		if len(key) >= 32 {
			encKey = []byte(key[:32])
		} else if isProductionMode() {
			log.Println("[auth] WARNING: ENCRYPTION_KEY/JWT_SECRET too short for production, generating random key")
			encKey = generateRandomKey(32)
		} else if len(key) >= 16 {
			padded := make([]byte, 32)
			copy(padded, key)
			copy(padded[len(key):], key)
			encKey = padded
			log.Println("[auth] WARNING: encryption key shorter than 32 bytes, key was padded - set ENCRYPTION_KEY for production")
		} else {
			encKey = generateRandomKey(32)
			log.Println("[auth] WARNING: no valid encryption key found, using random key - data encrypted with this key will not survive restart")
		}
	})
	return encKey
}

func isProductionMode() bool {
	return os.Getenv("GIN_MODE") == "release"
}

func generateRandomKey(size int) []byte {
	key := make([]byte, size)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		log.Fatalf("[auth] failed to generate random key: %v", err)
	}
	return key
}

func EncryptField(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	key := getEncryptionKey()
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, aesGCM.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ciphertext := aesGCM.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func DecryptField(encoded string) (string, error) {
	if encoded == "" {
		return "", nil
	}
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return encoded, nil
	}
	key := getEncryptionKey()
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonceSize := aesGCM.NonceSize()
	if len(data) < nonceSize {
		return "", errors.New("ciphertext too short")
	}
	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := aesGCM.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}
