package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Server    ServerConfig    `yaml:"server"`
	Upstream  UpstreamConfig  `yaml:"upstream"`
	Database  DatabaseConfig  `yaml:"database"`
	Auth      AuthConfig      `yaml:"auth"`
	OIDC      OIDCConfig      `yaml:"oidc"`
	WebAuthn  WebAuthnConfig  `yaml:"webauthn"`
	RateLimit RateLimitConfig `yaml:"rate_limit"`
	Logging   LoggingConfig   `yaml:"logging"`
	Metrics   MetricsConfig   `yaml:"metrics"`
	Security  SecurityConfig  `yaml:"security"`
	Telemetry TelemetryConfig `yaml:"telemetry"`
}

type TelemetryConfig struct {
	Disabled bool `yaml:"disabled"`
}

type WebAuthnConfig struct {
	RPDisplayName string   `yaml:"rp_display_name"`
	RPID          string   `yaml:"rp_id"`
	RPOrigins     []string `yaml:"rp_origins"`
}

type SecurityConfig struct {
	MaxFailedLogins        int    `yaml:"max_failed_logins"`
	PasswordMinLength      int    `yaml:"password_min_length"`
	PasswordRequireSpecial bool   `yaml:"password_require_special"`
	PasswordRequireNumber  bool   `yaml:"password_require_number"`
	UserCacheTTL           int    `yaml:"user_cache_ttl"`
	Enforce2FA             bool   `yaml:"enforce_2fa"`
	PasswordExpiryDays     int    `yaml:"password_expiry_days"`
	RequestBodyLimit       int    `yaml:"request_body_limit"`
	ResponseBodyLimit      int    `yaml:"response_body_limit"`
	HeaderXFrameOptions    string `yaml:"header_x_frame_options"`
	HeaderCSP              string `yaml:"header_csp"`
	HeaderReferrerPolicy   string `yaml:"header_referrer_policy"`
	HeaderXSSProtection    string `yaml:"header_xss_protection"`
	HSTSMaxAge             int    `yaml:"hsts_max_age"`
	SecurityHeadersEnabled bool   `yaml:"security_headers_enabled"`
	AdminAllowedNetworks   string `yaml:"admin_allowed_networks"`
	EncryptionKey          string `yaml:"encryption_key"`
}

type ServerConfig struct {
	Listen         string        `yaml:"listen"`
	ReadTimeout    time.Duration `yaml:"read_timeout"`
	WriteTimeout   time.Duration `yaml:"write_timeout"`
	AllowedOrigins string        `yaml:"allowed_origins"`
	AllowedHosts   []string      `yaml:"allowed_hosts"`
	TLS            TLSConfig     `yaml:"tls"`
}

type TLSConfig struct {
	Disabled bool           `yaml:"disabled"`
	CertFile string         `yaml:"cert_file"`
	KeyFile  string         `yaml:"key_file"`
	AutoCert AutoCertConfig `yaml:"auto_cert"`
}

type AutoCertConfig struct {
	Domain             string `yaml:"domain"`
	CacheDir           string `yaml:"cache_dir"`
	Email              string `yaml:"email"`
	DNSProvider        string `yaml:"dns_provider"`
	CloudflareAPIToken string `yaml:"cloudflare_api_token"`
}

type UpstreamConfig struct {
	URL     string        `yaml:"url"`
	Timeout time.Duration `yaml:"timeout"`
	Type    string        `yaml:"type"` // "ollama", "llama.cpp", "lm-studio", "bitnet", or "" (generic)
}

type DatabaseConfig struct {
	Path string `yaml:"path"`
}

type AuthConfig struct {
	JWTSecret  string        `yaml:"jwt_secret"`
	JWTExpiry  time.Duration `yaml:"jwt_expiry"`
	AdminEmail string        `yaml:"admin_email"`
}

type OIDCConfig struct {
	Providers []OIDCProviderConfig `yaml:"providers"`
}

type OIDCProviderConfig struct {
	ProviderType string `yaml:"provider_type"`
	ClientID     string `yaml:"client_id"`
	ClientSecret string `yaml:"client_secret"`
	IssuerURL    string `yaml:"issuer_url"`
	Enabled      bool   `yaml:"enabled"`
}

type RateLimitConfig struct {
	DefaultRPM int  `yaml:"default_rpm"`
	Enabled    bool `yaml:"enabled"`
}

type LoggingConfig struct {
	Level                    string `yaml:"level"`
	APILogEnabled            bool   `yaml:"api_log_enabled"`
	APILogRetentionDays      int    `yaml:"api_log_retention_days"`
	AdminLogEnabled          bool   `yaml:"admin_log_enabled"`
	AdminLogRetentionDays    int    `yaml:"admin_log_retention_days"`
	SecurityLogEnabled       bool   `yaml:"security_log_enabled"`
	SecurityLogRetentionDays int    `yaml:"security_log_retention_days"`
	AuditFlushInterval       int    `yaml:"audit_flush_interval"` // seconds
}

type MetricsConfig struct {
	FlushInterval time.Duration `yaml:"flush_interval"`
}

func Load(path string) (*Config, error) {
	cfg := defaults()

	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("reading config: %w", err)
	}

	if err == nil {
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("parsing config: %w", err)
		}
	}

	_ = godotenv.Load() // optional .env file; does not overwrite existing env vars
	applyEnvOverrides(cfg)

	if err := validate(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

func defaults() *Config {
	return &Config{
		Server: ServerConfig{
			Listen:       "",
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 30 * time.Minute,
			TLS: TLSConfig{
				AutoCert: AutoCertConfig{
					CacheDir: "./data/certs",
				},
			},
		},
		Upstream: UpstreamConfig{
			URL:     "http://localhost:11434",
			Timeout: 120 * time.Second,
			Type:    "ollama",
		},
		Database: DatabaseConfig{
			Path: "./data/lmgate.db",
		},
		Auth: AuthConfig{
			JWTExpiry: 24 * time.Hour,
		},
		RateLimit: RateLimitConfig{
			DefaultRPM: 60,
			Enabled:    true,
		},
		Logging: LoggingConfig{
			Level:                    "info",
			APILogEnabled:            true,
			APILogRetentionDays:      90,
			AdminLogEnabled:          true,
			AdminLogRetentionDays:    30,
			SecurityLogEnabled:       true,
			SecurityLogRetentionDays: 180,
			AuditFlushInterval:       5,
		},
		Metrics: MetricsConfig{
			FlushInterval: 30 * time.Second,
		},
		Security: SecurityConfig{
			MaxFailedLogins:        5,
			PasswordMinLength:      12,
			PasswordRequireSpecial: true,
			PasswordRequireNumber:  true,
			UserCacheTTL:           30,
			RequestBodyLimit:       10,
			ResponseBodyLimit:      100,
			HeaderXFrameOptions:    "DENY",
			HeaderCSP:              "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self' data:",
			HeaderReferrerPolicy:   "strict-origin-when-cross-origin",
			HeaderXSSProtection:    "1; mode=block",
			HSTSMaxAge:             31536000,
			SecurityHeadersEnabled: true,
		},
	}
}

func applyEnvOverrides(cfg *Config) {
	if v := os.Getenv("LMGATE_LISTEN"); v != "" {
		cfg.Server.Listen = v
	}
	if v := os.Getenv("LMGATE_TLS_DISABLED"); v != "" {
		cfg.Server.TLS.Disabled = strings.EqualFold(v, "true") || v == "1"
	}
	if v := os.Getenv("LMGATE_TLS_CERT_FILE"); v != "" {
		cfg.Server.TLS.CertFile = v
	}
	if v := os.Getenv("LMGATE_TLS_KEY_FILE"); v != "" {
		cfg.Server.TLS.KeyFile = v
	}
	if v := os.Getenv("LMGATE_TLS_AUTOCERT_DOMAIN"); v != "" {
		cfg.Server.TLS.AutoCert.Domain = v
	}
	if v := os.Getenv("LMGATE_TLS_AUTOCERT_CACHE_DIR"); v != "" {
		cfg.Server.TLS.AutoCert.CacheDir = v
	}
	if v := os.Getenv("LMGATE_TLS_AUTOCERT_EMAIL"); v != "" {
		cfg.Server.TLS.AutoCert.Email = v
	}
	if v := os.Getenv("LMGATE_TLS_AUTOCERT_DNS_PROVIDER"); v != "" {
		cfg.Server.TLS.AutoCert.DNSProvider = v
	}
	if v := os.Getenv("LMGATE_TLS_AUTOCERT_CF_API_TOKEN"); v != "" {
		cfg.Server.TLS.AutoCert.CloudflareAPIToken = v
	}
	if v := os.Getenv("LMGATE_ALLOWED_ORIGINS"); v != "" {
		cfg.Server.AllowedOrigins = v
	}
	if v := os.Getenv("LMGATE_ALLOWED_HOSTS"); v != "" {
		cfg.Server.AllowedHosts = strings.Split(v, ",")
	}
	if v := os.Getenv("LMGATE_UPSTREAM_URL"); v != "" {
		cfg.Upstream.URL = v
	}
	if v := os.Getenv("LMGATE_UPSTREAM_TYPE"); v != "" {
		cfg.Upstream.Type = v
	}
	if v := os.Getenv("LMGATE_UPSTREAM_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.Upstream.Timeout = d
		}
	}
	if v := os.Getenv("LMGATE_DATABASE_PATH"); v != "" {
		cfg.Database.Path = v
	}
	if v := os.Getenv("LMGATE_AUTH_JWT_SECRET"); v != "" {
		cfg.Auth.JWTSecret = v
	}
	if v := os.Getenv("LMGATE_AUTH_JWT_EXPIRY"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.Auth.JWTExpiry = d
		}
	}
	if v := os.Getenv("LMGATE_AUTH_ADMIN_EMAIL"); v != "" {
		cfg.Auth.AdminEmail = v
	}
	if v := os.Getenv("LMGATE_RATE_LIMIT_DEFAULT_RPM"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.RateLimit.DefaultRPM = n
		}
	}
	if v := os.Getenv("LMGATE_RATE_LIMIT_ENABLED"); v != "" {
		cfg.RateLimit.Enabled = strings.EqualFold(v, "true") || v == "1"
	}
	if v := os.Getenv("LMGATE_LOG_LEVEL"); v != "" {
		cfg.Logging.Level = v
	}
	if v := os.Getenv("LMGATE_API_LOG_ENABLED"); v != "" {
		cfg.Logging.APILogEnabled = strings.EqualFold(v, "true") || v == "1"
	}
	if v := os.Getenv("LMGATE_API_LOG_RETENTION_DAYS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Logging.APILogRetentionDays = n
		}
	}
	if v := os.Getenv("LMGATE_ADMIN_LOG_ENABLED"); v != "" {
		cfg.Logging.AdminLogEnabled = strings.EqualFold(v, "true") || v == "1"
	}
	if v := os.Getenv("LMGATE_ADMIN_LOG_RETENTION_DAYS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Logging.AdminLogRetentionDays = n
		}
	}
	if v := os.Getenv("LMGATE_SECURITY_LOG_ENABLED"); v != "" {
		cfg.Logging.SecurityLogEnabled = strings.EqualFold(v, "true") || v == "1"
	}
	if v := os.Getenv("LMGATE_SECURITY_LOG_RETENTION_DAYS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Logging.SecurityLogRetentionDays = n
		}
	}
	if v := os.Getenv("LMGATE_AUDIT_FLUSH_INTERVAL"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Logging.AuditFlushInterval = n
		}
	}
	if v := os.Getenv("LMGATE_METRICS_FLUSH_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.Metrics.FlushInterval = d
		}
	}
	if v := os.Getenv("LMGATE_MAX_FAILED_LOGINS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Security.MaxFailedLogins = n
		}
	}
	if v := os.Getenv("LMGATE_PASSWORD_MIN_LENGTH"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Security.PasswordMinLength = n
		}
	}
	if v := os.Getenv("LMGATE_PASSWORD_REQUIRE_SPECIAL"); v != "" {
		cfg.Security.PasswordRequireSpecial = strings.EqualFold(v, "true") || v == "1"
	}
	if v := os.Getenv("LMGATE_PASSWORD_REQUIRE_NUMBER"); v != "" {
		cfg.Security.PasswordRequireNumber = strings.EqualFold(v, "true") || v == "1"
	}
	if v := os.Getenv("LMGATE_USER_CACHE_TTL"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Security.UserCacheTTL = n
		}
	}
	if v := os.Getenv("LMGATE_REQUEST_BODY_LIMIT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Security.RequestBodyLimit = n
		}
	}
	if v := os.Getenv("LMGATE_RESPONSE_BODY_LIMIT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Security.ResponseBodyLimit = n
		}
	}
	if v := os.Getenv("LMGATE_SECURITY_HEADERS_ENABLED"); v != "" {
		cfg.Security.SecurityHeadersEnabled = strings.EqualFold(v, "true") || v == "1"
	}
	if v := os.Getenv("LMGATE_HEADER_X_FRAME_OPTIONS"); v != "" {
		cfg.Security.HeaderXFrameOptions = v
	}
	if v := os.Getenv("LMGATE_HEADER_CSP"); v != "" {
		cfg.Security.HeaderCSP = v
	}
	if v := os.Getenv("LMGATE_HEADER_REFERRER_POLICY"); v != "" {
		cfg.Security.HeaderReferrerPolicy = v
	}
	if v := os.Getenv("LMGATE_HEADER_XSS_PROTECTION"); v != "" {
		cfg.Security.HeaderXSSProtection = v
	}
	if v := os.Getenv("LMGATE_HSTS_MAX_AGE"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Security.HSTSMaxAge = n
		}
	}
	if v := os.Getenv("LMGATE_ADMIN_ALLOWED_NETWORKS"); v != "" {
		cfg.Security.AdminAllowedNetworks = v
	}
	if v := os.Getenv("LMGATE_ENCRYPTION_KEY"); v != "" {
		cfg.Security.EncryptionKey = v
	}
	if v := os.Getenv("LMGATE_WEBAUTHN_RP_DISPLAY_NAME"); v != "" {
		cfg.WebAuthn.RPDisplayName = v
	}
	if v := os.Getenv("LMGATE_WEBAUTHN_RP_ID"); v != "" {
		cfg.WebAuthn.RPID = v
	}
	if v := os.Getenv("LMGATE_WEBAUTHN_RP_ORIGINS"); v != "" {
		cfg.WebAuthn.RPOrigins = strings.Split(v, ",")
	}
	if v := os.Getenv("LMGATE_TELEMETRY_DISABLED"); v != "" {
		cfg.Telemetry.Disabled = strings.EqualFold(v, "true") || v == "1"
	}
}

func validate(cfg *Config) error {
	if cfg.Auth.JWTSecret == "" {
		return fmt.Errorf("auth.jwt_secret is required (set LMGATE_AUTH_JWT_SECRET)")
	}
	if len(cfg.Auth.JWTSecret) < 32 {
		return fmt.Errorf("auth.jwt_secret must be at least 32 characters")
	}
	if cfg.Security.EncryptionKey == "" {
		return fmt.Errorf("security.encryption_key is required (set LMGATE_ENCRYPTION_KEY)")
	}
	if len(cfg.Security.EncryptionKey) < 32 {
		return fmt.Errorf("security.encryption_key must be at least 32 characters")
	}
	if cfg.Upstream.URL == "" {
		return fmt.Errorf("upstream.url is required")
	}
	return nil
}
