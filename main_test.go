package main

import (
	"testing"
)

func TestIsAllowedHost_EmptyList(t *testing.T) {
	if !isAllowedHost("anything.com", nil) {
		t.Error("empty allowed list should allow all hosts")
	}
}

func TestIsAllowedHost_Match(t *testing.T) {
	allowed := []string{"example.com", "other.com"}
	if !isAllowedHost("example.com", allowed) {
		t.Error("expected match for 'example.com'")
	}
}

func TestIsAllowedHost_NoMatch(t *testing.T) {
	allowed := []string{"example.com"}
	if isAllowedHost("evil.com", allowed) {
		t.Error("expected no match for 'evil.com'")
	}
}

func TestIsAllowedHost_StripPort(t *testing.T) {
	allowed := []string{"example.com"}
	if !isAllowedHost("example.com:8080", allowed) {
		t.Error("expected match after stripping port")
	}
}

func TestListenURL_HTTP(t *testing.T) {
	got := listenURL("0.0.0.0:80", false)
	want := "http://0.0.0.0"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestListenURL_HTTPS(t *testing.T) {
	got := listenURL("0.0.0.0:443", true)
	want := "https://0.0.0.0"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestListenURL_CustomPort(t *testing.T) {
	got := listenURL("0.0.0.0:8443", true)
	want := "https://0.0.0.0:8443"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestGeneratePassword_Length(t *testing.T) {
	pw, err := generatePassword(16)
	if err != nil {
		t.Fatalf("generatePassword: %v", err)
	}
	if len(pw) != 16 {
		t.Errorf("length: got %d, want 16", len(pw))
	}
}

func TestGeneratePassword_Unique(t *testing.T) {
	pw1, err := generatePassword(16)
	if err != nil {
		t.Fatalf("generatePassword: %v", err)
	}
	pw2, err := generatePassword(16)
	if err != nil {
		t.Fatalf("generatePassword: %v", err)
	}
	if pw1 == pw2 {
		t.Error("expected two calls to produce different passwords")
	}
}
