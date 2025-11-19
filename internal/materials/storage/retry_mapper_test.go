package storage

import (
	"context"
	"errors"
	"testing"
	"time"

	backoff "github.com/cenkalti/backoff/v4"
)

type flakyMapper struct {
	Mapper
	prewarmFailures int
}

func (f *flakyMapper) Prewarm(ctx context.Context, key string) error {
	if f.prewarmFailures > 0 {
		f.prewarmFailures--
		return errors.New("flaky")
	}
	return f.Mapper.Prewarm(ctx, key)
}

func TestRetryingMapperRetriesPrewarm(t *testing.T) {
	base := NewInMemoryMapper("https://cdn")
	flaky := &flakyMapper{Mapper: base, prewarmFailures: 2}
	mapper := NewRetryingMapper(flaky, func() backoff.BackOff {
		b := backoff.NewConstantBackOff(10 * time.Millisecond)
		return backoff.WithMaxRetries(b, 3)
	})
	ctx := context.Background()
	if err := mapper.Prewarm(ctx, "key"); err != nil {
		t.Fatalf("expected retry to eventually succeed: %v", err)
	}
}
