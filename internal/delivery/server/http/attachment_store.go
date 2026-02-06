package http

import (
	"fmt"
	"net/http"

	"alex/internal/infra/attachments"
)

// AttachmentStore persists decoded attachment payloads and serves them via a stable URL.
type AttachmentStore struct {
	store *attachments.Store
}

func NewAttachmentStore(cfg attachments.StoreConfig) (*AttachmentStore, error) {
	store, err := attachments.NewStore(cfg)
	if err != nil {
		return nil, err
	}
	return &AttachmentStore{store: store}, nil
}

func (s *AttachmentStore) StoreBytes(name, mediaType string, data []byte) (string, error) {
	if s == nil || s.store == nil {
		return "", fmt.Errorf("attachment store is not initialized")
	}
	return s.store.StoreBytes(name, mediaType, data)
}

func (s *AttachmentStore) Handler() http.Handler {
	if s == nil || s.store == nil {
		return http.NotFoundHandler()
	}
	return s.store.Handler()
}

// LocalDir returns the local storage directory, or an empty string for non-local providers.
func (s *AttachmentStore) LocalDir() string {
	if s == nil || s.store == nil {
		return ""
	}
	return s.store.LocalDir()
}
