-- +goose Up
ALTER TABLE materials
    ADD COLUMN IF NOT EXISTS retention_ttl_seconds BIGINT NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE materials
    DROP COLUMN IF EXISTS retention_ttl_seconds;
