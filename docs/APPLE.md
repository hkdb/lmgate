# macOS / Apple Silicon Guide

Omnigate images are **Linux-only** — they bundle Ollama inside a Docker container, which on macOS runs in a Linux VM without access to the Apple Silicon GPU (Metal). This means Ollama inside Docker on a Mac is limited to CPU-only inference.

For the best performance on Apple Silicon, run Ollama **natively** on macOS to leverage Metal GPU acceleration, and connect LM Gate to it.

## Recommended Setup

### 1. Install Ollama

Download and install Ollama from [ollama.com](https://ollama.com). It runs natively on macOS and automatically uses the Apple Silicon GPU.

### 2. Deploy LM Gate

You have two options for running LM Gate alongside your native Ollama:

#### Option A: Docker Standalone (Recommended)

1. Clone repo: `git clone https://github.com/hkdb/lmgate.git`
2. Go into the repo: `cd lmgate`
3. Create .env with sample: `cp .env.sample .env`
4. Fill out the .env:

   ```bash
   # Required — JWT signing key (use a long random string)
   LMGATE_AUTH_JWT_SECRET=your-secret-key-here

   # Required — encryption key for secrets (use a different long random string)
   LMGATE_ENCRYPTION_KEY=your-encryption-key-here
   # Disable TLS if you want LM Gate to be behind a proxy that doesn't support TLS
   LMGATE_TLS_DISABLED=false

   # Point to Ollama running on the macOS host
   LMGATE_UPSTREAM_URL=http://host.docker.internal:11434

   # Email for the initial admin user (used with OIDC login)
   LMGATE_AUTH_ADMIN_EMAIL=admin@example.com

   # WebAuthn Relying Party settings (required for passkey/2FA support)
   LMGATE_WEBAUTHN_RP_ID=localhost
   LMGATE_WEBAUTHN_RP_DISPLAY_NAME=LM Gate
   LMGATE_WEBAUTHN_RP_ORIGINS=http://localhost:8080

   # One time install count telemetry ping (optional, enabled by default)
   # LMGATE_TELEMETRY_DISABLED=true
   ```

   **Important:** Use `http://host.docker.internal:11434` as the upstream URL. This is Docker Desktop's built-in DNS name that resolves to the macOS host, allowing the container to reach your native Ollama instance.

5. Launch LM Gate: `docker compose -f docker/docker-compose.standalone.yml up -d`

#### Option B: Binary

1. Create a directory: `mkdir lmgate`
2. Go into the directory: `cd lmgate`
3. Download binary:
   - Apple Silicon: `curl -o lmgate https://github.com/hkdb/lmgate/releases/latest/download/lmgate-macos-arm64`
   - Intel Mac: `curl -o lmgate https://github.com/hkdb/lmgate/releases/latest/download/lmgate-macos-amd64`
4. Download .env template: `curl -o .env https://raw.githubusercontent.com/hkdb/lmgate/refs/heads/main/.env.sample`
5. Fill out the .env:

   ```bash
   # Required — JWT signing key (use a long random string)
   LMGATE_AUTH_JWT_SECRET=your-secret-key-here

   # Required — encryption key for secrets (use a different long random string)
   LMGATE_ENCRYPTION_KEY=your-encryption-key-here
   # Disable TLS if you want LM Gate to be behind a proxy that doesn't support TLS
   LMGATE_TLS_DISABLED=false

   # Point to Ollama running locally
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

   **Note:** When running as a binary, use `http://localhost:11434` since both Ollama and LM Gate share the same host.

6. Launch LM Gate: `./lmgate`

See [CONFIGS.md](CONFIGS.md) for the full list of environment variables.
