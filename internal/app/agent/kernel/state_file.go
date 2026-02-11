package kernel

import (
	"os"
	"path/filepath"
)

// StateFile handles atomic read/write of STATE.md.
// The system treats STATE.md as opaque text owned entirely by the agent.
// It never parses or depends on the content — only reads it in and writes it out.
type StateFile struct {
	dir string // e.g. ~/.alex/kernel/{kernel_id}/
}

// NewStateFile creates a StateFile rooted in the given directory.
func NewStateFile(dir string) *StateFile {
	return &StateFile{dir: dir}
}

// Path returns the full path to the STATE.md file.
func (f *StateFile) Path() string {
	return filepath.Join(f.dir, "STATE.md")
}

// Read returns the current STATE.md content.
// Returns empty string if the file does not exist.
func (f *StateFile) Read() (string, error) {
	data, err := os.ReadFile(f.Path())
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// Write atomically replaces STATE.md via tmp+rename.
func (f *StateFile) Write(content string) error {
	if err := os.MkdirAll(f.dir, 0o755); err != nil {
		return err
	}
	tmp := f.Path() + ".tmp"
	if err := os.WriteFile(tmp, []byte(content), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, f.Path())
}

// Seed writes the initial content only if STATE.md does not already exist.
func (f *StateFile) Seed(content string) error {
	if _, err := os.Stat(f.Path()); err == nil {
		return nil // already exists, don't overwrite
	}
	return f.Write(content)
}
