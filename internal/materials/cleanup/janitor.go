package cleanup

import (
	"context"
	"errors"
	"fmt"
	"time"

	materialapi "alex/internal/materials/api"
	"alex/internal/materials/storage"
	"alex/internal/materials/store"
)

// TombstonePublisher propagates deletions to watchers.
type TombstonePublisher interface {
	PublishTombstone(ctx context.Context, requestID, materialID string) error
}

// Janitor sweeps expired materials and coordinates storage cleanup + events.
type Janitor struct {
	Store     store.Store
	Storage   storage.Mapper
	Publisher TombstonePublisher
	BatchSize int
	Statuses  []materialapi.MaterialStatus
	Now       func() time.Time
}

// Sweep executes a single cleanup pass.
func (j *Janitor) Sweep(ctx context.Context) (int, error) {
	if j == nil || j.Store == nil {
		return 0, errors.New("cleanup janitor requires store")
	}
	batch := j.BatchSize
	if batch <= 0 {
		batch = 256
	}
	now := time.Now().UTC()
	if j.Now != nil {
		now = j.Now()
	}
	deleted, err := j.Store.DeleteExpiredMaterials(ctx, store.DeleteExpiredMaterialsRequest{
		Statuses: j.Statuses,
		Limit:    batch,
		Now:      now,
	})
	if err != nil {
		return 0, err
	}
	for _, material := range deleted {
		if j.Storage != nil {
			keys := make([]string, 0, 1+len(material.PreviewAssetKeys))
			if material.StorageKey != "" {
				keys = append(keys, material.StorageKey)
			}
			keys = append(keys, material.PreviewAssetKeys...)
			for _, key := range keys {
				if key == "" {
					continue
				}
				if err := j.Storage.Delete(ctx, key); err != nil {
					return len(deleted), fmt.Errorf("delete storage object %s: %w", key, err)
				}
				if err := j.Storage.Refresh(ctx, key); err != nil {
					return len(deleted), fmt.Errorf("refresh cdn for %s: %w", key, err)
				}
			}
		}
		if j.Publisher != nil && material.RequestID != "" {
			if err := j.Publisher.PublishTombstone(ctx, material.RequestID, material.MaterialID); err != nil {
				return len(deleted), fmt.Errorf("publish tombstone %s: %w", material.MaterialID, err)
			}
		}
	}
	return len(deleted), nil
}
