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
	"alex/internal/materials/policy"
	materialports "alex/internal/materials/ports"
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
	events   materialports.EventPublisher
}

// AttachmentBrokerOption mutates the broker during construction.
type AttachmentBrokerOption func(*AttachmentBroker)

// WithEventPublisher attaches an event publisher to the broker so downstream
// runtimes/UI clients can be notified immediately.
func WithEventPublisher(publisher materialports.EventPublisher) AttachmentBrokerOption {
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
	DefaultKind         materialapi.MaterialKind
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

		kind := resolveMaterialKind(attachment.Kind, req.DefaultKind)
		format := attachment.Format
		if format == "" {
			format = inferFormat(mimeType)
		}
		retention := resolveAttachmentRetention(attachment.RetentionTTLSeconds, status, kind, req.DefaultRetentionTTL)
		previewAssets, err := b.generatePreviewAssets(ctx, previewGenerationInput{
			Name:     attachment.Name,
			Source:   attachment.Source,
			Format:   format,
			Kind:     kind,
			MimeType: mimeType,
			Payload:  payload,
			Upload:   uploadResult,
		})
		if err != nil {
			return nil, fmt.Errorf("attachment broker: preview assets %s: %w", key, err)
		}
		previewProfile := defaultPreviewProfile(kind, format)
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
			Kind:                kind,
			Format:              format,
			PreviewProfile:      previewProfile,
			PreviewAssets:       previewAssets,
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
		previewAssets := convertPreviewAssets(material.Descriptor.PreviewAssets)
		attachment := ports.Attachment{
			Name:           material.Descriptor.Name,
			MediaType:      material.Descriptor.MimeType,
			URI:            material.Storage.CDNURL,
			Source:         material.Descriptor.Source,
			Description:    material.Descriptor.Description,
			Kind:           materialKindLabel(material.Descriptor.Kind),
			Format:         material.Descriptor.Format,
			PreviewProfile: material.Descriptor.PreviewProfile,
			PreviewAssets:  previewAssets,
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

func inferFormat(mimeType string) string {
	mime := strings.ToLower(mimeType)
	switch mime {
	case "application/vnd.ms-powerpoint":
		return "ppt"
	case "application/vnd.openxmlformats-officedocument.presentationml.presentation":
		return "pptx"
	case "text/markdown", "application/x-markdown":
		return "markdown"
	case "text/html":
		return "html"
	case "application/pdf":
		return "pdf"
	}
	if idx := strings.Index(mime, "/"); idx != -1 && idx+1 < len(mime) {
		return mime[idx+1:]
	}
	return mime
}

func defaultPreviewProfile(kind materialapi.MaterialKind, format string) string {
	if format == "" {
		return ""
	}
	switch strings.ToLower(format) {
	case "ppt", "pptx":
		return "document.ppt"
	case "html":
		return "document.html"
	case "markdown":
		return "document.markdown"
	case "pdf":
		return "document.pdf"
	case "png", "jpg", "jpeg", "gif", "svg":
		if kind == materialapi.MaterialKindArtifact {
			return "document.image"
		}
	}
	return ""
}

func materialKindLabel(kind materialapi.MaterialKind) string {
	switch kind {
	case materialapi.MaterialKindArtifact:
		return "artifact"
	case materialapi.MaterialKindAttachment, materialapi.MaterialKindUnspecified:
		return "attachment"
	default:
		return ""
	}
}

func convertPreviewAssets(assets []*materialapi.PreviewAsset) []ports.AttachmentPreviewAsset {
	if len(assets) == 0 {
		return nil
	}
	converted := make([]ports.AttachmentPreviewAsset, 0, len(assets))
	for _, asset := range assets {
		if asset == nil {
			continue
		}
		converted = append(converted, ports.AttachmentPreviewAsset{
			AssetID:     asset.AssetID,
			Label:       asset.Label,
			MimeType:    asset.MimeType,
			CDNURL:      asset.CDNURL,
			PreviewType: asset.PreviewType,
		})
	}
	if len(converted) == 0 {
		return nil
	}
	return converted
}

func coalesce(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func resolveMaterialKind(label string, fallback materialapi.MaterialKind) materialapi.MaterialKind {
	trimmed := strings.ToLower(strings.TrimSpace(label))
	switch trimmed {
	case "artifact":
		return materialapi.MaterialKindArtifact
	case "attachment":
		return materialapi.MaterialKindAttachment
	}
	if fallback == materialapi.MaterialKindUnspecified {
		return materialapi.MaterialKindAttachment
	}
	return fallback
}

func resolveAttachmentRetention(ttlSeconds uint64, status materialapi.MaterialStatus, kind materialapi.MaterialKind, override time.Duration) time.Duration {
	return policy.DefaultEngine().ResolveRetention(ttlSeconds, status, kind, override)
}
