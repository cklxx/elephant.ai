package oauth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type FileTokenStore struct {
	mu   sync.Mutex
	path string
	data map[string]Token
	now  func() time.Time
}

func NewFileTokenStore(dir string) (*FileTokenStore, error) {
	if dir == "" {
		return nil, fmt.Errorf("dir required")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("mkdir %s: %w", dir, err)
	}
	store := &FileTokenStore{
		path: filepath.Join(dir, "tokens.json"),
		data: make(map[string]Token),
		now:  time.Now,
	}
	_ = store.load()
	return store, nil
}

func (s *FileTokenStore) EnsureSchema(_ context.Context) error { return nil }

func (s *FileTokenStore) Get(ctx context.Context, openID string) (Token, error) {
	if ctx != nil && ctx.Err() != nil {
		return Token{}, ctx.Err()
	}
	if openID == "" {
		return Token{}, fmt.Errorf("open_id required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if token, ok := s.data[openID]; ok {
		return token, nil
	}
	return Token{}, ErrTokenNotFound
}

func (s *FileTokenStore) Upsert(ctx context.Context, token Token) error {
	if ctx != nil && ctx.Err() != nil {
		return ctx.Err()
	}
	if token.OpenID == "" {
		return fmt.Errorf("open_id required")
	}
	if token.UpdatedAt.IsZero() {
		token.UpdatedAt = s.now()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[token.OpenID] = token
	return s.flushLocked()
}

func (s *FileTokenStore) Delete(ctx context.Context, openID string) error {
	if ctx != nil && ctx.Err() != nil {
		return ctx.Err()
	}
	if openID == "" {
		return fmt.Errorf("open_id required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, openID)
	return s.flushLocked()
}

func (s *FileTokenStore) load() error {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	var decoded map[string]Token
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	for k, v := range decoded {
		if k == "" || v.OpenID == "" {
			continue
		}
		s.data[k] = v
	}
	return nil
}

func (s *FileTokenStore) flushLocked() error {
	payload, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal tokens: %w", err)
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, payload, 0o600); err != nil {
		return fmt.Errorf("write tmp: %w", err)
	}
	if err := os.Rename(tmp, s.path); err != nil {
		return fmt.Errorf("rename: %w", err)
	}
	return nil
}

var _ TokenStore = (*FileTokenStore)(nil)
