package middleware

import (
	"time"

	"github.com/hkdb/lmgate/internal/auth"
	"github.com/hkdb/lmgate/internal/config"
	"github.com/hkdb/lmgate/internal/models"
	"github.com/gofiber/fiber/v2"
)

// AdminLog returns middleware that logs admin API requests.
// It should be path-scoped to /admin/api so it never touches the proxy chain.
func AdminLog(cfg *config.Config) fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()

		err := c.Next()

		if !cfg.Logging.AdminLogEnabled {
			return err
		}

		latency := time.Since(start).Milliseconds()
		ip := c.IP()

		entry := models.AuditLog{
			Method:     c.Method(),
			Path:       c.Path(),
			StatusCode: c.Response().StatusCode(),
			LatencyMs:  latency,
			IPAddr:     &ip,
			LogType:    "admin",
		}
		if v := c.Get("X-Real-IP"); v != "" {
			entry.XRealIP = &v
		}
		if v := c.Get("X-Forwarded-For"); v != "" {
			entry.XForwardedFor = &v
		}

		if u := auth.GetUser(c); u != nil {
			entry.UserID = &u.UserID
			if u.TokenID != "" {
				entry.TokenID = &u.TokenID
			}
		}

		go func() {
			if auditChan != nil {
				select {
				case auditChan <- entry:
				default:
				}
			}
		}()

		return err
	}
}
