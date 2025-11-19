package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	runtimeconfig "alex/internal/config"
)

// Store persists managed runtime configuration overrides.
type Store interface {
	LoadOverrides(ctx context.Context) (runtimeconfig.Overrides, error)
	SaveOverrides(ctx context.Context, overrides runtimeconfig.Overrides) error
}

// FileStore implements Store using a JSON file on disk.
type FileStore struct {
	path string
}

// NewFileStore returns a file-backed store rooted at the provided path.
func NewFileStore(path string) *FileStore {
	return &FileStore{path: path}
}

// LoadOverrides reads overrides from disk, returning an empty struct when missing.
func (s *FileStore) LoadOverrides(ctx context.Context) (runtimeconfig.Overrides, error) {
	if s == nil || strings.TrimSpace(s.path) == "" {
		return runtimeconfig.Overrides{}, nil
	}
	data, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return runtimeconfig.Overrides{}, nil
		}
		return runtimeconfig.Overrides{}, fmt.Errorf("read managed config: %w", err)
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return runtimeconfig.Overrides{}, nil
	}
	var overrides runtimeconfig.Overrides
	if err := json.Unmarshal(data, &overrides); err != nil {
		return runtimeconfig.Overrides{}, fmt.Errorf("parse managed config: %w", err)
	}
	return overrides, nil
}

// SaveOverrides writes overrides to disk, creating parent directories as required.
func (s *FileStore) SaveOverrides(ctx context.Context, overrides runtimeconfig.Overrides) error {
	if s == nil || strings.TrimSpace(s.path) == "" {
		return fmt.Errorf("store path not configured")
	}
	encoded, err := json.MarshalIndent(overrides, "", "  ")
	if err != nil {
		return fmt.Errorf("encode managed config: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("ensure managed config directory: %w", err)
	}
	if err := os.WriteFile(s.path, append(encoded, '\n'), 0o600); err != nil {
		return fmt.Errorf("write managed config: %w", err)
	}
	return nil
}
