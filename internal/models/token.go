package models

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type APIToken struct {
	ID             string  `json:"id"`
	UserID         string  `json:"user_id"`
	Name           string  `json:"name"`
	TokenHash      string  `json:"-"`
	TokenRedacted  string  `json:"token_redacted,omitempty"`
	RateLimit      int     `json:"rate_limit"`
	ExpiresAt      *string `json:"expires_at,omitempty"`
	Revoked        bool    `json:"revoked"`
	CreatedAt      string  `json:"created_at"`
}

func GenerateToken() (raw string, hash string, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", fmt.Errorf("generating random bytes: %w", err)
	}
	raw = "lmg_" + hex.EncodeToString(b)
	h := sha256.Sum256([]byte(raw))
	hash = hex.EncodeToString(h[:])
	return raw, hash, nil
}

func HashToken(raw string) string {
	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:])
}

func CreateAPIToken(db *sql.DB, userID, name string, rateLimit int, expiresAt *string) (*APIToken, string, error) {
	raw, hash, err := GenerateToken()
	if err != nil {
		return nil, "", err
	}

	redacted := raw[:8] + " ... " + raw[len(raw)-4:]

	t := &APIToken{
		ID:            uuid.New().String(),
		UserID:        userID,
		Name:          name,
		TokenHash:     hash,
		TokenRedacted: redacted,
		RateLimit:     rateLimit,
		ExpiresAt:     expiresAt,
		CreatedAt:     time.Now().UTC().Format(time.DateTime),
	}

	_, err = db.Exec(
		`INSERT INTO api_tokens (id, user_id, name, token_hash, token_redacted, rate_limit, expires_at, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		t.ID, t.UserID, t.Name, t.TokenHash, t.TokenRedacted, t.RateLimit, t.ExpiresAt, t.CreatedAt,
	)
	if err != nil {
		return nil, "", fmt.Errorf("inserting token: %w", err)
	}

	return t, raw, nil
}

func GetAPITokenByHash(db *sql.DB, hash string) (*APIToken, error) {
	t := &APIToken{}
	var expiresAt sql.NullString
	err := db.QueryRow(
		`SELECT id, user_id, name, token_hash, rate_limit, expires_at, revoked, created_at
		 FROM api_tokens WHERE token_hash = ?`, hash,
	).Scan(&t.ID, &t.UserID, &t.Name, &t.TokenHash, &t.RateLimit, &expiresAt, &t.Revoked, &t.CreatedAt)
	if err != nil {
		return nil, err
	}
	if expiresAt.Valid {
		t.ExpiresAt = &expiresAt.String
	}
	return t, nil
}

func ListAPITokens(db *sql.DB, userID string) ([]APIToken, error) {
	query := `SELECT id, user_id, name, rate_limit, expires_at, revoked, created_at FROM api_tokens`
	var args []any
	if userID != "" {
		query += ` WHERE user_id = ?`
		args = append(args, userID)
	}
	query += ` ORDER BY created_at DESC`

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tokens []APIToken
	for rows.Next() {
		var t APIToken
		var expiresAt sql.NullString
		if err := rows.Scan(&t.ID, &t.UserID, &t.Name, &t.RateLimit, &expiresAt, &t.Revoked, &t.CreatedAt); err != nil {
			return nil, err
		}
		if expiresAt.Valid {
			t.ExpiresAt = &expiresAt.String
		}
		tokens = append(tokens, t)
	}
	return tokens, rows.Err()
}

func RevokeAPIToken(db *sql.DB, id string) error {
	_, err := db.Exec(`UPDATE api_tokens SET revoked = 1 WHERE id = ?`, id)
	return err
}

func DeleteAPIToken(db *sql.DB, id string) error {
	_, err := db.Exec(`DELETE FROM api_tokens WHERE id = ?`, id)
	return err
}

func IsTokenExpired(t *APIToken) bool {
	if t.ExpiresAt == nil {
		return false
	}
	exp, err := time.Parse(time.DateTime, *t.ExpiresAt)
	if err != nil {
		return true
	}
	return time.Now().UTC().After(exp)
}
