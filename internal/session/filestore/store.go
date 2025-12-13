package filestore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"alex/internal/agent/ports"
	"alex/internal/logging"
	id "alex/internal/utils/id"
)

type store struct {
	baseDir string
	logger  logging.Logger
}

var sessionIDPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// New creates a file-backed session store rooted at baseDir.
func New(baseDir string) ports.SessionStore {
	if strings.HasPrefix(baseDir, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			baseDir = filepath.Join(home, baseDir[2:])
		}
	}
	baseDir = filepath.Clean(baseDir)
	_ = os.MkdirAll(baseDir, 0o755) // ignore error – directory may already exist

	return &store{
		baseDir: baseDir,
		logger:  logging.NewComponentLogger("SessionFileStore"),
	}
}

func (s *store) Create(ctx context.Context) (*ports.Session, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	sessionID := id.NewSessionID()
	if !isSafeSessionID(sessionID) {
		return nil, fmt.Errorf("invalid session ID")
	}

	session := &ports.Session{
		ID:        sessionID,
		Messages:  []ports.Message{},
		Todos:     []ports.Todo{},
		Metadata:  make(map[string]string),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Save with O_CREATE|O_EXCL to prevent overwrites
	path := filepath.Join(s.baseDir, fmt.Sprintf("%s.json", session.ID))
	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return nil, err
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("failed to create session file: %w", err)
	}
	defer func() {
		if closeErr := f.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("failed to close session file: %w", closeErr)
		}
	}()

	if _, err := f.Write(data); err != nil {
		return nil, fmt.Errorf("failed to write session: %w", err)
	}

	return session, nil
}

func (s *store) Get(ctx context.Context, id string) (*ports.Session, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !isSafeSessionID(id) {
		return nil, fmt.Errorf("invalid session ID")
	}

	path := filepath.Join(s.baseDir, fmt.Sprintf("%s.json", id))
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("session not found")
	}

	var session ports.Session
	if err := json.Unmarshal(data, &session); err != nil {
		// Do not log file path or preview, as session file may contain secrets (API keys, etc.)
		logging.OrNop(s.logger).Error("Failed to decode session file: %v", err)
		return nil, fmt.Errorf("failed to decode session: %w", err)
	}
	session.Attachments = sanitizeAttachmentMap(session.Attachments)

	if attachments, err := s.loadAttachments(id); err != nil {
		return nil, err
	} else if len(attachments) > 0 {
		session.Attachments = mergeAttachmentMaps(session.Attachments, attachments, true)
	}
	return &session, nil
}

func (s *store) Save(ctx context.Context, session *ports.Session) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if session == nil {
		return fmt.Errorf("session cannot be nil")
	}
	if !isSafeSessionID(session.ID) {
		return fmt.Errorf("invalid session ID")
	}

	session.UpdatedAt = time.Now()
	attachments := sanitizeAttachmentMap(session.Attachments)

	sessionCopy := *session
	sessionCopy.Attachments = nil

	data, err := json.MarshalIndent(sessionCopy, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(s.baseDir, fmt.Sprintf("%s.json", session.ID))
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return err
	}

	attachmentsPath := attachmentPath(s.baseDir, session.ID)
	if len(attachments) == 0 {
		_ = os.Remove(attachmentsPath) // best-effort cleanup for empty attachment sets
		return nil
	}

	attachmentData, err := json.MarshalIndent(attachments, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal attachments: %w", err)
	}
	if err := os.WriteFile(attachmentsPath, attachmentData, 0o644); err != nil {
		return fmt.Errorf("failed to write attachments: %w", err)
	}

	return nil
}

func (s *store) List(ctx context.Context) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(s.baseDir)
	if err != nil {
		return nil, err
	}
	var ids []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") {
			ids = append(ids, strings.TrimSuffix(entry.Name(), ".json"))
		}
	}
	return ids, nil
}

func (s *store) Delete(ctx context.Context, id string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if !isSafeSessionID(id) {
		return fmt.Errorf("invalid session ID")
	}

	var combined error

	mainPath := filepath.Join(s.baseDir, fmt.Sprintf("%s.json", id))
	if err := os.Remove(mainPath); err != nil && !os.IsNotExist(err) {
		combined = errors.Join(combined, fmt.Errorf("remove session file: %w", err))
	}

	// Remove companion files (e.g., todos, attachments) that follow the session ID prefix.
	pattern := filepath.Join(s.baseDir, fmt.Sprintf("%s_*", id))
	if matches, globErr := filepath.Glob(pattern); globErr == nil {
		for _, match := range matches {
			if err := os.RemoveAll(match); err != nil && !os.IsNotExist(err) {
				combined = errors.Join(combined, fmt.Errorf("remove companion file %s: %w", match, err))
			}
		}
	} else {
		combined = errors.Join(combined, fmt.Errorf("expand companion file pattern: %w", globErr))
	}

	return combined
}

func isSafeSessionID(id string) bool {
	return sessionIDPattern.MatchString(id)
}

func previewJSON(data []byte) string {
	const maxPreview = 512
	preview := strings.TrimSpace(string(data))
	preview = strings.ReplaceAll(preview, "\n", " ")
	preview = strings.ReplaceAll(preview, "\t", " ")
	if len(preview) > maxPreview {
		preview = preview[:maxPreview] + "... (truncated)"
	}
	return preview
}

func attachmentPath(baseDir, sessionID string) string {
	return filepath.Join(baseDir, fmt.Sprintf("%s_attachments.json", sessionID))
}

func (s *store) loadAttachments(sessionID string) (map[string]ports.Attachment, error) {
	path := attachmentPath(s.baseDir, sessionID)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read attachments: %w", err)
	}
	var attachments map[string]ports.Attachment
	if err := json.Unmarshal(data, &attachments); err != nil {
		return nil, fmt.Errorf("failed to decode attachments: %w", err)
	}
	return sanitizeAttachmentMap(attachments), nil
}

func sanitizeAttachmentMap(values map[string]ports.Attachment) map[string]ports.Attachment {
	if len(values) == 0 {
		return nil
	}
	sanitized := make(map[string]ports.Attachment, len(values))
	for key, att := range values {
		name := strings.TrimSpace(key)
		if name == "" {
			name = strings.TrimSpace(att.Name)
		}
		if name == "" {
			continue
		}
		uri := strings.TrimSpace(att.URI)
		if uri == "" || strings.HasPrefix(strings.ToLower(uri), "data:") {
			continue
		}
		att.Name = name
		att.URI = uri
		att.Data = ""
		sanitized[name] = att
	}
	if len(sanitized) == 0 {
		return nil
	}
	return sanitized
}

func mergeAttachmentMaps(base, overrides map[string]ports.Attachment, override bool) map[string]ports.Attachment {
	if len(overrides) == 0 {
		return base
	}
	if base == nil {
		base = make(map[string]ports.Attachment, len(overrides))
	}
	for key, att := range overrides {
		if key == "" {
			continue
		}
		if att.Name == "" {
			att.Name = key
		}
		if _, exists := base[key]; exists && !override {
			continue
		}
		base[key] = att
	}
	return base
}
