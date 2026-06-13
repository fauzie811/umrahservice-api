package ratelimit

import (
	"sync"
	"time"
)

type counter struct {
	count   int
	resetAt time.Time
}

// Limiter is a process-local fixed-window rate limiter (mirrors Laravel's
// cache-backed RateLimiter with a 60s decay).
type Limiter struct {
	mu      sync.Mutex
	windows map[string]*counter
}

func New() *Limiter {
	return &Limiter{windows: make(map[string]*counter)}
}

// Hit records one request for key. Returns false (with the time until reset)
// when the request exceeds max within window.
func (l *Limiter) Hit(key string, max int, window time.Duration) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	c := l.windows[key]
	if c == nil || now.After(c.resetAt) {
		c = &counter{resetAt: now.Add(window)}
		l.windows[key] = c
	}
	c.count++
	if c.count > max {
		return false, time.Until(c.resetAt)
	}
	return true, 0
}
