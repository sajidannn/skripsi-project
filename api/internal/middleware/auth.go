// Package middleware provides Gin middleware used by the API.
package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	jwt "github.com/golang-jwt/jwt/v5"
	"github.com/sajidannn/pos-api/internal/tenant"
)

// Claims is the set of custom JWT claims expected in every request.
type Claims struct {
	TenantID int    `json:"tenant_id"`
	UserID   int    `json:"user_id"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

// Auth returns a Gin middleware that validates the Bearer JWT and injects the
// tenant ID into the request context.
func Auth(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing or malformed Authorization header"})
			return
		}

		tokenStr := strings.TrimPrefix(authHeader, "Bearer ")

		claims := &Claims{}
		token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrSignatureInvalid
			}
			return []byte(secret), nil
		})

		if err != nil || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
			return
		}

		if claims.TenantID == 0 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "token missing tenant_id claim"})
			return
		}

		// Store tenant ID in both Gin context and request context.
		c.Set("tenant_id", claims.TenantID)
		c.Set("user_id", claims.UserID)
		c.Set("role", claims.Role)
		ctx := tenant.NewContext(c.Request.Context(), claims.TenantID)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

// RequireRole returns a Gin middleware that enforces role-based access control.
// It must be used AFTER the Auth middleware (which sets the "role" key in context).
// Call it per-route or per-group with one or more allowed roles, e.g.:
//
//	api.POST("/users", middleware.RequireRole("owner"), h.User.Create)
func RequireRole(roles ...string) gin.HandlerFunc {
	allowed := make(map[string]struct{}, len(roles))
	for _, r := range roles {
		allowed[r] = struct{}{}
	}

	return func(c *gin.Context) {
		role, _ := c.Get("role")
		roleStr, _ := role.(string)
		if _, ok := allowed[roleStr]; !ok {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": "you do not have permission to perform this action",
			})
			return
		}
		c.Next()
	}
}
