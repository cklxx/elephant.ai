package timer

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

// store persists timers as individual YAML files in a directory.
// Each timer is stored as {id}.yaml.
type store struct {
	dir string
	mu  sync.Mutex
}

// newStore creates a store backed by the given directory.
// The directory is created if it does not exist.
func newStore(dir string) (*store, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create timer store dir: %w", err)
	}
	return &store{dir: dir}, nil
}

// Save persists a timer to disk, creating or overwriting the file.
func (s *store) Save(t Timer) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := yaml.Marshal(&t)
	if err != nil {
		return fmt.Errorf("marshal timer %s: %w", t.ID, err)
	}
	path := s.filePath(t.ID)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write timer %s: %w", t.ID, err)
	}
	return nil
}

// Get loads a single timer by ID. Returns os.ErrNotExist if not found.
func (s *store) Get(id string) (Timer, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.readFile(s.filePath(id))
}

// Delete removes a timer file from disk.
func (s *store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	path := s.filePath(id)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete timer %s: %w", id, err)
	}
	return nil
}

// LoadAll reads all timer files from the store directory.
func (s *store) LoadAll() ([]Timer, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entries, err := os.ReadDir(s.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read timer store dir: %w", err)
	}

	var timers []Timer
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}
		t, err := s.readFile(filepath.Join(s.dir, entry.Name()))
		if err != nil {
			continue // skip corrupt files
		}
		timers = append(timers, t)
	}
	return timers, nil
}

func (s *store) readFile(path string) (Timer, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Timer{}, err
	}
	var t Timer
	if err := yaml.Unmarshal(data, &t); err != nil {
		return Timer{}, fmt.Errorf("unmarshal timer: %w", err)
	}
	return t, nil
}

func (s *store) filePath(id string) string {
	return filepath.Join(s.dir, id+".yaml")
}
