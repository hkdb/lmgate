package models

import (
	"testing"
)

func TestCreateWebAuthnCredential_And_List(t *testing.T) {
	db := testOpenDB(t)

	u, err := CreateUser(db, "wa@example.com", "WA", "pass", "user", "local", "", false)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	credID := []byte("cred-id-1")
	pubKey := []byte("pub-key-1")
	aaguid := []byte("aaguid-bytes")

	cred, err := CreateWebAuthnCredential(db, u.ID, "my-key", credID, pubKey, "none", aaguid, 0)
	if err != nil {
		t.Fatalf("CreateWebAuthnCredential: %v", err)
	}
	if cred.Name != "my-key" {
		t.Errorf("Name: got %q, want %q", cred.Name, "my-key")
	}

	list, err := ListWebAuthnCredentials(db, u.ID)
	if err != nil {
		t.Fatalf("ListWebAuthnCredentials: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("got %d credentials, want 1", len(list))
	}
	if string(list[0].CredentialID) != string(credID) {
		t.Errorf("CredentialID mismatch")
	}
}

func TestVerifyAndUpdateWebAuthnSignCount_Increasing(t *testing.T) {
	db := testOpenDB(t)

	u, err := CreateUser(db, "wa2@example.com", "WA2", "pass", "user", "local", "", false)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	credID := []byte("cred-sign-test")
	if _, err := CreateWebAuthnCredential(db, u.ID, "key", credID, []byte("pk"), "none", []byte("aa"), 1); err != nil {
		t.Fatalf("CreateWebAuthnCredential: %v", err)
	}

	if err := VerifyAndUpdateWebAuthnSignCount(db, credID, 2); err != nil {
		t.Fatalf("VerifyAndUpdateWebAuthnSignCount: %v", err)
	}
}

func TestVerifyAndUpdateWebAuthnSignCount_NotIncreasing(t *testing.T) {
	db := testOpenDB(t)

	u, err := CreateUser(db, "wa3@example.com", "WA3", "pass", "user", "local", "", false)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	credID := []byte("cred-clone-test")
	if _, err := CreateWebAuthnCredential(db, u.ID, "key", credID, []byte("pk"), "none", []byte("aa"), 5); err != nil {
		t.Fatalf("CreateWebAuthnCredential: %v", err)
	}

	err = VerifyAndUpdateWebAuthnSignCount(db, credID, 3)
	if err == nil {
		t.Fatal("expected error for non-increasing sign count")
	}
}

func TestHasWebAuthnCredentials(t *testing.T) {
	db := testOpenDB(t)

	u, err := CreateUser(db, "wa4@example.com", "WA4", "pass", "user", "local", "", false)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	has, err := HasWebAuthnCredentials(db, u.ID)
	if err != nil {
		t.Fatalf("HasWebAuthnCredentials: %v", err)
	}
	if has {
		t.Error("expected no credentials initially")
	}

	if _, err := CreateWebAuthnCredential(db, u.ID, "key", []byte("cid"), []byte("pk"), "none", []byte("aa"), 0); err != nil {
		t.Fatalf("CreateWebAuthnCredential: %v", err)
	}

	has, err = HasWebAuthnCredentials(db, u.ID)
	if err != nil {
		t.Fatalf("HasWebAuthnCredentials: %v", err)
	}
	if !has {
		t.Error("expected credentials to exist")
	}
}
