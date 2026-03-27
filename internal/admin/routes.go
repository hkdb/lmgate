package admin

import (
	"database/sql"
	"sync"
	"time"

	"github.com/hkdb/lmgate/internal/auth"
	"github.com/hkdb/lmgate/internal/config"
	"github.com/hkdb/lmgate/internal/metrics"
	"github.com/gofiber/fiber/v2"
)

type Admin struct {
	DB              *sql.DB
	Config          *config.Config
	Collector       *metrics.Collector
	Notifier        *LogNotifier
	MetricsNotifier *LogNotifier
	Providers       sync.Map // map[string]*auth.OIDCProvider
	twoFALimiter    *twofaRateLimiter
	SecurityLogger  func(*fiber.Ctx, int)
	Version         string
}

func New(db *sql.DB, cfg *config.Config, collector *metrics.Collector) *Admin {
	a := &Admin{
		DB:              db,
		Config:          cfg,
		Collector:       collector,
		Notifier:        NewLogNotifier(),
		MetricsNotifier: NewLogNotifier(),
		twoFALimiter:    newTwoFARateLimiter(cfg.Security.MaxFailedLogins, 5*time.Minute),
	}

	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			a.twoFALimiter.Cleanup()
		}
	}()

	return a
}

func (a *Admin) RegisterRoutes(app *fiber.App, jwtSecret string) {
	api := app.Group("/admin/api")

	// Public auth routes
	api.Post("/login", a.Login)
	api.Post("/logout", a.Logout)
	api.Get("/auth/providers", a.ListAuthProviders)
	api.Get("/oauth/callback", a.OAuthCallback)
	api.Get("/oauth/:provider", a.OAuthRedirect)

	// CSRF token seeding endpoint (cookie set by middleware on GET)
	api.Get("/csrf-token", func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusNoContent)
	})

	// Public 2FA login routes
	api.Post("/2fa/totp/login", a.TOTPLogin)
	api.Post("/2fa/recovery/login", a.RecoveryLogin)
	api.Post("/2fa/webauthn/login/begin", a.WebAuthnLoginBegin)
	api.Post("/2fa/webauthn/login/finish", a.WebAuthnLoginFinish)

	// Authenticated (non-admin) routes
	authenticated := api.Group("", auth.Middleware(a.DB, jwtSecret))
	authenticated.Get("/me", a.Me)
	authenticated.Post("/change-password", a.ChangePassword)

	// 2FA management (authenticated)
	authenticated.Post("/2fa/totp/setup", a.SetupTOTP)
	authenticated.Post("/2fa/totp/verify", a.VerifyTOTP)
	authenticated.Post("/2fa/totp/disable", a.DisableTOTP)
	authenticated.Get("/2fa/status", a.TwoFAStatus)
	authenticated.Post("/2fa/webauthn/register/begin", a.WebAuthnRegisterBegin)
	authenticated.Post("/2fa/webauthn/register/finish", a.WebAuthnRegisterFinish)
	authenticated.Delete("/2fa/webauthn/credentials/:id", a.WebAuthnDeleteCredential)
	authenticated.Post("/2fa/recovery/regenerate", a.RegenerateRecoveryCodes)

	// Version
	authenticated.Get("/version", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"version": a.Version})
	})

	// User token management (own tokens only)
	authenticated.Get("/my/tokens", a.ListMyTokens)
	authenticated.Post("/my/tokens", a.CreateMyToken)
	authenticated.Post("/my/tokens/:id/revoke", a.RevokeMyToken)
	authenticated.Delete("/my/tokens/:id", a.DeleteMyToken)

	// Model list (read-only, all authenticated users)
	authenticated.Get("/models", a.ListModels)

	// Protected admin routes
	protected := api.Group("", auth.Middleware(a.DB, jwtSecret), auth.RequireAdmin)

	// Users (admin-only)
	protected.Get("/users", a.ListUsers)
	protected.Post("/users", a.CreateUser)
	protected.Get("/users/:id", a.GetUser)
	protected.Put("/users/:id", a.UpdateUser)
	protected.Delete("/users/:id", a.DeleteUser)
	protected.Post("/users/:id/reset-2fa", a.ResetUser2FA)

	// Groups
	protected.Get("/groups", a.ListGroups)
	protected.Post("/groups", a.CreateGroup)
	protected.Get("/groups/:id", a.GetGroup)
	protected.Put("/groups/:id", a.UpdateGroup)
	protected.Delete("/groups/:id", a.DeleteGroup)
	protected.Post("/groups/:id/members", a.AddGroupMember)
	protected.Delete("/groups/:id/members/:userId", a.RemoveGroupMember)

	// Tokens
	protected.Get("/tokens", a.ListTokens)
	protected.Post("/tokens", a.CreateToken)
	protected.Delete("/tokens/:id", a.DeleteToken)
	protected.Post("/tokens/:id/revoke", a.RevokeToken)

	// ACLs
	protected.Get("/acls", a.ListACLs)
	protected.Post("/acls", a.CreateACL)
	protected.Delete("/acls/:id", a.DeleteACL)

	// Models
	protected.Get("/models", a.ListModels)
	protected.Get("/models/acl", a.ListModelACLs)
	protected.Post("/models/acl", a.CreateModelACL)
	protected.Delete("/models/acl/:id", a.DeleteModelACL)
	protected.Get("/models/upstream-type", a.GetUpstreamType)
	protected.Post("/models/pull", a.PullModel)
	protected.Post("/models/delete", a.DeleteModel)

	// Settings General
	protected.Get("/settings/general", a.GetSettingsGeneral)
	protected.Put("/settings/general", a.UpdateSettingsGeneral)

	// Settings OIDC (frontend Settings page)
	protected.Get("/settings/oidc", a.GetSettingsOIDC)
	protected.Post("/settings/oidc", a.CreateSettingsOIDC)
	protected.Put("/settings/oidc/:id", a.UpdateSettingsOIDC)
	protected.Delete("/settings/oidc/:id", a.DeleteSettingsOIDC)

	// OIDC Providers
	protected.Get("/oidc", a.ListOIDCProviders)
	protected.Post("/oidc", a.CreateOIDCProvider)
	protected.Put("/oidc/:id", a.UpdateOIDCProvider)
	protected.Delete("/oidc/:id", a.DeleteOIDCProvider)

	// Logs
	protected.Get("/logs/stream", a.StreamLogs)
	protected.Get("/logs/export", a.ExportLogs)
	protected.Get("/logs", a.GetLogs)
	protected.Delete("/logs", a.DeleteAllLogs)

	// Dashboard
	protected.Get("/dashboard", a.GetDashboard)

	// Metrics
	protected.Get("/metrics/stream", a.StreamMetrics)
	protected.Get("/metrics", a.GetUsageMetrics)
	protected.Get("/metrics/summary", a.GetMetricsSummary)
	protected.Get("/metrics/users/:id", a.GetUserMetrics)
}
