package policy

import (
	"context"
	"testing"
	"time"

	materialapi "alex/internal/materials/api"
)

type fakeRetentionStore struct {
	updates []struct {
		id  string
		ttl uint64
	}
	err error
}

func (f *fakeRetentionStore) UpdateRetention(ctx context.Context, materialID string, ttlSeconds uint64) error {
	if f.err != nil {
		return f.err
	}
	f.updates = append(f.updates, struct {
		id  string
		ttl uint64
	}{materialID, ttlSeconds})
	return nil
}

func TestEngineResolveRetentionMatchesLegacyPolicy(t *testing.T) {
	engine := NewEngine()
	cases := []struct {
		name     string
		status   materialapi.MaterialStatus
		kind     materialapi.MaterialKind
		override time.Duration
		expected time.Duration
	}{
		{"input default", materialapi.MaterialStatusInput, materialapi.MaterialKindAttachment, 0, 30 * 24 * time.Hour},
		{"intermediate default", materialapi.MaterialStatusIntermediate, materialapi.MaterialKindAttachment, 0, 7 * 24 * time.Hour},
		{"final attachment", materialapi.MaterialStatusFinal, materialapi.MaterialKindAttachment, 0, 0},
		{"final artifact", materialapi.MaterialStatusFinal, materialapi.MaterialKindArtifact, 0, DefaultArtifactRetention},
		{"override", materialapi.MaterialStatusIntermediate, materialapi.MaterialKindAttachment, 12 * time.Hour, 12 * time.Hour},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := engine.ResolveRetention(0, tc.status, tc.kind, tc.override)
			if got != tc.expected {
				t.Fatalf("expected %v, got %v", tc.expected, got)
			}
		})
	}
}

func TestEnginePinAndUnpinRecordsAudit(t *testing.T) {
	ctx := context.Background()
	audit := NewInMemoryAuditLogger()
	engine := NewEngine(WithAuditLogger(audit), WithArtifactRetention(4*time.Hour), WithNow(func() time.Time {
		return time.Unix(123, 0).UTC()
	}))
	store := &fakeRetentionStore{}

	if err := engine.PinMaterial(ctx, store, "mat-1", "alice", "long term ref"); err != nil {
		t.Fatalf("pin: %v", err)
	}
	if err := engine.UnpinMaterial(ctx, store, "mat-1", materialapi.MaterialStatusFinal, materialapi.MaterialKindArtifact, 0, "alice", "unpin request"); err != nil {
		t.Fatalf("unpin: %v", err)
	}

	if len(store.updates) != 2 {
		t.Fatalf("expected 2 updates, got %d", len(store.updates))
	}
	if store.updates[0].ttl != 0 {
		t.Fatalf("expected pin to set ttl=0, got %d", store.updates[0].ttl)
	}
	if store.updates[1].ttl != uint64((4*time.Hour)/time.Second) {
		t.Fatalf("expected unpin ttl to equal artifact ttl, got %d", store.updates[1].ttl)
	}
	entries := audit.Entries()
	if len(entries) != 2 {
		t.Fatalf("expected 2 audit entries, got %d", len(entries))
	}
	if entries[0].Action != "pin" || entries[1].Action != "unpin" {
		t.Fatalf("unexpected audit sequence: %+v", entries)
	}
	if entries[0].TTLSeconds != 0 || entries[1].TTLSeconds != store.updates[1].ttl {
		t.Fatalf("audit ttl mismatch: %+v", entries)
	}
}
