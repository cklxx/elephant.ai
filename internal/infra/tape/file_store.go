package tape

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"

	coretape "alex/internal/core/tape"
)

// FileStore is a file-backed TapeStore that stores one JSONL file per tape.
type FileStore struct {
	dir string
}

// NewFileStore returns a FileStore rooted at dir. The directory is created if
// it does not exist.
func NewFileStore(dir string) (*FileStore, error) {
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("resolve home dir: %w", err)
		}
		dir = filepath.Join(home, ".alex", "tapes")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create tape dir: %w", err)
	}
	return &FileStore{dir: dir}, nil
}

func (s *FileStore) tapePath(name string) string {
	return filepath.Join(s.dir, name+".jsonl")
}

// Append adds an entry to the named tape by appending a JSON line to the file.
func (s *FileStore) Append(_ context.Context, tapeName string, entry coretape.TapeEntry) error {
	f, err := os.OpenFile(s.tapePath(tapeName), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open tape file: %w", err)
	}
	defer f.Close()

	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		return fmt.Errorf("flock: %w", err)
	}
	defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN) //nolint:errcheck

	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("marshal entry: %w", err)
	}
	data = append(data, '\n')
	if _, err := f.Write(data); err != nil {
		return fmt.Errorf("write entry: %w", err)
	}
	return nil
}

// Query reads all entries from the tape file and applies query filters.
func (s *FileStore) Query(_ context.Context, tapeName string, q coretape.TapeQuery) ([]coretape.TapeEntry, error) {
	entries, err := s.readAll(tapeName)
	if err != nil {
		return nil, err
	}
	return applyQuery(entries, q)
}

// List returns all tape names by scanning the directory for .jsonl files.
func (s *FileStore) List(_ context.Context) ([]string, error) {
	dirEntries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, fmt.Errorf("read tape dir: %w", err)
	}
	var names []string
	for _, de := range dirEntries {
		if de.IsDir() {
			continue
		}
		name := de.Name()
		if strings.HasSuffix(name, ".jsonl") {
			names = append(names, strings.TrimSuffix(name, ".jsonl"))
		}
	}
	sort.Strings(names)
	return names, nil
}

// Delete removes a tape file.
func (s *FileStore) Delete(_ context.Context, tapeName string) error {
	err := os.Remove(s.tapePath(tapeName))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func (s *FileStore) readAll(tapeName string) ([]coretape.TapeEntry, error) {
	f, err := os.Open(s.tapePath(tapeName))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("open tape file: %w", err)
	}
	defer f.Close()

	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_SH); err != nil {
		return nil, fmt.Errorf("flock: %w", err)
	}
	defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN) //nolint:errcheck

	var entries []coretape.TapeEntry
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var e coretape.TapeEntry
		if err := json.Unmarshal(line, &e); err != nil {
			return nil, fmt.Errorf("unmarshal entry: %w", err)
		}
		entries = append(entries, e)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan tape file: %w", err)
	}
	return entries, nil
}
