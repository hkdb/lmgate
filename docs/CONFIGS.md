# LM Gate Configuration Reference

LM Gate uses a single `config.yaml` file. Every setting can also be overridden with a `LMGATE_`-prefixed environment variable.

## Full `config.yaml` Example

```yaml
server:
  listen: ":443"
  read_timeout: 30s
  write_timeout: 120s
  allowed_origins: ""       # CORS origins (comma-separated), empty = same-origin only
  allowed_hosts: []         # Allowed Host header values for HTTP->HTTPS redirect
  tls:
    disabled: false
    cert_file: ""
    key_file: ""
    auto_cert:
      domain: ""
      cache_dir: "./data/certs"
      email: ""
      dns_provider: ""            # "cloudflare" for DNS-01 challenge
      cloudflare_api_token: ""    # Cloudflare API token (Zone:DNS:Edit)

upstream:
  url: "http://localhost:11434"
  type: "ollama"              # "ollama", "llama.cpp", "lm-studio", "bitnet", or "" (generic)
  timeout: 120s

database:
  path: "./data/lmgate.db"

auth:
  jwt_secret: ""          # required — signing key for JWTs
  jwt_expiry: "24h"
  admin_email: ""         # email for the initial admin user

oidc:
  providers: []

rate_limit:
  default_rpm: 60
  enabled: true

logging:
  level: "info"
  api_log_enabled: true
  api_log_retention_days: 90
  admin_log_enabled: true
  admin_log_retention_days: 30
  security_log_enabled: true
  security_log_retention_days: 180
  audit_flush_interval: 5

metrics:
  flush_interval: 30s

telemetry:
  disabled: false            # set true to opt out of one time install count ping

security:
  max_failed_logins: 5
  password_min_length: 12
  password_require_special: true
  password_require_number: true
  user_cache_ttl: 30              # seconds
  request_body_limit: 10          # MB
  response_body_limit: 100        # MB
  security_headers_enabled: true
  header_x_frame_options: "DENY"
  header_csp: "default-src 'self'; ..."
  header_referrer_policy: "strict-origin-when-cross-origin"
  header_xss_protection: "1; mode=block"
  hsts_max_age: 31536000
  encryption_key: ""            # required — key for encrypting secrets (TOTP, OIDC)
  admin_allowed_networks: ""    # comma-separated IPs/CIDRs

webauthn:
  rp_display_name: "LM Gate"
  rp_id: ""               # e.g. "example.com" — required to enable passkey support
  rp_origins: []           # e.g. ["https://example.com"] — auto-derived from rp_id if unset
```

## Runtime vs Static Settings

Some security settings can be changed at runtime through the Settings page (no restart required):

| Setting | Default | Description |
|---------|---------|-------------|
| `max_failed_logins` | 5 | Failed login attempts before account lockout |
| `password_min_length` | 12 | Minimum password length |
| `password_require_special` | true | Require at least one special character |
| `password_require_number` | true | Require at least one digit |
| `user_cache_ttl` | 30 | User cache TTL in seconds |
| `admin_allowed_networks` | (empty) | Restrict /admin to these IPs/CIDRs (comma-separated) |
| `audit_flush_interval` | 5 | Audit log batch flush interval in seconds |

Static settings require a restart to take effect:

| Setting | Default | Description |
|---------|---------|-------------|
| `request_body_limit` | 10 | Max request body size (MB) |
| `response_body_limit` | 100 | Max response body size (MB) |
| `security_headers_enabled` | true | Enable security response headers |
| `header_x_frame_options` | DENY | X-Frame-Options header value |
| `header_csp` | (see config) | Content-Security-Policy header |
| `header_referrer_policy` | strict-origin-when-cross-origin | Referrer-Policy header |
| `header_xss_protection` | 1; mode=block | X-XSS-Protection header |
| `hsts_max_age` | 31536000 | HSTS max-age in seconds |
| `encryption_key` | (none) | Encryption key for secrets (required, min 32 chars) |

## `.env` File

LM Gate automatically loads a `.env` file from the working directory if one exists. Values in `.env` do **not** override environment variables that are already set, so explicit `export` or Docker `ENV` always takes precedence.

```bash
# .env (minimum required)
LMGATE_AUTH_ADMIN_EMAIL=admin@example.com
LMGATE_AUTH_JWT_SECRET=my-secret-key
LMGATE_ENCRYPTION_KEY=your-encryption-key
LMGATE_UPSTREAM_URL=http://localhost:11434
LMGATE_WEBAUTHN_RP_ID=domain.name
LMGATE_WEBAUTHN_RP_DISPLAY_NAME=LM Gate
LMGATE_WEBAUTHN_RP_ORIGINS=https://domain.name
# LMGATE_TELEMETRY_DISABLED=true
```

## Key Environment Variables

| Variable | Purpose |
|----------|---------|
| `LMGATE_AUTH_JWT_SECRET` | JWT signing key (required) |
| `LMGATE_ENCRYPTION_KEY` | Encryption key for secrets — TOTP, OIDC (required, min 32 chars) |
| `LMGATE_AUTH_ADMIN_EMAIL` | Initial admin email (required) |
| `LMGATE_UPSTREAM_URL` | Upstream LLM service URL |
| `LMGATE_UPSTREAM_TYPE` | Upstream type: `ollama`, `llama.cpp`, `lm-studio`, `bitnet`, or empty (generic) |
| `LMGATE_TLS_DISABLED` | Set `true` to run plain HTTP |
| `LMGATE_LISTEN` | Listen on a specific port |
| `LMGATE_DATABASE_PATH` | SQLite database path |
| `LMGATE_ALLOWED_ORIGINS` | CORS origins (comma-separated) |
| `LMGATE_ALLOWED_HOSTS` | Allowed Host header values for HTTP->HTTPS redirect (comma-separated) |
| `LMGATE_AUTH_JWT_EXPIRY` | JWT token expiry duration (e.g. `24h`) |
| `LMGATE_UPSTREAM_TIMEOUT` | Upstream request timeout (e.g. `120s`) |
| `LMGATE_RATE_LIMIT_ENABLED` | Enable/disable rate limiting |
| `LMGATE_RATE_LIMIT_DEFAULT_RPM` | Default requests per minute |
| `LMGATE_LOG_LEVEL` | Log level (e.g. `info`, `debug`) |
| `LMGATE_API_LOG_ENABLED` | Enable/disable API log writing |
| `LMGATE_API_LOG_RETENTION_DAYS` | API log retention in days (0 = keep forever) |
| `LMGATE_ADMIN_LOG_ENABLED` | Enable/disable admin log writing |
| `LMGATE_ADMIN_LOG_RETENTION_DAYS` | Admin log retention in days (0 = keep forever) |
| `LMGATE_SECURITY_LOG_ENABLED` | Enable/disable security log writing |
| `LMGATE_SECURITY_LOG_RETENTION_DAYS` | Security log retention in days (0 = keep forever) |
| `LMGATE_AUDIT_FLUSH_INTERVAL` | Audit log batch flush interval in seconds (default: 5) |
| `LMGATE_METRICS_FLUSH_INTERVAL` | Metrics flush interval (e.g. `30s`) |
| `LMGATE_MAX_FAILED_LOGINS` | Max failed login attempts before lockout |
| `LMGATE_PASSWORD_MIN_LENGTH` | Minimum password length |
| `LMGATE_PASSWORD_REQUIRE_SPECIAL` | Require special character in passwords |
| `LMGATE_PASSWORD_REQUIRE_NUMBER` | Require number in passwords |
| `LMGATE_USER_CACHE_TTL` | User cache TTL in seconds |
| `LMGATE_REQUEST_BODY_LIMIT` | Max request body size in MB |
| `LMGATE_RESPONSE_BODY_LIMIT` | Max response body size in MB |
| `LMGATE_SECURITY_HEADERS_ENABLED` | Enable/disable security headers |
| `LMGATE_HEADER_X_FRAME_OPTIONS` | X-Frame-Options header value |
| `LMGATE_HEADER_CSP` | Content-Security-Policy header value |
| `LMGATE_HEADER_REFERRER_POLICY` | Referrer-Policy header value |
| `LMGATE_HEADER_XSS_PROTECTION` | X-XSS-Protection header value |
| `LMGATE_HSTS_MAX_AGE` | HSTS max-age in seconds |
| `LMGATE_ADMIN_ALLOWED_NETWORKS` | Restrict /admin to these IPs/CIDRs (comma-separated, empty = unrestricted) |
| `LMGATE_TLS_CERT_FILE` | TLS certificate file path |
| `LMGATE_TLS_KEY_FILE` | TLS key file path |
| `LMGATE_TLS_AUTOCERT_DOMAIN` | Let's Encrypt auto-cert domain |
| `LMGATE_TLS_AUTOCERT_CACHE_DIR` | Auto-cert cache directory |
| `LMGATE_TLS_AUTOCERT_EMAIL` | Auto-cert registration email |
| `LMGATE_TLS_AUTOCERT_DNS_PROVIDER` | DNS-01 challenge provider (e.g. `cloudflare`) |
| `LMGATE_TLS_AUTOCERT_CF_API_TOKEN` | Cloudflare API token for DNS-01 (`Zone:DNS:Edit`) |
| `LMGATE_WEBAUTHN_RP_ID` | WebAuthn Relying Party domain (e.g. `example.com`). Required to enable passkey support |
| `LMGATE_WEBAUTHN_RP_DISPLAY_NAME` | WebAuthn display name (defaults to `LM Gate`) |
| `LMGATE_WEBAUTHN_RP_ORIGINS` | Comma-separated allowed origins (auto-derived from RP ID if unset) |
| `LMGATE_TELEMETRY_DISABLED` | Disable anonymous install telemetry (`true` to opt out) |

## Bootstrap Admin & Force Password Change

When LM Gate starts with an empty database, it creates a bootstrap admin user and logs the credentials to stdout. The bootstrap admin is required to change their password on first login.

Admins can also set the `force_password_change` flag on any user when creating or updating them via the API or dashboard. The user will be redirected to a password change form on their next login and cannot access other pages until they comply.
