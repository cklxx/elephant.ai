package lark

import (
	"context"
	"fmt"
	"strings"

	"alex/internal/infra/filestore"
	jsonx "alex/internal/shared/json"
)

type chatSessionBindingStoreDoc struct {
	Bindings []ChatSessionBinding `json:"bindings"`
}

// ChatSessionBindingLocalStore is a local (memory/file) ChatSessionBindingStore.
// When filePath is empty the store is in-memory only.
type ChatSessionBindingLocalStore struct {
	coll *filestore.Collection[string, ChatSessionBinding]
}

// NewChatSessionBindingMemoryStore creates an in-memory chat/session store.
func NewChatSessionBindingMemoryStore() *ChatSessionBindingLocalStore {
	return &ChatSessionBindingLocalStore{
		coll: newChatSessionBindingCollection(""),
	}
}

// NewChatSessionBindingFileStore creates a file-backed chat/session store under dir/chat_sessions.json.
func NewChatSessionBindingFileStore(dir string) (*ChatSessionBindingLocalStore, error) {
	trimmedDir := strings.TrimSpace(dir)
	if trimmedDir == "" {
		return nil, fmt.Errorf("chat session file store dir is required")
	}
	if err := filestore.EnsureDir(trimmedDir); err != nil {
		return nil, fmt.Errorf("create chat session store dir: %w", err)
	}
	coll := newChatSessionBindingCollection(trimmedDir + "/chat_sessions.json")
	if err := coll.Load(); err != nil {
		return nil, err
	}
	return &ChatSessionBindingLocalStore{coll: coll}, nil
}

func newChatSessionBindingCollection(filePath string) *filestore.Collection[string, ChatSessionBinding] {
	c := filestore.NewCollection[string, ChatSessionBinding](filestore.CollectionConfig{
		FilePath: filePath,
		Perm:     0o600,
		Name:     "chat_session_binding",
	})
	c.SetMarshalDoc(func(m map[string]ChatSessionBinding) ([]byte, error) {
		doc := chatSessionBindingStoreDoc{Bindings: make([]ChatSessionBinding, 0, len(m))}
		for _, b := range m {
			doc.Bindings = append(doc.Bindings, b)
		}
		return filestore.MarshalJSONIndent(doc)
	})
	c.SetUnmarshalDoc(func(data []byte) (map[string]ChatSessionBinding, error) {
		var doc chatSessionBindingStoreDoc
		if err := jsonx.Unmarshal(data, &doc); err != nil {
			return nil, fmt.Errorf("decode chat session store: %w", err)
		}
		m := make(map[string]ChatSessionBinding, len(doc.Bindings))
		for _, b := range doc.Bindings {
			key := chatSessionBindingKey(b.Channel, b.ChatID)
			if key == "::" {
				continue
			}
			m[key] = b
		}
		return m, nil
	})
	return c
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
	return s.coll.EnsureDir()
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
		binding.UpdatedAt = s.coll.Now()
	}
	key := chatSessionBindingKey(binding.Channel, binding.ChatID)
	return s.coll.Put(key, binding)
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
	binding, ok := s.coll.Get(key)
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
	return s.coll.Delete(key)
}

var _ ChatSessionBindingStore = (*ChatSessionBindingLocalStore)(nil)
