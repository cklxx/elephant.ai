package filestore

import (
	"alex/internal/agent/ports"
	"alex/internal/utils"
	id "alex/internal/utils/id"
	"context"
	"encoding/json"
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
	userID := id.UserIDFromContext(ctx)
	if userID == "" {
		return nil, fmt.Errorf("missing user context")
	}

	sessionID := id.NewSessionID()

	session := &ports.Session{
		ID:        sessionID,
		UserID:    userID,
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

func (s *store) Get(ctx context.Context, sessionID string) (*ports.Session, error) {
	path := filepath.Join(s.baseDir, fmt.Sprintf("%s.json", sessionID))
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}
	var session ports.Session
	if err := json.Unmarshal(data, &session); err != nil {
		if s.logger != nil {
			s.logger.Error("Failed to decode session file %s: %v. Preview: %s", path, err, previewJSON(data))
		}
		return nil, fmt.Errorf("failed to decode session %s: %w", sessionID, err)
	}
	if userID := id.UserIDFromContext(ctx); userID != "" {
		if session.UserID != "" && session.UserID != userID {
			return nil, fmt.Errorf("session does not belong to user")
		}
		if err := s.adoptSession(ctx, &session); err != nil {
			if s.logger != nil {
				s.logger.Warn("Failed to adopt session %s for user %s: %v", sessionID, userID, err)
			}
		}
	}
	return &session, nil
}

func (s *store) Save(ctx context.Context, session *ports.Session) error {
	userID := id.UserIDFromContext(ctx)
	if userID == "" {
		return fmt.Errorf("missing user context")
	}
	if session.UserID != "" && session.UserID != userID {
		return fmt.Errorf("session belongs to different user")
	}
	session.UserID = userID
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
	userID := id.UserIDFromContext(ctx)
	var ids []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") {
			idCandidate := strings.TrimSuffix(entry.Name(), ".json")
			if userID == "" {
				ids = append(ids, idCandidate)
				continue
			}
			data, readErr := os.ReadFile(filepath.Join(s.baseDir, entry.Name()))
			if readErr != nil {
				if s.logger != nil {
					s.logger.Error("Failed to read session file %s: %v", entry.Name(), readErr)
				}
				continue
			}
			var session ports.Session
			if jsonErr := json.Unmarshal(data, &session); jsonErr != nil {
				if s.logger != nil {
					s.logger.Error("Failed to decode session file %s: %v", entry.Name(), jsonErr)
				}
				continue
			}
			if err := s.adoptSession(ctx, &session); err != nil {
				if s.logger != nil {
					s.logger.Warn("Skipping session %s during list: %v", session.ID, err)
				}
				continue
			}
			if session.UserID == userID {
				ids = append(ids, session.ID)
			}
		}
	}
	return ids, nil
}

func (s *store) Delete(ctx context.Context, sessionID string) error {
	if userID := id.UserIDFromContext(ctx); userID != "" {
		session, err := s.Get(ctx, sessionID)
		if err != nil {
			return err
		}
		if session.UserID != "" && session.UserID != userID {
			return fmt.Errorf("session belongs to different user")
		}
	}
	path := filepath.Join(s.baseDir, fmt.Sprintf("%s.json", sessionID))
	err := os.Remove(path)
	// Ignore error if file doesn't exist - deletion goal achieved
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
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

func (s *store) adoptSession(ctx context.Context, session *ports.Session) error {
	if session == nil {
		return fmt.Errorf("session is nil")
	}
	userID := id.UserIDFromContext(ctx)
	if userID == "" {
		return nil
	}
	if session.UserID != "" && session.UserID != userID {
		return fmt.Errorf("session belongs to different user")
	}

	mutated := false
	if session.UserID == "" {
		session.UserID = userID
		mutated = true
	}
	for i := range session.Artifacts {
		if session.Artifacts[i].SessionID == "" {
			session.Artifacts[i].SessionID = session.ID
			mutated = true
		}
		if session.Artifacts[i].UserID == "" && userID != "" {
			session.Artifacts[i].UserID = userID
			mutated = true
		}
	}

	if !mutated {
		return nil
	}

	if err := s.Save(ctx, session); err != nil {
		return fmt.Errorf("persist adoption: %w", err)
	}
	return nil
}
