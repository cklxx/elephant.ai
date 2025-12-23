package http

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var attachmentFilePattern = regexp.MustCompile(`^[a-f0-9]{64}(\.[a-z0-9]{1,10})?$`)

// AttachmentStore persists decoded attachment payloads on disk and serves them via a stable URL.
//
// This is intentionally simple: it avoids storing base64 blobs in session/event payloads
// and replaces the old in-memory data cache with a file-backed store.
type AttachmentStore struct {
	dir string
}

func NewAttachmentStore(dir string) (*AttachmentStore, error) {
	trimmed := strings.TrimSpace(dir)
	if trimmed == "" {
		return nil, fmt.Errorf("attachment store dir is required")
	}
	if strings.HasPrefix(trimmed, "~/") {
		if home, err := os.UserHomeDir(); err == nil && strings.TrimSpace(home) != "" {
			trimmed = filepath.Join(home, strings.TrimPrefix(trimmed, "~/"))
		}
	}
	trimmed = filepath.Clean(trimmed)
	if err := os.MkdirAll(trimmed, 0o755); err != nil {
		return nil, fmt.Errorf("create attachment store dir: %w", err)
	}
	return &AttachmentStore{dir: trimmed}, nil
}

func (s *AttachmentStore) StoreBytes(name, mediaType string, data []byte) (string, error) {
	if s == nil {
		return "", fmt.Errorf("attachment store is nil")
	}
	if len(data) == 0 {
		return "", fmt.Errorf("attachment payload is empty")
	}

	hash := sha256.Sum256(data)
	id := hex.EncodeToString(hash[:])

	ext := sanitizeAttachmentExt(filepath.Ext(strings.TrimSpace(name)))
	if ext == "" {
		ext = extFromMediaType(mediaType)
	}

	filename := id + ext
	path := filepath.Join(s.dir, filename)
	if _, err := os.Stat(path); err == nil {
		return "/api/attachments/" + filename, nil
	} else if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("stat attachment: %w", err)
	}

	tmp, err := os.CreateTemp(s.dir, filename+".tmp-*")
	if err != nil {
		return "", fmt.Errorf("create temp attachment: %w", err)
	}
	tmpPath := tmp.Name()
	writeErr := func() error {
		if _, err := tmp.Write(data); err != nil {
			return err
		}
		return tmp.Close()
	}()
	if writeErr != nil {
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("write attachment: %w", writeErr)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		// Best effort: if another concurrent writer won the race, use the existing file.
		if _, statErr := os.Stat(path); statErr == nil {
			return "/api/attachments/" + filename, nil
		}
		return "", fmt.Errorf("finalize attachment: %w", err)
	}

	return "/api/attachments/" + filename, nil
}

func (s *AttachmentStore) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimSpace(strings.TrimPrefix(r.URL.Path, "/api/attachments/"))
		name = filepath.Base(name)
		if name == "" || !attachmentFilePattern.MatchString(strings.ToLower(name)) {
			http.NotFound(w, r)
			return
		}

		path := filepath.Join(s.dir, name)
		if rel, err := filepath.Rel(s.dir, path); err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			http.NotFound(w, r)
			return
		}
		http.ServeFile(w, r, path)
	})
}

func sanitizeAttachmentExt(ext string) string {
	trimmed := strings.ToLower(strings.TrimSpace(ext))
	if trimmed == "" {
		return ""
	}
	if !strings.HasPrefix(trimmed, ".") {
		return ""
	}
	trimmed = strings.TrimPrefix(trimmed, ".")
	if trimmed == "" || len(trimmed) > 10 {
		return ""
	}
	for _, r := range trimmed {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			continue
		}
		return ""
	}
	return "." + trimmed
}

func extFromMediaType(mediaType string) string {
	mt := strings.TrimSpace(mediaType)
	if mt == "" {
		return ""
	}
	exts, err := mime.ExtensionsByType(mt)
	if err != nil || len(exts) == 0 {
		return ""
	}
	return sanitizeAttachmentExt(exts[0])
}
