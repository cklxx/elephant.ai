package craftsync

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ArtifactMetadata captures the identifying fields for a craft artifact mirrored to the sandbox.
type ArtifactMetadata struct {
	ID            string    `json:"id"`
	UserID        string    `json:"user_id"`
	SessionID     string    `json:"session_id"`
	Name          string    `json:"name,omitempty"`
	MediaType     string    `json:"media_type,omitempty"`
	Description   string    `json:"description,omitempty"`
	Source        string    `json:"source,omitempty"`
	StorageKey    string    `json:"storage_key,omitempty"`
	URI           string    `json:"uri,omitempty"`
	Size          int64     `json:"size,omitempty"`
	Checksum      string    `json:"checksum,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	LocalFilename string    `json:"local_filename,omitempty"`
}

// Mirror persists craft artifacts into a sandbox-visible location.
type Mirror interface {
	Mirror(ctx context.Context, meta ArtifactMetadata, content []byte) (string, error)
	Remove(ctx context.Context, meta ArtifactMetadata) error
}

// FilesystemMirror writes artifacts to the local filesystem for sandbox sharing.
type FilesystemMirror struct {
	baseDir        string
	metadataName   string
	dirPermission  os.FileMode
	filePermission os.FileMode
}

// FilesystemOption customises filesystem mirror behaviour.
type FilesystemOption func(*FilesystemMirror)

// WithMetadataName overrides the metadata filename written alongside mirrored content.
func WithMetadataName(name string) FilesystemOption {
	return func(m *FilesystemMirror) {
		if strings.TrimSpace(name) != "" {
			m.metadataName = name
		}
	}
}

// WithPermissions overrides the directory and file permissions used for mirrored artifacts.
func WithPermissions(dirPerm, filePerm os.FileMode) FilesystemOption {
	return func(m *FilesystemMirror) {
		if dirPerm != 0 {
			m.dirPermission = dirPerm
		}
		if filePerm != 0 {
			m.filePermission = filePerm
		}
	}
}

// NewFilesystemMirror creates a filesystem-backed mirror rooted at baseDir.
func NewFilesystemMirror(baseDir string, opts ...FilesystemOption) (*FilesystemMirror, error) {
	if strings.TrimSpace(baseDir) == "" {
		baseDir = "data/crafts-mirror"
	}
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		return nil, fmt.Errorf("create craft mirror dir: %w", err)
	}
	mirror := &FilesystemMirror{
		baseDir:        baseDir,
		metadataName:   "metadata.json",
		dirPermission:  0o755,
		filePermission: 0o644,
	}
	for _, opt := range opts {
		opt(mirror)
	}
	return mirror, nil
}

// Mirror writes the artifact content and metadata to disk and returns the content path.
func (m *FilesystemMirror) Mirror(ctx context.Context, meta ArtifactMetadata, content []byte) (string, error) {
	if m == nil {
		return "", errors.New("filesystem mirror not configured")
	}
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return "", err
		}
	}
	if strings.TrimSpace(meta.ID) == "" {
		return "", errors.New("artifact id is required")
	}
	if strings.TrimSpace(meta.UserID) == "" {
		return "", errors.New("user id is required")
	}

	userDir := sanitizePathSegment(meta.UserID)
	sessionDir := sanitizePathSegment(meta.SessionID)
	artifactDir := filepath.Join(m.baseDir, userDir)
	if sessionDir != "" {
		artifactDir = filepath.Join(artifactDir, sessionDir)
	}
	artifactDir = filepath.Join(artifactDir, meta.ID)

	if err := os.MkdirAll(artifactDir, m.dirPermission); err != nil {
		return "", fmt.Errorf("create artifact dir: %w", err)
	}

	var (
		filename    string
		contentPath string
	)
	if len(content) > 0 {
		rawFilename := chooseFilename(meta.Name, meta.MediaType, meta.ID)
		safeFilename, err := ensureSafeFilename(rawFilename, meta.MediaType, meta.ID)
		if err != nil {
			return "", fmt.Errorf("derive safe filename: %w", err)
		}
		filename = safeFilename
		contentPath = filepath.Join(artifactDir, filename)
		if err := os.WriteFile(contentPath, content, m.filePermission); err != nil {
			return "", fmt.Errorf("write craft content: %w", err)
		}
	}

	metaCopy := meta
	metaCopy.LocalFilename = filename

	metadataPath := filepath.Join(artifactDir, m.metadataName)
	payload, err := json.MarshalIndent(metaCopy, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal metadata: %w", err)
	}
	if err := os.WriteFile(metadataPath, payload, m.filePermission); err != nil {
		return "", fmt.Errorf("write craft metadata: %w", err)
	}

	return contentPath, nil
}

// Remove deletes the mirrored artifact directory.
func (m *FilesystemMirror) Remove(ctx context.Context, meta ArtifactMetadata) error {
	if m == nil {
		return nil
	}
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	if strings.TrimSpace(meta.ID) == "" || strings.TrimSpace(meta.UserID) == "" {
		return nil
	}
	userDir := sanitizePathSegment(meta.UserID)
	sessionDir := sanitizePathSegment(meta.SessionID)
	artifactDir := filepath.Join(m.baseDir, userDir)
	if sessionDir != "" {
		artifactDir = filepath.Join(artifactDir, sessionDir)
	}
	artifactDir = filepath.Join(artifactDir, meta.ID)

	if err := os.RemoveAll(artifactDir); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove craft mirror: %w", err)
	}
	return nil
}

func chooseFilename(name, mediaType, fallback string) string {
	sanitized := sanitizeFilename(name)
	if sanitized == "" {
		sanitized = sanitizeFilename(fallback)
	}
	if sanitized == "" {
		sanitized = fallback
	}
	if filepath.Ext(sanitized) == "" {
		if ext := extensionForMediaType(mediaType); ext != "" {
			sanitized += ext
		}
	}
	return sanitized
}

func extensionForMediaType(mediaType string) string {
	switch strings.ToLower(strings.TrimSpace(mediaType)) {
	case "text/html", "text/html; charset=utf-8":
		return ".html"
	case "text/plain", "text/plain; charset=utf-8":
		return ".txt"
	case "application/json":
		return ".json"
	case "application/pdf":
		return ".pdf"
	case "image/png":
		return ".png"
	case "image/jpeg", "image/jpg":
		return ".jpg"
	case "image/gif":
		return ".gif"
	case "application/zip":
		return ".zip"
	default:
		return ""
	}
}

func sanitizeFilename(name string) string {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return ""
	}
	replacer := func(r rune) rune {
		switch r {
		case '/', '\\', ':', '*', '?', '"', '<', '>', '|':
			return '_'
		}
		if r < 32 {
			return '_'
		}
		return r
	}
	sanitized := strings.Map(replacer, trimmed)
	sanitized = strings.TrimSpace(sanitized)
	sanitized = strings.Trim(sanitized, ".")
	if sanitized == "" {
		return ""
	}
	return sanitized
}

func ensureSafeFilename(candidate, mediaType, fallbackID string) (string, error) {
	cleanedCandidate := collapseRepeatedDots(candidate)
	if isSafePathComponent(cleanedCandidate) {
		return cleanedCandidate, nil
	}

	fallback := collapseRepeatedDots(sanitizeFilename(fallbackID))
	if fallback == "" {
		fallback = "artifact"
	}

	ext := filepath.Ext(candidate)
	if ext == "" {
		ext = extensionForMediaType(mediaType)
	}

	safe := strings.Trim(fallback, ".")
	if safe == "" {
		safe = "artifact"
	}
	if ext != "" && !strings.HasSuffix(strings.ToLower(safe), strings.ToLower(ext)) {
		safe += ext
	}
	safe = collapseRepeatedDots(safe)
	safe = strings.Trim(safe, ".")
	if ext != "" && !strings.HasSuffix(safe, ext) {
		safe += ext
	}

	if !isSafePathComponent(safe) {
		return "", fmt.Errorf("invalid filename %q", safe)
	}

	return safe, nil
}

func collapseRepeatedDots(name string) string {
	for strings.Contains(name, "..") {
		name = strings.ReplaceAll(name, "..", ".")
	}
	return name
}

func isSafePathComponent(name string) bool {
	if name == "" {
		return false
	}
	if strings.ContainsAny(name, "/\\") {
		return false
	}
	if strings.Contains(name, "..") {
		return false
	}
	if filepath.Base(name) != name {
		return false
	}
	return true
}

func sanitizePathSegment(input string) string {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return "default"
	}
	var builder strings.Builder
	builder.Grow(len(trimmed))
	for _, r := range trimmed {
		switch {
		case r >= 'a' && r <= 'z':
			builder.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			builder.WriteRune(r)
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
		case r == '-' || r == '_' || r == '.':
			builder.WriteRune(r)
		default:
			builder.WriteRune('-')
		}
	}
	sanitized := strings.Trim(builder.String(), "-._")
	if sanitized == "" {
		return "default"
	}
	return sanitized
}
