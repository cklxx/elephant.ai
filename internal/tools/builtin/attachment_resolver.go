package builtin

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode"

	"alex/internal/agent/ports"
	"alex/internal/attachments"
	"alex/internal/config"
	"alex/internal/httpclient"
	"alex/internal/logging"
)

const (
	attachmentFetchTimeout              = 45 * time.Second
	attachmentTLSHandshakeTimeout       = 30 * time.Second
	attachmentFetchMaxBytes       int64 = 25 << 20
	attachmentPathPrefix                = "/api/attachments/"
)

var dataURIBase64Pattern = regexp.MustCompile(`(?is)^data:([^;,]+)?(;[^,]*)?,\s*(.+)$`)

func newAttachmentHTTPClient(timeout time.Duration, loggerName string) *http.Client {
	logger := logging.NewComponentLogger(loggerName)
	client := httpclient.New(timeout, logger)
	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		return client
	}
	cloned := transport.Clone()
	handshakeTimeout := attachmentTLSHandshakeTimeout
	if timeout > 0 && timeout < handshakeTimeout {
		handshakeTimeout = timeout
	}
	cloned.TLSHandshakeTimeout = handshakeTimeout
	client.Transport = cloned
	return client
}

func resolveAttachmentBytes(ctx context.Context, ref string, client *http.Client) ([]byte, string, error) {
	trimmed := strings.TrimSpace(ref)
	if trimmed == "" {
		return nil, "", errors.New("empty attachment reference")
	}
	if client == nil {
		client = newAttachmentHTTPClient(attachmentFetchTimeout, "AttachmentFetch")
	}

	if att, ok := resolveAttachmentFromContext(ctx, trimmed); ok {
		return resolveBytesFromAttachment(ctx, att, client)
	}
	return resolveBytesFromReference(ctx, trimmed, client)
}

func resolveAttachmentFromContext(ctx context.Context, ref string) (ports.Attachment, bool) {
	attachments, _ := ports.GetAttachmentContext(ctx)
	if len(attachments) == 0 {
		return ports.Attachment{}, false
	}

	name := normalizePlaceholder(ref)
	if name == "" {
		name = ref
	}
	if att, ok := lookupAttachmentCaseInsensitive(attachments, name); ok {
		return att, true
	}
	if looksLikeURL(ref) {
		if att, ok := lookupAttachmentByURI(attachments, ref); ok {
			return att, true
		}
	}
	return ports.Attachment{}, false
}

func resolveBytesFromAttachment(ctx context.Context, att ports.Attachment, client *http.Client) ([]byte, string, error) {
	if payload := strings.TrimSpace(att.Data); payload != "" {
		decoded, mediaType, err := decodeInlinePayload(payload, att.MediaType)
		if err != nil {
			return nil, "", err
		}
		return decoded, mediaType, nil
	}

	uri := resolveAttachmentURI(att)
	if uri == "" {
		return nil, "", errors.New("attachment has no payload")
	}

	lower := strings.ToLower(uri)
	if strings.HasPrefix(lower, "data:") {
		decoded, mediaType, err := decodeDataURI(uri)
		if err != nil {
			return nil, "", err
		}
		if mediaType == "" {
			mediaType = strings.TrimSpace(att.MediaType)
		}
		if mediaType == "" {
			mediaType = http.DetectContentType(decoded)
		}
		return decoded, mediaType, nil
	}

	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") {
		return fetchAttachmentBytes(ctx, client, uri, att.MediaType)
	}

	if strings.HasPrefix(uri, attachmentPathPrefix) {
		if payload, mediaType, ok, err := readLocalAttachment(uri, att.MediaType); ok {
			return payload, mediaType, err
		}
	}

	return nil, "", fmt.Errorf("attachment %s has unsupported uri %q", att.Name, uri)
}

func resolveBytesFromReference(ctx context.Context, ref string, client *http.Client) ([]byte, string, error) {
	lower := strings.ToLower(ref)
	if strings.HasPrefix(lower, "data:") {
		return decodeDataURI(ref)
	}
	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") {
		return fetchAttachmentBytes(ctx, client, ref, "")
	}
	if strings.HasPrefix(ref, attachmentPathPrefix) {
		if payload, mediaType, ok, err := readLocalAttachment(ref, ""); ok {
			return payload, mediaType, err
		}
	}
	if looksLikeBase64(ref) {
		decoded, err := decodeBase64Payload(ref)
		if err != nil {
			return nil, "", err
		}
		return decoded, http.DetectContentType(decoded), nil
	}
	return nil, "", fmt.Errorf("unsupported attachment reference %q", ref)
}

func resolveAttachmentURI(att ports.Attachment) string {
	if uri := strings.TrimSpace(att.URI); uri != "" {
		return uri
	}
	if len(att.PreviewAssets) == 0 {
		return ""
	}

	for _, asset := range att.PreviewAssets {
		cdn := strings.TrimSpace(asset.CDNURL)
		if cdn == "" {
			continue
		}
		mimeType := strings.ToLower(strings.TrimSpace(asset.MimeType))
		if strings.HasPrefix(mimeType, "image/") {
			return cdn
		}
	}
	for _, asset := range att.PreviewAssets {
		if cdn := strings.TrimSpace(asset.CDNURL); cdn != "" {
			return cdn
		}
	}
	return ""
}

func fetchAttachmentBytes(ctx context.Context, client *http.Client, uri string, fallbackMediaType string) ([]byte, string, error) {
	requestCtx := ctx
	if requestCtx == nil {
		requestCtx = context.Background()
	}
	req, err := http.NewRequestWithContext(requestCtx, http.MethodGet, uri, nil)
	if err != nil {
		return nil, "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, "", fmt.Errorf("unexpected HTTP status %s", resp.Status)
	}

	reader := io.LimitReader(resp.Body, attachmentFetchMaxBytes+1)
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, "", err
	}
	if int64(len(data)) > attachmentFetchMaxBytes {
		return nil, "", fmt.Errorf("attachment exceeds %d bytes", attachmentFetchMaxBytes)
	}
	if len(data) == 0 {
		return nil, "", errors.New("empty attachment payload")
	}

	mediaType := strings.TrimSpace(resp.Header.Get("Content-Type"))
	if parsed, _, err := mime.ParseMediaType(mediaType); err == nil && parsed != "" {
		mediaType = parsed
	} else {
		mediaType = ""
	}
	if mediaType == "" {
		mediaType = strings.TrimSpace(fallbackMediaType)
		if parsed, _, err := mime.ParseMediaType(mediaType); err == nil && parsed != "" {
			mediaType = parsed
		} else {
			mediaType = ""
		}
	}
	if mediaType == "" {
		mediaType = http.DetectContentType(data)
	}

	return data, mediaType, nil
}

func readLocalAttachment(uri string, fallbackMediaType string) ([]byte, string, bool, error) {
	if !strings.HasPrefix(uri, attachmentPathPrefix) {
		return nil, "", false, nil
	}
	fileCfg, _, err := config.LoadFileConfig(config.WithEnv(config.DefaultEnvLookup))
	if err != nil {
		return nil, "", true, err
	}
	if fileCfg.Attachments == nil {
		return nil, "", true, errors.New("attachment store not configured")
	}

	raw := fileCfg.Attachments
	provider := strings.TrimSpace(raw.Provider)
	if provider == "" {
		provider = attachments.ProviderLocal
	}
	if provider != attachments.ProviderLocal {
		return nil, "", true, fmt.Errorf("attachment store provider %q is not local", provider)
	}
	dir := strings.TrimSpace(raw.Dir)
	if dir == "" {
		return nil, "", true, errors.New("attachment store dir is empty")
	}
	if strings.HasPrefix(dir, "~/") {
		if home, err := os.UserHomeDir(); err == nil && strings.TrimSpace(home) != "" {
			dir = filepath.Join(home, strings.TrimPrefix(dir, "~/"))
		}
	}
	dir = filepath.Clean(dir)

	relative := strings.TrimPrefix(uri, attachmentPathPrefix)
	relative = path.Clean("/" + relative)
	relative = strings.TrimPrefix(relative, "/")
	if relative == "" || relative == "." {
		return nil, "", true, errors.New("attachment path is empty")
	}

	pathOnDisk := filepath.Join(dir, filepath.FromSlash(relative))
	if rel, err := filepath.Rel(dir, pathOnDisk); err != nil || strings.HasPrefix(rel, "..") {
		return nil, "", true, errors.New("attachment path escapes store dir")
	}
	data, err := os.ReadFile(pathOnDisk)
	if err != nil {
		return nil, "", true, err
	}
	mediaType := strings.TrimSpace(fallbackMediaType)
	if mediaType == "" {
		mediaType = http.DetectContentType(data)
	}
	return data, mediaType, true, nil
}

func decodeInlinePayload(payload string, mediaType string) ([]byte, string, error) {
	lower := strings.ToLower(strings.TrimSpace(payload))
	if strings.HasPrefix(lower, "data:") {
		decoded, mimeType, err := decodeDataURI(payload)
		if err != nil {
			return nil, "", err
		}
		if mimeType == "" {
			mimeType = strings.TrimSpace(mediaType)
		}
		if mimeType == "" {
			mimeType = http.DetectContentType(decoded)
		}
		return decoded, mimeType, nil
	}

	decoded, err := decodeBase64Payload(payload)
	if err != nil {
		return nil, "", err
	}
	resolvedMediaType := strings.TrimSpace(mediaType)
	if resolvedMediaType == "" {
		resolvedMediaType = http.DetectContentType(decoded)
	}
	return decoded, resolvedMediaType, nil
}

func decodeDataURI(value string) ([]byte, string, error) {
	match := dataURIBase64Pattern.FindStringSubmatch(strings.TrimSpace(value))
	if match == nil {
		return nil, "", errors.New("invalid data URI")
	}
	mimeType := strings.TrimSpace(match[1])
	meta := strings.ToLower(match[2])
	payload := strings.TrimSpace(match[3])
	if payload == "" {
		return nil, "", errors.New("empty data URI payload")
	}
	if !strings.Contains(meta, "base64") {
		return nil, "", errors.New("only base64 data URIs are supported")
	}
	decoded, err := decodeBase64Payload(payload)
	if err != nil {
		return nil, "", fmt.Errorf("decode data URI: %w", err)
	}
	if mimeType == "" {
		mimeType = http.DetectContentType(decoded)
	}
	return decoded, mimeType, nil
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
		return nil, errors.New("payload is empty")
	}
	return decoded, nil
}

func normalizePlaceholder(value string) string {
	trimmed := strings.TrimSpace(value)
	if len(trimmed) < 3 {
		return ""
	}
	if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
		return strings.TrimSpace(trimmed[1 : len(trimmed)-1])
	}
	return ""
}

func lookupAttachmentCaseInsensitive(attachments map[string]ports.Attachment, name string) (ports.Attachment, bool) {
	if attachments == nil {
		return ports.Attachment{}, false
	}
	if att, ok := attachments[name]; ok {
		return att, true
	}
	for key, att := range attachments {
		if strings.EqualFold(key, name) {
			return att, true
		}
	}
	return ports.Attachment{}, false
}

func lookupAttachmentByURI(attachments map[string]ports.Attachment, uri string) (ports.Attachment, bool) {
	if attachments == nil {
		return ports.Attachment{}, false
	}
	trimmed := strings.TrimSpace(uri)
	if trimmed == "" {
		return ports.Attachment{}, false
	}
	for _, att := range attachments {
		if strings.EqualFold(strings.TrimSpace(att.URI), trimmed) {
			return att, true
		}
	}
	return ports.Attachment{}, false
}

func looksLikeURL(value string) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return false
	}
	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.Scheme == "" {
		return false
	}
	scheme := strings.ToLower(parsed.Scheme)
	return scheme == "http" || scheme == "https"
}

func looksLikeBase64(value string) bool {
	trimmed := strings.TrimSpace(value)
	if len(trimmed) < 16 {
		return false
	}
	for _, r := range trimmed {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case r == '+', r == '/', r == '=':
		default:
			return false
		}
	}
	return true
}
