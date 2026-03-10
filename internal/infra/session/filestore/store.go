package filestore

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"alex/internal/domain/agent/ports"
	storage "alex/internal/domain/agent/ports/storage"
	fstore "alex/internal/infra/filestore"
	"alex/internal/shared/json"
	"alex/internal/shared/logging"
	id "alex/internal/shared/utils/id"
)

type store struct {
	baseDir string
	logger  logging.Logger
}

type sessionEntry struct {
	id      string
	modTime time.Time
}

var sessionIDPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// New creates a file-backed session store rooted at baseDir.
func New(baseDir string) storage.SessionStore {
	baseDir = filepath.Clean(fstore.ResolvePath(baseDir, ""))
	_ = os.MkdirAll(baseDir, 0o755) // ignore error – directory may already exist

	return &store{
		baseDir: baseDir,
		logger:  logging.NewComponentLogger("SessionFileStore"),
	}
}

func (s *store) Create(ctx context.Context) (*storage.Session, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	sessionID := id.NewSessionID()
	if !isSafeSessionID(sessionID) {
		return nil, fmt.Errorf("invalid session ID")
	}

	session := storage.NewSession(sessionID, time.Now())

	// Save with O_CREATE|O_EXCL to prevent overwrites
	path := filepath.Join(s.baseDir, fmt.Sprintf("%s.json", session.ID))
	data, err := jsonx.MarshalIndent(session, "", "  ")
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

func (s *store) Get(ctx context.Context, id string) (*storage.Session, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !isSafeSessionID(id) {
		return nil, fmt.Errorf("invalid session ID")
	}

	path := filepath.Join(s.baseDir, fmt.Sprintf("%s.json", id))
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, storage.ErrSessionNotFound
	}

	var session storage.Session
	if err := jsonx.Unmarshal(data, &session); err != nil {
		// Do not log file path or preview, as session file may contain secrets (API keys, etc.)
		logging.OrNop(s.logger).Error("Failed to decode session file: %v", err)
		return nil, fmt.Errorf("failed to decode session: %w", err)
	}
	session.Attachments = sanitizeAttachmentMap(session.Attachments)

	if attachments, err := s.loadAttachments(id); err != nil {
		return nil, err
	} else if len(attachments) > 0 {
		session.Attachments = ports.MergeAttachmentMaps(session.Attachments, attachments, true)
	}
	return &session, nil
}

func (s *store) Save(ctx context.Context, session *storage.Session) error {
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

	data, err := jsonx.MarshalIndent(sessionCopy, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(s.baseDir, fmt.Sprintf("%s.json", session.ID))
	if err := fstore.AtomicWrite(path, data, 0o644); err != nil {
		return err
	}

	attachmentsPath := attachmentPath(s.baseDir, session.ID)
	if len(attachments) == 0 {
		_ = os.Remove(attachmentsPath) // best-effort cleanup for empty attachment sets
		return nil
	}

	attachmentData, err := jsonx.MarshalIndent(attachments, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal attachments: %w", err)
	}
	if err := fstore.AtomicWrite(attachmentsPath, attachmentData, 0o644); err != nil {
		return fmt.Errorf("failed to write attachments: %w", err)
	}

	return nil
}

func (s *store) List(ctx context.Context, limit int, offset int) ([]string, error) {
	entries, err := s.listSessionEntries(ctx)
	if err != nil {
		return nil, err
	}

	if limit <= 0 {
		limit = 200
	}
	if offset < 0 {
		offset = 0
	}

	if offset >= len(entries) {
		return []string{}, nil
	}
	end := offset + limit
	if end > len(entries) {
		end = len(entries)
	}

	ids := make([]string, 0, end-offset)
	for _, entry := range entries[offset:end] {
		ids = append(ids, entry.id)
	}
	return ids, nil
}

// ListSessionItems returns lightweight list rows without loading full message payloads.
func (s *store) ListSessionItems(ctx context.Context, limit int, offset int) ([]storage.SessionListItem, error) {
	entries, err := s.listSessionEntries(ctx)
	if err != nil {
		return nil, err
	}

	if limit <= 0 {
		limit = 200
	}
	if offset < 0 {
		offset = 0
	}
	if offset >= len(entries) {
		return []storage.SessionListItem{}, nil
	}
	end := offset + limit
	if end > len(entries) {
		end = len(entries)
	}

	items := make([]storage.SessionListItem, 0, end-offset)
	for _, entry := range entries[offset:end] {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		item, err := s.readSessionListItem(entry.id)
		if err != nil {
			continue
		}
		if item.ID == "" {
			item.ID = entry.id
		}
		if item.UpdatedAt.IsZero() {
			item.UpdatedAt = entry.modTime
		}
		if item.CreatedAt.IsZero() {
			item.CreatedAt = item.UpdatedAt
		}
		items = append(items, item)
	}
	return items, nil
}

func (s *store) listSessionEntries(ctx context.Context) ([]sessionEntry, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(s.baseDir)
	if err != nil {
		return nil, err
	}

	sessions := make([]sessionEntry, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".json") || strings.HasSuffix(name, "_attachments.json") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		sessions = append(sessions, sessionEntry{
			id:      strings.TrimSuffix(name, ".json"),
			modTime: info.ModTime(),
		})
	}

	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].modTime.After(sessions[j].modTime)
	})
	return sessions, nil
}

func (s *store) readSessionListItem(sessionID string) (storage.SessionListItem, error) {
	path := filepath.Join(s.baseDir, fmt.Sprintf("%s.json", sessionID))
	file, err := os.Open(path)
	if err != nil {
		return storage.SessionListItem{}, err
	}
	defer func() { _ = file.Close() }()

	var payload struct {
		ID        string            `json:"id"`
		Metadata  map[string]string `json:"metadata"`
		CreatedAt time.Time         `json:"created_at"`
		UpdatedAt time.Time         `json:"updated_at"`
	}
	if err := jsonx.NewDecoder(file).Decode(&payload); err != nil {
		return storage.SessionListItem{}, err
	}

	return storage.SessionListItem{
		ID:        strings.TrimSpace(payload.ID),
		Title:     strings.TrimSpace(payload.Metadata["title"]),
		CreatedAt: payload.CreatedAt,
		UpdatedAt: payload.UpdatedAt,
	}, nil
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
	if err := jsonx.Unmarshal(data, &attachments); err != nil {
		return nil, fmt.Errorf("failed to decode attachments: %w", err)
	}
	return sanitizeAttachmentMap(attachments), nil
}

func sanitizeAttachmentMap(values map[string]ports.Attachment) map[string]ports.Attachment {
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
