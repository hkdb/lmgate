package middleware

import (
	"database/sql"
	"encoding/json"
	"strings"

	"github.com/hkdb/lmgate/internal/auth"
	"github.com/gofiber/fiber/v2"
)

func ModelACL(db *sql.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Only check POST requests (model is in the request body)
		if c.Method() != "POST" {
			return c.Next()
		}

		body := c.Body()
		if len(body) == 0 {
			return c.Next()
		}

		var payload struct {
			Model string `json:"model"`
		}
		if err := json.Unmarshal(body, &payload); err != nil {
			return c.Next()
		}

		if payload.Model == "" {
			return c.Next()
		}

		// Store model name for audit logging
		c.Locals("request_model", payload.Model)

		u := auth.GetUser(c)
		if u == nil {
			return c.Next()
		}

		// Admins bypass ACL
		if u.Role == "admin" {
			return c.Next()
		}

		allowed, err := checkModelAccess(db, u.UserID, payload.Model)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "acl check failed"})
		}

		if !allowed {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "access to model denied",
				"model": payload.Model,
			})
		}

		return c.Next()
	}
}

func checkModelAccess(db *sql.DB, userID, model string) (bool, error) {
	rows, err := db.Query(
		`SELECT model, allowed FROM model_acls WHERE user_id = ?`, userID,
	)
	if err != nil {
		return false, err
	}
	defer rows.Close()

	var acls []struct {
		pattern string
		allowed bool
	}

	for rows.Next() {
		var a struct {
			pattern string
			allowed bool
		}
		if err := rows.Scan(&a.pattern, &a.allowed); err != nil {
			return false, err
		}
		acls = append(acls, a)
	}

	if err := rows.Err(); err != nil {
		return false, err
	}

	// No ACLs = allow all
	if len(acls) == 0 {
		return true, nil
	}

	// Check for exact match first, then wildcard patterns
	for _, acl := range acls {
		if acl.pattern == model {
			return acl.allowed, nil
		}
	}

	for _, acl := range acls {
		if matchPattern(acl.pattern, model) {
			return acl.allowed, nil
		}
	}

	// No matching ACL = deny (if ACLs exist but none match)
	return false, nil
}

func matchPattern(pattern, model string) bool {
	// Wildcard: "*" matches everything
	if pattern == "*" {
		return true
	}

	// Prefix wildcard: "llama3:*" matches "llama3:8b", "llama3:70b", etc.
	if strings.HasSuffix(pattern, ":*") {
		prefix := strings.TrimSuffix(pattern, ":*")
		return strings.HasPrefix(model, prefix+":")
	}

	// Suffix wildcard: "*:latest" matches "llama3:latest", "mistral:latest"
	if strings.HasPrefix(pattern, "*:") {
		suffix := strings.TrimPrefix(pattern, "*:")
		return strings.HasSuffix(model, ":"+suffix)
	}

	return false
}
