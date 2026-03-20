package middleware

import (
	"sync"
	"time"

	"github.com/hkdb/lmgate/internal/auth"
	"github.com/hkdb/lmgate/internal/config"
	"github.com/gofiber/fiber/v2"
	"golang.org/x/time/rate"
)

type limiterEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

var (
	limiters sync.Map
)

func RateLimit(cfg *config.Config, onRateLimit ...func(*fiber.Ctx, int)) fiber.Handler {
	var rateLimitCb func(*fiber.Ctx, int)
	if len(onRateLimit) > 0 {
		rateLimitCb = onRateLimit[0]
	}
	// Sweep unused entries every 10 minutes
	go func() {
		ticker := time.NewTicker(10 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			now := time.Now()
			limiters.Range(func(key, value any) bool {
				entry := value.(*limiterEntry)
				if now.Sub(entry.lastSeen) > 30*time.Minute {
					limiters.Delete(key)
				}
				return true
			})
		}
	}()

	return func(c *fiber.Ctx) error {
		if !cfg.RateLimit.Enabled {
			return c.Next()
		}

		u := auth.GetUser(c)

		rpm := cfg.RateLimit.DefaultRPM
		if tokenRPM, ok := c.Locals("token_rate_limit").(int); ok && tokenRPM > 0 {
			rpm = tokenRPM
		}

		// Use IP-based rate limiting for unauthenticated requests
		key := "ip:" + c.IP()
		if u != nil {
			key = u.UserID
			if u.TokenID != "" {
				key = u.TokenID
			}
		}

		entry, _ := limiters.LoadOrStore(key, &limiterEntry{
			limiter:  rate.NewLimiter(rate.Every(time.Minute/time.Duration(rpm)), rpm),
			lastSeen: time.Now(),
		})

		le := entry.(*limiterEntry)
		le.lastSeen = time.Now()

		if !le.limiter.Allow() {
			if rateLimitCb != nil {
				rateLimitCb(c, fiber.StatusTooManyRequests)
			}
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"error": "rate limit exceeded",
			})
		}

		return c.Next()
	}
}
