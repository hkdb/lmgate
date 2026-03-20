package models

import (
	"database/sql"
	"fmt"

	"github.com/google/uuid"
)

type WebAuthnCredential struct {
	ID              string `json:"id"`
	UserID          string `json:"user_id"`
	Name            string `json:"name"`
	CredentialID    []byte `json:"-"`
	PublicKey       []byte `json:"-"`
	AttestationType string `json:"-"`
	AAGUID          []byte `json:"-"`
	SignCount       uint32 `json:"sign_count"`
	CreatedAt       string `json:"created_at"`
}

func CreateWebAuthnCredential(db *sql.DB, userID, name string, credentialID, publicKey []byte, attestationType string, aaguid []byte, signCount uint32) (*WebAuthnCredential, error) {
	cred := &WebAuthnCredential{
		ID:              uuid.New().String(),
		UserID:          userID,
		Name:            name,
		CredentialID:    credentialID,
		PublicKey:       publicKey,
		AttestationType: attestationType,
		AAGUID:          aaguid,
		SignCount:        signCount,
	}

	_, err := db.Exec(
		`INSERT INTO webauthn_credentials (id, user_id, name, credential_id, public_key, attestation_type, aaguid, sign_count)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		cred.ID, cred.UserID, cred.Name, cred.CredentialID, cred.PublicKey, cred.AttestationType, cred.AAGUID, cred.SignCount,
	)
	if err != nil {
		return nil, err
	}

	return cred, nil
}

func ListWebAuthnCredentials(db *sql.DB, userID string) ([]WebAuthnCredential, error) {
	rows, err := db.Query(
		`SELECT id, user_id, name, credential_id, public_key, attestation_type, aaguid, sign_count, created_at
		 FROM webauthn_credentials WHERE user_id = ? ORDER BY created_at`, userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var creds []WebAuthnCredential
	for rows.Next() {
		var c WebAuthnCredential
		var aaguid sql.RawBytes
		if err := rows.Scan(&c.ID, &c.UserID, &c.Name, &c.CredentialID, &c.PublicKey, &c.AttestationType, &aaguid, &c.SignCount, &c.CreatedAt); err != nil {
			return nil, err
		}
		c.AAGUID = []byte(aaguid)
		creds = append(creds, c)
	}
	return creds, rows.Err()
}

func GetWebAuthnCredentialByCredentialID(db *sql.DB, credentialID []byte) (*WebAuthnCredential, error) {
	c := &WebAuthnCredential{}
	var aaguid sql.RawBytes
	err := db.QueryRow(
		`SELECT id, user_id, name, credential_id, public_key, attestation_type, aaguid, sign_count, created_at
		 FROM webauthn_credentials WHERE credential_id = ?`, credentialID,
	).Scan(&c.ID, &c.UserID, &c.Name, &c.CredentialID, &c.PublicKey, &c.AttestationType, &aaguid, &c.SignCount, &c.CreatedAt)
	if err != nil {
		return nil, err
	}
	c.AAGUID = []byte(aaguid)
	return c, nil
}

func UpdateWebAuthnSignCount(db *sql.DB, credentialID []byte, signCount uint32) error {
	_, err := db.Exec(`UPDATE webauthn_credentials SET sign_count = ? WHERE credential_id = ?`, signCount, credentialID)
	return err
}

func VerifyAndUpdateWebAuthnSignCount(db *sql.DB, credentialID []byte, newSignCount uint32) error {
	var storedCount uint32
	err := db.QueryRow(`SELECT sign_count FROM webauthn_credentials WHERE credential_id = ?`, credentialID).Scan(&storedCount)
	if err != nil {
		return fmt.Errorf("looking up credential: %w", err)
	}

	if newSignCount > 0 && newSignCount <= storedCount {
		return fmt.Errorf("sign count not increasing: stored=%d new=%d (possible credential cloning)", storedCount, newSignCount)
	}

	_, err = db.Exec(`UPDATE webauthn_credentials SET sign_count = ? WHERE credential_id = ?`, newSignCount, credentialID)
	return err
}

func DeleteWebAuthnCredential(db *sql.DB, id, userID string) error {
	_, err := db.Exec(`DELETE FROM webauthn_credentials WHERE id = ? AND user_id = ?`, id, userID)
	return err
}

func HasWebAuthnCredentials(db *sql.DB, userID string) (bool, error) {
	var count int
	err := db.QueryRow(`SELECT COUNT(*) FROM webauthn_credentials WHERE user_id = ?`, userID).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
