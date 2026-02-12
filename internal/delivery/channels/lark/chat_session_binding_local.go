package lark

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	jsonx "alex/internal/shared/json"
)

type chatSessionBindingStoreDoc struct {
	Bindings []ChatSessionBinding `json:"bindings"`
}

// ChatSessionBindingLocalStore is a local (memory/file) ChatSessionBindingStore.
// When filePath is empty the store is in-memory only.
type ChatSessionBindingLocalStore struct {
	mu       sync.RWMutex
	filePath string
	bindings map[string]ChatSessionBinding
	now      func() time.Time
}

// NewChatSessionBindingMemoryStore creates an in-memory chat/session store.
func NewChatSessionBindingMemoryStore() *ChatSessionBindingLocalStore {
	return newChatSessionBindingLocalStore("")
}

// NewChatSessionBindingFileStore creates a file-backed chat/session store under dir/chat_sessions.json.
func NewChatSessionBindingFileStore(dir string) (*ChatSessionBindingLocalStore, error) {
	trimmedDir := strings.TrimSpace(dir)
	if trimmedDir == "" {
		return nil, fmt.Errorf("chat session file store dir is required")
	}
	if err := os.MkdirAll(trimmedDir, 0o755); err != nil {
		return nil, fmt.Errorf("create chat session store dir: %w", err)
	}
	store := newChatSessionBindingLocalStore(filepath.Join(trimmedDir, "chat_sessions.json"))
	if err := store.load(); err != nil {
		return nil, err
	}
	return store, nil
}

func newChatSessionBindingLocalStore(filePath string) *ChatSessionBindingLocalStore {
	return &ChatSessionBindingLocalStore{
		filePath: filePath,
		bindings: make(map[string]ChatSessionBinding),
		now:      time.Now,
	}
}

func chatSessionBindingKey(channel, chatID string) string {
	return strings.TrimSpace(channel) + "::" + strings.TrimSpace(chatID)
}

// EnsureSchema validates file store readiness. Memory mode is no-op.
func (s *ChatSessionBindingLocalStore) EnsureSchema(ctx context.Context) error {
	if ctx != nil && ctx.Err() != nil {
		return ctx.Err()
	}
	if s == nil {
		return fmt.Errorf("chat session binding store not initialized")
	}
	if s.filePath == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(s.filePath), 0o755); err != nil {
		return fmt.Errorf("ensure chat session binding directory: %w", err)
	}
	return nil
}

// SaveBinding stores the chat/session mapping.
func (s *ChatSessionBindingLocalStore) SaveBinding(ctx context.Context, binding ChatSessionBinding) error {
	if ctx != nil && ctx.Err() != nil {
		return ctx.Err()
	}
	if s == nil {
		return fmt.Errorf("chat session binding store not initialized")
	}
	binding.Channel = strings.TrimSpace(binding.Channel)
	binding.ChatID = strings.TrimSpace(binding.ChatID)
	binding.SessionID = strings.TrimSpace(binding.SessionID)
	if binding.Channel == "" || binding.ChatID == "" || binding.SessionID == "" {
		return fmt.Errorf("channel, chat_id and session_id are required")
	}
	if binding.UpdatedAt.IsZero() {
		binding.UpdatedAt = s.now()
	}
	key := chatSessionBindingKey(binding.Channel, binding.ChatID)

	s.mu.Lock()
	defer s.mu.Unlock()
	s.bindings[key] = binding
	return s.persistLocked()
}

// GetBinding returns the chat/session mapping when present.
func (s *ChatSessionBindingLocalStore) GetBinding(ctx context.Context, channel, chatID string) (ChatSessionBinding, bool, error) {
	if ctx != nil && ctx.Err() != nil {
		return ChatSessionBinding{}, false, ctx.Err()
	}
	if s == nil {
		return ChatSessionBinding{}, false, fmt.Errorf("chat session binding store not initialized")
	}
	key := chatSessionBindingKey(channel, chatID)
	if key == "::" {
		return ChatSessionBinding{}, false, nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	binding, ok := s.bindings[key]
	return binding, ok, nil
}

// DeleteBinding removes a chat/session mapping.
func (s *ChatSessionBindingLocalStore) DeleteBinding(ctx context.Context, channel, chatID string) error {
	if ctx != nil && ctx.Err() != nil {
		return ctx.Err()
	}
	if s == nil {
		return fmt.Errorf("chat session binding store not initialized")
	}
	key := chatSessionBindingKey(channel, chatID)
	if key == "::" {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.bindings, key)
	return s.persistLocked()
}

func (s *ChatSessionBindingLocalStore) load() error {
	if s.filePath == "" {
		return nil
	}
	data, err := os.ReadFile(s.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read chat session store: %w", err)
	}
	var doc chatSessionBindingStoreDoc
	if err := jsonx.Unmarshal(data, &doc); err != nil {
		return fmt.Errorf("decode chat session store: %w", err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, binding := range doc.Bindings {
		key := chatSessionBindingKey(binding.Channel, binding.ChatID)
		if key == "::" {
			continue
		}
		s.bindings[key] = binding
	}
	return nil
}

func (s *ChatSessionBindingLocalStore) persistLocked() error {
	if s.filePath == "" {
		return nil
	}
	doc := chatSessionBindingStoreDoc{
		Bindings: make([]ChatSessionBinding, 0, len(s.bindings)),
	}
	for _, binding := range s.bindings {
		doc.Bindings = append(doc.Bindings, binding)
	}
	data, err := jsonx.MarshalIndent(doc, "", "  ")
	if err != nil {
		return fmt.Errorf("encode chat session store: %w", err)
	}
	data = append(data, '\n')
	tmp := s.filePath + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("write chat session temp file: %w", err)
	}
	if err := os.Rename(tmp, s.filePath); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("commit chat session file: %w", err)
	}
	return nil
}

var _ ChatSessionBindingStore = (*ChatSessionBindingLocalStore)(nil)
