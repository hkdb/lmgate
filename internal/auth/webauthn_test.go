package auth

import (
	"testing"

	"github.com/go-webauthn/webauthn/webauthn"
)

func TestStoreAndLoadWebAuthnSession(t *testing.T) {
	session := &webauthn.SessionData{
		Challenge: "test-challenge-123",
	}

	StoreWebAuthnSession("key1", session)

	loaded, ok := LoadWebAuthnSession("key1")
	if !ok {
		t.Fatal("expected session to be found")
	}
	if loaded.Challenge != "test-challenge-123" {
		t.Errorf("Challenge: got %q, want %q", loaded.Challenge, "test-challenge-123")
	}
}

func TestLoadWebAuthnSession_NotFound(t *testing.T) {
	_, ok := LoadWebAuthnSession("nonexistent-key")
	if ok {
		t.Fatal("expected session not to be found")
	}
}

func TestLoadWebAuthnSession_DeletesAfterLoad(t *testing.T) {
	session := &webauthn.SessionData{
		Challenge: "one-time",
	}

	StoreWebAuthnSession("key-once", session)

	_, ok := LoadWebAuthnSession("key-once")
	if !ok {
		t.Fatal("first load should succeed")
	}

	_, ok = LoadWebAuthnSession("key-once")
	if ok {
		t.Fatal("second load should fail (session deleted after first load)")
	}
}
