package ports

import (
	"context"
	"time"

	agentports "alex/internal/agent/ports"
	materialapi "alex/internal/materials/api"
)

// MigrationRequest describes a batch of attachments that should be normalized.
type MigrationRequest struct {
	Context     *materialapi.RequestContext
	Attachments map[string]agentports.Attachment
	Status      materialapi.MaterialStatus
	Origin      string
	Retention   time.Duration
}

// Migrator normalizes attachment maps by uploading inline payloads.
type Migrator interface {
	Normalize(ctx context.Context, req MigrationRequest) (map[string]agentports.Attachment, error)
}
