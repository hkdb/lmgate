package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_MissingFile_UsesDefaults(t *testing.T) {
	t.Setenv("LMGATE_AUTH_JWT_SECRET", "test-secret-that-is-at-least-32-chars-long!!")
	t.Setenv("LMGATE_ENCRYPTION_KEY", "encryption-key-that-is-at-least-32-chars!!")

	cfg, err := Load(filepath.Join(t.TempDir(), "nonexistent.yaml"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.Upstream.URL != "http://localhost:11434" {
		t.Errorf("expected default upstream URL, got %q", cfg.Upstream.URL)
	}
	if cfg.RateLimit.DefaultRPM != 60 {
		t.Errorf("expected default RPM 60, got %d", cfg.RateLimit.DefaultRPM)
	}
}

func TestLoad_MissingJWTSecret_ReturnsError(t *testing.T) {
	t.Setenv("LMGATE_AUTH_JWT_SECRET", "")
	t.Setenv("LMGATE_ENCRYPTION_KEY", "encryption-key-that-is-at-least-32-chars!!")

	_, err := Load(filepath.Join(t.TempDir(), "nonexistent.yaml"))
	if err == nil {
		t.Fatal("expected error for missing JWT secret")
	}
}

func TestLoad_ShortJWTSecret_ReturnsError(t *testing.T) {
	t.Setenv("LMGATE_AUTH_JWT_SECRET", "short")
	t.Setenv("LMGATE_ENCRYPTION_KEY", "encryption-key-that-is-at-least-32-chars!!")

	_, err := Load(filepath.Join(t.TempDir(), "nonexistent.yaml"))
	if err == nil {
		t.Fatal("expected error for short JWT secret")
	}
}

func TestLoad_MissingEncryptionKey_ReturnsError(t *testing.T) {
	t.Setenv("LMGATE_AUTH_JWT_SECRET", "test-secret-that-is-at-least-32-chars-long!!")
	t.Setenv("LMGATE_ENCRYPTION_KEY", "")

	_, err := Load(filepath.Join(t.TempDir(), "nonexistent.yaml"))
	if err == nil {
		t.Fatal("expected error for missing encryption key")
	}
}

func TestLoad_EnvOverrides(t *testing.T) {
	t.Setenv("LMGATE_AUTH_JWT_SECRET", "test-secret-that-is-at-least-32-chars-long!!")
	t.Setenv("LMGATE_ENCRYPTION_KEY", "encryption-key-that-is-at-least-32-chars!!")
	t.Setenv("LMGATE_LISTEN", "9090")
	t.Setenv("LMGATE_UPSTREAM_URL", "http://other:1234")
	t.Setenv("LMGATE_RATE_LIMIT_DEFAULT_RPM", "120")

	cfg, err := Load(filepath.Join(t.TempDir(), "nonexistent.yaml"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.Server.Listen != "9090" {
		t.Errorf("Listen: got %q, want %q", cfg.Server.Listen, "9090")
	}
	if cfg.Upstream.URL != "http://other:1234" {
		t.Errorf("Upstream URL: got %q, want %q", cfg.Upstream.URL, "http://other:1234")
	}
	if cfg.RateLimit.DefaultRPM != 120 {
		t.Errorf("DefaultRPM: got %d, want 120", cfg.RateLimit.DefaultRPM)
	}
}

func TestLoad_ValidYAML(t *testing.T) {
	t.Setenv("LMGATE_AUTH_JWT_SECRET", "test-secret-that-is-at-least-32-chars-long!!")
	t.Setenv("LMGATE_ENCRYPTION_KEY", "encryption-key-that-is-at-least-32-chars!!")

	yamlContent := `
upstream:
  url: "http://myhost:8080"
  type: "lm-studio"
rate_limit:
  default_rpm: 200
`
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(cfgPath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("writing yaml: %v", err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.Upstream.URL != "http://myhost:8080" {
		t.Errorf("Upstream URL: got %q, want %q", cfg.Upstream.URL, "http://myhost:8080")
	}
	if cfg.Upstream.Type != "lm-studio" {
		t.Errorf("Upstream Type: got %q, want %q", cfg.Upstream.Type, "lm-studio")
	}
	if cfg.RateLimit.DefaultRPM != 200 {
		t.Errorf("DefaultRPM: got %d, want 200", cfg.RateLimit.DefaultRPM)
	}
}
