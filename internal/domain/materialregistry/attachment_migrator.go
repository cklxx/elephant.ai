package materialregistry

import (
	"context"
	"encoding/base64"
	"fmt"
	neturl "net/url"
	"strings"
	"unicode"

	"alex/internal/domain/agent/ports"
	materialports "alex/internal/domain/materialregistry/ports"
	"alex/internal/shared/logging"
	"alex/internal/shared/utils"
)

type AttachmentStorer interface {
	StoreBytes(name, mediaType string, data []byte) (string, error)
}

type AttachmentStoreMigrator struct {
	store   AttachmentStorer
	fetcher materialports.RemoteFetcher
	logger  logging.Logger
	cdnBase string
}

func NewAttachmentStoreMigrator(store AttachmentStorer, fetcher materialports.RemoteFetcher, cdnBase string, logger logging.Logger) *AttachmentStoreMigrator {
	return &AttachmentStoreMigrator{
		store:   store,
		fetcher: fetcher,
		logger:  logging.OrNop(logger),
		cdnBase: strings.TrimRight(strings.TrimSpace(cdnBase), "/"),
	}
}

func (m *AttachmentStoreMigrator) Normalize(ctx context.Context, req materialports.MigrationRequest) (map[string]ports.Attachment, error) {
	if len(req.Attachments) == 0 || m.store == nil {
		return req.Attachments, nil
	}

	result := make(map[string]ports.Attachment, len(req.Attachments))
	for key, att := range req.Attachments {
		hosted := m.isHosted(att.URI)
		if hosted && att.Data == "" {
			result[key] = att
			continue
		}
		if hosted && att.Data != "" {
			att.Data = ""
			result[key] = att
			continue
		}
		if !m.needsUpload(att) {
			result[key] = att
			continue
		}

		payload, mediaType, err := m.capturePayload(ctx, att)
		if err != nil {
			m.logger.Warn("skip attachment migration for %s: %v", att.Name, err)
			result[key] = att
			continue
		}

		uri, err := m.store.StoreBytes(att.Name, mediaType, payload)
		if err != nil {
			m.logger.Warn("store attachment %s: %v", att.Name, err)
			result[key] = att
			continue
		}

		att.URI = uri
		att.Data = ""
		result[key] = att
	}

	return result, nil
}

func (m *AttachmentStoreMigrator) isHosted(uri string) bool {
	trimmed := strings.TrimSpace(uri)
	if trimmed == "" {
		return false
	}
	if m.cdnBase != "" && strings.HasPrefix(trimmed, m.cdnBase) {
		return true
	}
	parsed, err := neturl.Parse(trimmed)
	if err != nil {
		return false
	}
	if parsed.Scheme == "" {
		return strings.HasPrefix(trimmed, attachmentsPathPrefix)
	}
	return strings.HasPrefix(parsed.Path, attachmentsPathPrefix)
}

func (m *AttachmentStoreMigrator) needsUpload(att ports.Attachment) bool {
	if utils.HasContent(att.Data) {
		return true
	}
	uri := utils.TrimLower(att.URI)
	if uri == "" {
		return false
	}
	if strings.HasPrefix(uri, "data:") {
		return true
	}
	return strings.HasPrefix(uri, "http://") || strings.HasPrefix(uri, "https://")
}

func (m *AttachmentStoreMigrator) capturePayload(ctx context.Context, att ports.Attachment) ([]byte, string, error) {
	if utils.HasContent(att.Data) {
		decoded, err := decodeBase64Payload(att.Data)
		if err != nil {
			return nil, "", fmt.Errorf("decode base64 payload: %w", err)
		}
		return decoded, att.MediaType, nil
	}

	uri := strings.TrimSpace(att.URI)
	lower := strings.ToLower(uri)
	if strings.HasPrefix(lower, "data:") {
		data, mediaType, err := decodeDataURI(uri)
		if err != nil {
			return nil, "", fmt.Errorf("decode data uri: %w", err)
		}
		if att.MediaType != "" {
			return data, att.MediaType, nil
		}
		return data, mediaType, nil
	}

	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") {
		return m.fetcher.Fetch(ctx, uri, att.MediaType)
	}

	return nil, "", fmt.Errorf("attachment %s has no transferable payload", att.Name)
}

func decodeBase64Payload(encoded string) ([]byte, error) {
	clean := strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}, encoded)
	decoded, err := decodeBase64(clean)
	if err != nil {
		return nil, err
	}
	if len(decoded) == 0 {
		return nil, fmt.Errorf("payload is empty")
	}
	return decoded, nil
}

const attachmentsPathPrefix = "/api/attachments/"

func decodeDataURI(value string) ([]byte, string, error) {
	trimmed := strings.TrimSpace(value)
	parts := strings.SplitN(trimmed, ",", 2)
	if len(parts) != 2 {
		return nil, "", fmt.Errorf("invalid data uri")
	}
	header := parts[0]
	payload := strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}, parts[1])

	mediaType := strings.TrimPrefix(header, "data:")
	mediaType = strings.TrimSuffix(mediaType, ";base64")

	decoded, err := decodeBase64(payload)
	if err != nil {
		return nil, "", fmt.Errorf("decode base64: %w", err)
	}
	if mediaType == "" {
		mediaType = "application/octet-stream"
	}

	return decoded, mediaType, nil
}

var _ materialports.Migrator = (*AttachmentStoreMigrator)(nil)

func decodeBase64(value string) ([]byte, error) {
	if decoded, err := base64.StdEncoding.DecodeString(value); err == nil {
		return decoded, nil
	}
	return base64.RawStdEncoding.DecodeString(value)
}
