package configcenter

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"alex/internal/serverconfig"
)

// ErrNotFound is returned when no persisted configuration exists.
var ErrNotFound = errors.New("configuration not found")

// Store represents a persistence backend for server configuration snapshots.
type Store interface {
	Load() (serverconfig.Config, error)
	Save(serverconfig.Config) error
}

// FileStore persists configuration snapshots to a JSON file on disk.
type FileStore struct {
	path string
	mu   sync.Mutex
}

// FileStoreConfig configures a FileStore instance.
type FileStoreConfig struct {
	Path string
}

const configCenterPathEnv = "ALEX_CONFIG_CENTER_PATH"

// NewFileStore constructs a FileStore pointing at the provided path. When the
// path is empty it falls back to configs/server-config.json or the optional
// override provided via the ALEX_CONFIG_CENTER_PATH environment variable.
func NewFileStore(cfg FileStoreConfig) *FileStore {
	path := strings.TrimSpace(cfg.Path)
	if path == "" {
		path = strings.TrimSpace(os.Getenv(configCenterPathEnv))
	}
	if path == "" {
		path = filepath.Join("configs", "server-config.json")
	}
	return &FileStore{path: path}
}

// Load reads the configuration snapshot from disk.
func (s *FileStore) Load() (serverconfig.Config, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return serverconfig.Config{}, ErrNotFound
		}
		return serverconfig.Config{}, fmt.Errorf("read config center file: %w", err)
	}
	if len(data) == 0 {
		return serverconfig.Config{}, ErrNotFound
	}

	var cfg serverconfig.Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return serverconfig.Config{}, fmt.Errorf("parse config center file: %w", err)
	}
	return cfg, nil
}

// Save writes the configuration snapshot to disk atomically.
func (s *FileStore) Save(cfg serverconfig.Config) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("create config center dir: %w", err)
	}

	encoded, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("encode config center file: %w", err)
	}
	encoded = append(encoded, '\n')

	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, encoded, 0o600); err != nil {
		return fmt.Errorf("write config center tmp file: %w", err)
	}
	if err := os.Rename(tmp, s.path); err != nil {
		return fmt.Errorf("replace config center file: %w", err)
	}
	return nil
}

// Path exposes the location of the backing file (primarily for observability).
func (s *FileStore) Path() string {
	if s == nil {
		return ""
	}
	return s.path
}
