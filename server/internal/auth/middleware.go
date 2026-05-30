package auth

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var wsUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing authorization header"})
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid authorization format"})
			return
		}

		token := parts[1]
		claims, err := ValidateAccessToken(token)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
			return
		}

		userID, _ := claims["user_id"].(float64)
		username, _ := claims["username"].(string)
		role, _ := claims["role"].(string)

		c.Set("user_id", int(userID))
		c.Set("username", username)
		c.Set("role", role)
		c.Next()
	}
}

func RequireRole(roles ...string) gin.HandlerFunc {
	roleSet := make(map[string]bool, len(roles))
	for _, r := range roles {
		roleSet[r] = true
	}
	return func(c *gin.Context) {
		role, exists := c.Get("role")
		if !exists {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
			return
		}
		r, ok := role.(string)
		if !ok || !roleSet[r] {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "insufficient permissions"})
			return
		}
		c.Next()
	}
}

func WebsocketAuth(token string) (bool, string) {
	if token == "" {
		return false, ""
	}
	claims, err := ValidateAccessToken(token)
	if err != nil {
		return false, ""
	}
	username, _ := claims["username"].(string)
	return true, username
}

func WSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.Query("token")
		if token == "" {
			authHeader := c.GetHeader("Authorization")
			if authHeader != "" {
				parts := strings.SplitN(authHeader, " ", 2)
				if len(parts) == 2 && strings.ToLower(parts[0]) == "bearer" {
					token = parts[1]
				}
			}
			if token == "" {
				token = c.GetHeader("Sec-WebSocket-Protocol")
			}
		}
		if token == "" {
			rejectWSAuth(c, "missing token")
			return
		}

		claims, err := ValidateAccessToken(token)
		if err != nil {
			rejectWSAuth(c, "invalid or expired token")
			return
		}

		userID, _ := claims["user_id"].(float64)
		username, _ := claims["username"].(string)
		role, _ := claims["role"].(string)

		c.Set("user_id", int(userID))
		c.Set("username", username)
		c.Set("role", role)
		c.Next()
	}
}

func rejectWSAuth(c *gin.Context, reason string) {
	conn, err := wsUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		c.Header("Connection", "close")
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	conn.WriteMessage(websocket.TextMessage, []byte("\r\n\x1b[1;31m✗ 认证失败: "+reason+"\x1b[0m\r\n"))
	conn.WriteMessage(websocket.CloseMessage,
		websocket.FormatCloseMessage(4001, reason))
	conn.Close()
}
