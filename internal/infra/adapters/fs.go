package adapters

import (
	"os"
	"path/filepath"
	"strings"

	agent "alex/internal/domain/agent/ports/agent"
)

// FileEventAppender implements agent.EventAppender using OS file operations.
type FileEventAppender struct{}

var _ agent.EventAppender = (*FileEventAppender)(nil)

// NewFileEventAppender creates a new FileEventAppender.
func NewFileEventAppender() *FileEventAppender {
	return &FileEventAppender{}
}

// AppendLine appends a line to the given file path, creating directories as needed.
func (a *FileEventAppender) AppendLine(path string, line string) {
	trimmedPath := strings.TrimSpace(path)
	if trimmedPath == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(trimmedPath), 0o755); err != nil {
		return
	}
	f, err := os.OpenFile(trimmedPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.WriteString(strings.TrimSpace(line) + "\n")
}

// OSAtomicWriter implements agent.AtomicFileWriter using OS file operations.
type OSAtomicWriter struct{}

var _ agent.AtomicFileWriter = (*OSAtomicWriter)(nil)

// NewOSAtomicWriter creates a new OSAtomicWriter.
func NewOSAtomicWriter() *OSAtomicWriter {
	return &OSAtomicWriter{}
}

// WriteFileAtomically writes data to a temporary file and renames it to the target path.
func (w *OSAtomicWriter) WriteFileAtomically(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	tmp, err := os.CreateTemp(dir, ".tmp-compaction-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()

	cleanup := func() {
		_ = os.Remove(tmpPath)
	}
	defer cleanup()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(perm); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}
