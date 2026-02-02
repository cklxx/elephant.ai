package memory

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

func newTestDecisionStore(t *testing.T) *FileDecisionStore {
	t.Helper()
	dir := t.TempDir()
	return NewFileDecisionStore(dir)
}

func TestRecordDecisionStoresAndReturnsWithID(t *testing.T) {
	store := newTestDecisionStore(t)
	ctx := context.Background()

	entry, err := store.RecordDecision(ctx, DecisionEntry{
		UserID:    "user-1",
		SessionID: "sess-1",
		Decision:  "Use PostgreSQL for persistence",
		Rationale: "ACID compliance needed",
		Context:   "Choosing a database for user data",
		Alternatives: []string{"MongoDB", "SQLite"},
		Tags:         []string{"architecture", "database"},
	})
	if err != nil {
		t.Fatalf("RecordDecision returned error: %v", err)
	}
	if entry.ID == "" {
		t.Fatal("expected generated ID")
	}
	if entry.CreatedAt.IsZero() {
		t.Fatal("expected CreatedAt to be set")
	}
	if entry.Decision != "Use PostgreSQL for persistence" {
		t.Fatalf("unexpected decision: %s", entry.Decision)
	}
	if entry.UserID != "user-1" {
		t.Fatalf("unexpected user_id: %s", entry.UserID)
	}
}

func TestRecordDecisionRejectsEmptyUser(t *testing.T) {
	store := newTestDecisionStore(t)
	ctx := context.Background()

	_, err := store.RecordDecision(ctx, DecisionEntry{
		Decision: "something",
	})
	if err == nil {
		t.Fatal("expected error for missing user_id")
	}
}

func TestRecordDecisionRejectsEmptyDecision(t *testing.T) {
	store := newTestDecisionStore(t)
	ctx := context.Background()

	_, err := store.RecordDecision(ctx, DecisionEntry{
		UserID: "user-1",
	})
	if err == nil {
		t.Fatal("expected error for missing decision")
	}
}

func TestResolveDecisionUpdatesOutcome(t *testing.T) {
	store := newTestDecisionStore(t)
	ctx := context.Background()

	entry, err := store.RecordDecision(ctx, DecisionEntry{
		UserID:   "user-1",
		Decision: "Use PostgreSQL",
		Tags:     []string{"database"},
	})
	if err != nil {
		t.Fatalf("RecordDecision: %v", err)
	}

	err = store.ResolveDecision(ctx, entry.ID, "Worked well, excellent query performance", true)
	if err != nil {
		t.Fatalf("ResolveDecision: %v", err)
	}

	decisions, err := store.RecentDecisions(ctx, "user-1", 10)
	if err != nil {
		t.Fatalf("RecentDecisions: %v", err)
	}
	if len(decisions) != 1 {
		t.Fatalf("expected 1 decision, got %d", len(decisions))
	}
	if decisions[0].Outcome != "Worked well, excellent query performance" {
		t.Fatalf("unexpected outcome: %s", decisions[0].Outcome)
	}
	if !decisions[0].OutcomeSuccess {
		t.Fatal("expected outcome_success to be true")
	}
	if decisions[0].ResolvedAt == nil {
		t.Fatal("expected resolved_at to be set")
	}
}

func TestResolveDecisionNonExistentIDReturnsError(t *testing.T) {
	store := newTestDecisionStore(t)
	ctx := context.Background()

	err := store.ResolveDecision(ctx, "nonexistent-id", "outcome", true)
	if err == nil {
		t.Fatal("expected error for non-existent decision ID")
	}
}

func TestSearchDecisionsByTags(t *testing.T) {
	store := newTestDecisionStore(t)
	ctx := context.Background()

	_, err := store.RecordDecision(ctx, DecisionEntry{
		UserID:   "user-1",
		Decision: "Use PostgreSQL",
		Tags:     []string{"database", "architecture"},
	})
	if err != nil {
		t.Fatalf("RecordDecision 1: %v", err)
	}
	_, err = store.RecordDecision(ctx, DecisionEntry{
		UserID:   "user-1",
		Decision: "Use React for frontend",
		Tags:     []string{"frontend", "architecture"},
	})
	if err != nil {
		t.Fatalf("RecordDecision 2: %v", err)
	}
	_, err = store.RecordDecision(ctx, DecisionEntry{
		UserID:   "user-1",
		Decision: "Use Redis for caching",
		Tags:     []string{"database", "caching"},
	})
	if err != nil {
		t.Fatalf("RecordDecision 3: %v", err)
	}

	results, err := store.SearchDecisions(ctx, DecisionQuery{
		UserID: "user-1",
		Tags:   []string{"database"},
	})
	if err != nil {
		t.Fatalf("SearchDecisions: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results for tag 'database', got %d", len(results))
	}
}

func TestSearchDecisionsByText(t *testing.T) {
	store := newTestDecisionStore(t)
	ctx := context.Background()

	_, err := store.RecordDecision(ctx, DecisionEntry{
		UserID:    "user-1",
		Decision:  "Use PostgreSQL",
		Rationale: "ACID compliance needed for financial data",
		Context:   "Database selection phase",
	})
	if err != nil {
		t.Fatalf("RecordDecision 1: %v", err)
	}
	_, err = store.RecordDecision(ctx, DecisionEntry{
		UserID:    "user-1",
		Decision:  "Use React",
		Rationale: "Large ecosystem and team expertise",
		Context:   "Frontend framework selection",
	})
	if err != nil {
		t.Fatalf("RecordDecision 2: %v", err)
	}

	// Search in Decision field
	results, err := store.SearchDecisions(ctx, DecisionQuery{
		UserID: "user-1",
		Text:   "PostgreSQL",
	})
	if err != nil {
		t.Fatalf("SearchDecisions by decision: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result for 'PostgreSQL', got %d", len(results))
	}

	// Search in Rationale field
	results, err = store.SearchDecisions(ctx, DecisionQuery{
		UserID: "user-1",
		Text:   "financial",
	})
	if err != nil {
		t.Fatalf("SearchDecisions by rationale: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result for 'financial', got %d", len(results))
	}

	// Search in Context field
	results, err = store.SearchDecisions(ctx, DecisionQuery{
		UserID: "user-1",
		Text:   "frontend framework",
	})
	if err != nil {
		t.Fatalf("SearchDecisions by context: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result for 'frontend framework', got %d", len(results))
	}

	// Case-insensitive search
	results, err = store.SearchDecisions(ctx, DecisionQuery{
		UserID: "user-1",
		Text:   "postgresql",
	})
	if err != nil {
		t.Fatalf("SearchDecisions case-insensitive: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result for case-insensitive 'postgresql', got %d", len(results))
	}
}

func TestSearchDecisionsOnlyUnresolved(t *testing.T) {
	store := newTestDecisionStore(t)
	ctx := context.Background()

	entry1, err := store.RecordDecision(ctx, DecisionEntry{
		UserID:   "user-1",
		Decision: "Use PostgreSQL",
		Tags:     []string{"database"},
	})
	if err != nil {
		t.Fatalf("RecordDecision 1: %v", err)
	}
	_, err = store.RecordDecision(ctx, DecisionEntry{
		UserID:   "user-1",
		Decision: "Use Redis",
		Tags:     []string{"database"},
	})
	if err != nil {
		t.Fatalf("RecordDecision 2: %v", err)
	}

	// Resolve the first decision
	err = store.ResolveDecision(ctx, entry1.ID, "Good choice", true)
	if err != nil {
		t.Fatalf("ResolveDecision: %v", err)
	}

	results, err := store.SearchDecisions(ctx, DecisionQuery{
		UserID:         "user-1",
		OnlyUnresolved: true,
	})
	if err != nil {
		t.Fatalf("SearchDecisions: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 unresolved result, got %d", len(results))
	}
	if results[0].Decision != "Use Redis" {
		t.Fatalf("expected unresolved decision 'Use Redis', got %q", results[0].Decision)
	}
}

func TestSearchDecisionsWithSinceFilter(t *testing.T) {
	store := newTestDecisionStore(t)
	ctx := context.Background()

	now := time.Now()
	oldTime := now.Add(-48 * time.Hour)
	recentTime := now.Add(-1 * time.Hour)

	_, err := store.RecordDecision(ctx, DecisionEntry{
		UserID:    "user-1",
		Decision:  "Old decision",
		CreatedAt: oldTime,
	})
	if err != nil {
		t.Fatalf("RecordDecision old: %v", err)
	}
	_, err = store.RecordDecision(ctx, DecisionEntry{
		UserID:    "user-1",
		Decision:  "Recent decision",
		CreatedAt: recentTime,
	})
	if err != nil {
		t.Fatalf("RecordDecision recent: %v", err)
	}

	cutoff := now.Add(-24 * time.Hour)
	results, err := store.SearchDecisions(ctx, DecisionQuery{
		UserID: "user-1",
		Since:  cutoff,
	})
	if err != nil {
		t.Fatalf("SearchDecisions: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result since cutoff, got %d", len(results))
	}
	if results[0].Decision != "Recent decision" {
		t.Fatalf("expected 'Recent decision', got %q", results[0].Decision)
	}
}

func TestRecentDecisionsReverseChronological(t *testing.T) {
	store := newTestDecisionStore(t)
	ctx := context.Background()

	now := time.Now()
	for i := 0; i < 5; i++ {
		_, err := store.RecordDecision(ctx, DecisionEntry{
			UserID:    "user-1",
			Decision:  fmt.Sprintf("Decision %d", i),
			CreatedAt: now.Add(time.Duration(i) * time.Hour),
		})
		if err != nil {
			t.Fatalf("RecordDecision %d: %v", i, err)
		}
	}

	results, err := store.RecentDecisions(ctx, "user-1", 10)
	if err != nil {
		t.Fatalf("RecentDecisions: %v", err)
	}
	if len(results) != 5 {
		t.Fatalf("expected 5 results, got %d", len(results))
	}

	// Verify reverse chronological order
	for i := 1; i < len(results); i++ {
		if results[i-1].CreatedAt.Before(results[i].CreatedAt) {
			t.Fatalf("results not in reverse chronological order: %v before %v",
				results[i-1].CreatedAt, results[i].CreatedAt)
		}
	}

	// Most recent should be "Decision 4"
	if results[0].Decision != "Decision 4" {
		t.Fatalf("expected most recent to be 'Decision 4', got %q", results[0].Decision)
	}
}

func TestRecentDecisionsRespectsLimit(t *testing.T) {
	store := newTestDecisionStore(t)
	ctx := context.Background()

	now := time.Now()
	for i := 0; i < 10; i++ {
		_, err := store.RecordDecision(ctx, DecisionEntry{
			UserID:    "user-1",
			Decision:  fmt.Sprintf("Decision %d", i),
			CreatedAt: now.Add(time.Duration(i) * time.Minute),
		})
		if err != nil {
			t.Fatalf("RecordDecision %d: %v", i, err)
		}
	}

	results, err := store.RecentDecisions(ctx, "user-1", 3)
	if err != nil {
		t.Fatalf("RecentDecisions: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	// Should be the 3 most recent
	if results[0].Decision != "Decision 9" {
		t.Fatalf("expected 'Decision 9', got %q", results[0].Decision)
	}
	if results[2].Decision != "Decision 7" {
		t.Fatalf("expected 'Decision 7', got %q", results[2].Decision)
	}
}

func TestDecisionConcurrentAccess(t *testing.T) {
	store := newTestDecisionStore(t)
	ctx := context.Background()

	const goroutines = 10
	const decisionsPerGoroutine = 5

	var wg sync.WaitGroup
	errCh := make(chan error, goroutines*decisionsPerGoroutine)

	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(gID int) {
			defer wg.Done()
			for i := 0; i < decisionsPerGoroutine; i++ {
				_, err := store.RecordDecision(ctx, DecisionEntry{
					UserID:   "user-1",
					Decision: fmt.Sprintf("Decision from goroutine %d, item %d", gID, i),
					Tags:     []string{"concurrent"},
				})
				if err != nil {
					errCh <- err
				}
			}
		}(g)
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Fatalf("concurrent RecordDecision error: %v", err)
	}

	results, err := store.RecentDecisions(ctx, "user-1", 100)
	if err != nil {
		t.Fatalf("RecentDecisions: %v", err)
	}
	expected := goroutines * decisionsPerGoroutine
	if len(results) != expected {
		t.Fatalf("expected %d decisions, got %d", expected, len(results))
	}
}

func TestDecisionEmptyStoreReturnsEmptyResults(t *testing.T) {
	store := newTestDecisionStore(t)
	ctx := context.Background()

	results, err := store.RecentDecisions(ctx, "user-1", 10)
	if err != nil {
		t.Fatalf("RecentDecisions on empty store: %v", err)
	}
	if results != nil && len(results) != 0 {
		t.Fatalf("expected empty results on empty store, got %d", len(results))
	}

	searchResults, err := store.SearchDecisions(ctx, DecisionQuery{
		UserID: "user-1",
		Text:   "anything",
	})
	if err != nil {
		t.Fatalf("SearchDecisions on empty store: %v", err)
	}
	if searchResults != nil && len(searchResults) != 0 {
		t.Fatalf("expected empty search results on empty store, got %d", len(searchResults))
	}
}

func TestSearchDecisionsLimitIsRespected(t *testing.T) {
	store := newTestDecisionStore(t)
	ctx := context.Background()

	for i := 0; i < 10; i++ {
		_, err := store.RecordDecision(ctx, DecisionEntry{
			UserID:   "user-1",
			Decision: fmt.Sprintf("Decision %d about architecture", i),
			Tags:     []string{"architecture"},
		})
		if err != nil {
			t.Fatalf("RecordDecision %d: %v", i, err)
		}
	}

	results, err := store.SearchDecisions(ctx, DecisionQuery{
		UserID: "user-1",
		Tags:   []string{"architecture"},
		Limit:  3,
	})
	if err != nil {
		t.Fatalf("SearchDecisions: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 results with limit, got %d", len(results))
	}
}

func TestDecisionFilePersistence(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()

	// Write with one store instance.
	store1 := NewFileDecisionStore(dir)
	entry, err := store1.RecordDecision(ctx, DecisionEntry{
		UserID:   "user-1",
		Decision: "Persist to disk",
		Tags:     []string{"storage"},
	})
	if err != nil {
		t.Fatalf("RecordDecision: %v", err)
	}

	// Read with a fresh store instance (cold cache).
	store2 := NewFileDecisionStore(dir)
	results, err := store2.RecentDecisions(ctx, "user-1", 10)
	if err != nil {
		t.Fatalf("RecentDecisions from new store: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 persisted result, got %d", len(results))
	}
	if results[0].ID != entry.ID {
		t.Fatalf("expected ID %s, got %s", entry.ID, results[0].ID)
	}
	if results[0].Decision != "Persist to disk" {
		t.Fatalf("expected 'Persist to disk', got %q", results[0].Decision)
	}
}

func TestResolveDecisionFromDisk(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()

	// Record with one store.
	store1 := NewFileDecisionStore(dir)
	entry, err := store1.RecordDecision(ctx, DecisionEntry{
		UserID:   "user-1",
		Decision: "Test resolve from disk",
	})
	if err != nil {
		t.Fatalf("RecordDecision: %v", err)
	}

	// Resolve with a fresh store (not in cache).
	store2 := NewFileDecisionStore(dir)
	err = store2.ResolveDecision(ctx, entry.ID, "Resolved from cold", true)
	if err != nil {
		t.Fatalf("ResolveDecision from cold store: %v", err)
	}

	results, err := store2.RecentDecisions(ctx, "user-1", 10)
	if err != nil {
		t.Fatalf("RecentDecisions: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Outcome != "Resolved from cold" {
		t.Fatalf("expected outcome 'Resolved from cold', got %q", results[0].Outcome)
	}
	if !results[0].OutcomeSuccess {
		t.Fatal("expected outcome_success to be true")
	}
}

func TestSearchDecisionsCombinedFilters(t *testing.T) {
	store := newTestDecisionStore(t)
	ctx := context.Background()

	now := time.Now()

	// Old resolved decision
	e1, err := store.RecordDecision(ctx, DecisionEntry{
		UserID:    "user-1",
		Decision:  "Use PostgreSQL for users",
		CreatedAt: now.Add(-72 * time.Hour),
		Tags:      []string{"database"},
	})
	if err != nil {
		t.Fatalf("RecordDecision 1: %v", err)
	}
	_ = store.ResolveDecision(ctx, e1.ID, "Good", true)

	// Recent unresolved decision
	_, err = store.RecordDecision(ctx, DecisionEntry{
		UserID:    "user-1",
		Decision:  "Use Redis for caching",
		CreatedAt: now.Add(-1 * time.Hour),
		Tags:      []string{"database", "caching"},
	})
	if err != nil {
		t.Fatalf("RecordDecision 2: %v", err)
	}

	// Recent resolved decision
	e3, err := store.RecordDecision(ctx, DecisionEntry{
		UserID:    "user-1",
		Decision:  "Use MongoDB for logs",
		CreatedAt: now.Add(-30 * time.Minute),
		Tags:      []string{"database", "logging"},
	})
	if err != nil {
		t.Fatalf("RecordDecision 3: %v", err)
	}
	_ = store.ResolveDecision(ctx, e3.ID, "Meh", false)

	// Combined: database tag, unresolved, since 24 hours ago
	results, err := store.SearchDecisions(ctx, DecisionQuery{
		UserID:         "user-1",
		Tags:           []string{"database"},
		OnlyUnresolved: true,
		Since:          now.Add(-24 * time.Hour),
	})
	if err != nil {
		t.Fatalf("SearchDecisions: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result with combined filters, got %d", len(results))
	}
	if results[0].Decision != "Use Redis for caching" {
		t.Fatalf("expected 'Use Redis for caching', got %q", results[0].Decision)
	}
}
