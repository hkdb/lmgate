package models

import (
	"database/sql"
	"fmt"
	"time"
)

type TOTPSecret struct {
	UserID          string `json:"user_id"`
	SecretEncrypted string `json:"-"`
	SecretSalt      string `json:"-"`
	Verified        bool   `json:"verified"`
	CreatedAt       string `json:"created_at"`
}

func SaveTOTPSecret(db *sql.DB, userID, secretEncrypted, secretSalt string) error {
	_, err := db.Exec(
		`INSERT INTO totp_secrets (user_id, secret_encrypted, secret_salt, verified)
		 VALUES (?, ?, ?, 0)
		 ON CONFLICT(user_id) DO UPDATE SET secret_encrypted = excluded.secret_encrypted, secret_salt = excluded.secret_salt, verified = 0`,
		userID, secretEncrypted, secretSalt,
	)
	return err
}

func GetTOTPSecret(db *sql.DB, userID string) (*TOTPSecret, error) {
	t := &TOTPSecret{}
	err := db.QueryRow(
		`SELECT user_id, secret_encrypted, secret_salt, verified, created_at FROM totp_secrets WHERE user_id = ?`, userID,
	).Scan(&t.UserID, &t.SecretEncrypted, &t.SecretSalt, &t.Verified, &t.CreatedAt)
	if err != nil {
		return nil, err
	}
	return t, nil
}

func MarkTOTPVerified(db *sql.DB, userID string) error {
	_, err := db.Exec(`UPDATE totp_secrets SET verified = 1 WHERE user_id = ?`, userID)
	return err
}

func DeleteTOTPSecret(db *sql.DB, userID string) error {
	_, err := db.Exec(`DELETE FROM totp_secrets WHERE user_id = ?`, userID)
	return err
}

func CheckAndUpdateTOTPUsed(db *sql.DB, userID string) (bool, error) {
	var lastUsed sql.NullString
	err := db.QueryRow(`SELECT totp_last_used_at FROM totp_secrets WHERE user_id = ?`, userID).Scan(&lastUsed)
	if err != nil {
		return false, err
	}

	now := time.Now().UTC()
	if lastUsed.Valid && lastUsed.String != "" {
		t, err := time.Parse(time.RFC3339, lastUsed.String)
		if err == nil && now.Sub(t) < 30*time.Second {
			return false, nil // code was already used in this window
		}
	}

	_, err = db.Exec(`UPDATE totp_secrets SET totp_last_used_at = ? WHERE user_id = ?`, now.Format(time.RFC3339), userID)
	return true, err
}

func ResetUser2FA(db *sql.DB, userID string) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM totp_secrets WHERE user_id = ?`, userID); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM webauthn_credentials WHERE user_id = ?`, userID); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM recovery_codes WHERE user_id = ?`, userID); err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE users SET totp_enabled = 0, updated_at = datetime('now') WHERE id = ?`, userID); err != nil {
		return err
	}

	return tx.Commit()
}
