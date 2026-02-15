package admin

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"alex/internal/infra/filestore"
	runtimeconfig "alex/internal/shared/config"
	"gopkg.in/yaml.v3"
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
	var doc struct {
		Overrides runtimeconfig.Overrides `yaml:"overrides"`
	}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return runtimeconfig.Overrides{}, fmt.Errorf("parse managed config: %w", err)
	}
	return doc.Overrides, nil
}

// SaveOverrides writes overrides to disk, creating parent directories as required.
func (s *FileStore) SaveOverrides(ctx context.Context, overrides runtimeconfig.Overrides) error {
	if s == nil || strings.TrimSpace(s.path) == "" {
		return fmt.Errorf("store path not configured")
	}
	doc := map[string]any{}
	if data, err := os.ReadFile(s.path); err == nil {
		if len(bytes.TrimSpace(data)) > 0 {
			if err := yaml.Unmarshal(data, &doc); err != nil {
				return fmt.Errorf("parse managed config: %w", err)
			}
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("read managed config: %w", err)
	}

	if overrides == (runtimeconfig.Overrides{}) {
		delete(doc, "overrides")
	} else {
		doc["overrides"] = overrides
	}

	encoded, err := yaml.Marshal(doc)
	if err != nil {
		return fmt.Errorf("encode managed config: %w", err)
	}
	encoded = append(encoded, '\n')
	if err := filestore.AtomicWrite(s.path, encoded, 0o600); err != nil {
		return fmt.Errorf("write managed config: %w", err)
	}
	return nil
}
