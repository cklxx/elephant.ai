package blobstore

import (
	"context"
	"io"
	"time"
)

// PutOptions controls how objects are uploaded to the blob store.
type PutOptions struct {
	ContentType   string
	ContentLength int64
}

// BlobStore provides the minimal set of operations required by the server to persist artifacts.
type BlobStore interface {
	PutObject(ctx context.Context, key string, body io.Reader, opts PutOptions) (string, error)
	GetSignedURL(ctx context.Context, key string, expiry time.Duration) (string, error)
	DeleteObject(ctx context.Context, key string) error
}
