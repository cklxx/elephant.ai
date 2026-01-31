package attachments

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strings"

	"alex/internal/agent/ports"
)

// inlineRetentionLimit is the maximum decoded payload size (bytes) for which
// small text-like attachments keep their Data field populated after
// persistence.  This allows frontends to render previews without an extra
// fetch while still offloading the bulk of binary/large content.
const inlineRetentionLimit = 4096

var dataURIPattern = regexp.MustCompile(`(?is)^data:([^;,]+)?(;[^,]*)?,\s*(.+)$`)

// StorePersister implements ports.AttachmentPersister by delegating to Store.
type StorePersister struct {
	store *Store
}

// NewStorePersister creates a persister backed by the given Store.
func NewStorePersister(store *Store) *StorePersister {
	return &StorePersister{store: store}
}

// Persist writes the inline payload to the backing Store and returns a copy
// with URI populated and Data cleared (subject to inline retention rules).
func (p *StorePersister) Persist(ctx context.Context, att ports.Attachment) (ports.Attachment, error) {
	if p == nil || p.store == nil {
		return att, nil
	}
	if ctx != nil && ctx.Err() != nil {
		return att, ctx.Err()
	}

	// Already has an external URI and no inline data → nothing to do.
	if att.Data == "" && !isDataURI(att.URI) && strings.TrimSpace(att.URI) != "" {
		if att.Fingerprint == "" {
			att.Fingerprint = fingerprintFromURI(att.URI)
		}
		return att, nil
	}

	payload, mediaType := decodeAttachmentInline(att)
	if len(payload) == 0 {
		return att, nil
	}

	if att.Fingerprint == "" {
		att.Fingerprint = attachmentFingerprint(payload)
	}

	uri, err := p.store.StoreBytes(att.Name, mediaType, payload)
	if err != nil {
		return att, err
	}

	att.URI = uri
	if att.MediaType == "" {
		att.MediaType = mediaType
	}

	if shouldRetainInline(att.MediaType, len(payload)) {
		att.Data = base64.StdEncoding.EncodeToString(payload)
	} else {
		att.Data = ""
	}

	return att, nil
}

// decodeAttachmentInline extracts the raw bytes and media type from the
// attachment's inline payload (Data field or data: URI).
func decodeAttachmentInline(att ports.Attachment) ([]byte, string) {
	mediaType := strings.TrimSpace(att.MediaType)
	if mediaType == "" {
		mediaType = "application/octet-stream"
	}

	// Prefer explicit Data field.
	if att.Data != "" {
		lower := strings.ToLower(strings.TrimSpace(att.Data))
		if strings.HasPrefix(lower, "data:") {
			decoded, mt := decodeDataURIPayload(att.Data)
			if mt != "" {
				mediaType = mt
			}
			return decoded, mediaType
		}
		decoded, err := DecodeBase64(strings.TrimSpace(att.Data))
		if err != nil || len(decoded) == 0 {
			return nil, ""
		}
		return decoded, mediaType
	}

	// Fallback to data: URI.
	if isDataURI(att.URI) {
		decoded, mt := decodeDataURIPayload(att.URI)
		if mt != "" {
			mediaType = mt
		}
		return decoded, mediaType
	}

	return nil, ""
}

func decodeDataURIPayload(uri string) ([]byte, string) {
	match := dataURIPattern.FindStringSubmatch(strings.TrimSpace(uri))
	if match == nil {
		return nil, ""
	}
	mimeType := strings.TrimSpace(match[1])
	meta := strings.ToLower(match[2])
	payload := strings.TrimSpace(match[3])
	if payload == "" {
		return nil, ""
	}
	if !strings.Contains(meta, "base64") {
		return nil, ""
	}
	decoded, err := DecodeBase64(payload)
	if err != nil || len(decoded) == 0 {
		return nil, ""
	}
	if mimeType == "" {
		mimeType = http.DetectContentType(decoded)
	}
	return decoded, mimeType
}

func isDataURI(uri string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(uri)), "data:")
}

func attachmentFingerprint(payload []byte) string {
	if len(payload) == 0 {
		return ""
	}
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func fingerprintFromURI(uri string) string {
	trimmed := strings.TrimSpace(uri)
	if trimmed == "" || strings.HasPrefix(strings.ToLower(trimmed), "data:") {
		return ""
	}
	parsed, err := url.Parse(trimmed)
	pathValue := trimmed
	if err == nil && parsed.Path != "" {
		pathValue = parsed.Path
	}
	base := strings.ToLower(strings.TrimSpace(path.Base(pathValue)))
	if base == "" {
		return ""
	}
	if !attachmentFilePattern.MatchString(base) {
		return ""
	}
	if idx := strings.IndexByte(base, '.'); idx > 0 {
		base = base[:idx]
	}
	return base
}

// shouldRetainInline returns true when the decoded payload is small enough
// and text-like enough to keep inline after persistence.
func shouldRetainInline(mediaType string, size int) bool {
	if size <= 0 || size > inlineRetentionLimit {
		return false
	}
	media := strings.ToLower(strings.TrimSpace(mediaType))
	if media == "" {
		return false
	}
	if strings.HasPrefix(media, "text/") {
		return true
	}
	return strings.Contains(media, "markdown") || strings.Contains(media, "json")
}
