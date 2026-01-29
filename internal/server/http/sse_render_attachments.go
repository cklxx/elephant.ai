package http

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"alex/internal/agent/ports"
)

func sanitizeAttachmentsForStream(attachments map[string]ports.Attachment, sent *stringLRU, cache *DataCache, store *AttachmentStore, forceInclude bool) map[string]ports.Attachment {
	if len(attachments) == 0 {
		return nil
	}

	sanitized := make(map[string]ports.Attachment, len(attachments))
	for name, attachment := range attachments {
		sanitized[name] = normalizeAttachmentPayload(attachment, cache, store)
	}

	if forceInclude {
		for name, attachment := range sanitized {
			sent.Set(name, attachmentDigest(attachment))
		}
		return sanitized
	}

	// Fast-path: when nothing has been sent yet, reuse the original map to
	// avoid duplicating attachment payloads in memory. We still populate the
	// sent registry so duplicates can be skipped on later deliveries.
	if sent.Len() == 0 {
		for name, attachment := range sanitized {
			sent.Set(name, attachmentDigest(attachment))
		}
		return sanitized
	}

	var unsent map[string]ports.Attachment
	for name, attachment := range sanitized {
		digest := attachmentDigest(attachment)
		if prevDigest, alreadySent := sent.Get(name); alreadySent && prevDigest == digest {
			continue
		}
		if unsent == nil {
			unsent = make(map[string]ports.Attachment)
		}
		unsent[name] = attachment
		sent.Set(name, digest)
	}

	return unsent
}

func isHTMLAttachment(att ports.Attachment) bool {
	media := strings.ToLower(strings.TrimSpace(att.MediaType))
	format := strings.ToLower(strings.TrimSpace(att.Format))
	profile := strings.ToLower(strings.TrimSpace(att.PreviewProfile))
	return strings.Contains(media, "html") || format == "html" || strings.Contains(profile, "document.html")
}

func shouldPersistHTML(att ports.Attachment) bool {
	if !isHTMLAttachment(att) {
		return false
	}
	if strings.TrimSpace(att.URI) != "" && !strings.HasPrefix(strings.ToLower(strings.TrimSpace(att.URI)), "data:") && strings.TrimSpace(att.Data) == "" {
		return false
	}
	return true
}

func persistHTMLAttachment(att ports.Attachment, store *AttachmentStore) (ports.Attachment, bool) {
	if store == nil || !shouldPersistHTML(att) {
		return att, false
	}

	mediaType := strings.TrimSpace(att.MediaType)
	if mediaType == "" {
		mediaType = "text/html"
	}

	var payload []byte
	switch {
	case att.Data != "":
		if decoded, err := base64.StdEncoding.DecodeString(att.Data); err == nil {
			payload = decoded
		}
	case strings.HasPrefix(att.URI, "data:"):
		if ct, decoded, ok := decodeDataURI(att.URI); ok {
			if ct != "" {
				mediaType = ct
			}
			payload = decoded
		}
	}

	if len(payload) == 0 {
		return att, false
	}

	uri, err := store.StoreBytes(att.Name, mediaType, payload)
	if err != nil || strings.TrimSpace(uri) == "" {
		return att, false
	}

	att.URI = uri
	att.Data = ""
	if att.MediaType == "" {
		att.MediaType = mediaType
	}
	return ensureHTMLPreview(att), true
}

func ensureHTMLPreview(att ports.Attachment) ports.Attachment {
	if !isHTMLAttachment(att) {
		return att
	}

	if att.MediaType == "" {
		att.MediaType = "text/html"
	}
	if att.Format == "" {
		att.Format = "html"
	}
	if att.PreviewProfile == "" {
		att.PreviewProfile = "document.html"
	}

	hasHTMLPreview := false
	for _, asset := range att.PreviewAssets {
		if strings.Contains(strings.ToLower(asset.MimeType), "html") {
			hasHTMLPreview = true
			break
		}
	}

	if !hasHTMLPreview && strings.TrimSpace(att.URI) != "" {
		att.PreviewAssets = append(att.PreviewAssets, ports.AttachmentPreviewAsset{
			AssetID:     fmt.Sprintf("%s-html", strings.TrimSpace(att.Name)),
			Label:       "HTML preview",
			MimeType:    att.MediaType,
			CDNURL:      att.URI,
			PreviewType: "iframe",
		})
	}

	return att
}

func shouldRetainInlinePayload(mediaType string, size int) bool {
	if size <= 0 || size > inlineAttachmentRetentionLimit {
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

// persistToStore decodes inline attachment payloads (base64 Data or data: URIs)
// and writes them to the persistent AttachmentStore. This works for any media
// type, not just HTML. Returns the updated attachment and true on success.
func persistToStore(att ports.Attachment, store *AttachmentStore) (ports.Attachment, bool) {
	if store == nil {
		return att, false
	}

	// Nothing inline to persist.
	if att.Data == "" && !strings.HasPrefix(att.URI, "data:") {
		return att, false
	}

	mediaType := strings.TrimSpace(att.MediaType)
	if mediaType == "" {
		mediaType = "application/octet-stream"
	}

	var payload []byte
	switch {
	case att.Data != "":
		if decoded, err := base64.StdEncoding.DecodeString(att.Data); err == nil {
			payload = decoded
		}
	case strings.HasPrefix(att.URI, "data:"):
		if ct, decoded, ok := decodeDataURI(att.URI); ok {
			if ct != "" {
				mediaType = ct
			}
			payload = decoded
		}
	}

	if len(payload) == 0 {
		return att, false
	}

	uri, err := store.StoreBytes(att.Name, mediaType, payload)
	if err != nil || strings.TrimSpace(uri) == "" {
		return att, false
	}

	att.URI = uri
	if att.MediaType == "" {
		att.MediaType = mediaType
	}
	// Retain inline payload for small text-like assets so frontends can
	// render them without an additional fetch.
	if shouldRetainInlinePayload(att.MediaType, len(payload)) {
		att.Data = base64.StdEncoding.EncodeToString(payload)
	} else {
		att.Data = ""
	}
	return att, true
}

// normalizeAttachmentPayload converts inline payloads (Data or data URIs) into cache-backed URLs
// or persistent attachment store entries so SSE streams do not push large base64 blobs to the client.
func normalizeAttachmentPayload(att ports.Attachment, cache *DataCache, store *AttachmentStore) ports.Attachment {
	// HTML attachments get special preview enrichment via persistHTMLAttachment.
	if store != nil && shouldPersistHTML(att) {
		if rewritten, ok := persistHTMLAttachment(att, store); ok {
			return rewritten
		}
	}

	// For all other attachment types, try the persistent store first so
	// assets get CDN-backed, session-independent URLs.
	if store != nil {
		if rewritten, ok := persistToStore(att, store); ok {
			return ensureHTMLPreview(rewritten)
		}
	}

	// Already points to an external or cached resource.
	if att.Data == "" && att.URI != "" && !strings.HasPrefix(att.URI, "data:") {
		return ensureHTMLPreview(att)
	}

	if cache == nil {
		return att
	}

	mediaType := strings.TrimSpace(att.MediaType)
	if mediaType == "" {
		mediaType = "application/octet-stream"
	}

	// Prefer explicit data payloads.
	if att.Data != "" {
		if decoded, err := base64.StdEncoding.DecodeString(att.Data); err == nil && len(decoded) > 0 {
			if url := cache.StoreBytes(mediaType, decoded); url != "" {
				att.URI = url
				att.Data = ""
				if att.MediaType == "" {
					att.MediaType = mediaType
				}
				if shouldRetainInlinePayload(att.MediaType, len(decoded)) {
					att.Data = base64.StdEncoding.EncodeToString(decoded)
				}
				return ensureHTMLPreview(att)
			}
		}
	}

	// Fallback to data URIs when present.
	if strings.HasPrefix(att.URI, "data:") {
		rawURI := att.URI
		if cached := cache.MaybeStoreDataURI(rawURI); cached != nil {
			if url, ok := cached["url"].(string); ok && url != "" {
				att.URI = url
			}
			if ct, ok := cached["content_type"].(string); ok && ct != "" {
				att.MediaType = ct
			} else if att.MediaType == "" {
				att.MediaType = mediaType
			}
			if ct, payload, ok := decodeDataURI(rawURI); ok && shouldRetainInlinePayload(ct, len(payload)) {
				att.Data = base64.StdEncoding.EncodeToString(payload)
				if att.MediaType == "" {
					att.MediaType = ct
				}
			} else {
				att.Data = ""
			}
			return ensureHTMLPreview(att)
		}
	}

	return ensureHTMLPreview(att)
}

func sanitizeUntypedAttachments(value any, sent *stringLRU, cache *DataCache, store *AttachmentStore) any {
	raw, ok := value.(map[string]any)
	if !ok {
		return sanitizeEnvelopeValue(value, sent, cache, store)
	}

	attachments := make(map[string]ports.Attachment)
	for name, entry := range raw {
		entryMap, ok := entry.(map[string]any)
		if !ok || !isAttachmentRecord(entryMap) {
			continue
		}
		att := attachmentFromMap(entryMap)
		if att.Name == "" {
			att.Name = name
		}
		attachments[name] = att
	}

	if len(attachments) == 0 {
		return sanitizeEnvelopePayload(raw, sent, cache, store)
	}

	sanitized := sanitizeAttachmentsForStream(attachments, sent, cache, store, false)
	if len(sanitized) == 0 {
		return nil
	}
	return sanitized
}

func isAttachmentRecord(entry map[string]any) bool {
	if entry == nil {
		return false
	}
	_, hasData := entry["data"]
	_, hasURI := entry["uri"]
	_, hasMediaType := entry["media_type"]
	_, hasName := entry["name"]
	return hasData || hasURI || hasMediaType || hasName
}

func attachmentFromMap(entry map[string]any) ports.Attachment {
	att := ports.Attachment{}

	if v, ok := entry["name"].(string); ok {
		att.Name = v
	}
	if v, ok := entry["media_type"].(string); ok {
		att.MediaType = v
	}
	if v, ok := entry["uri"].(string); ok {
		att.URI = v
	}
	if v, ok := entry["data"].(string); ok {
		att.Data = v
	}
	if v, ok := entry["source"].(string); ok {
		att.Source = v
	}
	if v, ok := entry["description"].(string); ok {
		att.Description = v
	}
	if v, ok := entry["kind"].(string); ok {
		att.Kind = v
	}
	if v, ok := entry["format"].(string); ok {
		att.Format = v
	}

	return att
}

func attachmentDigest(att ports.Attachment) string {
	encoded, err := json.Marshal(att)
	if err != nil {
		return att.Name
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:])
}
