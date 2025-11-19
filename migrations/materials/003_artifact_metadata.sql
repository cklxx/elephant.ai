-- +goose Up
ALTER TABLE materials
    ADD COLUMN IF NOT EXISTS kind TEXT NOT NULL DEFAULT 'attachment',
    ADD COLUMN IF NOT EXISTS format TEXT,
    ADD COLUMN IF NOT EXISTS preview_profile TEXT,
    ADD COLUMN IF NOT EXISTS preview_assets JSONB NOT NULL DEFAULT '[]'::jsonb;

-- +goose Down
ALTER TABLE materials
    DROP COLUMN IF EXISTS preview_assets,
    DROP COLUMN IF EXISTS preview_profile,
    DROP COLUMN IF EXISTS format,
    DROP COLUMN IF EXISTS kind;
