package middleware

import (
	"database/sql"
	"encoding/json"
	"strings"

	"github.com/hkdb/lmgate/internal/auth"
	"github.com/hkdb/lmgate/internal/models"
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

		groupIDs, _ := models.GetGroupIDsForUser(db, u.UserID)

		allowed, err := checkModelAccess(db, u.UserID, payload.Model, groupIDs)
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

func checkModelAccess(db *sql.DB, userID, model string, groupIDs []string) (bool, error) {
	// Step 1: Check user-specific ACLs
	rows, err := db.Query(
		`SELECT model, allowed FROM model_acls WHERE user_id = ? AND (group_id IS NULL OR group_id = '')`, userID,
	)
	if err != nil {
		return false, err
	}
	defer rows.Close()

	type aclEntry struct {
		pattern string
		allowed bool
	}

	var userACLs []aclEntry
	for rows.Next() {
		var a aclEntry
		if err := rows.Scan(&a.pattern, &a.allowed); err != nil {
			return false, err
		}
		userACLs = append(userACLs, a)
	}
	if err := rows.Err(); err != nil {
		return false, err
	}

	// If user-specific ACL matches, return that result
	for _, acl := range userACLs {
		if acl.pattern == model {
			return acl.allowed, nil
		}
	}
	for _, acl := range userACLs {
		if matchPattern(acl.pattern, model) {
			return acl.allowed, nil
		}
	}

	// Step 2: Check group ACLs
	if len(groupIDs) > 0 {
		placeholders := strings.Repeat("?,", len(groupIDs))
		placeholders = placeholders[:len(placeholders)-1]

		args := make([]any, len(groupIDs))
		for i, id := range groupIDs {
			args[i] = id
		}

		groupRows, err := db.Query(
			`SELECT model, allowed FROM model_acls WHERE group_id IN (`+placeholders+`)`,
			args...,
		)
		if err != nil {
			return false, err
		}
		defer groupRows.Close()

		var groupACLs []aclEntry
		for groupRows.Next() {
			var a aclEntry
			if err := groupRows.Scan(&a.pattern, &a.allowed); err != nil {
				return false, err
			}
			groupACLs = append(groupACLs, a)
		}
		if err := groupRows.Err(); err != nil {
			return false, err
		}

		// Among matching group ACLs: if any allow, allow; if all deny, deny
		hasMatch := false
		for _, acl := range groupACLs {
			if acl.pattern == model || matchPattern(acl.pattern, model) {
				hasMatch = true
				if acl.allowed {
					return true, nil
				}
			}
		}
		if hasMatch {
			return false, nil
		}
	}

	// Step 3: Check if any ACLs exist at all
	var totalACLs int
	err = db.QueryRow(`SELECT COUNT(*) FROM model_acls`).Scan(&totalACLs)
	if err != nil {
		return false, err
	}

	// No ACLs at all → allow (default open)
	if totalACLs == 0 {
		return true, nil
	}

	// Check if any ACLs target this user or their groups specifically
	var userACLCount int
	err = db.QueryRow(`SELECT COUNT(*) FROM model_acls WHERE user_id = ? AND (group_id IS NULL OR group_id = '')`, userID).Scan(&userACLCount)
	if err != nil {
		return false, err
	}

	if userACLCount > 0 {
		// User has ACLs but none matched → deny
		return false, nil
	}

	if len(groupIDs) > 0 {
		placeholders := strings.Repeat("?,", len(groupIDs))
		placeholders = placeholders[:len(placeholders)-1]
		args := make([]any, len(groupIDs))
		for i, id := range groupIDs {
			args[i] = id
		}
		var groupACLCount int
		err = db.QueryRow(`SELECT COUNT(*) FROM model_acls WHERE group_id IN (`+placeholders+`)`, args...).Scan(&groupACLCount)
		if err != nil {
			return false, err
		}
		if groupACLCount > 0 {
			// User's groups have ACLs but none matched → deny
			return false, nil
		}
	}

	// No ACLs targeting this user or their groups → allow
	return true, nil
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
