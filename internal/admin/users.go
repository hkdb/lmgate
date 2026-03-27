package admin

import (
	"fmt"
	"log"
	"strings"
	"time"
	"unicode"

	"github.com/gofiber/fiber/v2"
	"golang.org/x/crypto/bcrypt"

	"github.com/hkdb/lmgate/internal/auth"
	"github.com/hkdb/lmgate/internal/config"
	"github.com/hkdb/lmgate/internal/models"
)

// sanitizeLog strips control characters (newlines, tabs, etc.) from user input
// to prevent log injection attacks.
func sanitizeLog(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return '_'
		}
		return r
	}, s)
}

// dummyHash is a pre-computed bcrypt hash used to prevent timing-based user enumeration.
// When a login attempt targets a non-existent user, we still run bcrypt.CompareHashAndPassword
// against this hash so the response time is indistinguishable from a wrong-password attempt.
var dummyHash, _ = bcrypt.GenerateFromPassword([]byte("dummy-password-for-timing"), bcrypt.DefaultCost)

func (a *Admin) Login(c *fiber.Ctx) error {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request"})
	}

	ip := c.IP()

	// Check account lockout
	locked, err := models.IsAccountLocked(a.DB, req.Email)
	if err == nil && locked {
		log.Printf("login attempt for locked account: email=%s ip=%s", sanitizeLog(req.Email), sanitizeLog(ip))
		return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{"error": "account temporarily locked, try again later"})
	}

	user, err := models.GetUserByEmail(a.DB, req.Email)
	if err != nil {
		// Run bcrypt compare against dummy hash to prevent timing-based user enumeration
		bcrypt.CompareHashAndPassword(dummyHash, []byte(req.Password))
		log.Printf("failed login attempt: email=%s ip=%s reason=user_not_found", sanitizeLog(req.Email), sanitizeLog(ip))
		models.RecordFailedLogin(a.DB, req.Email, a.Config.Security.MaxFailedLogins, 15*time.Minute)
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid credentials"})
	}

	if !models.CheckPassword(user, req.Password) {
		log.Printf("failed login attempt: email=%s ip=%s reason=invalid_password", sanitizeLog(req.Email), sanitizeLog(ip))
		models.RecordFailedLogin(a.DB, req.Email, a.Config.Security.MaxFailedLogins, 15*time.Minute)
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid credentials"})
	}

	if user.Disabled {
		log.Printf("failed login attempt: email=%s ip=%s reason=account_disabled", sanitizeLog(req.Email), sanitizeLog(ip))
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "account disabled"})
	}

	// Clear failed attempts on successful login
	models.ClearFailedLogins(a.DB, req.Email)

	// Check if 2FA is required
	hasWebAuthn, _ := models.HasWebAuthnCredentials(a.DB, user.ID)
	if user.TOTPEnabled || hasWebAuthn {
		var methods []string
		if user.TOTPEnabled {
			methods = append(methods, "totp")
		}
		if hasWebAuthn {
			methods = append(methods, "webauthn")
		}

		twofaToken, err := auth.SignTwoFAToken(a.Config.Auth.JWTSecret, user.ID, methods)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to create 2FA token"})
		}

		return c.JSON(fiber.Map{
			"requires_2fa": true,
			"twofa_token":  twofaToken,
			"methods":      methods,
		})
	}

	token, err := auth.SignJWT(a.Config.Auth.JWTSecret, user.ID, user.Email, user.Role, a.Config.Auth.JWTExpiry)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to create token"})
	}

	auth.SetAuthCookie(c, token, int(a.Config.Auth.JWTExpiry.Seconds()), a.Config.Server.TLS.Disabled)

	return c.JSON(fiber.Map{
		"user": fiber.Map{
			"id":                    user.ID,
			"email":                 user.Email,
			"display_name":          user.DisplayName,
			"role":                  user.Role,
			"force_password_change": user.ForcePasswordChange,
			"totp_enabled":          user.TOTPEnabled,
			"enforce_2fa":           a.Config.Security.Enforce2FA && user.AuthProvider == "local" && !user.TOTPEnabled && !hasWebAuthn,
			"password_expired":      models.IsPasswordExpired(user, a.Config.Security.PasswordExpiryDays),
		},
	})
}

func validatePasswordStrength(password string, cfg *config.Config) error {
	if len(password) < cfg.Security.PasswordMinLength {
		return fmt.Errorf("password must be at least %d characters", cfg.Security.PasswordMinLength)
	}
	if cfg.Security.PasswordRequireSpecial {
		hasSpecial := false
		for _, r := range password {
			if strings.ContainsRune("!@#$%^&*()_+-=[]{}|;':\",./<>?`~", r) {
				hasSpecial = true
				break
			}
		}
		if !hasSpecial {
			return fmt.Errorf("password must contain at least one special character")
		}
	}
	if cfg.Security.PasswordRequireNumber {
		hasNumber := false
		for _, r := range password {
			if unicode.IsDigit(r) {
				hasNumber = true
				break
			}
		}
		if !hasNumber {
			return fmt.Errorf("password must contain at least one number")
		}
	}
	return nil
}

func (a *Admin) ListUsers(c *fiber.Ctx) error {
	users, err := models.ListUsers(a.DB)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to list users"})
	}
	return c.JSON(users)
}

func (a *Admin) CreateUser(c *fiber.Ctx) error {
	var req struct {
		Email               string `json:"email"`
		DisplayName         string `json:"display_name"`
		Password            string `json:"password"`
		Role                string `json:"role"`
		ForcePasswordChange *bool  `json:"force_password_change"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request"})
	}

	if req.Email == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "email is required"})
	}

	if req.Password == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "password is required"})
	}

	if err := validatePasswordStrength(req.Password, a.Config); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	if req.Role == "" {
		req.Role = "user"
	}

	forcePC := true
	if req.ForcePasswordChange != nil {
		forcePC = *req.ForcePasswordChange
	}

	user, err := models.CreateUser(a.DB, req.Email, req.DisplayName, req.Password, req.Role, "local", "", forcePC)
	if err != nil {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "user already exists or creation failed"})
	}

	return c.Status(fiber.StatusCreated).JSON(user)
}

func (a *Admin) GetUser(c *fiber.Ctx) error {
	user, err := models.GetUserByID(a.DB, c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "user not found"})
	}
	return c.JSON(user)
}

func (a *Admin) UpdateUser(c *fiber.Ctx) error {
	var req struct {
		Email               string `json:"email"`
		DisplayName         string `json:"display_name"`
		Role                string `json:"role"`
		Disabled            bool   `json:"disabled"`
		Password            string `json:"password,omitempty"`
		ForcePasswordChange bool   `json:"force_password_change"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request"})
	}

	id := c.Params("id")

	if req.Password != "" {
		if err := validatePasswordStrength(req.Password, a.Config); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}
		if err := models.UpdateUserPassword(a.DB, id, req.Password); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to update password"})
		}
	}

	if err := models.UpdateUser(a.DB, id, req.Email, req.DisplayName, req.Role, req.Disabled, req.ForcePasswordChange); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to update user"})
	}

	auth.InvalidateUserCache(id)

	user, err := models.GetUserByID(a.DB, id)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "user not found"})
	}
	return c.JSON(user)
}

func (a *Admin) Me(c *fiber.Ctx) error {
	u := auth.GetUser(c)
	if u == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "not authenticated"})
	}

	user, err := models.GetUserByID(a.DB, u.UserID)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "user not found"})
	}

	hasWebAuthn, _ := models.HasWebAuthnCredentials(a.DB, user.ID)

	return c.JSON(fiber.Map{
		"id":                    user.ID,
		"email":                 user.Email,
		"display_name":          user.DisplayName,
		"role":                  user.Role,
		"force_password_change": user.ForcePasswordChange,
		"totp_enabled":          user.TOTPEnabled,
		"webauthn_enabled":      hasWebAuthn,
		"enforce_2fa":           a.Config.Security.Enforce2FA && user.AuthProvider == "local" && !user.TOTPEnabled && !hasWebAuthn,
		"password_expired":      models.IsPasswordExpired(user, a.Config.Security.PasswordExpiryDays),
	})
}

func (a *Admin) Logout(c *fiber.Ctx) error {
	auth.ClearAuthCookie(c, a.Config.Server.TLS.Disabled)
	return c.JSON(fiber.Map{"status": "ok"})
}

func (a *Admin) DeleteUser(c *fiber.Ctx) error {
	id := c.Params("id")
	auth.InvalidateUserCache(id)

	if err := models.DeleteUser(a.DB, id); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to delete user"})
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (a *Admin) ChangePassword(c *fiber.Ctx) error {
	u := auth.GetUser(c)
	if u == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "not authenticated"})
	}

	var req struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request"})
	}

	if req.CurrentPassword == "" || req.NewPassword == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "current_password and new_password are required"})
	}

	user, err := models.GetUserByID(a.DB, u.UserID)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "user not found"})
	}

	if !models.CheckPassword(user, req.CurrentPassword) {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "current password is incorrect"})
	}

	if err := validatePasswordStrength(req.NewPassword, a.Config); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	if err := models.UpdateUserPassword(a.DB, u.UserID, req.NewPassword); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to update password"})
	}

	if err := models.ClearForcePasswordChange(a.DB, u.UserID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to clear force password change flag"})
	}

	auth.InvalidateUserCache(u.UserID)

	hasWebAuthn, _ := models.HasWebAuthnCredentials(a.DB, user.ID)

	return c.JSON(fiber.Map{
		"id":                    user.ID,
		"email":                 user.Email,
		"display_name":          user.DisplayName,
		"role":                  user.Role,
		"force_password_change": false,
		"password_expired":      false,
		"totp_enabled":          user.TOTPEnabled,
		"webauthn_enabled":      hasWebAuthn,
		"enforce_2fa":           a.Config.Security.Enforce2FA && user.AuthProvider == "local" && !user.TOTPEnabled && !hasWebAuthn,
	})
}
