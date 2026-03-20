package auth

import (
	"testing"
	"time"
)

func TestSignJWT_RoundTrip(t *testing.T) {
	secret := "test-secret-that-is-at-least-32-chars-long!!"
	tokenStr, err := SignJWT(secret, "user-1", "user@example.com", "admin", time.Hour)
	if err != nil {
		t.Fatalf("SignJWT: %v", err)
	}

	claims, err := VerifyJWT(secret, tokenStr)
	if err != nil {
		t.Fatalf("VerifyJWT: %v", err)
	}

	if claims.UserID != "user-1" {
		t.Errorf("UserID: got %q, want %q", claims.UserID, "user-1")
	}
	if claims.Email != "user@example.com" {
		t.Errorf("Email: got %q, want %q", claims.Email, "user@example.com")
	}
	if claims.Role != "admin" {
		t.Errorf("Role: got %q, want %q", claims.Role, "admin")
	}
}

func TestVerifyJWT_WrongSecret(t *testing.T) {
	tokenStr, err := SignJWT("secret-aaaaaaaaaaaaaaaaaaaaaaaaaaaa", "u1", "a@b.com", "user", time.Hour)
	if err != nil {
		t.Fatalf("SignJWT: %v", err)
	}

	_, err = VerifyJWT("secret-bbbbbbbbbbbbbbbbbbbbbbbbbbbb", tokenStr)
	if err == nil {
		t.Fatal("expected error verifying with wrong secret")
	}
}

func TestVerifyJWT_Expired(t *testing.T) {
	secret := "test-secret-that-is-at-least-32-chars-long!!"
	tokenStr, err := SignJWT(secret, "u1", "a@b.com", "user", -1*time.Hour)
	if err != nil {
		t.Fatalf("SignJWT: %v", err)
	}

	_, err = VerifyJWT(secret, tokenStr)
	if err == nil {
		t.Fatal("expected error for expired token")
	}
}

func TestSignTwoFAToken_RoundTrip(t *testing.T) {
	secret := "test-secret-that-is-at-least-32-chars-long!!"
	methods := []string{"totp", "webauthn"}

	tokenStr, err := SignTwoFAToken(secret, "user-1", methods)
	if err != nil {
		t.Fatalf("SignTwoFAToken: %v", err)
	}

	claims, err := VerifyTwoFAToken(secret, tokenStr)
	if err != nil {
		t.Fatalf("VerifyTwoFAToken: %v", err)
	}

	if claims.UserID != "user-1" {
		t.Errorf("UserID: got %q, want %q", claims.UserID, "user-1")
	}
	if claims.Purpose != "2fa" {
		t.Errorf("Purpose: got %q, want %q", claims.Purpose, "2fa")
	}
	if len(claims.Methods) != 2 || claims.Methods[0] != "totp" {
		t.Errorf("Methods: got %v, want [totp webauthn]", claims.Methods)
	}
}

func TestVerifyTwoFAToken_WrongSecret(t *testing.T) {
	tokenStr, err := SignTwoFAToken("secret-aaaaaaaaaaaaaaaaaaaaaaaaaaaa", "u1", []string{"totp"})
	if err != nil {
		t.Fatalf("SignTwoFAToken: %v", err)
	}

	_, err = VerifyTwoFAToken("secret-bbbbbbbbbbbbbbbbbbbbbbbbbbbb", tokenStr)
	if err == nil {
		t.Fatal("expected error verifying 2FA token with wrong secret")
	}
}
