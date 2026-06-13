package router

import (
	"bytes"
	"encoding/json"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"umrahservice-api/internal/ratelimit"
)

func tooMany(c *gin.Context, retryAfter time.Duration) {
	secs := int(math.Ceil(retryAfter.Seconds()))
	if secs < 1 {
		secs = 1
	}
	c.Header("Retry-After", strconv.Itoa(secs))
	c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"message": "Too Many Attempts."})
}

// apiThrottle mirrors throttle:api — 120/min by client IP. (The principal is
// not yet resolved at group entry, so we key by IP, matching Laravel where
// $request->user() is null at throttle time for token requests.)
func apiThrottle(l *ratelimit.Limiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		if ok, retry := l.Hit("api:"+c.ClientIP(), 120, time.Minute); !ok {
			tooMany(c, retry)
			return
		}
		c.Next()
	}
}

// loginThrottle mirrors throttle:login — 5/min/IP and 20/min/email.
func loginThrottle(l *ratelimit.Limiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		if ok, retry := l.Hit("login-ip:"+c.ClientIP(), 5, time.Minute); !ok {
			tooMany(c, retry)
			return
		}
		email := peekEmail(c)
		if ok, retry := l.Hit("login-email:"+email, 20, time.Minute); !ok {
			tooMany(c, retry)
			return
		}
		c.Next()
	}
}

// peekEmail reads the email field without consuming the body for the handler.
func peekEmail(c *gin.Context) string {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return "unknown"
	}
	c.Request.Body = io.NopCloser(bytes.NewReader(body))

	var j struct {
		Email string `json:"email"`
	}
	if json.Unmarshal(body, &j) == nil && j.Email != "" {
		return strings.ToLower(strings.TrimSpace(j.Email))
	}
	if vals, err := url.ParseQuery(string(body)); err == nil {
		if e := vals.Get("email"); e != "" {
			return strings.ToLower(strings.TrimSpace(e))
		}
	}
	return "unknown"
}
