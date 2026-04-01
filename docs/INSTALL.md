# Installation Guide

This is the installation guide for LM Gate.

You can find all the different methods of installation within this single doc or click on any of the environments below to see the environment specific docs respectively. 

## Types of installation

| Type | Environments | Description |
| --- |--------|-------------|
| 1 | [**Docker Standalone**](STANDALONE.md) | LM Gate only, point to an existing LLM backend |
| 2 | [**Omnigate (CPU)**](OMNIGATE.md) | All-in-one (Ollama + LM Gate) - CPU only |
| 3 | [**Omnigate (NVIDIA)**](OMNIGATE.md) | All-in-one (Ollama + LM Gate) with NVIDIA GPU acceleration |
| 4 | [**Omnigate (AMD)**](OMNIGATE.md) | All-in-one (Ollama + LM Gate) with AMD GPU acceleration via ROCm |
| 5 | [**Omnigate (Intel)**](OMNIGATE.md) | All-in-one (Ollama + LM Gate) with Intel iGPU acceleration (Experimental) |
| 6 | [**Binary**](BINARY.md) | Download and run the binary directly |

**Note:** Omnigate images are for **Linux only**. macOS users should see the [Apple Silicon Guide](APPLE.md) for the recommended setup using native Ollama with the standalone Docker image or binary. AMD APU users (including Ryzen AI 9 series) should use the AMD variant, as ROCm supports RDNA integrated graphics.

## **Type 1** - Docker Standalone

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

## **Type 2, 3, 4, and 5** - Omnigate

If you are starting fresh on a brand new VM, the `omnigate` images are the recommended way to deploy LM Gate with Ollama prepackaged in a single container for NVIDIA, AMD, and CPU-only environments.

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


## **Type 6** - Binary

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

