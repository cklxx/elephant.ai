package markdown

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"alex/internal/infra/filestore"
	"alex/internal/shared/logging"
)

// StoreConfig configures a VersionedStore.
type StoreConfig struct {
	Dir        string         // root directory for files
	AutoCommit bool           // if true, commit pending changes before each write
	Logger     logging.Logger // optional
}

// VersionedStore manages markdown files in a git-backed directory.
// All writes use atomic tmp+rename; git operations are serialised by a mutex.
type VersionedStore struct {
	dir        string
	git        *gitOperations
	autoCommit bool
	logger     logging.Logger
	mu         sync.Mutex
}

// NewVersionedStore creates a new store. Call Init before first use.
func NewVersionedStore(cfg StoreConfig) *VersionedStore {
	logger := logging.OrNop(cfg.Logger)
	return &VersionedStore{
		dir:        cfg.Dir,
		git:        newGitOperations(cfg.Dir, logger),
		autoCommit: cfg.AutoCommit,
		logger:     logger,
	}
}

// Init ensures the directory exists and contains a git repository.
func (s *VersionedStore) Init(ctx context.Context) error {
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return fmt.Errorf("mkdir store dir: %w", err)
	}
	// Ensure .tmp files from atomic writes are never committed.
	gitignore := filepath.Join(s.dir, ".gitignore")
	if _, err := os.Stat(gitignore); os.IsNotExist(err) {
		_ = os.WriteFile(gitignore, []byte("*.tmp\n"), 0o644)
	}
	return s.git.init(ctx)
}

// Read returns the content of fileName (relative to the store dir).
// Returns empty string if the file does not exist.
func (s *VersionedStore) Read(fileName string) (string, error) {
	data, err := os.ReadFile(filepath.Join(s.dir, fileName))
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// Write atomically writes content to fileName, staging the change in git.
// If AutoCommit is enabled and there are uncommitted changes, they are
// committed with a snapshot message before the new write.
func (s *VersionedStore) Write(ctx context.Context, fileName, content string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.autoCommit {
		if changed, err := s.git.hasChanges(ctx); err == nil && changed {
			if commitErr := s.commitAllLocked(ctx, "auto: pre-write snapshot"); commitErr != nil {
				s.logger.Debug("auto-commit before write failed: %v", commitErr)
			}
		}
	}

	if err := s.atomicWrite(fileName, content); err != nil {
		return err
	}
	return s.git.add(ctx, fileName)
}

// Seed writes the file only if it does not already exist.
func (s *VersionedStore) Seed(ctx context.Context, fileName, content string) error {
	target := filepath.Join(s.dir, fileName)
	if _, err := os.Stat(target); err == nil {
		return nil // already exists
	}
	return s.Write(ctx, fileName, content)
}

// CommitAll stages all changes and commits with the given message.
// Returns true if a commit was created, false if nothing to commit.
func (s *VersionedStore) CommitAll(ctx context.Context, msg string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.commitAllLockedCheck(ctx, msg)
}

// History returns the N most recent commits for the given file.
func (s *VersionedStore) History(ctx context.Context, fileName string, n int) ([]CommitEntry, error) {
	return s.git.log(ctx, fileName, n)
}

// commitAllLocked does add+commit while holding the mutex. Ignores "nothing to commit" errors.
func (s *VersionedStore) commitAllLocked(ctx context.Context, msg string) error {
	_ = s.git.add(ctx, ".")
	return s.git.commit(ctx, msg)
}

// commitAllLockedCheck does add+commit, returning (true, nil) on success,
// (false, nil) when clean.
func (s *VersionedStore) commitAllLockedCheck(ctx context.Context, msg string) (bool, error) {
	changed, err := s.git.hasChanges(ctx)
	if err != nil {
		return false, err
	}
	if !changed {
		return false, nil
	}
	_ = s.git.add(ctx, ".")
	if err := s.git.commit(ctx, msg); err != nil {
		return false, err
	}
	return true, nil
}

func (s *VersionedStore) atomicWrite(fileName, content string) error {
	target := filepath.Join(s.dir, fileName)
	return filestore.AtomicWrite(target, []byte(content), 0o644)
}
