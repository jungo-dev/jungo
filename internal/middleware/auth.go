package middleware

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/jungo-dev/junkit/response"
)

// Auth validates the API key provided in the Authorization Bearer header.
func Auth(apiKey string, responder response.Responder) gin.HandlerFunc {
	return func(c *gin.Context) {
		token, ok := bearerToken(c)
		if !ok || subtle.ConstantTimeCompare([]byte(token), []byte(apiKey)) != 1 {
			responder.Send(c, http.StatusUnauthorized, "unauthorized_token")
			c.Abort()
			return
		}
		c.Next()
	}
}

// bearerToken extracts the token from an "Authorization: Bearer <token>" header.
func bearerToken(c *gin.Context) (string, bool) {
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
		return "", false
	}

	token := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
	if token == "" {
		return "", false
	}
	return token, true
}
