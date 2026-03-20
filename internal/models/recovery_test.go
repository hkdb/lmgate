package models

import (
	"testing"
)

func TestGenerateRecoveryCodes_ReturnsTen(t *testing.T) {
	db := testOpenDB(t)

	u, err := CreateUser(db, "rc@example.com", "RC", "pass", "user", "local", "", false)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	codes, err := GenerateRecoveryCodes(db, u.ID)
	if err != nil {
		t.Fatalf("GenerateRecoveryCodes: %v", err)
	}
	if len(codes) != 10 {
		t.Errorf("got %d codes, want 10", len(codes))
	}
}

func TestValidateRecoveryCode_Valid(t *testing.T) {
	db := testOpenDB(t)

	u, err := CreateUser(db, "rc2@example.com", "RC2", "pass", "user", "local", "", false)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	codes, err := GenerateRecoveryCodes(db, u.ID)
	if err != nil {
		t.Fatalf("GenerateRecoveryCodes: %v", err)
	}

	valid, err := ValidateRecoveryCode(db, u.ID, codes[0])
	if err != nil {
		t.Fatalf("ValidateRecoveryCode: %v", err)
	}
	if !valid {
		t.Error("expected valid recovery code")
	}
}

func TestValidateRecoveryCode_Invalid(t *testing.T) {
	db := testOpenDB(t)

	u, err := CreateUser(db, "rc3@example.com", "RC3", "pass", "user", "local", "", false)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	if _, err := GenerateRecoveryCodes(db, u.ID); err != nil {
		t.Fatalf("GenerateRecoveryCodes: %v", err)
	}

	valid, err := ValidateRecoveryCode(db, u.ID, "not-a-real-code")
	if err != nil {
		t.Fatalf("ValidateRecoveryCode: %v", err)
	}
	if valid {
		t.Error("expected invalid recovery code")
	}
}

func TestValidateRecoveryCode_Used(t *testing.T) {
	db := testOpenDB(t)

	u, err := CreateUser(db, "rc4@example.com", "RC4", "pass", "user", "local", "", false)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	codes, err := GenerateRecoveryCodes(db, u.ID)
	if err != nil {
		t.Fatalf("GenerateRecoveryCodes: %v", err)
	}

	// Use the code once
	if _, err := ValidateRecoveryCode(db, u.ID, codes[0]); err != nil {
		t.Fatalf("first ValidateRecoveryCode: %v", err)
	}

	// Second use should fail
	valid, err := ValidateRecoveryCode(db, u.ID, codes[0])
	if err != nil {
		t.Fatalf("second ValidateRecoveryCode: %v", err)
	}
	if valid {
		t.Error("expected used recovery code to be invalid")
	}
}

func TestCountUnusedRecoveryCodes(t *testing.T) {
	db := testOpenDB(t)

	u, err := CreateUser(db, "rc5@example.com", "RC5", "pass", "user", "local", "", false)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	codes, err := GenerateRecoveryCodes(db, u.ID)
	if err != nil {
		t.Fatalf("GenerateRecoveryCodes: %v", err)
	}

	count, err := CountUnusedRecoveryCodes(db, u.ID)
	if err != nil {
		t.Fatalf("CountUnusedRecoveryCodes: %v", err)
	}
	if count != 10 {
		t.Errorf("unused count: got %d, want 10", count)
	}

	// Use one code
	if _, err := ValidateRecoveryCode(db, u.ID, codes[0]); err != nil {
		t.Fatalf("ValidateRecoveryCode: %v", err)
	}

	count, err = CountUnusedRecoveryCodes(db, u.ID)
	if err != nil {
		t.Fatalf("CountUnusedRecoveryCodes: %v", err)
	}
	if count != 9 {
		t.Errorf("unused count after use: got %d, want 9", count)
	}
}
