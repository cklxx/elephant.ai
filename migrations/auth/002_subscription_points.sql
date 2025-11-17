-- Subscription and points scaffolding
-- Applies incremental schema additions for the roadmap documented in docs/auth_module_architecture.md
-- Usage:
--   psql "$AUTH_DATABASE_URL" -f migrations/auth/002_subscription_points.sql

CREATE TABLE IF NOT EXISTS auth_plans (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tier TEXT NOT NULL,
    display_name TEXT NOT NULL,
    monthly_price_cents INTEGER NOT NULL CHECK (monthly_price_cents >= 0),
    currency TEXT NOT NULL DEFAULT 'USD',
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (tier)
);

CREATE TABLE IF NOT EXISTS auth_subscriptions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES auth_users(id) ON DELETE CASCADE,
    tier TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'active',
    auto_renew BOOLEAN NOT NULL DEFAULT TRUE,
    current_period_start TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    current_period_end TIMESTAMPTZ,
    external_customer_id TEXT,
    external_subscription_id TEXT,
    external_plan_id TEXT,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_auth_subscriptions_user ON auth_subscriptions (user_id);
CREATE INDEX IF NOT EXISTS idx_auth_subscriptions_status ON auth_subscriptions (status);
CREATE INDEX IF NOT EXISTS idx_auth_subscriptions_period_end ON auth_subscriptions (current_period_end);

CREATE TABLE IF NOT EXISTS auth_points_ledger (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES auth_users(id) ON DELETE CASCADE,
    delta BIGINT NOT NULL,
    balance_after BIGINT,
    reason TEXT NOT NULL,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (delta <> 0)
);

CREATE INDEX IF NOT EXISTS idx_auth_points_ledger_user ON auth_points_ledger (user_id);
CREATE INDEX IF NOT EXISTS idx_auth_points_ledger_created_at ON auth_points_ledger (created_at);
