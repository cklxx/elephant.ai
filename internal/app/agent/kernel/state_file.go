package kernel

import (
	"context"
	"os"
	"path/filepath"

	"alex/internal/infra/markdown"
)

const (
	stateFileName        = "STATE.md"
	initFileName         = "INIT.md"
	systemPromptFileName = "SYSTEM_PROMPT.md"
)

// StateFile handles atomic read/write of kernel markdown artifacts
// (STATE.md / INIT.md / SYSTEM_PROMPT.md).
// STATE.md remains opaque markdown: kernel only upserts a bounded runtime block
// for observability and otherwise treats the rest as agent-owned content.
//
// When constructed with NewVersionedStateFile, all reads/writes are delegated
// to a VersionedStore for git-backed versioning.
type StateFile struct {
	dir   string                   // e.g. ~/.alex/kernel/{kernel_id}/
	store *markdown.VersionedStore // nil = legacy (plain filesystem) mode
}

// NewStateFile creates a StateFile rooted in the given directory.
// Uses plain filesystem I/O without git versioning.
func NewStateFile(dir string) *StateFile {
	return &StateFile{dir: dir}
}

// NewVersionedStateFile creates a StateFile backed by a VersionedStore.
// All reads/writes are delegated to the store.
func NewVersionedStateFile(dir string, store *markdown.VersionedStore) *StateFile {
	return &StateFile{dir: dir, store: store}
}

func (f *StateFile) namedPath(fileName string) string {
	return filepath.Join(f.dir, fileName)
}

// Path returns the full path to the STATE.md file.
func (f *StateFile) Path() string {
	return f.namedPath(stateFileName)
}

// InitPath returns the full path to INIT.md.
func (f *StateFile) InitPath() string {
	return f.namedPath(initFileName)
}

// SystemPromptPath returns the full path to SYSTEM_PROMPT.md.
func (f *StateFile) SystemPromptPath() string {
	return f.namedPath(systemPromptFileName)
}

func (f *StateFile) readNamed(fileName string) (string, error) {
	if f.store != nil {
		return f.store.Read(fileName)
	}
	data, err := os.ReadFile(f.namedPath(fileName))
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (f *StateFile) writeNamed(fileName, content string) error {
	if f.store != nil {
		return f.store.Write(context.Background(), fileName, content)
	}
	if err := os.MkdirAll(f.dir, 0o755); err != nil {
		return err
	}
	target := f.namedPath(fileName)
	tmp := target + ".tmp"
	if err := os.WriteFile(tmp, []byte(content), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, target)
}

func (f *StateFile) seedNamed(fileName, content string) error {
	if f.store != nil {
		return f.store.Seed(context.Background(), fileName, content)
	}
	_, err := os.Stat(f.namedPath(fileName))
	if err == nil {
		return nil // already exists, don't overwrite
	}
	if !os.IsNotExist(err) {
		return err
	}
	return f.writeNamed(fileName, content)
}

// Read returns the current STATE.md content.
// Returns empty string if the file does not exist.
func (f *StateFile) Read() (string, error) {
	return f.readNamed(stateFileName)
}

// ReadInit returns the current INIT.md content.
// Returns empty string if the file does not exist.
func (f *StateFile) ReadInit() (string, error) {
	return f.readNamed(initFileName)
}

// ReadSystemPrompt returns the current SYSTEM_PROMPT.md content.
// Returns empty string if the file does not exist.
func (f *StateFile) ReadSystemPrompt() (string, error) {
	return f.readNamed(systemPromptFileName)
}

// Write atomically replaces STATE.md via tmp+rename.
func (f *StateFile) Write(content string) error {
	return f.writeNamed(stateFileName, content)
}

// WriteInit atomically replaces INIT.md via tmp+rename.
func (f *StateFile) WriteInit(content string) error {
	return f.writeNamed(initFileName, content)
}

// WriteSystemPrompt atomically replaces SYSTEM_PROMPT.md via tmp+rename.
func (f *StateFile) WriteSystemPrompt(content string) error {
	return f.writeNamed(systemPromptFileName, content)
}

// Seed writes the initial content only if STATE.md does not already exist.
func (f *StateFile) Seed(content string) error {
	return f.seedNamed(stateFileName, content)
}

// SeedInit writes INIT.md only when it does not already exist.
func (f *StateFile) SeedInit(content string) error {
	return f.seedNamed(initFileName, content)
}

// SeedSystemPrompt writes SYSTEM_PROMPT.md only when it does not already exist.
func (f *StateFile) SeedSystemPrompt(content string) error {
	return f.seedNamed(systemPromptFileName, content)
}

// CommitCycleBoundary commits all pending changes with the given message.
// No-op when the store is nil (legacy mode).
func (f *StateFile) CommitCycleBoundary(ctx context.Context, msg string) error {
	if f.store == nil {
		return nil
	}
	_, err := f.store.CommitAll(ctx, msg)
	return err
}
