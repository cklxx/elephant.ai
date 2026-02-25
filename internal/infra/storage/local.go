package storage

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// ErrReadOnly is returned when a write operation is attempted in read-only mode.
var ErrReadOnly = errors.New("storage: read-only mode")

// Manager controls access to a rooted filesystem tree.
type Manager struct {
	root           string
	readOnly       bool
	allowOverwrite bool
}

// Config represents configuration for a Manager instance.
type Config struct {
	Root           string
	ReadOnly       bool
	AllowOverwrite bool
}

// NewManager creates a Manager with the provided configuration.
func NewManager(cfg Config) (*Manager, error) {
	if strings.TrimSpace(cfg.Root) == "" {
		return nil, errors.New("storage root is required")
	}
	root := filepath.Clean(cfg.Root)
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, fmt.Errorf("create root: %w", err)
	}
	return &Manager{
		root:           root,
		readOnly:       cfg.ReadOnly,
		allowOverwrite: cfg.AllowOverwrite,
	}, nil
}

// Root returns the canonical root path.
func (m *Manager) Root() string {
	return m.root
}

// Resolve converts a relative path into an absolute path within the root.
func (m *Manager) Resolve(rel string) (string, error) {
	cleaned := filepath.Clean(rel)
	if strings.HasPrefix(cleaned, "..") {
		return "", fmt.Errorf("resolve: path %q escapes root", rel)
	}
	full := filepath.Join(m.root, cleaned)
	return full, nil
}

// EnsureDirectory ensures a directory exists relative to the root.
func (m *Manager) EnsureDirectory(rel string) (string, error) {
	abs, err := m.Resolve(rel)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		return "", fmt.Errorf("ensure directory: %w", err)
	}
	return abs, nil
}

// EnsureDir ensures the parent directory for the given relative path exists.
func (m *Manager) EnsureDir(rel string) (string, error) {
	abs, err := m.Resolve(rel)
	if err != nil {
		return "", err
	}
	dir := abs
	if !strings.HasSuffix(rel, string(filepath.Separator)) {
		dir = filepath.Dir(abs)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("ensure dir: %w", err)
	}
	return abs, nil
}

// WriteFile writes data to a file relative to the root.
func (m *Manager) WriteFile(rel string, data []byte, perm fs.FileMode) (string, error) {
	if m.readOnly {
		return "", ErrReadOnly
	}
	abs, err := m.Resolve(rel)
	if err != nil {
		return "", err
	}
	if !m.allowOverwrite {
		if _, err := os.Stat(abs); err == nil {
			return "", fmt.Errorf("write file: %s already exists", abs)
		}
	}
	if _, err := m.EnsureDir(rel); err != nil {
		return "", err
	}
	if err := os.WriteFile(abs, data, perm); err != nil {
		return "", fmt.Errorf("write file: %w", err)
	}
	return abs, nil
}

// Exists returns true when the file exists under the root.
func (m *Manager) Exists(rel string) (bool, error) {
	abs, err := m.Resolve(rel)
	if err != nil {
		return false, err
	}
	_, err = os.Stat(abs)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, err
}

// ReadFile reads a file within the root.
func (m *Manager) ReadFile(rel string) ([]byte, error) {
	abs, err := m.Resolve(rel)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}
	return data, nil
}

// Delete removes a file if it exists and manager is not read-only.
func (m *Manager) Delete(rel string) error {
	if m.readOnly {
		return ErrReadOnly
	}
	abs, err := m.Resolve(rel)
	if err != nil {
		return err
	}
	if err := os.Remove(abs); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("delete file: %w", err)
	}
	return nil
}
