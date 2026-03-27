package tls

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/go-acme/lego/v4/certcrypto"
	"github.com/go-acme/lego/v4/certificate"
	"github.com/go-acme/lego/v4/lego"
	"github.com/go-acme/lego/v4/providers/dns/cloudflare"
	"github.com/go-acme/lego/v4/registration"

	"github.com/hkdb/lmgate/internal/config"
)

// acmeUser implements registration.User for the lego ACME client.
type acmeUser struct {
	email        string
	registration *registration.Resource
	key          crypto.PrivateKey
}

func (u *acmeUser) GetEmail() string                        { return u.email }
func (u *acmeUser) GetRegistration() *registration.Resource { return u.registration }
func (u *acmeUser) GetPrivateKey() crypto.PrivateKey        { return u.key }

func resolveDNS(cfg config.TLSConfig) (*Result, error) {
	if cfg.AutoCert.DNSProvider != "cloudflare" {
		return nil, fmt.Errorf("unsupported dns_provider %q (supported: cloudflare)", cfg.AutoCert.DNSProvider)
	}
	if cfg.AutoCert.CloudflareAPIToken == "" {
		return nil, fmt.Errorf("cloudflare_api_token is required when dns_provider is cloudflare")
	}

	log.Printf("TLS: using Let's Encrypt (DNS-01 via Cloudflare) for %s", cfg.AutoCert.Domain)

	cacheDir := cfg.AutoCert.CacheDir
	if err := os.MkdirAll(cacheDir, 0700); err != nil {
		return nil, fmt.Errorf("creating cache dir: %w", err)
	}

	certFile := filepath.Join(cacheDir, "dns-cert.pem")
	keyFile := filepath.Join(cacheDir, "dns-key.pem")

	// Check for existing valid certificate.
	if cert, err := tls.LoadX509KeyPair(certFile, keyFile); err == nil {
		if leaf, err := x509.ParseCertificate(cert.Certificate[0]); err == nil {
			if time.Until(leaf.NotAfter) > 30*24*time.Hour {
				log.Printf("TLS: existing DNS-01 certificate valid until %s", leaf.NotAfter.Format(time.DateOnly))
				startRenewalLoop(cfg, certFile, keyFile)
				return &Result{CertFile: certFile, KeyFile: keyFile}, nil
			}
			log.Printf("TLS: existing certificate expires soon (%s), renewing", leaf.NotAfter.Format(time.DateOnly))
		}
	}

	// Obtain a new certificate.
	if err := obtainCert(cfg, certFile, keyFile); err != nil {
		return nil, err
	}

	startRenewalLoop(cfg, certFile, keyFile)
	return &Result{CertFile: certFile, KeyFile: keyFile}, nil
}

func obtainCert(cfg config.TLSConfig, certFile, keyFile string) error {
	user, err := loadOrCreateAccount(cfg)
	if err != nil {
		return fmt.Errorf("ACME account: %w", err)
	}

	legoCfg := lego.NewConfig(user)
	legoCfg.Certificate.KeyType = certcrypto.RSA2048

	client, err := lego.NewClient(legoCfg)
	if err != nil {
		return fmt.Errorf("lego client: %w", err)
	}

	cfCfg := cloudflare.NewDefaultConfig()
	cfCfg.AuthToken = cfg.AutoCert.CloudflareAPIToken
	provider, err := cloudflare.NewDNSProviderConfig(cfCfg)
	if err != nil {
		return fmt.Errorf("cloudflare provider: %w", err)
	}
	if err := client.Challenge.SetDNS01Provider(provider); err != nil {
		return fmt.Errorf("setting DNS-01 provider: %w", err)
	}

	// Register if needed.
	if user.registration == nil {
		reg, err := client.Registration.Register(registration.RegisterOptions{TermsOfServiceAgreed: true})
		if err != nil {
			return fmt.Errorf("ACME registration: %w", err)
		}
		user.registration = reg
	}

	request := certificate.ObtainRequest{
		Domains: []string{cfg.AutoCert.Domain},
		Bundle:  true,
	}
	certificates, err := client.Certificate.Obtain(request)
	if err != nil {
		return fmt.Errorf("obtaining certificate: %w", err)
	}

	if err := os.WriteFile(certFile, certificates.Certificate, 0600); err != nil {
		return fmt.Errorf("writing cert: %w", err)
	}
	if err := os.WriteFile(keyFile, certificates.PrivateKey, 0600); err != nil {
		return fmt.Errorf("writing key: %w", err)
	}

	log.Printf("TLS: DNS-01 certificate obtained for %s", cfg.AutoCert.Domain)
	return nil
}

func loadOrCreateAccount(cfg config.TLSConfig) (*acmeUser, error) {
	accountKeyPath := filepath.Join(cfg.AutoCert.CacheDir, "acme-account.pem")

	var privKey *ecdsa.PrivateKey
	if data, err := os.ReadFile(accountKeyPath); err == nil {
		block, _ := pem.Decode(data)
		if block == nil {
			return nil, fmt.Errorf("invalid PEM in account key file")
		}
		key, err := x509.ParseECPrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parsing account key: %w", err)
		}
		privKey = key
	}

	if privKey == nil {
		var err error
		privKey, err = ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			return nil, fmt.Errorf("generating account key: %w", err)
		}
		keyBytes, err := x509.MarshalECPrivateKey(privKey)
		if err != nil {
			return nil, fmt.Errorf("marshaling account key: %w", err)
		}
		pemData := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyBytes})
		if err := os.WriteFile(accountKeyPath, pemData, 0600); err != nil {
			return nil, fmt.Errorf("saving account key: %w", err)
		}
	}

	return &acmeUser{
		email: cfg.AutoCert.Email,
		key:   privKey,
	}, nil
}

func startRenewalLoop(cfg config.TLSConfig, certFile, keyFile string) {
	go func() {
		ticker := time.NewTicker(12 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			cert, err := tls.LoadX509KeyPair(certFile, keyFile)
			if err != nil {
				log.Printf("TLS renewal: failed to load cert: %v", err)
				continue
			}
			leaf, err := x509.ParseCertificate(cert.Certificate[0])
			if err != nil {
				log.Printf("TLS renewal: failed to parse cert: %v", err)
				continue
			}
			if time.Until(leaf.NotAfter) > 30*24*time.Hour {
				continue
			}
			log.Printf("TLS renewal: certificate expires %s, renewing", leaf.NotAfter.Format(time.DateOnly))
			if err := obtainCert(cfg, certFile, keyFile); err != nil {
				log.Printf("TLS renewal: failed: %v", err)
			}
		}
	}()
}
