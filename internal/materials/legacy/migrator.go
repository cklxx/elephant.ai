package legacy

import (
	"context"
	"strings"
	"time"

	"alex/internal/agent/ports"
	materialapi "alex/internal/materials/api"
	"alex/internal/materials/broker"
)

// MigrationRequest describes a batch of attachments that should be normalized.
type MigrationRequest struct {
	Context     *materialapi.RequestContext
	Attachments map[string]ports.Attachment
	Status      materialapi.MaterialStatus
	Origin      string
	Retention   time.Duration
}

// Migrator normalizes attachment maps by uploading inline payloads.
type Migrator interface {
	Normalize(ctx context.Context, req MigrationRequest) (map[string]ports.Attachment, error)
}

// BrokerMigrator uses the attachment broker to migrate legacy payloads.
type BrokerMigrator struct {
	Broker *broker.AttachmentBroker
}

// NewBrokerMigrator builds a migrator backed by the attachment broker.
func NewBrokerMigrator(b *broker.AttachmentBroker) *BrokerMigrator {
	return &BrokerMigrator{Broker: b}
}

// Normalize uploads inline payloads and returns CDN-backed attachments.
func (m *BrokerMigrator) Normalize(ctx context.Context, req MigrationRequest) (map[string]ports.Attachment, error) {
	if m == nil || m.Broker == nil || len(req.Attachments) == 0 {
		return req.Attachments, nil
	}
	toUpload := make(map[string]ports.Attachment)
	for key, att := range req.Attachments {
		if needsUpload(att) {
			toUpload[key] = att
		}
	}
	if len(toUpload) == 0 {
		return req.Attachments, nil
	}
	normalized, err := m.Broker.RegisterToolOutputs(ctx, broker.RegisterToolOutputsRequest{
		Context:             req.Context,
		Attachments:         toUpload,
		DefaultStatus:       req.Status,
		Origin:              req.Origin,
		DefaultRetentionTTL: req.Retention,
	})
	if err != nil {
		return nil, err
	}
	result := make(map[string]ports.Attachment, len(req.Attachments)+len(normalized))
	for key, att := range req.Attachments {
		result[key] = att
	}
	for key, att := range normalized {
		result[key] = att
	}
	return result, nil
}

func needsUpload(att ports.Attachment) bool {
	if att.Data != "" {
		return true
	}
	if strings.HasPrefix(att.URI, "data:") || att.URI == "" {
		return true
	}
	return false
}

var _ Migrator = (*BrokerMigrator)(nil)
