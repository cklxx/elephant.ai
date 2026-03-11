package ports

import (
	"context"

	agentports "alex/internal/domain/agent/ports"
)

// MigrationRequest describes a batch of attachments that should be normalized.
type MigrationRequest struct {
	Attachments map[string]agentports.Attachment
}

// Migrator normalizes attachment maps by uploading inline payloads.
type Migrator interface {
	Normalize(ctx context.Context, req MigrationRequest) (map[string]agentports.Attachment, error)
}
