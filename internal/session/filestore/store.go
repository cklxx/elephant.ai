package filestore

import (
	"alex/internal/agent/ports"
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
}

func New(baseDir string) ports.SessionStore {
	if strings.HasPrefix(baseDir, "~/") {
		home, _ := os.UserHomeDir()
		baseDir = filepath.Join(home, baseDir[2:])
	}
	_ = os.MkdirAll(baseDir, 0755) // Ignore error - directory may already exist
	return &store{baseDir: baseDir}
}

func (s *store) Create(ctx context.Context) (*ports.Session, error) {
	session := &ports.Session{
		ID:        fmt.Sprintf("session-%d", time.Now().Unix()),
		Messages:  []ports.Message{},
		Todos:     []ports.Todo{},
		Metadata:  make(map[string]string),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	// Save immediately to ensure session can be retrieved later
	if err := s.Save(ctx, session); err != nil {
		return nil, err
	}
	return session, nil
}

func (s *store) Get(ctx context.Context, id string) (*ports.Session, error) {
	path := filepath.Join(s.baseDir, fmt.Sprintf("%s.json", id))
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("session not found: %s", id)
	}
	var session ports.Session
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, err
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
	path := filepath.Join(s.baseDir, fmt.Sprintf("%s.json", id))
	return os.Remove(path)
}
