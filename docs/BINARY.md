# **Type 6** - Binary - Installation Guide

You can also download and run the binary in any environment separately.

1. Create a directory: `mkdir lmgate`
2. Go into the directory: `cd lmgate`
3. Download binary:
   - Linux:
      - amd64: `curl -o lmgate https://github.com/hkdb/lmgate/releases/latest/download/lmgate-linux-amd64`
      - arm64: `curl -o lmgate https://github.com/hkdb/lmgate/releases/latest/download/lmgate-linux-arm64`
   - macOS:
      - amd64: `curl -o lmgate https://github.com/hkdb/lmgate/releases/latest/download/lmgate-macos-amd64`
      - arm64: `curl -o lmgate https://github.com/hkdb/lmgate/releases/latest/download/lmgate-macos-arm64`
   - FreeBSD:
      - amd64: `curl -o lmgate https://github.com/hkdb/lmgate/releases/latest/download/lmgate-freebsd-amd64`
      - arm64: `curl -o lmgate https://github.com/hkdb/lmgate/releases/latest/download/lmgate-freebsd-arm64`
   - Windows (download and rename to `lmgate.exe`):
      - [amd64](https://github.com/hkdb/lmgate/releases/latest/download/lmgate-windows-amd64.exe)
      - [arm64](https://github.com/hkdb/lmgate/releases/latest/download/lmgate-windows-arm64.exe)
4. Download .env template:
   - Linux/macOS/FreeBSD: `curl -o .env https://raw.githubusercontent.com/hkdb/lmgate/refs/heads/main/.env.sample`
   - Windows (download and rename to `.env`): [.env.sample](https://raw.githubusercontent.com/hkdb/lmgate/refs/heads/main/.env.sample)
5. Fill out the .env:

   ```bash
   # Required — JWT signing key (use a long random string)
   LMGATE_AUTH_JWT_SECRET=your-secret-key-here
   
   # Required — encryption key for secrets (use a different long random string)
   LMGATE_ENCRYPTION_KEY=your-encryption-key-here
   # Disable TLS if you want LM Gate to be behind a proxy that doesn't support TLS
   LMGATE_TLS_DISABLED=false
   
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
6. Launch LM Gate:
   - Linux/macOS/FreeBSD: `./lmgate`
   - Windows: `.\lmgate.exe`

