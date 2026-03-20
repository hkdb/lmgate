package middleware

import (
	"log"
	"time"

	"github.com/hkdb/lmgate/internal/auth"
	"github.com/hkdb/lmgate/internal/config"
	"github.com/hkdb/lmgate/internal/models"
	"github.com/gofiber/fiber/v2"
)

// NewSecurityLogger returns a callback for auth failures.
// It logs to stdout in a fail2ban-compatible format and sends an entry to the audit channel.
func NewSecurityLogger(cfg *config.Config) func(*fiber.Ctx, int) {
	return func(c *fiber.Ctx, statusCode int) {
		ip := c.IP()
		log.Printf("[SECURITY] %s %d %s %s ip=%s",
			time.Now().UTC().Format(time.RFC3339), statusCode, c.Method(), c.Path(), ip)

		if !cfg.Logging.SecurityLogEnabled {
			return
		}

		entry := models.AuditLog{
			Method:     c.Method(),
			Path:       c.Path(),
			StatusCode: statusCode,
			IPAddr:     &ip,
			LogType:    "security",
		}
		if v := c.Get("X-Real-IP"); v != "" {
			entry.XRealIP = &v
		}
		if v := c.Get("X-Forwarded-For"); v != "" {
			entry.XForwardedFor = &v
		}
		if auditChan != nil {
			select {
			case auditChan <- entry:
			default:
			}
		}
	}
}

// NewRateLimitLogger returns a callback for rate limit (429) events.
// It logs to stdout in a fail2ban-compatible format and sends an entry to the audit channel.
func NewRateLimitLogger(cfg *config.Config) func(*fiber.Ctx, int) {
	return func(c *fiber.Ctx, statusCode int) {
		ip := c.IP()
		u := auth.GetUser(c)
		email := ""
		if u != nil {
			email = " user=" + u.Email
		}
		log.Printf("[SECURITY] %s %d %s %s ip=%s%s",
			time.Now().UTC().Format(time.RFC3339), statusCode, c.Method(), c.Path(), ip, email)

		if !cfg.Logging.AdminLogEnabled {
			return
		}

		entry := models.AuditLog{
			Method:     c.Method(),
			Path:       c.Path(),
			StatusCode: statusCode,
			IPAddr:     &ip,
			LogType:    "admin",
		}
		if v := c.Get("X-Real-IP"); v != "" {
			entry.XRealIP = &v
		}
		if v := c.Get("X-Forwarded-For"); v != "" {
			entry.XForwardedFor = &v
		}
		if u != nil {
			entry.UserID = &u.UserID
			if u.TokenID != "" {
				entry.TokenID = &u.TokenID
			}
		}
		if auditChan != nil {
			select {
			case auditChan <- entry:
			default:
			}
		}
	}
}
