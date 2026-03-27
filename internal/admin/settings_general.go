package admin

import (
	"net"
	"strings"
	"time"

	"github.com/hkdb/lmgate/internal/auth"
	"github.com/hkdb/lmgate/internal/middleware"
	"github.com/hkdb/lmgate/internal/models"
	"github.com/gofiber/fiber/v2"
)

func (a *Admin) GetSettingsGeneral(c *fiber.Ctx) error {
	defaults := models.GeneralSettings{
		RateLimitEnabled:         a.Config.RateLimit.Enabled,
		RateLimitDefaultRPM:      a.Config.RateLimit.DefaultRPM,
		APILogEnabled:            a.Config.Logging.APILogEnabled,
		APILogRetentionDays:      a.Config.Logging.APILogRetentionDays,
		AdminLogEnabled:          a.Config.Logging.AdminLogEnabled,
		AdminLogRetentionDays:    a.Config.Logging.AdminLogRetentionDays,
		SecurityLogEnabled:       a.Config.Logging.SecurityLogEnabled,
		SecurityLogRetentionDays: a.Config.Logging.SecurityLogRetentionDays,
		AuditFlushInterval:       a.Config.Logging.AuditFlushInterval,
		MaxFailedLogins:          a.Config.Security.MaxFailedLogins,
		PasswordMinLength:        a.Config.Security.PasswordMinLength,
		PasswordRequireSpecial:   a.Config.Security.PasswordRequireSpecial,
		PasswordRequireNumber:    a.Config.Security.PasswordRequireNumber,
		UserCacheTTL:             a.Config.Security.UserCacheTTL,
		Enforce2FA:               a.Config.Security.Enforce2FA,
		PasswordExpiryDays:       a.Config.Security.PasswordExpiryDays,
		AdminAllowedNetworks:     a.Config.Security.AdminAllowedNetworks,
		GatewayAllowedNetworks:   a.Config.Security.GatewayAllowedNetworks,
	}

	settings, err := models.GetGeneralSettings(a.DB, defaults)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to load settings",
		})
	}

	return c.JSON(settings)
}

func (a *Admin) UpdateSettingsGeneral(c *fiber.Ctx) error {
	var req models.GeneralSettings
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}

	if req.RateLimitDefaultRPM < 1 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "rate_limit_default_rpm must be at least 1",
		})
	}

	if req.APILogRetentionDays < 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "api_log_retention_days cannot be negative",
		})
	}

	if req.AdminLogRetentionDays < 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "admin_log_retention_days cannot be negative",
		})
	}

	if req.SecurityLogRetentionDays < 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "security_log_retention_days cannot be negative",
		})
	}

	if req.AuditFlushInterval < 1 || req.AuditFlushInterval > 300 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "audit_flush_interval must be between 1 and 300 seconds",
		})
	}

	if req.MaxFailedLogins < 1 || req.MaxFailedLogins > 100 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "max_failed_logins must be between 1 and 100",
		})
	}

	if req.PasswordMinLength < 8 || req.PasswordMinLength > 128 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "password_min_length must be between 8 and 128",
		})
	}

	if req.UserCacheTTL < 5 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "user_cache_ttl must be at least 5",
		})
	}

	if req.PasswordExpiryDays < 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "password_expiry_days cannot be negative",
		})
	}

	if req.AdminAllowedNetworks != "" {
		for _, entry := range strings.Split(req.AdminAllowedNetworks, ",") {
			entry = strings.TrimSpace(entry)
			if entry == "" {
				continue
			}
			if strings.Contains(entry, "/") {
				if _, _, err := net.ParseCIDR(entry); err != nil {
					return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
						"error": "invalid CIDR in admin_allowed_networks: " + entry,
					})
				}
				continue
			}
			if net.ParseIP(entry) == nil {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
					"error": "invalid IP in admin_allowed_networks: " + entry,
				})
			}
		}
	}

	if req.GatewayAllowedNetworks != "" {
		for _, entry := range strings.Split(req.GatewayAllowedNetworks, ",") {
			entry = strings.TrimSpace(entry)
			if entry == "" {
				continue
			}
			if strings.Contains(entry, "/") {
				if _, _, err := net.ParseCIDR(entry); err != nil {
					return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
						"error": "invalid CIDR in gateway_allowed_networks: " + entry,
					})
				}
				continue
			}
			if net.ParseIP(entry) == nil {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
					"error": "invalid IP in gateway_allowed_networks: " + entry,
				})
			}
		}
	}

	if req.Enforce2FA {
		u := auth.GetUser(c)
		admin, err := models.GetUserByID(a.DB, u.UserID)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to verify admin"})
		}
		hasWebAuthn, _ := models.HasWebAuthnCredentials(a.DB, u.UserID)
		if !admin.TOTPEnabled && !hasWebAuthn {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "you must set up 2FA on your own account before enforcing it",
			})
		}
	}

	if err := models.SaveGeneralSettings(a.DB, req); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to save settings",
		})
	}

	// Apply to in-memory config so changes take effect immediately
	a.Config.RateLimit.Enabled = req.RateLimitEnabled
	a.Config.RateLimit.DefaultRPM = req.RateLimitDefaultRPM
	a.Config.Logging.APILogEnabled = req.APILogEnabled
	a.Config.Logging.APILogRetentionDays = req.APILogRetentionDays
	a.Config.Logging.AdminLogEnabled = req.AdminLogEnabled
	a.Config.Logging.AdminLogRetentionDays = req.AdminLogRetentionDays
	a.Config.Logging.SecurityLogEnabled = req.SecurityLogEnabled
	a.Config.Logging.SecurityLogRetentionDays = req.SecurityLogRetentionDays
	a.Config.Logging.AuditFlushInterval = req.AuditFlushInterval
	middleware.SetAuditFlushInterval(time.Duration(req.AuditFlushInterval) * time.Second)
	a.Config.Security.MaxFailedLogins = req.MaxFailedLogins
	a.Config.Security.PasswordMinLength = req.PasswordMinLength
	a.Config.Security.PasswordRequireSpecial = req.PasswordRequireSpecial
	a.Config.Security.PasswordRequireNumber = req.PasswordRequireNumber
	a.Config.Security.UserCacheTTL = req.UserCacheTTL
	a.Config.Security.Enforce2FA = req.Enforce2FA
	a.Config.Security.PasswordExpiryDays = req.PasswordExpiryDays
	a.Config.Security.AdminAllowedNetworks = req.AdminAllowedNetworks
	a.Config.Security.GatewayAllowedNetworks = req.GatewayAllowedNetworks
	middleware.ParseGatewayACL(req.GatewayAllowedNetworks)
	auth.SetUserCacheTTL(time.Duration(req.UserCacheTTL) * time.Second)

	return c.JSON(req)
}
