package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	materialapi "alex/internal/materials/api"
	"alex/internal/materials/store"
)

// pool abstracts the subset of pgxpool.Pool used by the store for easier testing.
type pool interface {
	BeginTx(ctx context.Context, txOptions pgx.TxOptions) (pgx.Tx, error)
}

// Store writes materials, lineage edges, and system attributes into Postgres tables.
type Store struct {
	pool pool
}

// New builds a Store backed by the provided connection pool.
func New(pool pool) (*Store, error) {
	if pool == nil {
		return nil, errors.New("postgres store requires pool")
	}
	return &Store{pool: pool}, nil
}

// InsertMaterials persists the provided materials and lineage graph.
func (s *Store) InsertMaterials(ctx context.Context, materials []store.MaterialRecord) error {
	if len(materials) == 0 {
		return nil
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) // no-op if committed

	for _, material := range materials {
		if material.MaterialID == "" {
			return errors.New("material id is required")
		}
		if material.Descriptor == nil {
			return fmt.Errorf("material %s missing descriptor", material.MaterialID)
		}
		if material.Storage == nil {
			return fmt.Errorf("material %s missing storage", material.MaterialID)
		}

		descriptor := material.Descriptor
		storage := material.Storage
		contextInfo := material.Context

		tagsJSON, err := jsonBytes(descriptor.Tags)
		if err != nil {
			return fmt.Errorf("material %s tags: %w", material.MaterialID, err)
		}
		annotationsJSON, err := jsonBytes(descriptor.Annotations)
		if err != nil {
			return fmt.Errorf("material %s annotations: %w", material.MaterialID, err)
		}
		systemAttrsJSON, err := marshalSystemAttributes(material.SystemAttributes)
		if err != nil {
			return fmt.Errorf("material %s system attributes: %w", material.MaterialID, err)
		}

		_, err = tx.Exec(ctx, `
INSERT INTO materials (
    material_id,
    request_id,
    task_id,
    agent_iteration,
    tool_call_id,
    conversation_id,
    user_id,
    name,
    placeholder,
    mime_type,
    description,
    source,
    origin,
    status,
    visibility,
    tags,
    annotations,
    storage_key,
    cdn_url,
    content_hash,
    size_bytes,
    system_attributes,
    retention_ttl_seconds
) VALUES (
    $1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23
)
ON CONFLICT (material_id) DO UPDATE SET
    request_id = EXCLUDED.request_id,
    task_id = EXCLUDED.task_id,
    agent_iteration = EXCLUDED.agent_iteration,
    tool_call_id = EXCLUDED.tool_call_id,
    conversation_id = EXCLUDED.conversation_id,
    user_id = EXCLUDED.user_id,
    name = EXCLUDED.name,
    placeholder = EXCLUDED.placeholder,
    mime_type = EXCLUDED.mime_type,
    description = EXCLUDED.description,
    source = EXCLUDED.source,
    origin = EXCLUDED.origin,
    status = EXCLUDED.status,
    visibility = EXCLUDED.visibility,
    tags = EXCLUDED.tags,
    annotations = EXCLUDED.annotations,
    storage_key = EXCLUDED.storage_key,
    cdn_url = EXCLUDED.cdn_url,
    content_hash = EXCLUDED.content_hash,
    size_bytes = EXCLUDED.size_bytes,
    system_attributes = EXCLUDED.system_attributes,
    retention_ttl_seconds = EXCLUDED.retention_ttl_seconds;
`,
			material.MaterialID,
			contextValue(contextInfo, func(c *materialapi.RequestContext) string { return c.RequestID }),
			contextValue(contextInfo, func(c *materialapi.RequestContext) string { return c.TaskID }),
			contextUint(contextInfo, func(c *materialapi.RequestContext) uint32 { return c.AgentIteration }),
			contextValue(contextInfo, func(c *materialapi.RequestContext) string { return c.ToolCallID }),
			contextValue(contextInfo, func(c *materialapi.RequestContext) string { return c.ConversationID }),
			contextValue(contextInfo, func(c *materialapi.RequestContext) string { return c.UserID }),
			descriptor.Name,
			descriptor.Placeholder,
			descriptor.MimeType,
			descriptor.Description,
			descriptor.Source,
			descriptor.Origin,
			descriptor.Status,
			int32(descriptor.Visibility),
			tagsJSON,
			annotationsJSON,
			storage.StorageKey,
			storage.CDNURL,
			storage.ContentHash,
			storage.SizeBytes,
			systemAttrsJSON,
			descriptor.RetentionTTLSeconds,
		)
		if err != nil {
			return fmt.Errorf("insert material %s: %w", material.MaterialID, err)
		}

		for _, edge := range material.Lineage {
			if edge.ParentMaterialID == "" {
				continue
			}
			_, err = tx.Exec(ctx, `
INSERT INTO material_lineage (
    parent_material_id,
    child_material_id,
    derivation_type,
    parameters_hash
) VALUES ($1,$2,$3,$4)
ON CONFLICT (parent_material_id, child_material_id) DO UPDATE SET
    derivation_type = EXCLUDED.derivation_type,
    parameters_hash = EXCLUDED.parameters_hash;
`, edge.ParentMaterialID, material.MaterialID, edge.DerivationType, edge.ParametersHash)
			if err != nil {
				return fmt.Errorf("insert lineage for %s: %w", material.MaterialID, err)
			}
		}

		for _, binding := range material.AccessBindings {
			if binding == nil {
				continue
			}
			if binding.Principal == "" || binding.Scope == "" || binding.Capability == "" {
				return fmt.Errorf("material %s access binding missing fields", material.MaterialID)
			}
			_, err = tx.Exec(ctx, `
INSERT INTO material_access_bindings (
    material_id,
    principal,
    scope,
    capability,
    expires_at
) VALUES ($1,$2,$3,$4,$5)
ON CONFLICT (material_id, principal, scope, capability) DO UPDATE SET
    expires_at = EXCLUDED.expires_at;
`, material.MaterialID, binding.Principal, binding.Scope, binding.Capability, nullableTime(binding.ExpiresAt))
			if err != nil {
				return fmt.Errorf("insert access binding for %s: %w", material.MaterialID, err)
			}
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit material insert: %w", err)
	}
	return nil
}

// DeleteExpiredMaterials removes materials whose retention windows elapsed.
func (s *Store) DeleteExpiredMaterials(ctx context.Context, req store.DeleteExpiredMaterialsRequest) ([]store.DeletedMaterial, error) {
	cutoff := req.Now
	if cutoff.IsZero() {
		cutoff = time.Now().UTC()
	}
	limit := req.Limit
	if limit <= 0 {
		limit = 100
	}
	statusFilters := statusStrings(req.Statuses)
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("begin cleanup tx: %w", err)
	}
	defer tx.Rollback(ctx)
	args := []any{cutoff}
	param := 2
	statusClause := ""
	if len(statusFilters) > 0 {
		statusClause = fmt.Sprintf(" AND status = ANY($%d)", param)
		args = append(args, statusFilters)
		param++
	}
	args = append(args, limit)
	limitPlaceholder := fmt.Sprintf("$%d", param)
	query := fmt.Sprintf(`
WITH expired AS (
    SELECT material_id, request_id, storage_key
    FROM materials
    WHERE retention_ttl_seconds > 0
      AND created_at + (retention_ttl_seconds || ' seconds')::interval <= $1
      %s
    ORDER BY created_at
    LIMIT %s
    FOR UPDATE SKIP LOCKED
)
DELETE FROM materials m
USING expired e
WHERE m.material_id = e.material_id
RETURNING e.material_id, e.request_id, e.storage_key;
`, statusClause, limitPlaceholder)
	rows, err := tx.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("delete expired materials: %w", err)
	}
	defer rows.Close()
	var deleted []store.DeletedMaterial
	for rows.Next() {
		var record store.DeletedMaterial
		if err := rows.Scan(&record.MaterialID, &record.RequestID, &record.StorageKey); err != nil {
			return nil, fmt.Errorf("scan deleted material: %w", err)
		}
		deleted = append(deleted, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate deleted materials: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit cleanup tx: %w", err)
	}
	return deleted, nil
}

func statusStrings(statuses []materialapi.MaterialStatus) []string {
	values := make([]string, 0, len(statuses))
	for _, status := range statuses {
		switch status {
		case materialapi.MaterialStatusInput:
			values = append(values, "input")
		case materialapi.MaterialStatusIntermediate:
			values = append(values, "intermediate")
		case materialapi.MaterialStatusFinal:
			values = append(values, "final")
		}
	}
	return values
}

func marshalSystemAttributes(attrs *materialapi.SystemAttributes) ([]byte, error) {
	if attrs == nil {
		return jsonBytes(map[string]any{})
	}
	payload := map[string]any{
		"domain_tags":      attrs.DomainTags,
		"compliance_tags":  attrs.ComplianceTags,
		"embeddings_ref":   attrs.EmbeddingsRef,
		"vector_index_key": attrs.VectorIndexKey,
		"extra":            attrs.Extra,
	}
	return jsonBytes(payload)
}

func jsonBytes(v any) ([]byte, error) {
	if v == nil {
		return []byte("{}"), nil
	}
	switch val := v.(type) {
	case map[string]string:
		if len(val) == 0 {
			return []byte("{}"), nil
		}
	case map[string]any:
		if len(val) == 0 {
			return []byte("{}"), nil
		}
	}
	return json.Marshal(v)
}

func contextValue(ctx *materialapi.RequestContext, getter func(*materialapi.RequestContext) string) string {
	if ctx == nil {
		return ""
	}
	return getter(ctx)
}

func contextUint(ctx *materialapi.RequestContext, getter func(*materialapi.RequestContext) uint32) uint32 {
	if ctx == nil {
		return 0
	}
	return getter(ctx)
}

func nullableTime(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t.UTC()
}
