package app

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"sync"
	"time"

	"alex/internal/agent/ports"
	attachmentsutil "alex/internal/attachments"
	"alex/internal/tools"
	"alex/internal/utils"

	api "github.com/agent-infra/sandbox-sdk-go"
)

const (
	maxArchiveDuration          = 30 * time.Second
	maxRemoteAttachmentBytes    = 8 * 1024 * 1024 // 8 MiB safeguard for remote downloads
	remoteAttachmentHTTPTimeout = 15 * time.Second
)

// AttachmentArchiver persists generated attachments to a backing store.
type AttachmentArchiver interface {
	Persist(ctx context.Context, sessionID, taskID string, attachments map[string]ports.Attachment)
}

// AttachmentScanReporter emits events whenever the scanner returns a verdict
// that callers should surface to end users.
type AttachmentScanReporter interface {
	ReportAttachmentScan(sessionID, taskID, placeholder string, attachment ports.Attachment, result AttachmentScanResult)
}

// SandboxAttachmentArchiverConfig customizes remote mirroring behavior.
type SandboxAttachmentArchiverConfig struct {
	AllowedRemoteHosts []string
	BlockedRemoteHosts []string
	Scanner            AttachmentScanner
	ScanReporter       AttachmentScanReporter
}

// NewSandboxAttachmentArchiver returns an AttachmentArchiver that writes files into the sandbox workspace.
func NewSandboxAttachmentArchiver(manager *tools.SandboxManager, baseDir string, cfg SandboxAttachmentArchiverConfig) AttachmentArchiver {
	if manager == nil {
		return nil
	}
	dir := strings.TrimSpace(baseDir)
	if dir == "" {
		dir = attachmentsutil.DefaultWorkspaceSessionDir
	}
	return &sandboxAttachmentArchiver{
		sandbox: manager,
		baseDir: dir,
		logger:  utils.NewComponentLogger("AttachmentArchiver"),
		httpClient: &http.Client{
			Timeout: remoteAttachmentHTTPTimeout,
		},
		hostFilter:   newHostFilter(cfg),
		scanner:      cfg.Scanner,
		scanReporter: cfg.ScanReporter,
	}
}

type sandboxAttachmentArchiver struct {
	sandbox      *tools.SandboxManager
	baseDir      string
	logger       *utils.Logger
	ensuredDirs  sync.Map
	digestCache  sync.Map
	httpClient   *http.Client
	hostFilter   *hostFilter
	scanner      AttachmentScanner
	scanReporter AttachmentScanReporter
}

func (a *sandboxAttachmentArchiver) Persist(ctx context.Context, sessionID, taskID string, attachments map[string]ports.Attachment) {
	if len(attachments) == 0 {
		return
	}
	session := attachmentsutil.SanitizeSessionID(sessionID)
	if session == "" {
		session = "session"
	}

	cloned := cloneAttachments(attachments)
	go a.write(context.Background(), session, taskID, cloned)
}

func (a *sandboxAttachmentArchiver) write(ctx context.Context, session, taskID string, attachments map[string]ports.Attachment) {
	if len(attachments) == 0 {
		return
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, maxArchiveDuration)
	defer cancel()

	if err := a.sandbox.Initialize(timeoutCtx); err != nil {
		a.logger.Warn("Sandbox initialization failed for attachment archive: %v", err)
		return
	}

	fileClient := a.sandbox.File()
	if fileClient == nil {
		a.logger.Warn("Sandbox file client unavailable; skipping attachment archive")
		return
	}

	targetDir := path.Join(a.baseDir, session, "attachments")
	if err := a.ensureDirectory(timeoutCtx, targetDir); err != nil {
		a.logger.Warn("Failed to ensure sandbox directory %s: %v", targetDir, err)
		return
	}

	cache := a.getSessionCache(session)

	for key, attachment := range attachments {

		payload, mediaType, ok := a.preparePayload(timeoutCtx, session, taskID, key, attachment)
		if !ok {
			continue
		}
		var err error

		digest := digestAttachment(payload)
		if existing, ok := cache.HasDigest(digest); ok {
			a.logger.Debug("Skipping duplicate attachment %s; already stored as %s", key, existing)
			continue
		}

		filename := attachmentsutil.SanitizeFileName(key, attachment.Name, mediaType)
		if filename == "" {
			filename = defaultAttachmentFilename(mediaType)
		}
		filename = cache.Reserve(filename)

		writeCtx, cancel := context.WithTimeout(context.Background(), maxArchiveDuration)
		_, err = fileClient.WriteFile(writeCtx, &api.FileWriteRequest{
			File:    path.Join(targetDir, filename),
			Content: base64.StdEncoding.EncodeToString(payload),
			Encoding: func() *api.FileContentEncoding {
				value := api.FileContentEncodingBase64
				return value.Ptr()
			}(),
		})
		cancel()
		if err != nil {
			cache.Release(filename)
			a.logger.Warn("Failed to write attachment %s/%s: %v", session, filename, err)
			continue
		}
		cache.Remember(digest, filename)
		a.logger.Debug("Archived attachment %s for session %s", filename, session)
	}
}

func (a *sandboxAttachmentArchiver) preparePayload(ctx context.Context, session, taskID, placeholder string, attachment ports.Attachment) ([]byte, string, bool) {
	payload, mediaType, err := a.decodeAttachmentPayload(ctx, attachment)
	if err != nil {
		if a != nil && a.logger != nil {
			a.logger.Debug("Skipping attachment %s: %v", placeholder, err)
		}
		return nil, "", false
	}
	if a == nil || a.scanner == nil {
		return payload, mediaType, true
	}
	result, err := a.scanner.Scan(ctx, AttachmentScanRequest{
		SessionID:   session,
		Placeholder: placeholder,
		Attachment:  attachment,
		MediaType:   mediaType,
		Payload:     payload,
	})
	if err != nil {
		if a.logger != nil {
			a.logger.Warn("Attachment scan failed for %s/%s: %v", session, placeholder, err)
		}
		return payload, mediaType, true
	}
	if result.Verdict == AttachmentScanVerdictInfected {
		if a.logger != nil {
			detail := result.Details
			if detail == "" {
				detail = "malware detected"
			}
			a.logger.Warn("Attachment %s flagged as infected; skipping (%s)", placeholder, detail)
		}
		a.reportScanVerdict(session, taskID, placeholder, attachment, result)
		return nil, "", false
	}
	return payload, mediaType, true
}

func (a *sandboxAttachmentArchiver) reportScanVerdict(session, taskID, placeholder string, attachment ports.Attachment, result AttachmentScanResult) {
	if a == nil || a.scanReporter == nil {
		return
	}
	if result.Verdict != AttachmentScanVerdictInfected {
		return
	}
	a.scanReporter.ReportAttachmentScan(session, taskID, placeholder, attachment, result)
}

func digestAttachment(payload []byte) string {
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func defaultAttachmentFilename(mediaType string) string {
	ext := attachmentsutil.InferExtension(mediaType)
	if ext == "" {
		ext = "bin"
	}
	return fmt.Sprintf("attachment_%d.%s", time.Now().UnixNano(), ext)
}

func (a *sandboxAttachmentArchiver) getSessionCache(session string) *attachmentCache {
	value, _ := a.digestCache.LoadOrStore(session, &attachmentCache{})
	cache, _ := value.(*attachmentCache)
	return cache
}

func (a *sandboxAttachmentArchiver) ensureDirectory(ctx context.Context, dir string) error {
	if dir == "" || dir == "/" {
		return nil
	}
	if _, loaded := a.ensuredDirs.LoadOrStore(dir, struct{}{}); loaded {
		return nil
	}

	shell := a.sandbox.Shell()
	if shell == nil {
		a.ensuredDirs.Delete(dir)
		return fmt.Errorf("sandbox shell client unavailable")
	}

	cmd := fmt.Sprintf("mkdir -p %s", shellQuote(dir))
	req := &api.ShellExecRequest{Command: cmd}

	execCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	resp, err := shell.ExecCommand(execCtx, req)
	if err != nil {
		a.ensuredDirs.Delete(dir)
		return err
	}

	data := resp.GetData()
	if data != nil {
		if exit := data.GetExitCode(); exit != nil && *exit != 0 {
			a.ensuredDirs.Delete(dir)
			return fmt.Errorf("mkdir exited with %d", *exit)
		}
	}
	return nil
}

func (a *sandboxAttachmentArchiver) decodeAttachmentPayload(ctx context.Context, att ports.Attachment) ([]byte, string, error) {
	data := strings.TrimSpace(att.Data)
	if data != "" {
		return decodeDataString(data, att.MediaType)
	}

	uri := strings.TrimSpace(att.URI)
	if uri != "" {
		if strings.HasPrefix(uri, "data:") {
			return decodeDataURI(uri)
		}
		return a.fetchRemoteAttachment(ctx, uri, att.MediaType)
	}

	return nil, "", fmt.Errorf("attachment %s missing inline data", att.Name)
}

func (a *sandboxAttachmentArchiver) fetchRemoteAttachment(ctx context.Context, rawURL, declaredMediaType string) ([]byte, string, error) {
	if a == nil || a.httpClient == nil {
		return nil, "", fmt.Errorf("remote attachment mirroring disabled: http client unavailable")
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, "", fmt.Errorf("invalid attachment URI: %w", err)
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return nil, "", fmt.Errorf("unsupported attachment URI scheme: %s", parsed.Scheme)
	}
	host := strings.ToLower(parsed.Hostname())
	if err := a.validateRemoteHost(host); err != nil {
		return nil, "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return nil, "", err
	}
	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("failed to download attachment: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= http.StatusBadRequest {
		return nil, "", fmt.Errorf("remote attachment download failed with status %d", resp.StatusCode)
	}
	if resp.ContentLength > maxRemoteAttachmentBytes {
		return nil, "", fmt.Errorf("remote attachment exceeds %d bytes", maxRemoteAttachmentBytes)
	}
	reader := io.LimitReader(resp.Body, maxRemoteAttachmentBytes+1)
	payload, err := io.ReadAll(reader)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read remote attachment: %w", err)
	}
	if len(payload) > maxRemoteAttachmentBytes {
		return nil, "", fmt.Errorf("remote attachment exceeds %d bytes", maxRemoteAttachmentBytes)
	}
	mediaType := strings.TrimSpace(declaredMediaType)
	if mediaType == "" {
		mediaType = sanitizeContentType(resp.Header.Get("Content-Type"))
	}
	if mediaType == "" {
		mediaType = "application/octet-stream"
	}
	return payload, mediaType, nil
}

func (a *sandboxAttachmentArchiver) validateRemoteHost(host string) error {
	if host == "" || a == nil || a.hostFilter == nil {
		return nil
	}
	if a.hostFilter.Allows(host) {
		return nil
	}
	return fmt.Errorf("remote attachment host %s not allowed", host)
}

func decodeDataString(raw string, mediaType string) ([]byte, string, error) {
	if strings.HasPrefix(raw, "data:") {
		return decodeDataURI(raw)
	}
	decoded, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return nil, "", fmt.Errorf("invalid base64 payload: %w", err)
	}
	return decoded, mediaType, nil
}

func decodeDataURI(raw string) ([]byte, string, error) {
	if !strings.HasPrefix(raw, "data:") {
		return nil, "", fmt.Errorf("invalid data URI")
	}
	parts := strings.SplitN(raw, ",", 2)
	if len(parts) != 2 {
		return nil, "", fmt.Errorf("invalid data URI")
	}

	meta := strings.TrimPrefix(parts[0], "data:")
	payload := parts[1]

	mediaType := "application/octet-stream"
	base64Encoded := false

	if meta != "" {
		for _, segment := range strings.Split(meta, ";") {
			segment = strings.TrimSpace(segment)
			if segment == "" {
				continue
			}
			if segment == "base64" {
				base64Encoded = true
				continue
			}
			if !strings.Contains(segment, "/") {
				continue
			}
			mediaType = segment
		}
	}

	if base64Encoded || isLikelyBase64(payload) {
		decoded, err := base64.StdEncoding.DecodeString(payload)
		return decoded, mediaType, err
	}

	decoded, err := url.QueryUnescape(payload)
	if err != nil {
		return nil, "", err
	}
	return []byte(decoded), mediaType, nil
}

func isLikelyBase64(value string) bool {
	if len(value)%4 != 0 {
		return false
	}
	for _, r := range value {
		if r == '=' || (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '+' || r == '/' {
			continue
		}
		return false
	}
	return true
}

func cloneAttachments(values map[string]ports.Attachment) map[string]ports.Attachment {
	if len(values) == 0 {
		return nil
	}
	cloned := make(map[string]ports.Attachment, len(values))
	for key, att := range values {
		cloned[key] = att
	}
	return cloned
}

type hostFilter struct {
	allowAll      bool
	allowedExact  map[string]struct{}
	allowedSuffix []string
	blockedExact  map[string]struct{}
	blockedSuffix []string
}

func newHostFilter(cfg SandboxAttachmentArchiverConfig) *hostFilter {
	filter := &hostFilter{allowAll: len(cfg.AllowedRemoteHosts) == 0}
	filter.allowedExact, filter.allowedSuffix = compileHostPatterns(cfg.AllowedRemoteHosts)
	filter.blockedExact, filter.blockedSuffix = compileHostPatterns(cfg.BlockedRemoteHosts)
	if filter.allowAll && len(filter.blockedExact) == 0 && len(filter.blockedSuffix) == 0 {
		if len(filter.allowedExact) == 0 && len(filter.allowedSuffix) == 0 {
			return nil
		}
	}
	return filter
}

func compileHostPatterns(values []string) (map[string]struct{}, []string) {
	if len(values) == 0 {
		return nil, nil
	}
	exact := make(map[string]struct{})
	var suffixes []string
	for _, raw := range values {
		value := strings.TrimSpace(strings.ToLower(raw))
		if value == "" {
			continue
		}
		if strings.HasPrefix(value, "*.") {
			suffix := strings.TrimPrefix(value, "*")
			if !strings.HasPrefix(suffix, ".") {
				suffix = "." + strings.TrimPrefix(suffix, ".")
			}
			suffixes = append(suffixes, suffix)
			continue
		}
		if strings.HasPrefix(value, ".") {
			suffixes = append(suffixes, value)
			continue
		}
		exact[value] = struct{}{}
	}
	return exact, suffixes
}

func (f *hostFilter) Allows(host string) bool {
	if f == nil {
		return true
	}
	value := strings.ToLower(strings.TrimSpace(host))
	if value == "" {
		return false
	}
	if _, blocked := f.blockedExact[value]; blocked {
		return false
	}
	for _, suffix := range f.blockedSuffix {
		if strings.HasSuffix(value, suffix) {
			return false
		}
	}
	if f.allowAll {
		return true
	}
	if _, ok := f.allowedExact[value]; ok {
		return true
	}
	for _, suffix := range f.allowedSuffix {
		if strings.HasSuffix(value, suffix) {
			return true
		}
	}
	return false
}

func shellQuote(value string) string {
	if !strings.ContainsRune(value, '\'') {
		return fmt.Sprintf("'%s'", value)
	}
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

type attachmentCache struct {
	mu        sync.Mutex
	digests   map[string]string
	filenames map[string]struct{}
}

func (c *attachmentCache) HasDigest(hash string) (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.digests == nil {
		return "", false
	}
	filename, ok := c.digests[hash]
	return filename, ok
}

func (c *attachmentCache) Remember(hash, filename string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.digests == nil {
		c.digests = make(map[string]string)
	}
	c.digests[hash] = filename
	if c.filenames == nil {
		c.filenames = make(map[string]struct{})
	}
	c.filenames[filename] = struct{}{}
}

func (c *attachmentCache) Reserve(preferred string) string {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.filenames == nil {
		c.filenames = make(map[string]struct{})
	}

	base, ext := splitPreferredFilename(preferred)
	candidate := preferred
	if candidate == "" {
		candidate = base + ext
	}

	counter := 1
	for {
		if candidate == "" {
			candidate = fmt.Sprintf("attachment_%d%s", time.Now().UnixNano(), ext)
		}
		if _, exists := c.filenames[candidate]; !exists {
			c.filenames[candidate] = struct{}{}
			return candidate
		}
		candidate = fmt.Sprintf("%s_%d%s", base, counter, ext)
		counter++
	}
}

func (c *attachmentCache) Release(filename string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.filenames != nil {
		delete(c.filenames, filename)
	}
}

func splitPreferredFilename(preferred string) (string, string) {
	if strings.TrimSpace(preferred) == "" {
		return "attachment", ".bin"
	}
	ext := path.Ext(preferred)
	base := strings.TrimSuffix(preferred, ext)
	if base == "" {
		base = "attachment"
	}
	if ext == "" {
		ext = ".bin"
	}
	return base, ext
}

func sanitizeContentType(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if idx := strings.Index(value, ";"); idx != -1 {
		value = value[:idx]
	}
	return strings.TrimSpace(value)
}
