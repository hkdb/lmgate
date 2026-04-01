# **Type 2, 3, 4, and 5** - Omnigate - Installation Guide

**Note:** Omnigate images are for **Linux only**. macOS users should see the [Apple Silicon Guide](APPLE.md) for the recommended setup using native Ollama with the standalone Docker image or binary. AMD APU users (including Ryzen AI 9 series) should use the AMD variant, as ROCm supports RDNA integrated graphics.

If you are starting fresh on a brand new VM, the `omnigate` images are the recommended way to deploy LM Gate with Ollama prepackaged in a single container for NVIDIA, AMD, Intel, and CPU-only environments.

For example, if you want to deploy to a fresh VM, SSH into the VM and follow these steps:

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
5. Launch LM Gate: 

   - CPU Only: `docker compose -f docker/docker-compose.omni.yml up -d`
   - AMD: `docker compose -f docker/docker-compose.omni.amd.yml up -d`
   - NVIDIA: `docker compose -f docker/docker-compose.omni.nvidia.yml up -d`
   - Intel (Experimental): `docker compose -f docker/docker-compose.omni.intel.yml up -d`

If you have a proxy running on the same machine and want to run it behind the proxy, use one of these commands instead:
- CPU Only: `docker compose -f docker/docker-compose.proxied.omni.yml up -d`
- AMD: `docker compose -f docker/docker-compose.proxied.omni.amd.yml up -d`
- NVIDIA: `docker compose -f docker/docker-compose.proxied.omni.nvidia.yml up -d`
- Intel (Experimental): `docker compose -f docker/docker-compose.proxied.omni.intel.yml up -d`
