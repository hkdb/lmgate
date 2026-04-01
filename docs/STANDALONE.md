# **Type 1** - Docker Standalone - Installation Guide

If you already have an existing LLM backend deployed, you can follow the below steps to deploy LM Gate. However, keep in mind that you must deploy LM Gate in the same closed environment as the LLM backend in order for this to be somewhat secure. Otherwise, proxying to an open LLM backend still leaves an open LLM backend exposed to the internet.

For example, if you have Ollama installed on a VM, SSH into the VM and follow these steps:

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
5. Launch LM Gate: `docker compose -f docker/docker-compose.standalone.yml up -d`

If you have a proxy running on the same machine and want to run it behind the proxy, run `docker compose -f docker/docker-compose.proxied.standalone.yml` instead.
