-- Authentication schema bootstrap for local development
-- Usage:
--   psql "$AUTH_DATABASE_URL" -f migrations/auth/001_init.sql

CREATE EXTENSION IF NOT EXISTS pgcrypto;
CREATE EXTENSION IF NOT EXISTS citext;

CREATE TABLE IF NOT EXISTS auth_users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email CITEXT NOT NULL UNIQUE,
    display_name TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'active',
    password_hash TEXT NOT NULL,
    points_balance BIGINT NOT NULL DEFAULT 0,
    subscription_tier TEXT NOT NULL DEFAULT 'free',
    subscription_expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (status IN ('active', 'disabled')),
    CHECK (subscription_tier IN ('free', 'supporter', 'professional'))
);

CREATE INDEX IF NOT EXISTS idx_auth_users_status ON auth_users (status);
CREATE INDEX IF NOT EXISTS idx_auth_users_subscription_tier ON auth_users (subscription_tier);

CREATE TABLE IF NOT EXISTS auth_user_identities (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES auth_users(id) ON DELETE CASCADE,
    provider TEXT NOT NULL,
    provider_uid TEXT NOT NULL,
    access_token TEXT,
    refresh_token TEXT,
    expires_at TIMESTAMPTZ,
    scopes TEXT[],
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (provider, provider_uid)
);

CREATE INDEX IF NOT EXISTS idx_auth_user_identities_user ON auth_user_identities (user_id);

CREATE TABLE IF NOT EXISTS auth_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES auth_users(id) ON DELETE CASCADE,
    refresh_token_hash TEXT NOT NULL,
    refresh_token_fingerprint TEXT NOT NULL,
    user_agent TEXT,
    ip_address INET,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL,
    last_used_at TIMESTAMPTZ,
    CHECK (expires_at > created_at)
);

CREATE INDEX IF NOT EXISTS idx_auth_sessions_user ON auth_sessions (user_id);
CREATE INDEX IF NOT EXISTS idx_auth_sessions_expiry ON auth_sessions (expires_at);
CREATE INDEX IF NOT EXISTS idx_auth_sessions_last_used ON auth_sessions (last_used_at);
CREATE UNIQUE INDEX IF NOT EXISTS idx_auth_sessions_refresh_fingerprint ON auth_sessions (refresh_token_fingerprint);

CREATE TABLE IF NOT EXISTS auth_states (
    state TEXT PRIMARY KEY,
    provider TEXT NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_auth_states_expires_at ON auth_states (expires_at);
