package storage

import "context"

// UploadRequest captures the inputs AttachmentBroker forwards to the storage
// mapper before writing catalog entries.
type UploadRequest struct {
	Name     string
	MimeType string
	Data     []byte
	Source   string
}

// UploadResult contains the normalized metadata persisted by the mapper.
type UploadResult struct {
	StorageKey  string
	CDNURL      string
	ContentHash string
	SizeBytes   uint64
}

// Mapper abstracts the underlying object store / CDN plumbing so Attachment
// Broker logic stays unit testable.
type Mapper interface {
	Upload(ctx context.Context, req UploadRequest) (UploadResult, error)
	Delete(ctx context.Context, storageKey string) error
	Prewarm(ctx context.Context, storageKey string) error
	Refresh(ctx context.Context, storageKey string) error
}
