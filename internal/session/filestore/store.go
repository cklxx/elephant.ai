package filestore

import (
	"alex/internal/agent/ports"
	"alex/internal/utils"
	id "alex/internal/utils/id"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type store struct {
	baseDir string
	logger  *utils.Logger
}

func New(baseDir string) ports.SessionStore {
	if strings.HasPrefix(baseDir, "~/") {
		home, _ := os.UserHomeDir()
		baseDir = filepath.Join(home, baseDir[2:])
	}
	_ = os.MkdirAll(baseDir, 0755) // Ignore error - directory may already exist
	return &store{
		baseDir: baseDir,
		logger:  utils.NewComponentLogger("SessionFileStore"),
	}
}

func (s *store) Create(ctx context.Context) (*ports.Session, error) {
	sessionID := id.NewSessionID()

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

	// Create file exclusively (fail if exists)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
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
	path := filepath.Join(s.baseDir, fmt.Sprintf("%s.json", id))
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("session not found")
	}
	var session ports.Session
	if err := json.Unmarshal(data, &session); err != nil {
		if s.logger != nil {
			s.logger.Error("Failed to decode session file %s: %v. Preview: %s", path, err, previewJSON(data))
		}
		return nil, fmt.Errorf("failed to decode session: %w", err)
	}
	return &session, nil
}

func (s *store) Save(ctx context.Context, session *ports.Session) error {
	session.UpdatedAt = time.Now()
	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(s.baseDir, fmt.Sprintf("%s.json", session.ID))
	return os.WriteFile(path, data, 0644)
}

func (s *store) List(ctx context.Context) ([]string, error) {
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
