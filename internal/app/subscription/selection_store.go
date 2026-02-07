package subscription

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	runtimeconfig "alex/internal/shared/config"
	"alex/internal/shared/json"
)

const (
	selectionStoreVersion  = 1
	selectionStoreFilename = "llm_selection.json"
)

// SelectionScope identifies where an LLM selection applies.
// Channel is required. CLI selections omit ChatID/UserID.
type SelectionScope struct {
	Channel string
	ChatID  string
	UserID  string
}

func (s SelectionScope) key() (string, error) {
	channel := strings.ToLower(strings.TrimSpace(s.Channel))
	if channel == "" {
		return "", fmt.Errorf("channel required")
	}
	chatID := strings.TrimSpace(s.ChatID)
	userID := strings.TrimSpace(s.UserID)
	if chatID == "" && userID == "" {
		return channel, nil
	}
	if chatID == "" || userID == "" {
		return "", fmt.Errorf("chat_id and user_id required")
	}
	return fmt.Sprintf("%s:chat=%s:user=%s", channel, chatID, userID), nil
}

type selectionStoreDoc struct {
	Version    int                  `json:"version"`
	Selections map[string]Selection `json:"selections,omitempty"`
}

// ResolveSelectionStorePath returns the file path used to persist pinned LLM selections.
//
// Priority:
//  1. Explicit ALEX_LLM_SELECTION_PATH.
//  2. Sibling to the resolved config path (defaults to ~/.alex/llm_selection.json).
func ResolveSelectionStorePath(envLookup runtimeconfig.EnvLookup, homeDir func() (string, error)) string {
	if envLookup == nil {
		envLookup = runtimeconfig.DefaultEnvLookup
	}
	if value, ok := envLookup("ALEX_LLM_SELECTION_PATH"); ok {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	configPath, _ := runtimeconfig.ResolveConfigPath(envLookup, homeDir)
	return filepath.Join(filepath.Dir(configPath), selectionStoreFilename)
}

// SelectionStore persists pinned LLM selections without mutating managed overrides YAML.
type SelectionStore struct {
	path string
	mu   sync.Mutex
}

func NewSelectionStore(path string) *SelectionStore {
	return &SelectionStore{path: strings.TrimSpace(path)}
}

func (s *SelectionStore) Get(ctx context.Context, scope SelectionScope) (Selection, bool, error) {
	if s == nil {
		return Selection{}, false, nil
	}
	if ctx != nil && ctx.Err() != nil {
		return Selection{}, false, ctx.Err()
	}
	key, err := scope.key()
	if err != nil {
		return Selection{}, false, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	doc, err := s.loadDocLocked(ctx)
	if err != nil {
		return Selection{}, false, err
	}
	selection, ok := doc.Selections[key]
	return selection, ok, nil
}

// GetWithFallback tries each scope in order and returns the first match.
// A single lock acquisition and file read is used for all lookups.
func (s *SelectionStore) GetWithFallback(ctx context.Context, scopes ...SelectionScope) (Selection, SelectionScope, bool, error) {
	if s == nil || len(scopes) == 0 {
		return Selection{}, SelectionScope{}, false, nil
	}
	if ctx != nil && ctx.Err() != nil {
		return Selection{}, SelectionScope{}, false, ctx.Err()
	}

	keys := make([]string, len(scopes))
	for i, scope := range scopes {
		k, err := scope.key()
		if err != nil {
			return Selection{}, SelectionScope{}, false, err
		}
		keys[i] = k
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	doc, err := s.loadDocLocked(ctx)
	if err != nil {
		return Selection{}, SelectionScope{}, false, err
	}
	for i, k := range keys {
		if sel, ok := doc.Selections[k]; ok {
			return sel, scopes[i], true, nil
		}
	}
	return Selection{}, SelectionScope{}, false, nil
}

func (s *SelectionStore) Set(ctx context.Context, scope SelectionScope, selection Selection) error {
	if s == nil || s.path == "" {
		return fmt.Errorf("selection store not configured")
	}
	if ctx != nil && ctx.Err() != nil {
		return ctx.Err()
	}
	key, err := scope.key()
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	doc, err := s.loadDocLocked(ctx)
	if err != nil {
		return err
	}
	if doc.Selections == nil {
		doc.Selections = map[string]Selection{}
	}
	doc.Selections[key] = selection
	return s.saveDocLocked(ctx, doc)
}

func (s *SelectionStore) Clear(ctx context.Context, scope SelectionScope) error {
	if s == nil || s.path == "" {
		return nil
	}
	if ctx != nil && ctx.Err() != nil {
		return ctx.Err()
	}
	key, err := scope.key()
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	doc, err := s.loadDocLocked(ctx)
	if err != nil {
		return err
	}
	if len(doc.Selections) == 0 {
		return nil
	}
	delete(doc.Selections, key)
	if len(doc.Selections) == 0 {
		if err := os.Remove(s.path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("remove selection store: %w", err)
		}
		return nil
	}
	return s.saveDocLocked(ctx, doc)
}

func (s *SelectionStore) loadDocLocked(ctx context.Context) (selectionStoreDoc, error) {
	if s.path == "" {
		return selectionStoreDoc{}, fmt.Errorf("selection store path not configured")
	}
	if ctx != nil && ctx.Err() != nil {
		return selectionStoreDoc{}, ctx.Err()
	}

	data, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return selectionStoreDoc{Version: selectionStoreVersion, Selections: map[string]Selection{}}, nil
		}
		return selectionStoreDoc{}, fmt.Errorf("read selection store: %w", err)
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return selectionStoreDoc{Version: selectionStoreVersion, Selections: map[string]Selection{}}, nil
	}

	var doc selectionStoreDoc
	if err := jsonx.Unmarshal(data, &doc); err != nil {
		return selectionStoreDoc{}, fmt.Errorf("parse selection store: %w", err)
	}
	if doc.Version == 0 {
		doc.Version = selectionStoreVersion
	}
	if doc.Version != selectionStoreVersion {
		return selectionStoreDoc{}, fmt.Errorf("unsupported selection store version %d", doc.Version)
	}
	if doc.Selections == nil {
		doc.Selections = map[string]Selection{}
	}
	return doc, nil
}

func (s *SelectionStore) saveDocLocked(ctx context.Context, doc selectionStoreDoc) error {
	if s.path == "" {
		return fmt.Errorf("selection store path not configured")
	}
	if ctx != nil && ctx.Err() != nil {
		return ctx.Err()
	}
	doc.Version = selectionStoreVersion
	if doc.Selections == nil {
		doc.Selections = map[string]Selection{}
	}
	if len(doc.Selections) == 0 {
		if err := os.Remove(s.path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("remove selection store: %w", err)
		}
		return nil
	}
	data, err := jsonx.MarshalIndent(doc, "", "  ")
	if err != nil {
		return fmt.Errorf("encode selection store: %w", err)
	}
	data = append(data, '\n')

	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("ensure selection store directory: %w", err)
	}

	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("write selection store temp: %w", err)
	}
	if err := os.Rename(tmp, s.path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("commit selection store: %w", err)
	}
	return nil
}
