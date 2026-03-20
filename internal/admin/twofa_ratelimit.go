package admin

import (
	"log"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
)

type twofaAttempt struct {
	count     int
	firstSeen time.Time
}

type twofaRateLimiter struct {
	mu       sync.Mutex
	attempts map[string]*twofaAttempt
	maxFails int
	window   time.Duration
}

func newTwoFARateLimiter(maxFails int, window time.Duration) *twofaRateLimiter {
	return &twofaRateLimiter{
		attempts: make(map[string]*twofaAttempt),
		maxFails: maxFails,
		window:   window,
	}
}

// RecordFailure increments the failure count for a user and returns true if the user is now blocked.
func (r *twofaRateLimiter) RecordFailure(userID string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	a, ok := r.attempts[userID]
	if !ok || now.Sub(a.firstSeen) > r.window {
		r.attempts[userID] = &twofaAttempt{count: 1, firstSeen: now}
		return 1 >= r.maxFails
	}

	a.count++
	return a.count >= r.maxFails
}

// IsBlocked returns true if the user has exceeded the failure threshold within the window.
func (r *twofaRateLimiter) IsBlocked(userID string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	a, ok := r.attempts[userID]
	if !ok {
		return false
	}
	if time.Since(a.firstSeen) > r.window {
		delete(r.attempts, userID)
		return false
	}
	return a.count >= r.maxFails
}

// Clear removes the rate limit entry for a user (called on successful 2FA).
func (r *twofaRateLimiter) Clear(userID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.attempts, userID)
}

// Cleanup removes stale entries older than the window.
func (r *twofaRateLimiter) Cleanup() {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	for id, a := range r.attempts {
		if now.Sub(a.firstSeen) > r.window {
			delete(r.attempts, id)
		}
	}
}

// log2FASecurityEvent logs a 2FA security event to stdout and the audit log.
func (a *Admin) log2FASecurityEvent(c *fiber.Ctx, userID, reason string, statusCode int) {
	log.Printf("[SECURITY] 2fa user=%s reason=%s status=%d ip=%s path=%s",
		userID, reason, statusCode, c.IP(), c.Path())

	if a.SecurityLogger != nil {
		a.SecurityLogger(c, statusCode)
	}
}
