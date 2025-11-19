-- +goose Up
CREATE TABLE IF NOT EXISTS materials (
    material_id TEXT PRIMARY KEY,
    request_id TEXT NOT NULL,
    task_id TEXT,
    agent_iteration INTEGER NOT NULL DEFAULT 0,
    tool_call_id TEXT,
    conversation_id TEXT,
    user_id TEXT,
    name TEXT NOT NULL,
    placeholder TEXT,
    mime_type TEXT NOT NULL,
    description TEXT,
    source TEXT,
    origin TEXT,
    status TEXT NOT NULL,
    visibility SMALLINT NOT NULL,
    tags JSONB NOT NULL DEFAULT '{}'::jsonb,
    annotations JSONB NOT NULL DEFAULT '{}'::jsonb,
    storage_key TEXT NOT NULL,
    cdn_url TEXT,
    content_hash TEXT,
    size_bytes BIGINT,
    system_attributes JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS material_lineage (
    parent_material_id TEXT NOT NULL REFERENCES materials(material_id) ON DELETE CASCADE,
    child_material_id TEXT NOT NULL REFERENCES materials(material_id) ON DELETE CASCADE,
    derivation_type TEXT,
    parameters_hash TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (parent_material_id, child_material_id)
);

CREATE TABLE IF NOT EXISTS material_access_bindings (
    material_id TEXT NOT NULL REFERENCES materials(material_id) ON DELETE CASCADE,
    principal TEXT NOT NULL,
    scope TEXT NOT NULL,
    capability TEXT NOT NULL,
    expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (material_id, principal, scope, capability)
);

CREATE INDEX IF NOT EXISTS idx_materials_request_iteration
    ON materials (request_id, agent_iteration);
CREATE INDEX IF NOT EXISTS idx_materials_content_hash
    ON materials (content_hash);
CREATE INDEX IF NOT EXISTS idx_materials_tags
    ON materials USING GIN (tags);
CREATE INDEX IF NOT EXISTS idx_materials_annotations
    ON materials USING GIN (annotations);
CREATE INDEX IF NOT EXISTS idx_materials_system_attributes
    ON materials USING GIN (system_attributes);
CREATE INDEX IF NOT EXISTS idx_lineage_child
    ON material_lineage (child_material_id);

-- +goose Down
DROP INDEX IF EXISTS idx_lineage_child;
DROP INDEX IF EXISTS idx_materials_system_attributes;
DROP INDEX IF EXISTS idx_materials_annotations;
DROP INDEX IF EXISTS idx_materials_tags;
DROP INDEX IF EXISTS idx_materials_content_hash;
DROP INDEX IF EXISTS idx_materials_request_iteration;
DROP TABLE IF EXISTS material_access_bindings;
DROP TABLE IF EXISTS material_lineage;
DROP TABLE IF EXISTS materials;
