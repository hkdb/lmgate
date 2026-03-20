package crypto

import (
	"testing"
)

func TestRoundTrip(t *testing.T) {
	passphrase := "test-passphrase-that-is-long-enough-32chars!"
	plaintext := "my-secret-value"

	encoded, salt, err := EncryptSecret(plaintext, passphrase)
	if err != nil {
		t.Fatalf("EncryptSecret: %v", err)
	}
	if encoded == "" || salt == "" {
		t.Fatal("expected non-empty encoded and salt")
	}

	got, err := DecryptSecret(encoded, salt, passphrase)
	if err != nil {
		t.Fatalf("DecryptSecret: %v", err)
	}
	if got != plaintext {
		t.Fatalf("got %q, want %q", got, plaintext)
	}
}

func TestDecryptEmptySalt(t *testing.T) {
	_, err := DecryptSecret("something", "", "passphrase")
	if err == nil {
		t.Fatal("expected error for empty salt")
	}
}

func TestDecryptWrongPassphrase(t *testing.T) {
	passphrase := "correct-passphrase-that-is-long-enough!!"
	plaintext := "my-secret"

	encoded, salt, err := EncryptSecret(plaintext, passphrase)
	if err != nil {
		t.Fatalf("EncryptSecret: %v", err)
	}

	_, err = DecryptSecret(encoded, salt, "wrong-passphrase-that-is-also-long-enough")
	if err == nil {
		t.Fatal("expected error for wrong passphrase")
	}
}

func TestDecryptInvalidBase64(t *testing.T) {
	_, err := DecryptSecret("not-valid-base64!", "dGVzdA==", "passphrase")
	if err == nil {
		t.Fatal("expected error for invalid base64 encoded data")
	}

	_, err = DecryptSecret("dGVzdA==", "not-valid-base64!", "passphrase")
	if err == nil {
		t.Fatal("expected error for invalid base64 salt")
	}
}
