package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"umrahservice-api/internal/auth"
	"umrahservice-api/internal/broadcast"
	"umrahservice-api/internal/config"
	"umrahservice-api/internal/laravel"
	"umrahservice-api/internal/models"
	"umrahservice-api/internal/storage"
	"umrahservice-api/internal/support"
)

// Handler bundles dependencies shared by all API endpoints.
type Handler struct {
	DB          *gorm.DB
	Storage     *storage.Storage
	Cfg         *config.Config
	Broadcaster *broadcast.Broadcaster
	Laravel     *laravel.Client
}

func New(db *gorm.DB, store *storage.Storage, cfg *config.Config, b *broadcast.Broadcaster, l *laravel.Client) *Handler {
	return &Handler{DB: db, Storage: store, Cfg: cfg, Broadcaster: b, Laravel: l}
}

// principal returns the authenticated principal (always set inside auth group).
func (h *Handler) principal(c *gin.Context) *auth.Principal {
	return auth.Current(c)
}

// userPayload mirrors the controllers' `array_merge($user->toArray(), [roles, permissions])`.
// It returns the meaningful, non-secret user attributes plus photo_url, roles
// and permissions. (Deprecated/secret columns like two_factor_secret are omitted.)
func (h *Handler) userPayload(p *auth.Principal) gin.H {
	u := p.User
	out := gin.H{
		"id":                u.ID,
		"name":              u.Name,
		"email":             u.Email,
		"phone":             u.Phone,
		"photo":             u.Photo,
		"staff_id":          u.StaffID,
		"last_login_at":     support.ISO(u.LastLoginAt),
		"last_login_ip":     u.LastLoginIP,
		"email_verified_at": support.ISO(u.EmailVerifiedAt),
		"meta":              rawJSON(u.Meta),
		"created_at":        support.ISO(&u.CreatedAt),
		"updated_at":        support.ISO(&u.UpdatedAt),
		"photo_url":         h.photoURL(u.Photo),
		"roles":             nonNilStrings(p.Roles),
		"permissions":       nonNilStrings(p.Permissions),
	}
	return out
}

// photoURL returns the S3 URL for a stored path, or nil.
func (h *Handler) photoURL(path *string) *string {
	if path == nil || *path == "" {
		return nil
	}
	url := h.Storage.URL(*path)
	return &url
}

// validationError responds with Laravel's 422 ValidationException shape.
func validationError(c *gin.Context, errs map[string][]string) {
	c.JSON(http.StatusUnprocessableEntity, gin.H{
		"message": firstMessage(errs),
		"errors":  errs,
	})
}

func firstMessage(errs map[string][]string) string {
	for _, msgs := range errs {
		if len(msgs) > 0 {
			return msgs[0]
		}
	}
	return "The given data was invalid."
}

func forbidden(c *gin.Context) {
	c.JSON(http.StatusForbidden, gin.H{"message": "Forbidden"})
}

func notFound(c *gin.Context, msg string) {
	if msg == "" {
		msg = "Not found"
	}
	c.JSON(http.StatusNotFound, gin.H{"message": msg})
}

// abort403 mirrors Laravel's Gate::authorize failure (403 with this message).
func abort403(c *gin.Context) {
	c.JSON(http.StatusForbidden, gin.H{"message": "This action is unauthorized."})
}

func nonNilStrings(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// ensure models import is used by handlers that reference it indirectly.
var _ = models.TokenableUser
