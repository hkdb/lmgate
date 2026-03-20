package admin

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"net"
	"net/url"
	"strings"

	"github.com/hkdb/lmgate/internal/auth"
	"github.com/hkdb/lmgate/internal/crypto"
	"github.com/hkdb/lmgate/internal/models"
	"github.com/google/uuid"
	"github.com/gofiber/fiber/v2"
)

type oidcProviderRow struct {
	ID           string `json:"id"`
	ProviderType string `json:"provider_type"`
	ClientID     string `json:"client_id"`
	IssuerURL    string `json:"issuer_url"`
	Enabled      bool   `json:"enabled"`
	CreatedAt    string `json:"created_at"`
}

func (a *Admin) ListAuthProviders(c *fiber.Ctx) error {
	type providerInfo struct {
		ID      string `json:"id"`
		Name    string `json:"name"`
		AuthURL string `json:"auth_url"`
	}

	var result []providerInfo
	a.Providers.Range(func(key, value any) bool {
		provider := value.(*auth.OIDCProvider)
		result = append(result, providerInfo{
			ID:      provider.ID,
			Name:    key.(string),
			AuthURL: fmt.Sprintf("/admin/api/oauth/%s", key.(string)),
		})
		return true
	})

	return c.JSON(result)
}

func (a *Admin) OAuthRedirect(c *fiber.Ctx) error {
	providerType := c.Params("provider")

	p, ok := a.Providers.Load(providerType)
	if !ok {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "provider not found"})
	}

	provider := p.(*auth.OIDCProvider)
	state := generateState()
	nonce := generateState()

	// Store state in cookie for validation
	c.Cookie(&fiber.Cookie{
		Name:     "oauth_state",
		Value:    state,
		HTTPOnly: true,
		Secure:   !a.Config.Server.TLS.Disabled,
		SameSite: "Lax",
		MaxAge:   300,
	})

	// Store nonce in cookie to verify ID token binding
	c.Cookie(&fiber.Cookie{
		Name:     "oauth_nonce",
		Value:    nonce,
		HTTPOnly: true,
		Secure:   !a.Config.Server.TLS.Disabled,
		SameSite: "Lax",
		MaxAge:   300,
	})

	return c.Redirect(provider.AuthURL(state, nonce), fiber.StatusTemporaryRedirect)
}

func (a *Admin) OAuthCallback(c *fiber.Ctx) error {
	state := c.Query("state")
	storedState := c.Cookies("oauth_state")

	if state == "" || state != storedState {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid state"})
	}

	nonce := c.Cookies("oauth_nonce")
	if nonce == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "missing nonce"})
	}

	code := c.Query("code")
	if code == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "missing code"})
	}

	// Find the provider — try all loaded providers
	var userInfo *auth.OIDCUserInfo
	var providerType string

	a.Providers.Range(func(key, value any) bool {
		provider := value.(*auth.OIDCProvider)
		info, err := provider.Exchange(c.Context(), code, nonce)
		if err != nil {
			return true // try next
		}
		userInfo = info
		providerType = key.(string)
		return false // found it
	})

	if userInfo == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "authentication failed"})
	}

	user, err := auth.FindOrCreateOIDCUser(a.DB, providerType, userInfo, a.Config.Auth.AdminEmail)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to create user"})
	}

	if user.Disabled {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "account disabled"})
	}

	token, err := auth.SignJWT(a.Config.Auth.JWTSecret, user.ID, user.Email, user.Role, a.Config.Auth.JWTExpiry)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to create token"})
	}

	// Set JWT as cookie for dashboard
	auth.SetAuthCookie(c, token, int(a.Config.Auth.JWTExpiry.Seconds()), a.Config.Server.TLS.Disabled)

	// Clear state and nonce cookies
	c.Cookie(&fiber.Cookie{
		Name:   "oauth_state",
		Value:  "",
		MaxAge: -1,
	})
	c.Cookie(&fiber.Cookie{
		Name:   "oauth_nonce",
		Value:  "",
		MaxAge: -1,
	})

	return c.Redirect("/admin/", fiber.StatusTemporaryRedirect)
}

func (a *Admin) ListOIDCProviders(c *fiber.Ctx) error {
	rows, err := a.DB.Query(
		`SELECT id, provider_type, client_id, issuer_url, enabled, created_at FROM oidc_providers ORDER BY created_at`,
	)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to list providers"})
	}
	defer rows.Close()

	var providers []oidcProviderRow
	for rows.Next() {
		var p oidcProviderRow
		if err := rows.Scan(&p.ID, &p.ProviderType, &p.ClientID, &p.IssuerURL, &p.Enabled, &p.CreatedAt); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to scan provider"})
		}
		providers = append(providers, p)
	}

	return c.JSON(fiber.Map{"providers": providers})
}

func (a *Admin) CreateOIDCProvider(c *fiber.Ctx) error {
	var req struct {
		ProviderType string `json:"provider_type"`
		ClientID     string `json:"client_id"`
		ClientSecret string `json:"client_secret"`
		IssuerURL    string `json:"issuer_url"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request"})
	}

	if req.ProviderType == "" || req.ClientID == "" || req.ClientSecret == "" || req.IssuerURL == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "all fields are required"})
	}

	if err := validateIssuerURL(req.IssuerURL); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	encryptedSecret, salt, err := a.encryptSecret(req.ClientSecret)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to encrypt secret"})
	}

	id := uuid.New().String()
	_, err = a.DB.Exec(
		`INSERT INTO oidc_providers (id, provider_type, client_id, client_secret, client_secret_salt, issuer_url)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		id, req.ProviderType, req.ClientID, encryptedSecret, salt, req.IssuerURL,
	)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to create provider"})
	}

	if err := a.loadOIDCProvider(id); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": fmt.Sprintf("provider created but failed to initialize: %v", err)})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"id": id})
}

func (a *Admin) UpdateOIDCProvider(c *fiber.Ctx) error {
	id := c.Params("id")
	var req struct {
		ClientID     string `json:"client_id"`
		ClientSecret string `json:"client_secret"`
		IssuerURL    string `json:"issuer_url"`
		Enabled      *bool  `json:"enabled"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request"})
	}

	if req.Enabled != nil {
		if _, err := a.DB.Exec(`UPDATE oidc_providers SET enabled = ? WHERE id = ?`, *req.Enabled, id); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to update"})
		}
	}

	if req.ClientID != "" {
		if _, err := a.DB.Exec(`UPDATE oidc_providers SET client_id = ? WHERE id = ?`, req.ClientID, id); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to update"})
		}
	}

	if req.ClientSecret != "" {
		encryptedSecret, salt, err := a.encryptSecret(req.ClientSecret)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to encrypt secret"})
		}
		if _, err := a.DB.Exec(`UPDATE oidc_providers SET client_secret = ?, client_secret_salt = ? WHERE id = ?`, encryptedSecret, salt, id); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to update"})
		}
	}

	if req.IssuerURL != "" {
		if err := validateIssuerURL(req.IssuerURL); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}
		if _, err := a.DB.Exec(`UPDATE oidc_providers SET issuer_url = ? WHERE id = ?`, req.IssuerURL, id); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to update"})
		}
	}

	return c.JSON(fiber.Map{"status": "updated"})
}

func (a *Admin) DeleteOIDCProvider(c *fiber.Ctx) error {
	id := c.Params("id")

	// Get provider type before deleting
	var providerType string
	err := a.DB.QueryRow(`SELECT provider_type FROM oidc_providers WHERE id = ?`, id).Scan(&providerType)
	if err != nil && err != sql.ErrNoRows {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to lookup provider"})
	}

	if _, err := a.DB.Exec(`DELETE FROM oidc_providers WHERE id = ?`, id); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to delete"})
	}

	if providerType != "" {
		a.Providers.Delete(providerType)
	}

	return c.SendStatus(fiber.StatusNoContent)
}

func (a *Admin) LoadOIDCProviders() {
	rows, err := a.DB.Query(`SELECT id FROM oidc_providers WHERE enabled = 1`)
	if err != nil {
		return
	}
	defer rows.Close()

	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			continue
		}
		if err := a.loadOIDCProvider(id); err != nil {
			fmt.Printf("failed to load OIDC provider %s: %v\n", id, err)
		}
	}
}

func (a *Admin) loadOIDCProvider(id string) error {
	var providerType, clientID, clientSecret, clientSecretSalt, issuerURL string
	err := a.DB.QueryRow(
		`SELECT provider_type, client_id, client_secret, client_secret_salt, issuer_url FROM oidc_providers WHERE id = ?`, id,
	).Scan(&providerType, &clientID, &clientSecret, &clientSecretSalt, &issuerURL)
	if err != nil {
		return err
	}

	clientSecret, err = a.decryptSecret(clientSecret, clientSecretSalt)
	if err != nil {
		return fmt.Errorf("decrypting client secret: %w", err)
	}

	// Build redirect URL based on server config
	redirectURL := a.buildOIDCRedirectURL()

	provider, err := auth.NewOIDCProvider(providerType, clientID, clientSecret, issuerURL, redirectURL)
	if err != nil {
		return err
	}

	provider.ID = id
	a.Providers.Store(providerType, provider)
	return nil
}

func (a *Admin) buildOIDCRedirectURL() string {
	scheme := "https"
	if a.Config.Server.TLS.Disabled {
		scheme = "http"
	}

	if len(a.Config.Server.AllowedHosts) > 0 {
		return fmt.Sprintf("%s://%s/admin/api/oauth/callback", scheme, a.Config.Server.AllowedHosts[0])
	}

	if a.Config.Server.TLS.Disabled {
		return fmt.Sprintf("http://localhost%s/admin/api/oauth/callback", a.Config.Server.Listen)
	}

	return "https://localhost/admin/api/oauth/callback"
}

// Model ACL handlers

type modelACLRow struct {
	ID      string `json:"id"`
	UserID  string `json:"user_id"`
	Model   string `json:"model"`
	Allowed bool   `json:"allowed"`
}

func (a *Admin) ListACLs(c *fiber.Ctx) error {
	query := `SELECT id, user_id, model, allowed FROM model_acls`
	var args []any
	if userID := c.Query("user_id"); userID != "" {
		query += ` WHERE user_id = ?`
		args = append(args, userID)
	}
	query += ` ORDER BY user_id, model`

	rows, err := a.DB.Query(query, args...)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to list acls"})
	}
	defer rows.Close()

	var acls []modelACLRow
	for rows.Next() {
		var acl modelACLRow
		if err := rows.Scan(&acl.ID, &acl.UserID, &acl.Model, &acl.Allowed); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to scan acl"})
		}
		acls = append(acls, acl)
	}

	return c.JSON(fiber.Map{"acls": acls})
}

func (a *Admin) CreateACL(c *fiber.Ctx) error {
	var req struct {
		UserID  string `json:"user_id"`
		Model   string `json:"model"`
		Allowed bool   `json:"allowed"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request"})
	}

	if req.UserID == "" || req.Model == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "user_id and model are required"})
	}

	id := uuid.New().String()
	_, err := a.DB.Exec(
		`INSERT INTO model_acls (id, user_id, model, allowed) VALUES (?, ?, ?, ?)
		 ON CONFLICT(user_id, model) DO UPDATE SET allowed = excluded.allowed`,
		id, req.UserID, req.Model, req.Allowed,
	)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to create acl"})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"id": id})
}

func (a *Admin) DeleteACL(c *fiber.Ctx) error {
	if _, err := a.DB.Exec(`DELETE FROM model_acls WHERE id = ?`, c.Params("id")); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to delete acl"})
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (a *Admin) encryptSecret(plaintext string) (string, string, error) {
	return crypto.EncryptSecret(plaintext, a.Config.Security.EncryptionKey)
}

func (a *Admin) decryptSecret(encoded, saltEncoded string) (string, error) {
	return crypto.DecryptSecret(encoded, saltEncoded, a.Config.Security.EncryptionKey)
}

func validateIssuerURL(issuerURL string) error {
	u, err := url.Parse(issuerURL)
	if err != nil {
		return fmt.Errorf("invalid issuer URL: %w", err)
	}
	if u.Scheme != "https" {
		return fmt.Errorf("issuer URL must use https scheme")
	}
	host := u.Hostname()
	ip := net.ParseIP(host)
	if ip != nil {
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
			return fmt.Errorf("issuer URL must not point to a private/loopback address")
		}
	}
	if strings.EqualFold(host, "localhost") {
		return fmt.Errorf("issuer URL must not point to localhost")
	}
	return nil
}

func generateState() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("crypto/rand.Read failed: %v", err))
	}
	return hex.EncodeToString(b)
}

// Audit log pruning
func (a *Admin) PruneAuditLogs() {
	pruned, err := models.PruneAuditLogsByType(
		a.DB,
		a.Config.Logging.APILogRetentionDays,
		a.Config.Logging.AdminLogRetentionDays,
		a.Config.Logging.SecurityLogRetentionDays,
	)
	if err != nil {
		fmt.Printf("audit prune error: %v\n", err)
		return
	}
	if pruned > 0 {
		fmt.Printf("pruned %d audit log entries\n", pruned)
	}
}
