package postgres

import (
	"context"
	"testing"
	"time"

	pgxmock "github.com/pashagolub/pgxmock/v4"

	materialapi "alex/internal/materials/api"
	"alex/internal/materials/store"
)

func TestInsertMaterialsPersistsRowsAndLineage(t *testing.T) {
	pool, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to build pgx mock: %v", err)
	}
	defer pool.Close()

	s, err := New(pool)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	ctx := context.Background()
	record := store.MaterialRecord{
		MaterialID: "mat-123",
		Context:    &materialapi.RequestContext{RequestID: "req-1", AgentIteration: 2, ToolCallID: "tc-7"},
		Descriptor: &materialapi.MaterialDescriptor{
			Name:                "browser.png",
			Placeholder:         "[material:browser.png]",
			MimeType:            "image/png",
			Source:              "browser",
			Status:              "intermediate",
			Visibility:          materialapi.VisibilityShared,
			Tags:                map[string]string{"placeholder": "[material:browser.png]"},
			RetentionTTLSeconds: 3600,
			Kind:                materialapi.MaterialKindArtifact,
			Format:              "png",
			PreviewProfile:      "image.card",
			PreviewAssets: []*materialapi.PreviewAsset{{
				AssetID:     "mat-123-page-1",
				Label:       "Page 1",
				MimeType:    "image/png",
				CDNURL:      "https://cdn/materials/hash/page-1.png",
				PreviewType: "page",
			}},
		},
		Storage: &materialapi.MaterialStorage{
			StorageKey:  "materials/hash",
			CDNURL:      "https://cdn/materials/hash",
			ContentHash: "hash",
			SizeBytes:   4,
		},
		SystemAttributes: &materialapi.SystemAttributes{DomainTags: []string{"browser"}, ComplianceTags: []string{"pii-safe"}},
		Lineage:          []store.LineageRecord{{ParentMaterialID: "mat-parent", DerivationType: "transform", ParametersHash: "abc"}},
		AccessBindings: []*materialapi.AccessBinding{{
			Principal:  "runtime:agent",
			Scope:      "request",
			Capability: "read",
		}},
	}

	pool.ExpectBegin()
	pool.ExpectExec("INSERT INTO materials").WithArgs(
		record.MaterialID,
		record.Context.RequestID,
		record.Context.TaskID,
		record.Context.AgentIteration,
		record.Context.ToolCallID,
		record.Context.ConversationID,
		record.Context.UserID,
		record.Descriptor.Name,
		record.Descriptor.Placeholder,
		record.Descriptor.MimeType,
		record.Descriptor.Description,
		record.Descriptor.Source,
		record.Descriptor.Origin,
		record.Descriptor.Status,
		int32(record.Descriptor.Visibility),
		pgxmock.AnyArg(),
		pgxmock.AnyArg(),
		record.Storage.StorageKey,
		record.Storage.CDNURL,
		record.Storage.ContentHash,
		record.Storage.SizeBytes,
		pgxmock.AnyArg(),
		record.Descriptor.RetentionTTLSeconds,
		"artifact",
		record.Descriptor.Format,
		record.Descriptor.PreviewProfile,
		pgxmock.AnyArg(),
	).WillReturnResult(pgxmock.NewResult("INSERT", 1))
	pool.ExpectExec("INSERT INTO material_lineage").WithArgs(
		record.Lineage[0].ParentMaterialID,
		record.MaterialID,
		record.Lineage[0].DerivationType,
		record.Lineage[0].ParametersHash,
	).WillReturnResult(pgxmock.NewResult("INSERT", 1))
	pool.ExpectExec("INSERT INTO material_access_bindings").WithArgs(
		record.MaterialID,
		record.AccessBindings[0].Principal,
		record.AccessBindings[0].Scope,
		record.AccessBindings[0].Capability,
		nil,
	).WillReturnResult(pgxmock.NewResult("INSERT", 1))
	pool.ExpectCommit()

	if err := s.InsertMaterials(ctx, []store.MaterialRecord{record}); err != nil {
		t.Fatalf("insert materials: %v", err)
	}

	if err := pool.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestInsertMaterialsValidatesDescriptorAndStorage(t *testing.T) {
	pool, _ := pgxmock.NewPool()
	defer pool.Close()
	s, _ := New(pool)

	err := s.InsertMaterials(context.Background(), []store.MaterialRecord{{MaterialID: "mat"}})
	if err == nil {
		t.Fatalf("expected error for missing descriptor")
	}
}

func TestDeleteExpiredMaterialsRemovesRows(t *testing.T) {
	pool, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("mock pool: %v", err)
	}
	defer pool.Close()
	s, _ := New(pool)
	cutoff := time.Unix(123, 0).UTC()
	pool.ExpectBegin()
	rows := pgxmock.NewRows([]string{"material_id", "request_id", "storage_key"}).AddRow("mat-1", "req", "materials/foo")
	pool.ExpectQuery("WITH expired").WithArgs(cutoff, []string{"intermediate"}, 50).WillReturnRows(rows)
	pool.ExpectCommit()
	deleted, err := s.DeleteExpiredMaterials(context.Background(), store.DeleteExpiredMaterialsRequest{
		Statuses: []materialapi.MaterialStatus{materialapi.MaterialStatusIntermediate},
		Limit:    50,
		Now:      cutoff,
	})
	if err != nil {
		t.Fatalf("delete expired: %v", err)
	}
	if len(deleted) != 1 || deleted[0].MaterialID != "mat-1" {
		t.Fatalf("unexpected deleted materials: %+v", deleted)
	}
	if err := pool.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestUpdateRetentionPersistsTTL(t *testing.T) {
	pool, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("mock pool: %v", err)
	}
	defer pool.Close()
	s, _ := New(pool)
	pool.ExpectBegin()
	pool.ExpectExec("UPDATE materials").WithArgs(int64(3600), "mat-ttl").WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	pool.ExpectCommit()
	if err := s.UpdateRetention(context.Background(), "mat-ttl", 3600); err != nil {
		t.Fatalf("update retention: %v", err)
	}
	if err := pool.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}
