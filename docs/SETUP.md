# LM Gate Development Setup Guide

## Prerequisites

- Go 1.22+
- Node.js 20+ (for the admin dashboard)
- An LLM backend (e.g. [Ollama](https://ollama.com)) running and accessible

## 1. Install dependencies and test build

```bash
make web-install
make build
```

This builds the SvelteKit admin dashboard and compiles the Go binary.

Or if you prefer to use docker to build and test, see the [docker section](DOCKERBUILD.md).

## 2. Configure Environment

Create a `.env` file in the project root. lmgate automatically loads it on startup. Values in `.env` do **not** override environment variables already set in your shell or container, so explicit `export` or Docker `ENV` always takes precedence.

```bash
# Required — JWT signing key (use a long random string)
LMGATE_AUTH_JWT_SECRET=your-secret-key-here

# Required — encryption key for secrets (use a different long random string)
LMGATE_ENCRYPTION_KEY=your-encryption-key-here

# Disable TLS for local development
LMGATE_TLS_DISABLED=true

# Listen on a non-privileged port (ports below 1024 require root)
LMGATE_LISTEN=:8080

# Upstream LLM service (default: http://localhost:11434)
LMGATE_UPSTREAM_URL=http://localhost:11434

# Email for the initial admin user (used with OIDC login)
LMGATE_AUTH_ADMIN_EMAIL=admin@example.com

# WebAuthn Relying Party settings (required for passkey/2FA support)
LMGATE_WEBAUTHN_RP_ID=localhost
LMGATE_WEBAUTHN_RP_DISPLAY_NAME=LM Gate
LMGATE_WEBAUTHN_RP_ORIGINS=http://localhost:8080

# One time install count telemetry ping (optional, enabled by default)
# LMGATE_TELEMETRY_DISABLED=true
```

See [CONFIGS.md](CONFIGS.md) for the full list of environment variables.

## 3. Start the Server

```bash
./lmgate
```

Or for development (TLS disabled, auto-rebuild):

```bash
make dev
```

The admin dashboard is available at `http://localhost:8080/admin`.

## 4. First Admin User

On first startup, if no admin user exists, lmgate automatically creates one and logs the credentials:

```
BOOTSTRAP: Admin user created
  Email:    admin@lmgate.local
  Password: <randomly-generated>
Please change this password after logging in.
```

To customize the admin email, set `LMGATE_AUTH_ADMIN_EMAIL` before starting:

```bash
LMGATE_AUTH_ADMIN_EMAIL=you@example.com ./lmgate
```

The bootstrap only runs once — subsequent restarts skip it if an admin already exists.

After logging in at `http://localhost:8080/admin/login`, change the generated password via **Users > Edit** in the admin dashboard.

## 5. TLS Configuration

For production, configure one of the following TLS modes:

### Certificate Files

```yaml
server:
  tls:
    cert_file: "/path/to/cert.pem"
    key_file: "/path/to/key.pem"
```

### Let's Encrypt (Automatic)

```yaml
server:
  tls:
    auto_cert:
      domain: "lmgate.example.com"
      email: "you@example.com"
```

### Let's Encrypt via DNS-01 (Cloudflare)

For hosts behind firewalls or private networks where port 80 is not reachable:

```yaml
server:
  tls:
    auto_cert:
      domain: "lmgate.example.com"
      email: "you@example.com"
      dns_provider: "cloudflare"
      cloudflare_api_token: "your-cf-api-token"
```

Or via environment variables:

```bash
LMGATE_TLS_AUTOCERT_DNS_PROVIDER=cloudflare
LMGATE_TLS_AUTOCERT_CF_API_TOKEN=your-cf-api-token
```

The API token needs `Zone:DNS:Edit` permission for the relevant zone in Cloudflare.

### Disabled (Behind a Reverse Proxy)

```bash
LMGATE_TLS_DISABLED=true
```

See [CONFIGS.md](CONFIGS.md) for the full list of TLS-related environment variables.

## 6. Verify

Once running, confirm the proxy works:

```bash
# Health check (through the proxy — requires auth)
curl -H "Authorization: Bearer <your-token>" http://localhost:8080/api/tags

# Admin dashboard
open http://localhost:8080/admin
```

## Other `make` Targets

| Make Target | Description |
|-------------|-------------|
| `make build` | Build web frontend + Go binary |
| `make build-go` | Build Go binary only (web must already be built) |
| `make web` | Build the SvelteKit admin dashboard |
| `make dev` | Run with TLS disabled for local development |
| `make test` | Run Go tests |
| `make docker` | Build Docker image |
| `make docker-omni` | Build Omnigate Docker image (CPU only) |
| `make docker-omni-nvidia` | Build Omnigate Docker image (NVIDIA GPU) |
| `make docker-omni-amd` | Build Omnigate Docker image (AMD GPU) |
| `make docker-omni-intel` | Build Omnigate Docker image (Intel iGPU — Experimental) |
| `make clean` | Remove build artifacts |


