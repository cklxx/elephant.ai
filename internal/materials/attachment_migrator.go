package materials

import (
	"context"
	"fmt"
	"io"
	"mime"
	"net/http"
	neturl "net/url"
	"strings"
	"time"
	"unicode"

	"alex/internal/agent/ports"
	"alex/internal/attachments"
	"alex/internal/httpclient"
	"alex/internal/logging"
	materialports "alex/internal/materials/ports"
)

type AttachmentStorer interface {
	StoreBytes(name, mediaType string, data []byte) (string, error)
}

type AttachmentStoreMigrator struct {
	store         AttachmentStorer
	client        *http.Client
	logger        logging.Logger
	cdnBase       string
	maxFetchBytes int64
	allowLocal    bool
}

const defaultMaxFetchBytes = int64(25 << 20) // 25 MiB

func NewAttachmentStoreMigrator(store AttachmentStorer, client *http.Client, cdnBase string, logger logging.Logger) *AttachmentStoreMigrator {
	if client == nil {
		client = httpclient.New(45*time.Second, logger)
	}
	return &AttachmentStoreMigrator{
		store:         store,
		client:        client,
		logger:        logging.OrNop(logger),
		cdnBase:       strings.TrimRight(strings.TrimSpace(cdnBase), "/"),
		maxFetchBytes: defaultMaxFetchBytes,
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
	if strings.TrimSpace(att.Data) != "" {
		return true
	}
	uri := strings.ToLower(strings.TrimSpace(att.URI))
	if uri == "" {
		return false
	}
	if strings.HasPrefix(uri, "data:") {
		return true
	}
	return strings.HasPrefix(uri, "http://") || strings.HasPrefix(uri, "https://")
}

func (m *AttachmentStoreMigrator) capturePayload(ctx context.Context, att ports.Attachment) ([]byte, string, error) {
	if strings.TrimSpace(att.Data) != "" {
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
		return m.fetchRemote(ctx, uri, att.MediaType)
	}

	return nil, "", fmt.Errorf("attachment %s has no transferable payload", att.Name)
}

func (m *AttachmentStoreMigrator) fetchRemote(ctx context.Context, uri, mediaType string) ([]byte, string, error) {
	requestCtx := ctx
	if requestCtx == nil {
		requestCtx = context.Background()
	}
	opts := httpclient.DefaultURLValidationOptions()
	if m.allowLocal {
		opts.AllowLocalhost = true
	}
	parsed, err := httpclient.ValidateOutboundURL(uri, opts)
	if err != nil {
		return nil, "", err
	}
	// URL is validated by ValidateOutboundURL before request construction.
	// lgtm[go/ssrf]
	req, err := http.NewRequestWithContext(requestCtx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return nil, "", err
	}
	resp, err := m.client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return nil, "", fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	reader := io.LimitReader(resp.Body, m.maxFetchBytes+1)
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, "", err
	}
	if int64(len(data)) > m.maxFetchBytes {
		return nil, "", fmt.Errorf("attachment exceeds %d bytes", m.maxFetchBytes)
	}

	ct := strings.TrimSpace(mediaType)
	if ct == "" {
		ct = strings.TrimSpace(resp.Header.Get("Content-Type"))
	}
	if parsed, _, err := mime.ParseMediaType(ct); err == nil && parsed != "" {
		ct = parsed
	}
	if ct == "" {
		ct = "application/octet-stream"
	}

	return data, ct, nil
}

func decodeBase64Payload(encoded string) ([]byte, error) {
	clean := strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}, encoded)
	decoded, err := attachments.DecodeBase64(clean)
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

	decoded, err := attachments.DecodeBase64(payload)
	if err != nil {
		return nil, "", fmt.Errorf("decode base64: %w", err)
	}
	if mediaType == "" {
		mediaType = "application/octet-stream"
	}

	return decoded, mediaType, nil
}

var _ materialports.Migrator = (*AttachmentStoreMigrator)(nil)
