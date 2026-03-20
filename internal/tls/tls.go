package tls

import (
	"crypto/tls"
	"fmt"
	"log"

	"golang.org/x/crypto/acme/autocert"

	"github.com/hkdb/lmgate/internal/config"
)

type Result struct {
	TLSConfig *tls.Config
	CertFile  string
	KeyFile   string
}

func Resolve(cfg config.TLSConfig) (*Result, error) {
	if cfg.Disabled {
		return nil, nil
	}

	// Priority 1: Manual certs
	if cfg.CertFile != "" && cfg.KeyFile != "" {
		log.Printf("TLS: using manual certificates")
		return &Result{
			CertFile: cfg.CertFile,
			KeyFile:  cfg.KeyFile,
		}, nil
	}

	// Priority 2: Let's Encrypt auto-cert
	if cfg.AutoCert.Domain != "" {
		if cfg.AutoCert.DNSProvider != "" {
			return resolveDNS(cfg)
		}
		log.Printf("TLS: using Let's Encrypt (HTTP-01) for %s", cfg.AutoCert.Domain)
		m := &autocert.Manager{
			Cache:      autocert.DirCache(cfg.AutoCert.CacheDir),
			Prompt:     autocert.AcceptTOS,
			HostPolicy: autocert.HostWhitelist(cfg.AutoCert.Domain),
			Email:      cfg.AutoCert.Email,
		}
		return &Result{
			TLSConfig: m.TLSConfig(),
		}, nil
	}

	// Priority 3: Self-signed (default)
	log.Printf("TLS: generating self-signed certificate")
	log.Printf("WARNING: Self-signed certificates are not suitable for production. Configure cert_file/key_file or auto_cert.domain.")
	certFile, keyFile, err := EnsureSelfSigned(cfg.AutoCert.CacheDir)
	if err != nil {
		return nil, fmt.Errorf("self-signed cert: %w", err)
	}

	return &Result{
		CertFile: certFile,
		KeyFile:  keyFile,
	}, nil
}
