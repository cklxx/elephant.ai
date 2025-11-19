package app

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/url"
	"path"
	"regexp"
	"strings"
	"sync"
	"time"

	"alex/internal/agent/ports"
	"alex/internal/tools"
	"alex/internal/utils"

	api "github.com/agent-infra/sandbox-sdk-go"
)

const (
	defaultSandboxSessionDir = "/workspace/.alex/sessions"
	maxArchiveDuration       = 30 * time.Second
)

// AttachmentArchiver persists generated attachments to a backing store.
type AttachmentArchiver interface {
	Persist(ctx context.Context, sessionID string, attachments map[string]ports.Attachment)
}

// NewSandboxAttachmentArchiver returns an AttachmentArchiver that writes files into the sandbox workspace.
func NewSandboxAttachmentArchiver(manager *tools.SandboxManager, baseDir string) AttachmentArchiver {
	if manager == nil {
		return nil
	}
	dir := strings.TrimSpace(baseDir)
	if dir == "" {
		dir = defaultSandboxSessionDir
	}
	return &sandboxAttachmentArchiver{
		sandbox: manager,
		baseDir: dir,
		logger:  utils.NewComponentLogger("AttachmentArchiver"),
	}
}

type sandboxAttachmentArchiver struct {
	sandbox     *tools.SandboxManager
	baseDir     string
	logger      *utils.Logger
	ensuredDirs sync.Map
	digestCache sync.Map
}

func (a *sandboxAttachmentArchiver) Persist(ctx context.Context, sessionID string, attachments map[string]ports.Attachment) {
	if len(attachments) == 0 {
		return
	}
	session := sanitizeSessionID(sessionID)
	if session == "" {
		session = "session"
	}

	cloned := ports.CloneAttachmentMap(attachments)
	go a.write(context.Background(), session, cloned)
}

func (a *sandboxAttachmentArchiver) write(ctx context.Context, session string, attachments map[string]ports.Attachment) {
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
		if strings.EqualFold(strings.TrimSpace(attachment.Source), "user_upload") {
			continue
		}

		payload, mediaType, err := decodeAttachmentPayload(attachment)
		if err != nil {
			a.logger.Debug("Skipping attachment %s: %v", key, err)
			continue
		}

		digest := digestAttachment(payload)
		if existing, ok := cache.HasDigest(digest); ok {
			a.logger.Debug("Skipping duplicate attachment %s; already stored as %s", key, existing)
			continue
		}

		filename := sanitizeFileName(key, attachment.Name, mediaType)
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

func digestAttachment(payload []byte) string {
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func defaultAttachmentFilename(mediaType string) string {
	ext := inferExtension(mediaType)
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

func decodeAttachmentPayload(att ports.Attachment) ([]byte, string, error) {
	data := strings.TrimSpace(att.Data)
	if data != "" {
		return decodeDataString(data, att.MediaType)
	}

	uri := strings.TrimSpace(att.URI)
	if uri != "" {
		if strings.HasPrefix(uri, "data:") {
			return decodeDataURI(uri)
		}
		return nil, "", fmt.Errorf("remote URIs not supported for automatic archiving")
	}

	return nil, "", fmt.Errorf("attachment %s missing inline data", att.Name)
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

var (
	sessionSanitizer = regexp.MustCompile(`[^a-zA-Z0-9._-]`)
	fileSanitizer    = regexp.MustCompile(`[^a-zA-Z0-9._-]`)
)

func sanitizeSessionID(sessionID string) string {
	session := strings.TrimSpace(sessionID)
	session = sessionSanitizer.ReplaceAllString(session, "_")
	return strings.Trim(session, "._-")
}

func sanitizeFileName(candidate, attachmentName, mediaType string) string {
	name := strings.TrimSpace(attachmentName)
	if name == "" {
		name = strings.TrimSpace(candidate)
	}
	if name == "" {
		return ""
	}
	name = path.Base(name)
	name = fileSanitizer.ReplaceAllString(name, "_")
	if idx := strings.LastIndex(name, "."); idx == -1 {
		if ext := inferExtension(mediaType); ext != "" {
			name = name + "." + ext
		}
	}
	return name
}

func inferExtension(mediaType string) string {
	switch strings.ToLower(mediaType) {
	case "image/png":
		return "png"
	case "image/jpeg", "image/jpg":
		return "jpg"
	case "image/gif":
		return "gif"
	case "text/plain":
		return "txt"
	case "text/html":
		return "html"
	case "text/markdown":
		return "md"
	case "application/pdf":
		return "pdf"
	default:
		return ""
	}
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
