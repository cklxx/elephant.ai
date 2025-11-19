package storage

import (
	"context"
	"time"

	backoff "github.com/cenkalti/backoff/v4"
)

// RetryingMapper wraps a mapper and retries best-effort CDN operations.
type RetryingMapper struct {
	delegate     Mapper
	buildBackoff func() backoff.BackOff
}

// NewRetryingMapper creates a mapper that retries delete/prewarm/refresh.
func NewRetryingMapper(delegate Mapper, factory func() backoff.BackOff) *RetryingMapper {
	if factory == nil {
		factory = func() backoff.BackOff {
			b := backoff.NewExponentialBackOff()
			b.InitialInterval = 100 * time.Millisecond
			b.MaxElapsedTime = 3 * time.Second
			return b
		}
	}
	return &RetryingMapper{delegate: delegate, buildBackoff: factory}
}

func (m *RetryingMapper) Upload(ctx context.Context, req UploadRequest) (UploadResult, error) {
	return m.delegate.Upload(ctx, req)
}

func (m *RetryingMapper) Delete(ctx context.Context, key string) error {
	return m.retry(ctx, func() error { return m.delegate.Delete(ctx, key) })
}

func (m *RetryingMapper) Prewarm(ctx context.Context, key string) error {
	return m.retry(ctx, func() error { return m.delegate.Prewarm(ctx, key) })
}

func (m *RetryingMapper) Refresh(ctx context.Context, key string) error {
	return m.retry(ctx, func() error { return m.delegate.Refresh(ctx, key) })
}

func (m *RetryingMapper) retry(ctx context.Context, fn func() error) error {
	b := backoff.WithContext(m.buildBackoff(), ctx)
	return backoff.Retry(fn, b)
}

var _ Mapper = (*RetryingMapper)(nil)
