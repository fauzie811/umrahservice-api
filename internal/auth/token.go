package auth

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"umrahservice-api/internal/models"
)

const principalKey = "principal"

// Middleware returns a Gin middleware enforcing Sanctum bearer-token auth,
// equivalent to Laravel's `auth:sanctum` guard for the User model.
func Middleware(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		p, err := authenticate(db, c.GetHeader("Authorization"))
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"message": "Unauthenticated."})
			return
		}
		c.Set(principalKey, p)
		c.Next()
	}
}

// authenticate validates the bearer token and loads the principal.
func authenticate(db *gorm.DB, header string) (*Principal, error) {
	plain := bearer(header)
	if plain == "" {
		return nil, errUnauthenticated
	}

	id, secret, ok := strings.Cut(plain, "|")
	if !ok {
		return nil, errUnauthenticated
	}
	tokenID, err := strconv.ParseUint(id, 10, 64)
	if err != nil {
		return nil, errUnauthenticated
	}

	var token models.PersonalAccessToken
	if err := db.First(&token, tokenID).Error; err != nil {
		return nil, errUnauthenticated
	}

	if token.TokenableType != models.TokenableUser {
		return nil, errUnauthenticated
	}

	sum := sha256.Sum256([]byte(secret))
	want := hex.EncodeToString(sum[:])
	if subtle.ConstantTimeCompare([]byte(want), []byte(token.Token)) != 1 {
		return nil, errUnauthenticated
	}

	if token.ExpiresAt != nil && token.ExpiresAt.Before(time.Now()) {
		return nil, errUnauthenticated
	}

	var user models.User
	if err := db.First(&user, token.TokenableID).Error; err != nil {
		return nil, errUnauthenticated
	}

	// Touch last_used_at (best-effort, mirrors Sanctum).
	now := time.Now()
	db.Model(&token).UpdateColumn("last_used_at", now)
	token.LastUsedAt = &now

	return LoadPrincipal(db, &user, &token)
}

func bearer(header string) string {
	const prefix = "Bearer "
	if len(header) > len(prefix) && strings.EqualFold(header[:len(prefix)], prefix) {
		return strings.TrimSpace(header[len(prefix):])
	}
	return ""
}

// Current returns the authenticated principal from the Gin context.
func Current(c *gin.Context) *Principal {
	if v, ok := c.Get(principalKey); ok {
		if p, ok := v.(*Principal); ok {
			return p
		}
	}
	return nil
}

type authError string

func (e authError) Error() string { return string(e) }

const errUnauthenticated = authError("unauthenticated")
