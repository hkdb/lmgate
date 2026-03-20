package tls

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"time"
)

const (
	selfSignedCertFile = "self-signed.crt"
	selfSignedKeyFile  = "self-signed.key"
	certValidDays      = 365
	renewBeforeDays    = 30
)

func EnsureSelfSigned(cacheDir string) (certPath, keyPath string, err error) {
	if err := os.MkdirAll(cacheDir, 0750); err != nil {
		return "", "", fmt.Errorf("creating cert directory: %w", err)
	}

	certPath = filepath.Join(cacheDir, selfSignedCertFile)
	keyPath = filepath.Join(cacheDir, selfSignedKeyFile)

	if needsRegeneration(certPath) {
		if err := generateSelfSigned(certPath, keyPath); err != nil {
			return "", "", err
		}
	}

	return certPath, keyPath, nil
}

func needsRegeneration(certPath string) bool {
	data, err := os.ReadFile(certPath)
	if err != nil {
		return true
	}

	block, _ := pem.Decode(data)
	if block == nil {
		return true
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return true
	}

	return time.Until(cert.NotAfter) < renewBeforeDays*24*time.Hour
}

func generateSelfSigned(certPath, keyPath string) error {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return fmt.Errorf("generating key: %w", err)
	}

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return fmt.Errorf("generating serial: %w", err)
	}

	now := time.Now()
	template := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"lmgate"},
			CommonName:   "lmgate self-signed",
		},
		NotBefore:             now,
		NotAfter:              now.Add(certValidDays * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{"localhost"},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return fmt.Errorf("creating certificate: %w", err)
	}

	certFile, err := os.OpenFile(certPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("creating cert file: %w", err)
	}
	defer certFile.Close()

	if err := pem.Encode(certFile, &pem.Block{Type: "CERTIFICATE", Bytes: certDER}); err != nil {
		return fmt.Errorf("encoding cert: %w", err)
	}

	keyBytes, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return fmt.Errorf("marshaling key: %w", err)
	}

	keyFile, err := os.OpenFile(keyPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("creating key file: %w", err)
	}
	defer keyFile.Close()

	if err := pem.Encode(keyFile, &pem.Block{Type: "EC PRIVATE KEY", Bytes: keyBytes}); err != nil {
		return fmt.Errorf("encoding key: %w", err)
	}

	return nil
}
