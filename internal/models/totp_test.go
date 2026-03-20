package models

import (
	"database/sql"
	"testing"
)

func TestSaveTOTPSecret_And_Get(t *testing.T) {
	db := testOpenDB(t)

	u, err := CreateUser(db, "totp@example.com", "TOTP", "pass", "user", "local", "", false)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	if err := SaveTOTPSecret(db, u.ID, "encrypted-secret", "salt-value"); err != nil {
		t.Fatalf("SaveTOTPSecret: %v", err)
	}

	got, err := GetTOTPSecret(db, u.ID)
	if err != nil {
		t.Fatalf("GetTOTPSecret: %v", err)
	}

	if got.SecretEncrypted != "encrypted-secret" {
		t.Errorf("SecretEncrypted: got %q, want %q", got.SecretEncrypted, "encrypted-secret")
	}
	if got.SecretSalt != "salt-value" {
		t.Errorf("SecretSalt: got %q, want %q", got.SecretSalt, "salt-value")
	}
	if got.Verified {
		t.Error("expected Verified to be false initially")
	}
}

func TestMarkTOTPVerified(t *testing.T) {
	db := testOpenDB(t)

	u, err := CreateUser(db, "totp2@example.com", "TOTP2", "pass", "user", "local", "", false)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	if err := SaveTOTPSecret(db, u.ID, "enc", "salt"); err != nil {
		t.Fatalf("SaveTOTPSecret: %v", err)
	}

	if err := MarkTOTPVerified(db, u.ID); err != nil {
		t.Fatalf("MarkTOTPVerified: %v", err)
	}

	got, err := GetTOTPSecret(db, u.ID)
	if err != nil {
		t.Fatalf("GetTOTPSecret: %v", err)
	}
	if !got.Verified {
		t.Error("expected Verified to be true after marking")
	}
}

func TestDeleteTOTPSecret_Removes(t *testing.T) {
	db := testOpenDB(t)

	u, err := CreateUser(db, "totp3@example.com", "TOTP3", "pass", "user", "local", "", false)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	if err := SaveTOTPSecret(db, u.ID, "enc", "salt"); err != nil {
		t.Fatalf("SaveTOTPSecret: %v", err)
	}

	if err := DeleteTOTPSecret(db, u.ID); err != nil {
		t.Fatalf("DeleteTOTPSecret: %v", err)
	}

	_, err = GetTOTPSecret(db, u.ID)
	if err != sql.ErrNoRows {
		t.Errorf("expected sql.ErrNoRows, got %v", err)
	}
}
