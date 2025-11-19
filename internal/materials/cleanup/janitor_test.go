package cleanup

import (
	"context"
	"errors"
	"testing"
	"time"

	materialapi "alex/internal/materials/api"
	"alex/internal/materials/storage"
	"alex/internal/materials/store"
)

type fakeStore struct {
	deleted []store.DeletedMaterial
	err     error
}

func (f *fakeStore) InsertMaterials(ctx context.Context, materials []store.MaterialRecord) error {
	return nil
}

func (f *fakeStore) DeleteExpiredMaterials(ctx context.Context, req store.DeleteExpiredMaterialsRequest) ([]store.DeletedMaterial, error) {
	return f.deleted, f.err
}

func (f *fakeStore) UpdateRetention(ctx context.Context, materialID string, ttlSeconds uint64) error {
	return nil
}

type fakePublisher struct {
	requestID  string
	materialID string
	err        error
}

func (f *fakePublisher) PublishTombstone(ctx context.Context, requestID, materialID string) error {
	f.requestID = requestID
	f.materialID = materialID
	return f.err
}

type trackingMapper struct {
	storage.Mapper
	deletedKeys   []string
	refreshedKeys []string
	err           error
}

func (t *trackingMapper) Delete(ctx context.Context, key string) error {
	if t.err != nil {
		return t.err
	}
	t.deletedKeys = append(t.deletedKeys, key)
	return nil
}

func (t *trackingMapper) Refresh(ctx context.Context, key string) error {
	if t.err != nil {
		return t.err
	}
	t.refreshedKeys = append(t.refreshedKeys, key)
	return nil
}

func TestJanitorDeletesAndPublishes(t *testing.T) {
	mapper := &trackingMapper{Mapper: storage.NewInMemoryMapper("https://cdn")}
	store := &fakeStore{deleted: []store.DeletedMaterial{{
		MaterialID:       "mat-1",
		RequestID:        "req-1",
		StorageKey:       "materials/foo",
		PreviewAssetKeys: []string{"materials/foo-rendered"},
	}}}
	publisher := &fakePublisher{}
	janitor := &Janitor{Store: store, Storage: mapper, Publisher: publisher, Statuses: []materialapi.MaterialStatus{materialapi.MaterialStatusIntermediate}, Now: func() time.Time { return time.Unix(123, 0) }}
	count, err := janitor.Sweep(context.Background())
	if err != nil {
		t.Fatalf("sweep returned error: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 deletion, got %d", count)
	}
	if len(mapper.deletedKeys) != 2 {
		t.Fatalf("expected 2 storage deletes, got %+v", mapper.deletedKeys)
	}
	if mapper.deletedKeys[0] != "materials/foo" || mapper.deletedKeys[1] != "materials/foo-rendered" {
		t.Fatalf("unexpected delete order: %+v", mapper.deletedKeys)
	}
	if len(mapper.refreshedKeys) != 2 {
		t.Fatalf("expected 2 cdn refreshes, got %+v", mapper.refreshedKeys)
	}
	if publisher.requestID != "req-1" || publisher.materialID != "mat-1" {
		t.Fatalf("expected tombstone event, got %+v", publisher)
	}
}

func TestJanitorPropagatesErrors(t *testing.T) {
	store := &fakeStore{deleted: []store.DeletedMaterial{{MaterialID: "mat-err", RequestID: "req-err", StorageKey: "materials/err"}}}
	mapper := &trackingMapper{Mapper: storage.NewInMemoryMapper("https://cdn"), err: errors.New("boom")}
	janitor := &Janitor{Store: store, Storage: mapper}
	if _, err := janitor.Sweep(context.Background()); err == nil {
		t.Fatalf("expected error from mapper")
	}
}
