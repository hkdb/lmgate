package database

const schema = `
PRAGMA journal_mode = WAL;
PRAGMA synchronous = NORMAL;
PRAGMA busy_timeout = 5000;
PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS users (
    id TEXT PRIMARY KEY,
    email TEXT UNIQUE NOT NULL,
    display_name TEXT NOT NULL DEFAULT '',
    password_hash TEXT NOT NULL DEFAULT '',
    role TEXT NOT NULL DEFAULT 'user',
    auth_provider TEXT NOT NULL DEFAULT 'local',
    provider_sub TEXT,
    disabled INTEGER NOT NULL DEFAULT 0,
    force_password_change INTEGER NOT NULL DEFAULT 0,
    totp_enabled INTEGER NOT NULL DEFAULT 0,
    password_changed_at TEXT NOT NULL DEFAULT '',
    failed_login_count INTEGER NOT NULL DEFAULT 0,
    locked_until TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS api_tokens (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name TEXT NOT NULL DEFAULT '',
    token_hash TEXT UNIQUE NOT NULL,
    token_redacted TEXT NOT NULL DEFAULT '',
    rate_limit INTEGER NOT NULL DEFAULT 60,
    expires_at TEXT,
    revoked INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS model_acls (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    model TEXT NOT NULL,
    allowed INTEGER NOT NULL DEFAULT 1,
    UNIQUE(user_id, model)
);

CREATE TABLE IF NOT EXISTS oidc_providers (
    id TEXT PRIMARY KEY,
    provider_type TEXT NOT NULL,
    name TEXT NOT NULL DEFAULT '',
    client_id TEXT NOT NULL,
    client_secret TEXT NOT NULL,
    client_secret_salt TEXT NOT NULL DEFAULT '',
    issuer_url TEXT NOT NULL,
    scopes TEXT NOT NULL DEFAULT 'openid email profile',
    enabled INTEGER NOT NULL DEFAULT 1,
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS audit_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id TEXT,
    token_id TEXT,
    method TEXT NOT NULL,
    path TEXT NOT NULL,
    model TEXT,
    status_code INTEGER NOT NULL,
    latency_ms INTEGER NOT NULL,
    gateway_overhead_us INTEGER NOT NULL DEFAULT 0,
    upstream_ttfb_ms INTEGER NOT NULL DEFAULT 0,
    streaming INTEGER NOT NULL DEFAULT 0,
    prompt_tokens INTEGER NOT NULL DEFAULT 0,
    completion_tokens INTEGER NOT NULL DEFAULT 0,
    request_id TEXT,
    log_type TEXT NOT NULL DEFAULT 'api',
    ip_addr TEXT,
    x_real_ip TEXT,
    x_forwarded_for TEXT,
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS usage_metrics (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    model TEXT NOT NULL,
    prompt_tokens INTEGER NOT NULL DEFAULT 0,
    completion_tokens INTEGER NOT NULL DEFAULT 0,
    request_count INTEGER NOT NULL DEFAULT 1,
    period_start TEXT NOT NULL,
    period_end TEXT NOT NULL,
    UNIQUE(user_id, model, period_start)
);

CREATE TABLE IF NOT EXISTS totp_secrets (
    user_id TEXT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    secret_encrypted TEXT NOT NULL,
    secret_salt TEXT NOT NULL,
    verified INTEGER NOT NULL DEFAULT 0,
    totp_last_used_at TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS webauthn_credentials (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name TEXT NOT NULL DEFAULT '',
    credential_id BLOB NOT NULL UNIQUE,
    public_key BLOB NOT NULL,
    attestation_type TEXT NOT NULL DEFAULT '',
    aaguid BLOB,
    sign_count INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS recovery_codes (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    code_hash TEXT NOT NULL,
    used INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS app_settings (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_api_tokens_hash ON api_tokens(token_hash);
CREATE INDEX IF NOT EXISTS idx_api_tokens_user ON api_tokens(user_id);
CREATE INDEX IF NOT EXISTS idx_audit_logs_created ON audit_logs(created_at);
CREATE INDEX IF NOT EXISTS idx_audit_logs_user ON audit_logs(user_id);
CREATE INDEX IF NOT EXISTS idx_audit_logs_log_type ON audit_logs(log_type);
CREATE INDEX IF NOT EXISTS idx_usage_metrics_user ON usage_metrics(user_id);
CREATE INDEX IF NOT EXISTS idx_usage_metrics_period ON usage_metrics(period_start);
CREATE INDEX IF NOT EXISTS idx_model_acls_user ON model_acls(user_id);
CREATE INDEX IF NOT EXISTS idx_webauthn_credentials_user ON webauthn_credentials(user_id);
CREATE INDEX IF NOT EXISTS idx_recovery_codes_user ON recovery_codes(user_id);

CREATE TABLE IF NOT EXISTS groups (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    source TEXT NOT NULL DEFAULT 'local',
    source_id TEXT NOT NULL DEFAULT '',
    admin_role INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE(name, source)
);

CREATE TABLE IF NOT EXISTS user_groups (
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    group_id TEXT NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    PRIMARY KEY (user_id, group_id)
);

CREATE INDEX IF NOT EXISTS idx_user_groups_user ON user_groups(user_id);
CREATE INDEX IF NOT EXISTS idx_user_groups_group ON user_groups(group_id);
CREATE INDEX IF NOT EXISTS idx_groups_source ON groups(source);
`
