package broker

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"alex/internal/agent/ports"
	materialapi "alex/internal/materials/api"
	"alex/internal/materials/storage"
)

// RegistryClient captures the subset of API methods used by the broker.
type RegistryClient interface {
	RegisterMaterials(ctx context.Context, req *materialapi.RegisterMaterialsRequest) (*materialapi.RegisterMaterialsResponse, error)
	ListMaterials(ctx context.Context, req *materialapi.ListMaterialsRequest) (*materialapi.ListMaterialsResponse, error)
}

// AttachmentBroker normalizes tool attachments by uploading their payloads to
// Storage Mapper + Registry so runtime callers always receive CDN backed URIs.
type AttachmentBroker struct {
	registry RegistryClient
	storage  storage.Mapper
	events   EventPublisher
}

// AttachmentBrokerOption mutates the broker during construction.
type AttachmentBrokerOption func(*AttachmentBroker)

// EventPublisher publishes newly registered materials onto an external event bus.
type EventPublisher interface {
	PublishMaterial(ctx context.Context, material *materialapi.Material) error
}

// WithEventPublisher attaches an event publisher to the broker so downstream
// runtimes/UI clients can be notified immediately.
func WithEventPublisher(publisher EventPublisher) AttachmentBrokerOption {
	return func(b *AttachmentBroker) {
		b.events = publisher
	}
}

// NewAttachmentBroker builds a broker with the provided dependencies.
func NewAttachmentBroker(registry RegistryClient, storage storage.Mapper, opts ...AttachmentBrokerOption) (*AttachmentBroker, error) {
	if registry == nil {
		return nil, errors.New("attachment broker requires registry client")
	}
	if storage == nil {
		return nil, errors.New("attachment broker requires storage mapper")
	}
	broker := &AttachmentBroker{registry: registry, storage: storage}
	for _, opt := range opts {
		if opt != nil {
			opt(broker)
		}
	}
	return broker, nil
}

// RegisterToolOutputsRequest describes a batch of attachments produced by a
// tool call that should be persisted via the registry.
type RegisterToolOutputsRequest struct {
	Context             *materialapi.RequestContext
	Attachments         map[string]ports.Attachment
	DefaultStatus       materialapi.MaterialStatus
	Origin              string
	DefaultRetentionTTL time.Duration
}

// RegisterToolOutputs uploads the attachment payloads and records the catalog
// entries.
func (b *AttachmentBroker) RegisterToolOutputs(ctx context.Context, req RegisterToolOutputsRequest) (map[string]ports.Attachment, error) {
	if req.Context == nil {
		return nil, errors.New("attachment broker: missing request context")
	}
	if len(req.Attachments) == 0 {
		return nil, nil
	}

	status := req.DefaultStatus
	if status == materialapi.MaterialStatusUnspecified {
		status = materialapi.MaterialStatusIntermediate
	}

	register := &materialapi.RegisterMaterialsRequest{Context: req.Context}
	originalByStorageKey := make(map[string]string, len(req.Attachments))
	originalByName := make(map[string]string, len(req.Attachments))

	for key, attachment := range req.Attachments {
		payload, mimeType, err := extractPayload(attachment)
		if err != nil {
			return nil, fmt.Errorf("attachment broker: %s payload: %w", key, err)
		}
		if len(payload) == 0 {
			return nil, fmt.Errorf("attachment broker: attachment %s had no inline payload", key)
		}

		uploadResult, err := b.storage.Upload(ctx, storage.UploadRequest{
			Name:     attachment.Name,
			MimeType: mimeType,
			Data:     payload,
			Source:   attachment.Source,
		})
		if err != nil {
			return nil, fmt.Errorf("attachment broker: upload %s: %w", key, err)
		}
		if err := b.storage.Prewarm(ctx, uploadResult.StorageKey); err != nil {
			return nil, fmt.Errorf("attachment broker: prewarm %s: %w", key, err)
		}

		retention := retentionWindow(status, req.DefaultRetentionTTL)
		register.Materials = append(register.Materials, &materialapi.MaterialInput{
			Name:                attachment.Name,
			MimeType:            mimeType,
			Description:         attachment.Description,
			Source:              attachment.Source,
			Status:              status,
			Origin:              coalesce(req.Origin, attachment.Source),
			Tags:                map[string]string{"placeholder": key},
			Visibility:          materialapi.VisibilityShared,
			StorageKey:          uploadResult.StorageKey,
			CDNURL:              uploadResult.CDNURL,
			ContentHash:         uploadResult.ContentHash,
			SizeBytes:           uploadResult.SizeBytes,
			InlineBytes:         nil,
			Lineage:             nil,
			RetentionTTLSeconds: uint64(retention / time.Second),
		})
		originalByStorageKey[uploadResult.StorageKey] = key
		if attachment.Name != "" {
			originalByName[attachment.Name] = key
		}
	}

	resp, err := b.registry.RegisterMaterials(ctx, register)
	if err != nil {
		return nil, fmt.Errorf("attachment broker: register materials: %w", err)
	}

	if b.events != nil {
		for _, material := range resp.Materials {
			if err := b.events.PublishMaterial(ctx, material); err != nil {
				return nil, fmt.Errorf("attachment broker: publish material event: %w", err)
			}
		}
	}

	normalized := make(map[string]ports.Attachment, len(resp.Materials))
	for _, material := range resp.Materials {
		if material.Descriptor == nil || material.Storage == nil {
			continue
		}
		attachment := ports.Attachment{
			Name:        material.Descriptor.Name,
			MediaType:   material.Descriptor.MimeType,
			URI:         material.Storage.CDNURL,
			Source:      material.Descriptor.Source,
			Description: material.Descriptor.Description,
		}
		key := ""
		if material.Storage != nil {
			key = originalByStorageKey[material.Storage.StorageKey]
		}
		if key == "" && material.Descriptor != nil {
			if material.Descriptor.Tags != nil {
				key = material.Descriptor.Tags["placeholder"]
			}
			if key == "" {
				key = originalByName[material.Descriptor.Name]
			}
		}
		if key == "" {
			key = material.Descriptor.Placeholder
		}
		if key == "" {
			key = fmt.Sprintf("[material:%s]", material.MaterialID)
		}
		normalized[key] = attachment
		placeholder := material.Descriptor.Placeholder
		if placeholder != "" && placeholder != key {
			normalized[placeholder] = attachment
		}
	}

	return normalized, nil
}

// ListMaterials proxies registry listings for runtime callers.
func (b *AttachmentBroker) ListMaterials(ctx context.Context, req *materialapi.ListMaterialsRequest) (*materialapi.ListMaterialsResponse, error) {
	if req == nil {
		return nil, errors.New("attachment broker: missing list request")
	}
	return b.registry.ListMaterials(ctx, req)
}

func extractPayload(att ports.Attachment) ([]byte, string, error) {
	if att.MediaType == "" {
		return nil, "", errors.New("missing media type")
	}
	if att.Data != "" {
		bytes, err := base64.StdEncoding.DecodeString(att.Data)
		if err != nil {
			return nil, "", fmt.Errorf("decode base64: %w", err)
		}
		return bytes, att.MediaType, nil
	}
	if strings.HasPrefix(att.URI, "data:") {
		idx := strings.Index(att.URI, ";base64,")
		if idx == -1 {
			return nil, "", fmt.Errorf("invalid data uri for %s", att.Name)
		}
		encoded := att.URI[idx+8:]
		bytes, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			return nil, "", fmt.Errorf("decode data uri: %w", err)
		}
		return bytes, att.MediaType, nil
	}
	return nil, "", errors.New("attachment broker: no inline data or data URI provided")
}

func coalesce(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func retentionWindow(status materialapi.MaterialStatus, override time.Duration) time.Duration {
	if override > 0 {
		return override
	}
	switch status {
	case materialapi.MaterialStatusInput:
		return 30 * 24 * time.Hour
	case materialapi.MaterialStatusIntermediate:
		return 7 * 24 * time.Hour
	case materialapi.MaterialStatusFinal:
		return 0
	default:
		return 0
	}
}
