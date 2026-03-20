package models

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type RecoveryCode struct {
	ID        string `json:"id"`
	UserID    string `json:"user_id"`
	CodeHash  string `json:"-"`
	Used      bool   `json:"used"`
	CreatedAt string `json:"created_at"`
}

func GenerateRecoveryCodes(db *sql.DB, userID string) ([]string, error) {
	// Delete existing codes first
	if _, err := db.Exec(`DELETE FROM recovery_codes WHERE user_id = ?`, userID); err != nil {
		return nil, fmt.Errorf("deleting old codes: %w", err)
	}

	codes := make([]string, 10)
	for i := range codes {
		b := make([]byte, 8)
		if _, err := rand.Read(b); err != nil {
			return nil, fmt.Errorf("generating random bytes: %w", err)
		}
		codes[i] = hex.EncodeToString(b)
	}

	for _, code := range codes {
		hash, err := bcrypt.GenerateFromPassword([]byte(code), bcrypt.DefaultCost)
		if err != nil {
			return nil, fmt.Errorf("hashing recovery code: %w", err)
		}

		_, err = db.Exec(
			`INSERT INTO recovery_codes (id, user_id, code_hash) VALUES (?, ?, ?)`,
			uuid.New().String(), userID, string(hash),
		)
		if err != nil {
			return nil, fmt.Errorf("inserting recovery code: %w", err)
		}
	}

	return codes, nil
}

func ValidateRecoveryCode(db *sql.DB, userID, code string) (bool, error) {
	rows, err := db.Query(
		`SELECT id, code_hash FROM recovery_codes WHERE user_id = ? AND used = 0`, userID,
	)
	if err != nil {
		return false, err
	}
	defer rows.Close()

	var matchedID string
	for rows.Next() {
		var id, hash string
		if err := rows.Scan(&id, &hash); err != nil {
			return false, err
		}
		if bcrypt.CompareHashAndPassword([]byte(hash), []byte(code)) == nil {
			matchedID = id
		}
	}
	if err := rows.Err(); err != nil {
		return false, err
	}
	if matchedID == "" {
		return false, nil
	}

	_, err = db.Exec(`UPDATE recovery_codes SET used = 1 WHERE id = ?`, matchedID)
	return err == nil, err
}

func DeleteRecoveryCodes(db *sql.DB, userID string) error {
	_, err := db.Exec(`DELETE FROM recovery_codes WHERE user_id = ?`, userID)
	return err
}

func CountUnusedRecoveryCodes(db *sql.DB, userID string) (int, error) {
	var count int
	err := db.QueryRow(`SELECT COUNT(*) FROM recovery_codes WHERE user_id = ? AND used = 0`, userID).Scan(&count)
	return count, err
}
