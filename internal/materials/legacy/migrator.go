package legacy

import (
	"context"
	"strings"

	"alex/internal/agent/ports"
	"alex/internal/materials/broker"
	materialports "alex/internal/materials/ports"
)

// BrokerMigrator uses the attachment broker to migrate legacy payloads.
type BrokerMigrator struct {
	Broker *broker.AttachmentBroker
}

// NewBrokerMigrator builds a migrator backed by the attachment broker.
func NewBrokerMigrator(b *broker.AttachmentBroker) *BrokerMigrator {
	return &BrokerMigrator{Broker: b}
}

// Normalize uploads inline payloads and returns CDN-backed attachments.
func (m *BrokerMigrator) Normalize(ctx context.Context, req materialports.MigrationRequest) (map[string]ports.Attachment, error) {
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

var _ materialports.Migrator = (*BrokerMigrator)(nil)
