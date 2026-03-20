package admin

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image/png"
	"log"

	"github.com/hkdb/lmgate/internal/auth"
	"github.com/hkdb/lmgate/internal/models"
	"github.com/go-webauthn/webauthn/protocol"
	"github.com/gofiber/fiber/v2"
	"github.com/pquerna/otp/totp"
)

// --- TOTP Management (authenticated) ---

func (a *Admin) SetupTOTP(c *fiber.Ctx) error {
	u := auth.GetUser(c)
	if u == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "not authenticated"})
	}

	user, err := models.GetUserByID(a.DB, u.UserID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "user not found"})
	}

	if user.TOTPEnabled {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "TOTP already enabled"})
	}

	issuer := "LM Gate"
	if hosts := a.Config.Server.AllowedHosts; len(hosts) > 0 {
		issuer = fmt.Sprintf("LM Gate (%s)", hosts[0])
	}
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      issuer,
		AccountName: user.Email,
	})
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to generate TOTP secret"})
	}

	encryptedSecret, salt, err := a.encryptSecret(key.Secret())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to encrypt secret"})
	}

	if err := models.SaveTOTPSecret(a.DB, u.UserID, encryptedSecret, salt); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to save TOTP secret"})
	}

	img, err := key.Image(200, 200)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to generate QR code"})
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to encode QR code"})
	}

	qrDataURI := "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())

	return c.JSON(fiber.Map{
		"qr_code":    qrDataURI,
		"manual_key": key.Secret(),
	})
}

func (a *Admin) VerifyTOTP(c *fiber.Ctx) error {
	u := auth.GetUser(c)
	if u == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "not authenticated"})
	}

	var req struct {
		Code string `json:"code"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request"})
	}

	if req.Code == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "code is required"})
	}

	secret, err := models.GetTOTPSecret(a.DB, u.UserID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "TOTP not set up"})
	}

	if secret.Verified {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "TOTP already verified"})
	}

	plainSecret, err := a.decryptSecret(secret.SecretEncrypted, secret.SecretSalt)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to decrypt secret"})
	}

	if !totp.Validate(req.Code, plainSecret) {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid code"})
	}

	ok, err := models.CheckAndUpdateTOTPUsed(a.DB, u.UserID)
	if err != nil || !ok {
		return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{"error": "code already used, wait for next code"})
	}

	if err := models.MarkTOTPVerified(a.DB, u.UserID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to verify TOTP"})
	}

	if err := models.EnableTOTP(a.DB, u.UserID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to enable TOTP"})
	}

	auth.InvalidateUserCache(u.UserID)

	// Generate recovery codes if this is the first 2FA method
	codes, err := models.GenerateRecoveryCodes(a.DB, u.UserID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to generate recovery codes"})
	}

	return c.JSON(fiber.Map{
		"status":         "enabled",
		"recovery_codes": codes,
	})
}

func (a *Admin) DisableTOTP(c *fiber.Ctx) error {
	u := auth.GetUser(c)
	if u == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "not authenticated"})
	}

	var req struct {
		Code string `json:"code"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request"})
	}

	if req.Code == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "code is required"})
	}

	secret, err := models.GetTOTPSecret(a.DB, u.UserID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "TOTP not enabled"})
	}

	plainSecret, err := a.decryptSecret(secret.SecretEncrypted, secret.SecretSalt)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to decrypt secret"})
	}

	if !totp.Validate(req.Code, plainSecret) {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid code"})
	}

	ok, err := models.CheckAndUpdateTOTPUsed(a.DB, u.UserID)
	if err != nil || !ok {
		return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{"error": "code already used, wait for next code"})
	}

	if err := models.DeleteTOTPSecret(a.DB, u.UserID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to delete TOTP"})
	}

	if err := models.DisableTOTP(a.DB, u.UserID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to disable TOTP"})
	}

	auth.InvalidateUserCache(u.UserID)

	// If no other 2FA methods, remove recovery codes
	hasWebAuthn, _ := models.HasWebAuthnCredentials(a.DB, u.UserID)
	if !hasWebAuthn {
		_ = models.DeleteRecoveryCodes(a.DB, u.UserID)
	}

	return c.JSON(fiber.Map{"status": "disabled"})
}

func (a *Admin) TwoFAStatus(c *fiber.Ctx) error {
	u := auth.GetUser(c)
	if u == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "not authenticated"})
	}

	user, err := models.GetUserByID(a.DB, u.UserID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "user not found"})
	}

	webauthnCreds, _ := models.ListWebAuthnCredentials(a.DB, u.UserID)
	recoveryCount, _ := models.CountUnusedRecoveryCodes(a.DB, u.UserID)

	type credInfo struct {
		ID        string `json:"id"`
		Name      string `json:"name"`
		CreatedAt string `json:"created_at"`
	}

	creds := make([]credInfo, len(webauthnCreds))
	for i, c := range webauthnCreds {
		creds[i] = credInfo{ID: c.ID, Name: c.Name, CreatedAt: c.CreatedAt}
	}

	return c.JSON(fiber.Map{
		"totp_enabled":           user.TOTPEnabled,
		"webauthn_credentials":   creds,
		"recovery_codes_remaining": recoveryCount,
	})
}

// --- WebAuthn Management (authenticated) ---

func (a *Admin) WebAuthnRegisterBegin(c *fiber.Ctx) error {
	u := auth.GetUser(c)
	if u == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "not authenticated"})
	}

	wa := auth.GetWebAuthn()
	if wa == nil {
		return c.Status(fiber.StatusNotImplemented).JSON(fiber.Map{"error": "WebAuthn not configured"})
	}

	user, err := models.GetUserByID(a.DB, u.UserID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "user not found"})
	}

	waUser, err := auth.NewWebAuthnUser(user, a.DB)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to create webauthn user"})
	}

	options, session, err := wa.BeginRegistration(waUser)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to begin registration"})
	}

	auth.StoreWebAuthnSession("reg:"+u.UserID, session)

	return c.JSON(options)
}

func (a *Admin) WebAuthnRegisterFinish(c *fiber.Ctx) error {
	u := auth.GetUser(c)
	if u == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "not authenticated"})
	}

	wa := auth.GetWebAuthn()
	if wa == nil {
		return c.Status(fiber.StatusNotImplemented).JSON(fiber.Map{"error": "WebAuthn not configured"})
	}

	session, ok := auth.LoadWebAuthnSession("reg:" + u.UserID)
	if !ok {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "no registration session found"})
	}

	user, err := models.GetUserByID(a.DB, u.UserID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "user not found"})
	}

	waUser, err := auth.NewWebAuthnUser(user, a.DB)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to create webauthn user"})
	}

	parsedResponse, err := protocol.ParseCredentialCreationResponseBody(bytes.NewReader(c.Body()))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": fmt.Sprintf("invalid credential response: %v", err)})
	}

	credential, err := wa.CreateCredential(waUser, *session, parsedResponse)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": fmt.Sprintf("failed to create credential: %v", err)})
	}

	// Get name from query or default
	name := c.Query("name", "Security Key")

	cred, err := models.CreateWebAuthnCredential(
		a.DB, u.UserID, name,
		credential.ID, credential.PublicKey,
		credential.AttestationType,
		credential.Authenticator.AAGUID,
		credential.Authenticator.SignCount,
	)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to save credential"})
	}

	// Generate recovery codes if first 2FA method
	result := fiber.Map{
		"credential": fiber.Map{
			"id":         cred.ID,
			"name":       cred.Name,
			"created_at": cred.CreatedAt,
		},
	}

	existingCount, _ := models.CountUnusedRecoveryCodes(a.DB, u.UserID)
	if existingCount == 0 {
		codes, err := models.GenerateRecoveryCodes(a.DB, u.UserID)
		if err == nil {
			result["recovery_codes"] = codes
		}
	}

	return c.Status(fiber.StatusCreated).JSON(result)
}

func (a *Admin) WebAuthnDeleteCredential(c *fiber.Ctx) error {
	u := auth.GetUser(c)
	if u == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "not authenticated"})
	}

	credID := c.Params("id")
	if err := models.DeleteWebAuthnCredential(a.DB, credID, u.UserID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to delete credential"})
	}

	// If no more 2FA methods, clean up recovery codes
	hasWebAuthn, _ := models.HasWebAuthnCredentials(a.DB, u.UserID)
	user, _ := models.GetUserByID(a.DB, u.UserID)
	if !hasWebAuthn && (user == nil || !user.TOTPEnabled) {
		_ = models.DeleteRecoveryCodes(a.DB, u.UserID)
	}

	return c.SendStatus(fiber.StatusNoContent)
}

// --- 2FA Login (public routes) ---

func (a *Admin) TOTPLogin(c *fiber.Ctx) error {
	var req struct {
		TwoFAToken string `json:"twofa_token"`
		Code       string `json:"code"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request"})
	}

	claims, err := auth.VerifyTwoFAToken(a.Config.Auth.JWTSecret, req.TwoFAToken)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid or expired 2FA token"})
	}

	if a.twoFALimiter.IsBlocked(claims.UserID) {
		a.log2FASecurityEvent(c, claims.UserID, "2fa_rate_limited", fiber.StatusTooManyRequests)
		return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
			"error": "too many failed 2FA attempts, please log in again",
		})
	}

	secret, err := models.GetTOTPSecret(a.DB, claims.UserID)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "TOTP not configured"})
	}

	plainSecret, err := a.decryptSecret(secret.SecretEncrypted, secret.SecretSalt)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "internal error"})
	}

	if !totp.Validate(req.Code, plainSecret) {
		a.twoFALimiter.RecordFailure(claims.UserID)
		a.log2FASecurityEvent(c, claims.UserID, "2fa_invalid_code", fiber.StatusUnauthorized)
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid code"})
	}

	ok, err := models.CheckAndUpdateTOTPUsed(a.DB, claims.UserID)
	if err != nil || !ok {
		return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{"error": "code already used, wait for next code"})
	}

	a.twoFALimiter.Clear(claims.UserID)
	return a.issueAuthAfter2FA(c, claims.UserID)
}

func (a *Admin) RecoveryLogin(c *fiber.Ctx) error {
	var req struct {
		TwoFAToken string `json:"twofa_token"`
		Code       string `json:"code"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request"})
	}

	claims, err := auth.VerifyTwoFAToken(a.Config.Auth.JWTSecret, req.TwoFAToken)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid or expired 2FA token"})
	}

	if a.twoFALimiter.IsBlocked(claims.UserID) {
		a.log2FASecurityEvent(c, claims.UserID, "2fa_rate_limited", fiber.StatusTooManyRequests)
		return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
			"error": "too many failed 2FA attempts, please log in again",
		})
	}

	valid, err := models.ValidateRecoveryCode(a.DB, claims.UserID, req.Code)
	if err != nil || !valid {
		a.twoFALimiter.RecordFailure(claims.UserID)
		a.log2FASecurityEvent(c, claims.UserID, "2fa_invalid_code", fiber.StatusUnauthorized)
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid recovery code"})
	}

	a.twoFALimiter.Clear(claims.UserID)
	return a.issueAuthAfter2FA(c, claims.UserID)
}

func (a *Admin) WebAuthnLoginBegin(c *fiber.Ctx) error {
	var req struct {
		TwoFAToken string `json:"twofa_token"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request"})
	}

	wa := auth.GetWebAuthn()
	if wa == nil {
		return c.Status(fiber.StatusNotImplemented).JSON(fiber.Map{"error": "WebAuthn not configured"})
	}

	claims, err := auth.VerifyTwoFAToken(a.Config.Auth.JWTSecret, req.TwoFAToken)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid or expired 2FA token"})
	}

	if a.twoFALimiter.IsBlocked(claims.UserID) {
		a.log2FASecurityEvent(c, claims.UserID, "2fa_rate_limited", fiber.StatusTooManyRequests)
		return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
			"error": "too many failed 2FA attempts, please log in again",
		})
	}

	user, err := models.GetUserByID(a.DB, claims.UserID)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "user not found"})
	}

	waUser, err := auth.NewWebAuthnUser(user, a.DB)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to create webauthn user"})
	}

	options, session, err := wa.BeginLogin(waUser)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to begin login"})
	}

	auth.StoreWebAuthnSession("login:"+claims.UserID, session)

	return c.JSON(fiber.Map{
		"options":     options,
		"twofa_token": req.TwoFAToken,
	})
}

func (a *Admin) WebAuthnLoginFinish(c *fiber.Ctx) error {
	wa := auth.GetWebAuthn()
	if wa == nil {
		return c.Status(fiber.StatusNotImplemented).JSON(fiber.Map{"error": "WebAuthn not configured"})
	}

	// Parse the twofa_token from custom header since body is the webauthn assertion
	twofaToken := c.Get("X-TwoFA-Token")
	if twofaToken == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "missing twofa_token"})
	}

	claims, err := auth.VerifyTwoFAToken(a.Config.Auth.JWTSecret, twofaToken)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid or expired 2FA token"})
	}

	if a.twoFALimiter.IsBlocked(claims.UserID) {
		a.log2FASecurityEvent(c, claims.UserID, "2fa_rate_limited", fiber.StatusTooManyRequests)
		return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
			"error": "too many failed 2FA attempts, please log in again",
		})
	}

	session, ok := auth.LoadWebAuthnSession("login:" + claims.UserID)
	if !ok {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "no login session found"})
	}

	user, err := models.GetUserByID(a.DB, claims.UserID)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "user not found"})
	}

	waUser, err := auth.NewWebAuthnUser(user, a.DB)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "internal error"})
	}

	parsedResponse, err := protocol.ParseCredentialRequestResponseBody(bytes.NewReader(c.Body()))
	if err != nil {
		a.twoFALimiter.RecordFailure(claims.UserID)
		a.log2FASecurityEvent(c, claims.UserID, "2fa_invalid_assertion", fiber.StatusBadRequest)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid assertion"})
	}

	credential, err := wa.ValidateLogin(waUser, *session, parsedResponse)
	if err != nil {
		a.twoFALimiter.RecordFailure(claims.UserID)
		a.log2FASecurityEvent(c, claims.UserID, "2fa_authentication_failed", fiber.StatusUnauthorized)
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "authentication failed"})
	}

	// Verify and update sign count (detects credential cloning)
	if err := models.VerifyAndUpdateWebAuthnSignCount(a.DB, credential.ID, credential.Authenticator.SignCount); err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "credential verification failed"})
	}

	a.twoFALimiter.Clear(claims.UserID)
	return a.issueAuthAfter2FA(c, claims.UserID)
}

// --- Recovery (authenticated) ---

func (a *Admin) RegenerateRecoveryCodes(c *fiber.Ctx) error {
	u := auth.GetUser(c)
	if u == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "not authenticated"})
	}

	var req struct {
		Code string `json:"code"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request"})
	}

	// Verify current 2FA
	user, err := models.GetUserByID(a.DB, u.UserID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "user not found"})
	}

	verified := false
	if user.TOTPEnabled {
		secret, err := models.GetTOTPSecret(a.DB, u.UserID)
		if err == nil {
			plainSecret, err := a.decryptSecret(secret.SecretEncrypted, secret.SecretSalt)
			if err == nil && totp.Validate(req.Code, plainSecret) {
				ok, err := models.CheckAndUpdateTOTPUsed(a.DB, u.UserID)
				if err != nil || !ok {
					return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{"error": "code already used, wait for next code"})
				}
				verified = true
			}
		}
	}

	if !verified {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "2FA verification required"})
	}

	codes, err := models.GenerateRecoveryCodes(a.DB, u.UserID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to generate recovery codes"})
	}

	return c.JSON(fiber.Map{"recovery_codes": codes})
}

// --- Admin Reset ---

func (a *Admin) ResetUser2FA(c *fiber.Ctx) error {
	userID := c.Params("id")

	if err := models.ResetUser2FA(a.DB, userID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to reset 2FA"})
	}

	auth.InvalidateUserCache(userID)
	log.Printf("admin reset 2FA for user: %s", userID)

	return c.JSON(fiber.Map{"status": "ok"})
}

// --- Helpers ---

func (a *Admin) issueAuthAfter2FA(c *fiber.Ctx, userID string) error {
	user, err := models.GetUserByID(a.DB, userID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "user not found"})
	}

	if user.Disabled {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "account disabled"})
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
			"enforce_2fa":           false,
			"password_expired":      models.IsPasswordExpired(user, a.Config.Security.PasswordExpiryDays),
		},
	})
}
