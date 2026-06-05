package handlers

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"

	"umrahservice-api/internal/support"
)

// xsrfCookieLifetime mirrors Laravel's default session lifetime (120 minutes).
const xsrfCookieLifetime = 120 * 60

// CsrfCookie mirrors Sanctum's CsrfCookieController::show. The SPA client hits
// this before logging in to obtain an XSRF-TOKEN cookie. This API authenticates
// with Bearer tokens and does not validate CSRF, so the endpoint only needs to
// set a readable XSRF-TOKEN cookie and return 204 No Content for compatibility.
func (h *Handler) CsrfCookie(c *gin.Context) {
	token := support.RandomToken()

	c.SetSameSite(http.SameSiteLaxMode)
	// XSRF-TOKEN must be readable by JS (HttpOnly=false) so the client can echo
	// it back in the X-XSRF-TOKEN header. Value is URL-encoded like Laravel.
	c.SetCookie(
		"XSRF-TOKEN",
		url.QueryEscape(token),
		xsrfCookieLifetime,
		"/",
		"",
		h.secureCookies(),
		false,
	)

	c.Status(http.StatusNoContent)
}

// secureCookies marks cookies Secure when serving over HTTPS (production).
func (h *Handler) secureCookies() bool {
	return strings.HasPrefix(h.Cfg.AppURL, "https://")
}
