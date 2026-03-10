// Package leaderlock provides a file-based leader election lock using flock.
//
// Only one process at a time can hold the lock, making it safe for preventing
// duplicate scheduler execution across multiple instances on the same host.
package leaderlock

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// FileLock implements scheduler.LeaderLock using flock(2).
// The lock is non-blocking: Acquire returns immediately with (false, nil)
// if another process holds the lock.
type FileLock struct {
	path string
	name string
	file *os.File
}

// NewFileLock creates a file-based leader lock at the given path.
// The parent directory is created if it does not exist.
func NewFileLock(path, name string) (*FileLock, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create lock dir %s: %w", dir, err)
	}
	return &FileLock{path: path, name: name}, nil
}

// Acquire attempts a non-blocking exclusive flock on the lock file.
// Returns (true, nil) if the lock was acquired, (false, nil) if another
// process holds it, or (false, err) on unexpected errors.
func (l *FileLock) Acquire(_ context.Context) (bool, error) {
	f, err := os.OpenFile(l.path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return false, fmt.Errorf("open lock file %s: %w", l.path, err)
	}

	err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if err != nil {
		f.Close()
		if err == syscall.EWOULDBLOCK {
			return false, nil
		}
		return false, fmt.Errorf("flock %s: %w", l.path, err)
	}

	// Write PID for observability.
	_ = f.Truncate(0)
	_, _ = fmt.Fprintf(f, "%d\n", os.Getpid())

	l.file = f
	return true, nil
}

// Release unlocks and closes the lock file.
func (l *FileLock) Release(_ context.Context) error {
	if l.file == nil {
		return nil
	}
	f := l.file
	l.file = nil

	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_UN); err != nil {
		f.Close()
		return fmt.Errorf("unlock %s: %w", l.path, err)
	}
	return f.Close()
}

// Name returns the lock name for logging.
func (l *FileLock) Name() string {
	return l.name
}

// Path returns the lock file path.
func (l *FileLock) Path() string {
	return l.path
}
