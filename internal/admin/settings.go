package admin

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/gofiber/fiber/v2"
)

// settingsOIDCProvider is the shape the frontend Settings page expects.
type settingsOIDCProvider struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	IssuerURL    string `json:"issuer_url"`
	ClientID     string `json:"client_id"`
	Scopes       string `json:"scopes"`
	Enabled      bool   `json:"enabled"`
	CreatedAt    string `json:"created_at"`
}

func (a *Admin) GetSettingsOIDC(c *fiber.Ctx) error {
	rows, err := a.DB.Query(
		`SELECT id, provider_type, issuer_url, client_id, scopes, enabled, created_at
		 FROM oidc_providers ORDER BY created_at`,
	)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to list providers"})
	}
	defer rows.Close()

	providers := make([]settingsOIDCProvider, 0)
	for rows.Next() {
		var p settingsOIDCProvider
		if err := rows.Scan(&p.ID, &p.Name, &p.IssuerURL, &p.ClientID, &p.Scopes, &p.Enabled, &p.CreatedAt); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to scan provider"})
		}
		providers = append(providers, p)
	}

	return c.JSON(providers)
}

func (a *Admin) CreateSettingsOIDC(c *fiber.Ctx) error {
	var req struct {
		Name         string `json:"name"`
		IssuerURL    string `json:"issuer_url"`
		ClientID     string `json:"client_id"`
		ClientSecret string `json:"client_secret"`
		Scopes       string `json:"scopes"`
		Enabled      bool   `json:"enabled"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request"})
	}

	if req.Name == "" || req.ClientID == "" || req.ClientSecret == "" || req.IssuerURL == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "name, client_id, client_secret, and issuer_url are required"})
	}

	if req.Scopes == "" {
		req.Scopes = "openid email profile"
	}

	encryptedSecret, salt, err := a.encryptSecret(req.ClientSecret)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to encrypt secret"})
	}

	id := uuid.New().String()
	_, err = a.DB.Exec(
		`INSERT INTO oidc_providers (id, provider_type, client_id, client_secret, client_secret_salt, issuer_url, scopes, enabled)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		id, req.Name, req.ClientID, encryptedSecret, salt, req.IssuerURL, req.Scopes, req.Enabled,
	)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to create provider"})
	}

	if err := a.loadOIDCProvider(id); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("provider created but failed to initialize: %v", err),
		})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"id": id})
}

func (a *Admin) UpdateSettingsOIDC(c *fiber.Ctx) error {
	id := c.Params("id")
	var req struct {
		Name         string `json:"name"`
		IssuerURL    string `json:"issuer_url"`
		ClientID     string `json:"client_id"`
		ClientSecret string `json:"client_secret"`
		Scopes       string `json:"scopes"`
		Enabled      *bool  `json:"enabled"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request"})
	}

	if req.Name != "" {
		if _, err := a.DB.Exec(`UPDATE oidc_providers SET provider_type = ? WHERE id = ?`, req.Name, id); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to update"})
		}
	}
	if req.IssuerURL != "" {
		if _, err := a.DB.Exec(`UPDATE oidc_providers SET issuer_url = ? WHERE id = ?`, req.IssuerURL, id); err != nil {
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
	if req.Scopes != "" {
		if _, err := a.DB.Exec(`UPDATE oidc_providers SET scopes = ? WHERE id = ?`, req.Scopes, id); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to update"})
		}
	}
	if req.Enabled != nil {
		if _, err := a.DB.Exec(`UPDATE oidc_providers SET enabled = ? WHERE id = ?`, *req.Enabled, id); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to update"})
		}
	}

	return c.JSON(fiber.Map{"status": "updated"})
}

func (a *Admin) DeleteSettingsOIDC(c *fiber.Ctx) error {
	return a.DeleteOIDCProvider(c)
}
