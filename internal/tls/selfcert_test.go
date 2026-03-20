package tls

import (
	"crypto/x509"
	"encoding/pem"
	"os"
	"testing"
)

func TestEnsureSelfSigned_GeneratesCert(t *testing.T) {
	dir := t.TempDir()

	certPath, keyPath, err := EnsureSelfSigned(dir)
	if err != nil {
		t.Fatalf("EnsureSelfSigned: %v", err)
	}

	if _, err := os.Stat(certPath); err != nil {
		t.Errorf("cert file not found: %v", err)
	}
	if _, err := os.Stat(keyPath); err != nil {
		t.Errorf("key file not found: %v", err)
	}
}

func TestEnsureSelfSigned_Idempotent(t *testing.T) {
	dir := t.TempDir()

	cert1, key1, err := EnsureSelfSigned(dir)
	if err != nil {
		t.Fatalf("first EnsureSelfSigned: %v", err)
	}

	cert2, key2, err := EnsureSelfSigned(dir)
	if err != nil {
		t.Fatalf("second EnsureSelfSigned: %v", err)
	}

	if cert1 != cert2 || key1 != key2 {
		t.Error("expected same paths on second call")
	}
}

func TestEnsureSelfSigned_ValidCert(t *testing.T) {
	dir := t.TempDir()

	certPath, _, err := EnsureSelfSigned(dir)
	if err != nil {
		t.Fatalf("EnsureSelfSigned: %v", err)
	}

	data, err := os.ReadFile(certPath)
	if err != nil {
		t.Fatalf("reading cert: %v", err)
	}

	block, _ := pem.Decode(data)
	if block == nil {
		t.Fatal("failed to decode PEM block")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("parsing certificate: %v", err)
	}

	foundLocalhost := false
	for _, dns := range cert.DNSNames {
		if dns == "localhost" {
			foundLocalhost = true
		}
	}
	if !foundLocalhost {
		t.Errorf("cert DNS names %v do not include 'localhost'", cert.DNSNames)
	}
}
