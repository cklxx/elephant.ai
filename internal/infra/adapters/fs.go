package adapters

import (
	"os"
	"path/filepath"
	"strings"

	agent "alex/internal/domain/agent/ports/agent"
)

// fileEventAppender implements agent.EventAppender using OS file operations.
type fileEventAppender struct{}

var _ agent.EventAppender = (*fileEventAppender)(nil)

// NewFileEventAppender creates a new file-backed event appender.
func NewFileEventAppender() *fileEventAppender {
	return &fileEventAppender{}
}

// AppendLine appends a line to the given file path, creating directories as needed.
func (a *fileEventAppender) AppendLine(path string, line string) {
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

// osAtomicWriter implements agent.AtomicFileWriter using OS file operations.
type osAtomicWriter struct{}

var _ agent.AtomicFileWriter = (*osAtomicWriter)(nil)

// NewOSAtomicWriter creates a new atomic file writer backed by the OS filesystem.
func NewOSAtomicWriter() *osAtomicWriter {
	return &osAtomicWriter{}
}

// WriteFileAtomically writes data to a temporary file and renames it to the target path.
func (w *osAtomicWriter) WriteFileAtomically(path string, data []byte, perm os.FileMode) error {
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

