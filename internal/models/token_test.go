package models

import (
	"strings"
	"testing"
	"time"
)

func TestGenerateToken_Format(t *testing.T) {
	raw, hash, err := GenerateToken()
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}

	if !strings.HasPrefix(raw, "lmg_") {
		t.Errorf("raw token should start with 'lmg_', got %q", raw[:8])
	}
	if len(hash) != 64 {
		t.Errorf("hash length: got %d, want 64", len(hash))
	}
}

func TestHashToken_Deterministic(t *testing.T) {
	h1 := HashToken("lmg_test123")
	h2 := HashToken("lmg_test123")
	if h1 != h2 {
		t.Errorf("HashToken not deterministic: %q != %q", h1, h2)
	}
}

func TestCreateAPIToken_And_GetByHash(t *testing.T) {
	db := testOpenDB(t)

	u, err := CreateUser(db, "tok@example.com", "T", "pass", "user", "local", "", false)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	tok, raw, err := CreateAPIToken(db, u.ID, "test-token", 60, nil)
	if err != nil {
		t.Fatalf("CreateAPIToken: %v", err)
	}

	hash := HashToken(raw)
	got, err := GetAPITokenByHash(db, hash)
	if err != nil {
		t.Fatalf("GetAPITokenByHash: %v", err)
	}

	if got.ID != tok.ID {
		t.Errorf("ID mismatch: got %q, want %q", got.ID, tok.ID)
	}
	if got.Name != "test-token" {
		t.Errorf("Name: got %q, want %q", got.Name, "test-token")
	}
}

func TestRevokeAPIToken_SetsRevoked(t *testing.T) {
	db := testOpenDB(t)

	u, err := CreateUser(db, "rev@example.com", "R", "pass", "user", "local", "", false)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	tok, raw, err := CreateAPIToken(db, u.ID, "revoke-me", 60, nil)
	if err != nil {
		t.Fatalf("CreateAPIToken: %v", err)
	}

	if err := RevokeAPIToken(db, tok.ID); err != nil {
		t.Fatalf("RevokeAPIToken: %v", err)
	}

	hash := HashToken(raw)
	got, err := GetAPITokenByHash(db, hash)
	if err != nil {
		t.Fatalf("GetAPITokenByHash: %v", err)
	}
	if !got.Revoked {
		t.Error("expected token to be revoked")
	}
}

func TestIsTokenExpired(t *testing.T) {
	past := time.Now().UTC().Add(-time.Hour).Format(time.DateTime)
	future := time.Now().UTC().Add(time.Hour).Format(time.DateTime)

	tests := []struct {
		name     string
		token    *APIToken
		expected bool
	}{
		{"nil expiry", &APIToken{}, false},
		{"expired", &APIToken{ExpiresAt: &past}, true},
		{"not expired", &APIToken{ExpiresAt: &future}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsTokenExpired(tt.token); got != tt.expected {
				t.Errorf("IsTokenExpired: got %v, want %v", got, tt.expected)
			}
		})
	}
}
