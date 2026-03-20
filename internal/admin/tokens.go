package admin

import (
	"database/sql"
	"time"

	"github.com/hkdb/lmgate/internal/auth"
	"github.com/hkdb/lmgate/internal/models"
	"github.com/gofiber/fiber/v2"
)

type tokenResponse struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	Prefix     string  `json:"prefix"`
	UserEmail  string  `json:"user_email"`
	RateLimit  int     `json:"rate_limit"`
	ExpiresAt  *string `json:"expires_at,omitempty"`
	LastUsedAt *string `json:"last_used_at"`
	Status     string  `json:"status"`
	CreatedAt  string  `json:"created_at"`
}

func (a *Admin) ListTokens(c *fiber.Ctx) error {
	userID := c.Query("user_id")

	query := `SELECT t.id, t.name, t.token_redacted, t.rate_limit, t.expires_at, t.revoked, t.created_at, COALESCE(u.email, '')
		FROM api_tokens t LEFT JOIN users u ON t.user_id = u.id`
	var args []any
	if userID != "" {
		query += ` WHERE t.user_id = ?`
		args = append(args, userID)
	}
	query += ` ORDER BY t.created_at DESC`

	rows, err := a.DB.Query(query, args...)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to list tokens"})
	}
	defer rows.Close()

	var tokens []tokenResponse
	for rows.Next() {
		var (
			id, name, redacted, createdAt, email string
			rateLimit                            int
			expiresAt                            sql.NullString
			revoked                              bool
		)
		if err := rows.Scan(&id, &name, &redacted, &rateLimit, &expiresAt, &revoked, &createdAt, &email); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to list tokens"})
		}

		status := deriveTokenStatus(revoked, expiresAt)

		tr := tokenResponse{
			ID:        id,
			Name:      name,
			Prefix:    redacted,
			UserEmail: email,
			RateLimit: rateLimit,
			Status:    status,
			CreatedAt: createdAt,
		}
		if expiresAt.Valid {
			tr.ExpiresAt = &expiresAt.String
		}
		tokens = append(tokens, tr)
	}
	if err := rows.Err(); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to list tokens"})
	}

	if tokens == nil {
		tokens = []tokenResponse{}
	}
	return c.JSON(tokens)
}

func (a *Admin) CreateToken(c *fiber.Ctx) error {
	var req struct {
		UserID    string  `json:"user_id"`
		Name      string  `json:"name"`
		RateLimit int     `json:"rate_limit"`
		ExpiresAt *string `json:"expires_at,omitempty"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request"})
	}

	if req.UserID == "" {
		u := auth.GetUser(c)
		if u == nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
		}
		req.UserID = u.UserID
	}

	if req.RateLimit <= 0 {
		req.RateLimit = a.Config.RateLimit.DefaultRPM
	}

	token, rawToken, err := models.CreateAPIToken(a.DB, req.UserID, req.Name, req.RateLimit, req.ExpiresAt)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to create token"})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"token": rawToken,
		"id":    token.ID,
	})
}

func (a *Admin) RevokeToken(c *fiber.Ctx) error {
	if err := models.RevokeAPIToken(a.DB, c.Params("id")); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to revoke token"})
	}
	return c.JSON(fiber.Map{"status": "revoked"})
}

func (a *Admin) DeleteToken(c *fiber.Ctx) error {
	if err := models.DeleteAPIToken(a.DB, c.Params("id")); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to delete token"})
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (a *Admin) ListMyTokens(c *fiber.Ctx) error {
	u := auth.GetUser(c)
	if u == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
	}

	rows, err := a.DB.Query(
		`SELECT t.id, t.name, t.token_redacted, t.rate_limit, t.expires_at, t.revoked, t.created_at, COALESCE(u.email, '')
		FROM api_tokens t LEFT JOIN users u ON t.user_id = u.id
		WHERE t.user_id = ?
		ORDER BY t.created_at DESC`, u.UserID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to list tokens"})
	}
	defer rows.Close()

	var tokens []tokenResponse
	for rows.Next() {
		var (
			id, name, redacted, createdAt, email string
			rateLimit                            int
			expiresAt                            sql.NullString
			revoked                              bool
		)
		if err := rows.Scan(&id, &name, &redacted, &rateLimit, &expiresAt, &revoked, &createdAt, &email); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to list tokens"})
		}

		status := deriveTokenStatus(revoked, expiresAt)

		tr := tokenResponse{
			ID:        id,
			Name:      name,
			Prefix:    redacted,
			UserEmail: email,
			RateLimit: rateLimit,
			Status:    status,
			CreatedAt: createdAt,
		}
		if expiresAt.Valid {
			tr.ExpiresAt = &expiresAt.String
		}
		tokens = append(tokens, tr)
	}
	if err := rows.Err(); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to list tokens"})
	}

	if tokens == nil {
		tokens = []tokenResponse{}
	}
	return c.JSON(tokens)
}

func (a *Admin) CreateMyToken(c *fiber.Ctx) error {
	u := auth.GetUser(c)
	if u == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
	}

	var req struct {
		Name      string  `json:"name"`
		RateLimit int     `json:"rate_limit"`
		ExpiresAt *string `json:"expires_at,omitempty"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request"})
	}

	if req.RateLimit <= 0 {
		req.RateLimit = a.Config.RateLimit.DefaultRPM
	}

	token, rawToken, err := models.CreateAPIToken(a.DB, u.UserID, req.Name, req.RateLimit, req.ExpiresAt)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to create token"})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"token": rawToken,
		"id":    token.ID,
	})
}

func (a *Admin) RevokeMyToken(c *fiber.Ctx) error {
	u := auth.GetUser(c)
	if u == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
	}

	tokenID := c.Params("id")
	var ownerID string
	err := a.DB.QueryRow(`SELECT user_id FROM api_tokens WHERE id = ?`, tokenID).Scan(&ownerID)
	if err != nil || ownerID != u.UserID {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "token not found"})
	}

	if err := models.RevokeAPIToken(a.DB, tokenID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to revoke token"})
	}
	return c.JSON(fiber.Map{"status": "revoked"})
}

func (a *Admin) DeleteMyToken(c *fiber.Ctx) error {
	u := auth.GetUser(c)
	if u == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
	}

	tokenID := c.Params("id")
	var ownerID string
	err := a.DB.QueryRow(`SELECT user_id FROM api_tokens WHERE id = ?`, tokenID).Scan(&ownerID)
	if err != nil || ownerID != u.UserID {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "token not found"})
	}

	if err := models.DeleteAPIToken(a.DB, tokenID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to delete token"})
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func deriveTokenStatus(revoked bool, expiresAt sql.NullString) string {
	if revoked {
		return "revoked"
	}
	if expiresAt.Valid {
		exp, err := time.Parse(time.DateTime, expiresAt.String)
		if err == nil && time.Now().UTC().After(exp) {
			return "expired"
		}
	}
	return "active"
}
