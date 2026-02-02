package memory

import (
	"context"
	"sync"
	"testing"
	"time"
)

func newTestEntityStore(t *testing.T) *FileEntityStore {
	t.Helper()
	return NewFileEntityStore(t.TempDir())
}

func TestUpsertEntityCreatesNew(t *testing.T) {
	store := newTestEntityStore(t)
	ctx := context.Background()

	entity, err := store.UpsertEntity(ctx, Entity{
		UserID:      "user-1",
		Name:        "Alice",
		Type:        EntityPerson,
		Description: "Backend engineer",
		Tags:        []string{"team-a"},
	})
	if err != nil {
		t.Fatalf("UpsertEntity: %v", err)
	}
	if entity.ID == "" {
		t.Fatal("expected generated ID")
	}
	if entity.Name != "Alice" {
		t.Fatalf("expected name Alice, got %s", entity.Name)
	}
	if entity.MentionCount != 1 {
		t.Fatalf("expected mention count 1, got %d", entity.MentionCount)
	}
	if entity.FirstSeen.IsZero() {
		t.Fatal("expected FirstSeen set")
	}
	if entity.LastSeen.IsZero() {
		t.Fatal("expected LastSeen set")
	}
}

func TestUpsertEntityMergesExisting(t *testing.T) {
	store := newTestEntityStore(t)
	ctx := context.Background()

	original, err := store.UpsertEntity(ctx, Entity{
		UserID:      "user-1",
		Name:        "ProjectX",
		Type:        EntityProject,
		Description: "Initial project",
		Aliases:     []string{"PX"},
		Attributes:  map[string]string{"lang": "Go"},
		Tags:        []string{"infra"},
	})
	if err != nil {
		t.Fatalf("UpsertEntity (create): %v", err)
	}

	merged, err := store.UpsertEntity(ctx, Entity{
		UserID:      "user-1",
		Name:        "ProjectX",
		Type:        EntityProject,
		Description: "Updated description",
		Aliases:     []string{"PX", "proj-x"},
		Attributes:  map[string]string{"status": "active"},
		Tags:        []string{"infra", "backend"},
		Relations: []EntityRelation{
			{TargetID: "t1", RelationType: "depends_on"},
		},
	})
	if err != nil {
		t.Fatalf("UpsertEntity (merge): %v", err)
	}

	if merged.ID != original.ID {
		t.Fatalf("expected same ID %s, got %s", original.ID, merged.ID)
	}
	if merged.Description != "Updated description" {
		t.Fatalf("expected updated description, got %q", merged.Description)
	}
	if merged.MentionCount != 2 {
		t.Fatalf("expected mention count 2, got %d", merged.MentionCount)
	}
	// Aliases should be merged: PX + proj-x.
	if len(merged.Aliases) != 2 {
		t.Fatalf("expected 2 aliases, got %v", merged.Aliases)
	}
	// Attributes should be merged: lang + status.
	if merged.Attributes["lang"] != "Go" {
		t.Fatalf("expected attribute lang=Go, got %s", merged.Attributes["lang"])
	}
	if merged.Attributes["status"] != "active" {
		t.Fatalf("expected attribute status=active, got %s", merged.Attributes["status"])
	}
	// Tags should be merged: infra + backend.
	if len(merged.Tags) != 2 {
		t.Fatalf("expected 2 tags, got %v", merged.Tags)
	}
	// Relations should be merged.
	if len(merged.Relations) != 1 {
		t.Fatalf("expected 1 relation, got %d", len(merged.Relations))
	}
}

func TestGetEntityByID(t *testing.T) {
	store := newTestEntityStore(t)
	ctx := context.Background()

	created, err := store.UpsertEntity(ctx, Entity{
		UserID: "user-1",
		Name:   "Bob",
		Type:   EntityPerson,
	})
	if err != nil {
		t.Fatalf("UpsertEntity: %v", err)
	}

	got, err := store.GetEntity(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetEntity: %v", err)
	}
	if got.Name != "Bob" {
		t.Fatalf("expected name Bob, got %s", got.Name)
	}
	if got.ID != created.ID {
		t.Fatalf("expected ID %s, got %s", created.ID, got.ID)
	}
}

func TestGetEntityNotFound(t *testing.T) {
	store := newTestEntityStore(t)
	ctx := context.Background()

	_, err := store.GetEntity(ctx, "nonexistent-id")
	if err == nil {
		t.Fatal("expected error for non-existent entity")
	}
}

func TestSearchEntitiesByText(t *testing.T) {
	store := newTestEntityStore(t)
	ctx := context.Background()

	if _, err := store.UpsertEntity(ctx, Entity{
		UserID:      "user-1",
		Name:        "Kubernetes",
		Type:        EntityTool,
		Description: "Container orchestration platform",
	}); err != nil {
		t.Fatalf("UpsertEntity: %v", err)
	}
	if _, err := store.UpsertEntity(ctx, Entity{
		UserID:  "user-1",
		Name:    "Docker",
		Type:    EntityTool,
		Aliases: []string{"container runtime"},
	}); err != nil {
		t.Fatalf("UpsertEntity: %v", err)
	}
	if _, err := store.UpsertEntity(ctx, Entity{
		UserID: "user-1",
		Name:   "Alice",
		Type:   EntityPerson,
	}); err != nil {
		t.Fatalf("UpsertEntity: %v", err)
	}

	// Search by name substring.
	results, err := store.SearchEntities(ctx, EntityQuery{
		UserID: "user-1",
		Text:   "kube",
	})
	if err != nil {
		t.Fatalf("SearchEntities: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result for 'kube', got %d", len(results))
	}
	if results[0].Name != "Kubernetes" {
		t.Fatalf("expected Kubernetes, got %s", results[0].Name)
	}

	// Search by description substring.
	results, err = store.SearchEntities(ctx, EntityQuery{
		UserID: "user-1",
		Text:   "orchestration",
	})
	if err != nil {
		t.Fatalf("SearchEntities: %v", err)
	}
	if len(results) != 1 || results[0].Name != "Kubernetes" {
		t.Fatalf("expected Kubernetes via description, got %v", results)
	}

	// Search by alias substring.
	results, err = store.SearchEntities(ctx, EntityQuery{
		UserID: "user-1",
		Text:   "container",
	})
	if err != nil {
		t.Fatalf("SearchEntities: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results for 'container' (description + alias), got %d", len(results))
	}
}

func TestSearchEntitiesByType(t *testing.T) {
	store := newTestEntityStore(t)
	ctx := context.Background()

	if _, err := store.UpsertEntity(ctx, Entity{
		UserID: "user-1",
		Name:   "Go",
		Type:   EntityTool,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.UpsertEntity(ctx, Entity{
		UserID: "user-1",
		Name:   "Acme Corp",
		Type:   EntityOrganization,
	}); err != nil {
		t.Fatal(err)
	}

	results, err := store.SearchEntities(ctx, EntityQuery{
		UserID: "user-1",
		Types:  []EntityType{EntityTool},
	})
	if err != nil {
		t.Fatalf("SearchEntities: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(results))
	}
	if results[0].Name != "Go" {
		t.Fatalf("expected Go, got %s", results[0].Name)
	}
}

func TestSearchEntitiesByTags(t *testing.T) {
	store := newTestEntityStore(t)
	ctx := context.Background()

	if _, err := store.UpsertEntity(ctx, Entity{
		UserID: "user-1",
		Name:   "Redis",
		Type:   EntityTool,
		Tags:   []string{"database", "cache"},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.UpsertEntity(ctx, Entity{
		UserID: "user-1",
		Name:   "Postgres",
		Type:   EntityTool,
		Tags:   []string{"database"},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.UpsertEntity(ctx, Entity{
		UserID: "user-1",
		Name:   "Nginx",
		Type:   EntityTool,
		Tags:   []string{"proxy"},
	}); err != nil {
		t.Fatal(err)
	}

	results, err := store.SearchEntities(ctx, EntityQuery{
		UserID: "user-1",
		Tags:   []string{"database"},
	})
	if err != nil {
		t.Fatalf("SearchEntities: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 database-tagged entities, got %d", len(results))
	}
}

func TestSearchEntitiesByMinMentions(t *testing.T) {
	store := newTestEntityStore(t)
	ctx := context.Background()

	if _, err := store.UpsertEntity(ctx, Entity{
		UserID: "user-1",
		Name:   "Popular",
		Type:   EntityConcept,
	}); err != nil {
		t.Fatal(err)
	}
	// Upsert again to increment mention count to 2.
	if _, err := store.UpsertEntity(ctx, Entity{
		UserID: "user-1",
		Name:   "Popular",
		Type:   EntityConcept,
	}); err != nil {
		t.Fatal(err)
	}
	// Upsert again to increment mention count to 3.
	if _, err := store.UpsertEntity(ctx, Entity{
		UserID: "user-1",
		Name:   "Popular",
		Type:   EntityConcept,
	}); err != nil {
		t.Fatal(err)
	}

	if _, err := store.UpsertEntity(ctx, Entity{
		UserID: "user-1",
		Name:   "Rare",
		Type:   EntityConcept,
	}); err != nil {
		t.Fatal(err)
	}

	results, err := store.SearchEntities(ctx, EntityQuery{
		UserID:      "user-1",
		MinMentions: 3,
	})
	if err != nil {
		t.Fatalf("SearchEntities: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result with min 3 mentions, got %d", len(results))
	}
	if results[0].Name != "Popular" {
		t.Fatalf("expected Popular, got %s", results[0].Name)
	}
}

func TestAddRelation(t *testing.T) {
	store := newTestEntityStore(t)
	ctx := context.Background()

	source, err := store.UpsertEntity(ctx, Entity{
		UserID: "user-1",
		Name:   "Alice",
		Type:   EntityPerson,
	})
	if err != nil {
		t.Fatal(err)
	}
	target, err := store.UpsertEntity(ctx, Entity{
		UserID: "user-1",
		Name:   "ProjectX",
		Type:   EntityProject,
	})
	if err != nil {
		t.Fatal(err)
	}

	err = store.AddRelation(ctx, source.ID, EntityRelation{
		TargetID:     target.ID,
		RelationType: "works_on",
		Description:  "Alice works on ProjectX",
	})
	if err != nil {
		t.Fatalf("AddRelation: %v", err)
	}

	// Verify the relation was added.
	got, err := store.GetEntity(ctx, source.ID)
	if err != nil {
		t.Fatalf("GetEntity: %v", err)
	}
	if len(got.Relations) != 1 {
		t.Fatalf("expected 1 relation, got %d", len(got.Relations))
	}
	if got.Relations[0].TargetID != target.ID {
		t.Fatalf("expected relation target %s, got %s", target.ID, got.Relations[0].TargetID)
	}
	if got.Relations[0].RelationType != "works_on" {
		t.Fatalf("expected relation type works_on, got %s", got.Relations[0].RelationType)
	}

	// Adding the same relation again should be a no-op.
	err = store.AddRelation(ctx, source.ID, EntityRelation{
		TargetID:     target.ID,
		RelationType: "works_on",
	})
	if err != nil {
		t.Fatalf("AddRelation duplicate: %v", err)
	}
	got, _ = store.GetEntity(ctx, source.ID)
	if len(got.Relations) != 1 {
		t.Fatalf("expected 1 relation after duplicate add, got %d", len(got.Relations))
	}
}

func TestRecordMention(t *testing.T) {
	store := newTestEntityStore(t)
	ctx := context.Background()

	entity, err := store.UpsertEntity(ctx, Entity{
		UserID: "user-1",
		Name:   "Bob",
		Type:   EntityPerson,
	})
	if err != nil {
		t.Fatal(err)
	}
	if entity.MentionCount != 1 {
		t.Fatalf("expected initial mention count 1, got %d", entity.MentionCount)
	}
	originalLastSeen := entity.LastSeen

	// Small sleep to ensure LastSeen changes.
	time.Sleep(10 * time.Millisecond)

	err = store.RecordMention(ctx, entity.ID)
	if err != nil {
		t.Fatalf("RecordMention: %v", err)
	}

	got, err := store.GetEntity(ctx, entity.ID)
	if err != nil {
		t.Fatalf("GetEntity: %v", err)
	}
	if got.MentionCount != 2 {
		t.Fatalf("expected mention count 2, got %d", got.MentionCount)
	}
	if !got.LastSeen.After(originalLastSeen) {
		t.Fatal("expected LastSeen to be updated")
	}
}

func TestDeleteEntity(t *testing.T) {
	store := newTestEntityStore(t)
	ctx := context.Background()

	entity, err := store.UpsertEntity(ctx, Entity{
		UserID: "user-1",
		Name:   "Ephemeral",
		Type:   EntityConcept,
	})
	if err != nil {
		t.Fatal(err)
	}

	err = store.DeleteEntity(ctx, entity.ID)
	if err != nil {
		t.Fatalf("DeleteEntity: %v", err)
	}

	_, err = store.GetEntity(ctx, entity.ID)
	if err == nil {
		t.Fatal("expected error after deletion")
	}
}

func TestConcurrentAccess(t *testing.T) {
	store := newTestEntityStore(t)
	ctx := context.Background()

	// Create an initial entity to operate on.
	entity, err := store.UpsertEntity(ctx, Entity{
		UserID: "user-1",
		Name:   "Shared",
		Type:   EntityConcept,
	})
	if err != nil {
		t.Fatal(err)
	}

	const goroutines = 10
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			// Mix of operations.
			_ = store.RecordMention(ctx, entity.ID)
			_, _ = store.SearchEntities(ctx, EntityQuery{UserID: "user-1"})
			_, _ = store.GetEntity(ctx, entity.ID)
		}()
	}

	wg.Wait()

	// Verify the entity is still accessible and mention count increased.
	got, err := store.GetEntity(ctx, entity.ID)
	if err != nil {
		t.Fatalf("GetEntity after concurrent access: %v", err)
	}
	// Initial mention count is 1, plus goroutines increments.
	expectedMin := 1 + goroutines
	if got.MentionCount < expectedMin {
		t.Fatalf("expected mention count >= %d, got %d", expectedMin, got.MentionCount)
	}
}

func TestEmptyStoreReturnsEmptyResults(t *testing.T) {
	store := newTestEntityStore(t)
	ctx := context.Background()

	// Ensure user directory exists so loadEntities can work.
	results, err := store.SearchEntities(ctx, EntityQuery{
		UserID: "user-1",
		Text:   "anything",
	})
	if err != nil {
		t.Fatalf("SearchEntities on empty store: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected 0 results from empty store, got %d", len(results))
	}
}

func TestSearchEntitiesSortsByMentionCount(t *testing.T) {
	store := newTestEntityStore(t)
	ctx := context.Background()

	// Create entity with 1 mention.
	if _, err := store.UpsertEntity(ctx, Entity{
		UserID: "user-1",
		Name:   "LowMention",
		Type:   EntityConcept,
	}); err != nil {
		t.Fatal(err)
	}

	// Create entity with 3 mentions.
	for i := 0; i < 3; i++ {
		if _, err := store.UpsertEntity(ctx, Entity{
			UserID: "user-1",
			Name:   "HighMention",
			Type:   EntityConcept,
		}); err != nil {
			t.Fatal(err)
		}
	}

	results, err := store.SearchEntities(ctx, EntityQuery{
		UserID: "user-1",
	})
	if err != nil {
		t.Fatalf("SearchEntities: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].Name != "HighMention" {
		t.Fatalf("expected HighMention first, got %s", results[0].Name)
	}
	if results[1].Name != "LowMention" {
		t.Fatalf("expected LowMention second, got %s", results[1].Name)
	}
}

func TestSearchEntitiesLimit(t *testing.T) {
	store := newTestEntityStore(t)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		if _, err := store.UpsertEntity(ctx, Entity{
			UserID: "user-1",
			Name:   "Entity" + string(rune('A'+i)),
			Type:   EntityConcept,
		}); err != nil {
			t.Fatal(err)
		}
	}

	results, err := store.SearchEntities(ctx, EntityQuery{
		UserID: "user-1",
		Limit:  3,
	})
	if err != nil {
		t.Fatalf("SearchEntities: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 results (limit), got %d", len(results))
	}
}

func TestUpsertEntityValidation(t *testing.T) {
	store := newTestEntityStore(t)
	ctx := context.Background()

	if _, err := store.UpsertEntity(ctx, Entity{Name: "X", Type: EntityPerson}); err == nil {
		t.Fatal("expected error for missing UserID")
	}
	if _, err := store.UpsertEntity(ctx, Entity{UserID: "u", Type: EntityPerson}); err == nil {
		t.Fatal("expected error for missing Name")
	}
	if _, err := store.UpsertEntity(ctx, Entity{UserID: "u", Name: "X"}); err == nil {
		t.Fatal("expected error for missing Type")
	}
}

func TestSearchEntitiesTextCaseInsensitive(t *testing.T) {
	store := newTestEntityStore(t)
	ctx := context.Background()

	if _, err := store.UpsertEntity(ctx, Entity{
		UserID:      "user-1",
		Name:        "PostgreSQL",
		Type:        EntityTool,
		Description: "Relational Database",
	}); err != nil {
		t.Fatal(err)
	}

	// Search with different cases.
	for _, query := range []string{"postgresql", "POSTGRESQL", "PostgreSQL", "relational"} {
		results, err := store.SearchEntities(ctx, EntityQuery{
			UserID: "user-1",
			Text:   query,
		})
		if err != nil {
			t.Fatalf("SearchEntities(%q): %v", query, err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result for %q, got %d", query, len(results))
		}
	}
}

func TestFilePersistence(t *testing.T) {
	dir := t.TempDir()
	store1 := NewFileEntityStore(dir)
	ctx := context.Background()

	created, err := store1.UpsertEntity(ctx, Entity{
		UserID:      "user-1",
		Name:        "Persistent",
		Type:        EntityConcept,
		Description: "Should survive store recreation",
	})
	if err != nil {
		t.Fatalf("UpsertEntity: %v", err)
	}

	// Create a new store instance pointing at the same directory to verify
	// data was persisted to disk.
	store2 := NewFileEntityStore(dir)
	got, err := store2.GetEntity(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetEntity from new store: %v", err)
	}
	if got.Name != "Persistent" {
		t.Fatalf("expected name Persistent, got %s", got.Name)
	}
	if got.Description != "Should survive store recreation" {
		t.Fatalf("expected description preserved, got %q", got.Description)
	}
}

func TestAddRelationToNonExistentEntity(t *testing.T) {
	store := newTestEntityStore(t)
	ctx := context.Background()

	err := store.AddRelation(ctx, "nonexistent", EntityRelation{
		TargetID:     "target",
		RelationType: "works_on",
	})
	if err == nil {
		t.Fatal("expected error for non-existent source entity")
	}
}

func TestRecordMentionNonExistent(t *testing.T) {
	store := newTestEntityStore(t)
	ctx := context.Background()

	err := store.RecordMention(ctx, "nonexistent")
	if err == nil {
		t.Fatal("expected error for non-existent entity")
	}
}

func TestDeleteEntityNonExistent(t *testing.T) {
	store := newTestEntityStore(t)
	ctx := context.Background()

	err := store.DeleteEntity(ctx, "nonexistent")
	if err == nil {
		t.Fatal("expected error for non-existent entity")
	}
}
