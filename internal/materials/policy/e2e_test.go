package policy

import (
	"context"
	"testing"
	"time"

	materialapi "alex/internal/materials/api"
	"alex/internal/materials/cleanup"
	"alex/internal/materials/storage"
	"alex/internal/materials/store"
)

type lifecycleRecord struct {
	record  store.DeletedMaterial
	status  materialapi.MaterialStatus
	kind    materialapi.MaterialKind
	ttl     time.Duration
	created time.Time
}

type fakeLifecycleStore struct {
	materials map[string]*lifecycleRecord
	now       time.Time
}

func newFakeLifecycleStore(now time.Time) *fakeLifecycleStore {
	return &fakeLifecycleStore{materials: make(map[string]*lifecycleRecord), now: now}
}

func (f *fakeLifecycleStore) InsertMaterials(ctx context.Context, records []store.MaterialRecord) error {
	return nil
}

func (f *fakeLifecycleStore) UpdateRetention(ctx context.Context, materialID string, ttlSeconds uint64) error {
	if rec, ok := f.materials[materialID]; ok {
		rec.ttl = time.Duration(ttlSeconds) * time.Second
		return nil
	}
	return nil
}

func (f *fakeLifecycleStore) DeleteExpiredMaterials(ctx context.Context, req store.DeleteExpiredMaterialsRequest) ([]store.DeletedMaterial, error) {
	cutoff := req.Now
	if cutoff.IsZero() {
		cutoff = f.now
	}
	allowed := make(map[materialapi.MaterialStatus]bool)
	for _, status := range req.Statuses {
		allowed[status] = true
	}
	var deleted []store.DeletedMaterial
	for id, rec := range f.materials {
		ttl := rec.ttl
		if ttl <= 0 {
			continue
		}
		if len(allowed) > 0 && !allowed[rec.status] {
			continue
		}
		if rec.created.Add(ttl).After(cutoff) {
			continue
		}
		deleted = append(deleted, rec.record)
		delete(f.materials, id)
	}
	return deleted, nil
}

func TestLifecycleGovernanceEndToEnd(t *testing.T) {
	base := time.Unix(1, 0).UTC()
	audit := NewInMemoryAuditLogger()
	engine := NewEngine(WithAuditLogger(audit), WithArtifactRetention(time.Hour), WithNow(func() time.Time { return base }))
	lifecycleStore := newFakeLifecycleStore(base)
	mapper := storage.NewInMemoryMapper("https://cdn")
	// Seed artifact states
	lifecycleStore.materials["artifact-stale"] = &lifecycleRecord{
		record:  store.DeletedMaterial{MaterialID: "artifact-stale", RequestID: "req", StorageKey: "materials/a"},
		status:  materialapi.MaterialStatusFinal,
		kind:    materialapi.MaterialKindArtifact,
		ttl:     engine.ResolveRetention(0, materialapi.MaterialStatusFinal, materialapi.MaterialKindArtifact, 0),
		created: base,
	}
	lifecycleStore.materials["artifact-pinned"] = &lifecycleRecord{
		record:  store.DeletedMaterial{MaterialID: "artifact-pinned", RequestID: "req", StorageKey: "materials/b"},
		status:  materialapi.MaterialStatusFinal,
		kind:    materialapi.MaterialKindArtifact,
		ttl:     0,
		created: base,
	}
	lifecycleStore.materials["artifact-future"] = &lifecycleRecord{
		record:  store.DeletedMaterial{MaterialID: "artifact-future", RequestID: "req", StorageKey: "materials/c"},
		status:  materialapi.MaterialStatusFinal,
		kind:    materialapi.MaterialKindArtifact,
		ttl:     engine.ResolveRetention(0, materialapi.MaterialStatusFinal, materialapi.MaterialKindArtifact, 0),
		created: base.Add(90 * time.Minute),
	}

	janitor := &cleanup.Janitor{
		Store:    lifecycleStore,
		Storage:  mapper,
		Statuses: []materialapi.MaterialStatus{materialapi.MaterialStatusFinal},
		Now: func() time.Time {
			return base.Add(2 * time.Hour)
		},
	}

	deleted, err := janitor.Sweep(context.Background())
	if err != nil {
		t.Fatalf("janitor sweep: %v", err)
	}
	if deleted != 1 {
		t.Fatalf("expected 1 deletion (stale artifact), got %d", deleted)
	}
	if _, ok := lifecycleStore.materials["artifact-stale"]; ok {
		t.Fatalf("expected artifact-stale to be removed")
	}
	if _, ok := lifecycleStore.materials["artifact-pinned"]; !ok {
		t.Fatalf("pinned artifact should remain")
	}

	// Unpin artifact and ensure a later sweep clears it
	if err := engine.UnpinMaterial(context.Background(), lifecycleStore, "artifact-pinned", materialapi.MaterialStatusFinal, materialapi.MaterialKindArtifact, 0, "system", "release retention"); err != nil {
		t.Fatalf("unpin: %v", err)
	}
	janitor.Now = func() time.Time {
		return base.Add(2*time.Hour + 15*time.Minute)
	}
	deleted, err = janitor.Sweep(context.Background())
	if err != nil {
		t.Fatalf("second sweep: %v", err)
	}
	if deleted != 1 {
		t.Fatalf("expected pinned artifact to delete after unpin, got %d deletions", deleted)
	}
	if _, ok := lifecycleStore.materials["artifact-pinned"]; ok {
		t.Fatalf("artifact-pinned should be gone after unpin + sweep")
	}

	// Pin the remaining artifact and make sure sweeping with huge cutoff keeps it
	if err := engine.PinMaterial(context.Background(), lifecycleStore, "artifact-future", "system", "pin for audit"); err != nil {
		t.Fatalf("pin future artifact: %v", err)
	}
	janitor.Now = func() time.Time { return base.Add(24 * time.Hour) }
	deleted, err = janitor.Sweep(context.Background())
	if err != nil {
		t.Fatalf("final sweep: %v", err)
	}
	if deleted != 0 {
		t.Fatalf("pinned artifact should not be removed, got %d deletions", deleted)
	}
	if _, ok := lifecycleStore.materials["artifact-future"]; !ok {
		t.Fatalf("pinned artifact unexpectedly removed")
	}
	if len(audit.Entries()) == 2 {
		if audit.Entries()[0].Action != "unpin" || audit.Entries()[1].Action != "pin" {
			t.Fatalf("unexpected audit log order: %+v", audit.Entries())
		}
	} else {
		t.Fatalf("expected two audit entries, got %d", len(audit.Entries()))
	}
}
