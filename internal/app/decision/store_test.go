package decision

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	fp := filepath.Join(t.TempDir(), "decisions.json")
	s, err := NewStore(fp)
	require.NoError(t, err)
	return s
}

func newMemoryStore(t *testing.T) *Store {
	t.Helper()
	s, err := NewStore("")
	require.NoError(t, err)
	return s
}

func sampleDecision(id string) *Decision {
	return &Decision{
		ID:           id,
		Title:        "Use Go for backend",
		Description:  "Evaluated Go vs Rust vs Python for API server",
		DecidedBy:    "alice",
		Participants: []string{"alice", "bob", "charlie"},
		Alternatives: []string{"Rust", "Python"},
		Outcome:      "Go selected for ecosystem maturity and team familiarity",
		Tags:         []string{"backend", "language"},
		CreatedAt:    time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC),
	}
}

// ---------------------------------------------------------------------------
// CRUD
// ---------------------------------------------------------------------------

func TestAdd_And_Get(t *testing.T) {
	s := newTestStore(t)
	d := sampleDecision("d-1")
	require.NoError(t, s.Add(d))

	got := s.Get("d-1")
	require.NotNil(t, got)
	assert.Equal(t, "Use Go for backend", got.Title)
	assert.Equal(t, "alice", got.DecidedBy)
	assert.Equal(t, []string{"Rust", "Python"}, got.Alternatives)
}

func TestAdd_EmptyID(t *testing.T) {
	s := newMemoryStore(t)
	err := s.Add(&Decision{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ID is required")
}

func TestAdd_Duplicate(t *testing.T) {
	s := newMemoryStore(t)
	require.NoError(t, s.Add(sampleDecision("d-1")))
	err := s.Add(sampleDecision("d-1"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

func TestAdd_DefaultsCreatedAt(t *testing.T) {
	s := newMemoryStore(t)
	d := &Decision{ID: "d-1", Title: "test"}
	before := time.Now()
	require.NoError(t, s.Add(d))
	got := s.Get("d-1")
	assert.False(t, got.CreatedAt.IsZero())
	assert.True(t, !got.CreatedAt.Before(before))
}

func TestGet_NotFound(t *testing.T) {
	s := newMemoryStore(t)
	assert.Nil(t, s.Get("nonexistent"))
}

func TestGet_ReturnsCopy(t *testing.T) {
	s := newMemoryStore(t)
	require.NoError(t, s.Add(sampleDecision("d-1")))
	got := s.Get("d-1")
	got.Title = "mutated"
	assert.Equal(t, "Use Go for backend", s.Get("d-1").Title)
}

func TestUpdate(t *testing.T) {
	s := newMemoryStore(t)
	require.NoError(t, s.Add(sampleDecision("d-1")))
	d := s.Get("d-1")
	d.Outcome = "Revised: switched to Rust"
	require.NoError(t, s.Update(d))
	got := s.Get("d-1")
	assert.Equal(t, "Revised: switched to Rust", got.Outcome)
	assert.True(t, got.UpdatedAt.After(got.CreatedAt) || got.UpdatedAt.Equal(got.CreatedAt))
}

func TestUpdate_NotFound(t *testing.T) {
	s := newMemoryStore(t)
	err := s.Update(&Decision{ID: "missing"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestUpdate_EmptyID(t *testing.T) {
	s := newMemoryStore(t)
	err := s.Update(&Decision{})
	assert.Error(t, err)
}

func TestDelete(t *testing.T) {
	s := newMemoryStore(t)
	require.NoError(t, s.Add(sampleDecision("d-1")))
	require.NoError(t, s.Delete("d-1"))
	assert.Nil(t, s.Get("d-1"))
}

func TestDelete_NotFound(t *testing.T) {
	s := newMemoryStore(t)
	err := s.Delete("nope")
	assert.Error(t, err)
}

// ---------------------------------------------------------------------------
// List
// ---------------------------------------------------------------------------

func TestList_Empty(t *testing.T) {
	s := newMemoryStore(t)
	assert.Empty(t, s.List())
}

func TestList_SortedNewestFirst(t *testing.T) {
	s := newMemoryStore(t)
	d1 := sampleDecision("d-old")
	d1.CreatedAt = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	d2 := sampleDecision("d-new")
	d2.CreatedAt = time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	require.NoError(t, s.Add(d1))
	require.NoError(t, s.Add(d2))
	list := s.List()
	require.Len(t, list, 2)
	assert.Equal(t, "d-new", list[0].ID)
	assert.Equal(t, "d-old", list[1].ID)
}

// ---------------------------------------------------------------------------
// Search
// ---------------------------------------------------------------------------

func TestSearch_ByTopic(t *testing.T) {
	s := newMemoryStore(t)
	require.NoError(t, s.Add(sampleDecision("d-1")))
	d2 := &Decision{ID: "d-2", Title: "Deploy to AWS", CreatedAt: time.Now()}
	require.NoError(t, s.Add(d2))

	results := s.Search(SearchFilter{Topic: "backend"})
	require.Len(t, results, 1)
	assert.Equal(t, "d-1", results[0].ID)
}

func TestSearch_ByTopicInDescription(t *testing.T) {
	s := newMemoryStore(t)
	require.NoError(t, s.Add(sampleDecision("d-1")))
	results := s.Search(SearchFilter{Topic: "rust"})
	require.Len(t, results, 1)
}

func TestSearch_ByTopicInTags(t *testing.T) {
	s := newMemoryStore(t)
	require.NoError(t, s.Add(sampleDecision("d-1")))
	results := s.Search(SearchFilter{Topic: "language"})
	require.Len(t, results, 1)
}

func TestSearch_ByTags(t *testing.T) {
	s := newMemoryStore(t)
	require.NoError(t, s.Add(sampleDecision("d-1")))
	d2 := &Decision{ID: "d-2", Title: "CI system", Tags: []string{"infra"}, CreatedAt: time.Now()}
	require.NoError(t, s.Add(d2))

	results := s.Search(SearchFilter{Tags: []string{"backend", "language"}})
	require.Len(t, results, 1)
	assert.Equal(t, "d-1", results[0].ID)
}

func TestSearch_ByTags_CaseInsensitive(t *testing.T) {
	s := newMemoryStore(t)
	require.NoError(t, s.Add(sampleDecision("d-1")))
	results := s.Search(SearchFilter{Tags: []string{"Backend"}})
	require.Len(t, results, 1)
}

func TestSearch_ByParticipant_DecidedBy(t *testing.T) {
	s := newMemoryStore(t)
	require.NoError(t, s.Add(sampleDecision("d-1")))
	results := s.Search(SearchFilter{Participant: "alice"})
	require.Len(t, results, 1)
}

func TestSearch_ByParticipant_InList(t *testing.T) {
	s := newMemoryStore(t)
	require.NoError(t, s.Add(sampleDecision("d-1")))
	results := s.Search(SearchFilter{Participant: "bob"})
	require.Len(t, results, 1)
}

func TestSearch_ByParticipant_CaseInsensitive(t *testing.T) {
	s := newMemoryStore(t)
	require.NoError(t, s.Add(sampleDecision("d-1")))
	results := s.Search(SearchFilter{Participant: "ALICE"})
	require.Len(t, results, 1)
}

func TestSearch_ByParticipant_NoMatch(t *testing.T) {
	s := newMemoryStore(t)
	require.NoError(t, s.Add(sampleDecision("d-1")))
	results := s.Search(SearchFilter{Participant: "dave"})
	assert.Empty(t, results)
}

func TestSearch_ByDateRange(t *testing.T) {
	s := newMemoryStore(t)
	d1 := sampleDecision("d-old")
	d1.CreatedAt = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	d2 := sampleDecision("d-new")
	d2.CreatedAt = time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC)
	require.NoError(t, s.Add(d1))
	require.NoError(t, s.Add(d2))

	// After Feb 1 only
	results := s.Search(SearchFilter{After: time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)})
	require.Len(t, results, 1)
	assert.Equal(t, "d-new", results[0].ID)

	// Before Feb 1 only
	results = s.Search(SearchFilter{Before: time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)})
	require.Len(t, results, 1)
	assert.Equal(t, "d-old", results[0].ID)
}

func TestSearch_CombinedFilters(t *testing.T) {
	s := newMemoryStore(t)
	d1 := sampleDecision("d-1")
	d1.DecidedBy = "alice"
	d1.Tags = []string{"backend"}
	d1.CreatedAt = time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	require.NoError(t, s.Add(d1))

	d2 := &Decision{
		ID: "d-2", Title: "Frontend framework", DecidedBy: "bob",
		Tags: []string{"frontend"}, CreatedAt: time.Date(2026, 3, 5, 0, 0, 0, 0, time.UTC),
	}
	require.NoError(t, s.Add(d2))

	results := s.Search(SearchFilter{Participant: "alice", Tags: []string{"backend"}})
	require.Len(t, results, 1)
	assert.Equal(t, "d-1", results[0].ID)
}

func TestSearch_NoFilter_ReturnsAll(t *testing.T) {
	s := newMemoryStore(t)
	require.NoError(t, s.Add(sampleDecision("d-1")))
	require.NoError(t, s.Add(&Decision{ID: "d-2", Title: "x", CreatedAt: time.Now()}))
	results := s.Search(SearchFilter{})
	assert.Len(t, results, 2)
}

// ---------------------------------------------------------------------------
// Persistence round-trip
// ---------------------------------------------------------------------------

func TestPersistence_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "decisions.json")

	s1, err := NewStore(fp)
	require.NoError(t, err)
	require.NoError(t, s1.Add(sampleDecision("d-1")))
	require.NoError(t, s1.Add(&Decision{ID: "d-2", Title: "Another", CreatedAt: time.Now()}))

	// Load into a new store from same file.
	s2, err := NewStore(fp)
	require.NoError(t, err)
	list := s2.List()
	require.Len(t, list, 2)
	got := s2.Get("d-1")
	require.NotNil(t, got)
	assert.Equal(t, "Use Go for backend", got.Title)
	assert.Equal(t, []string{"Rust", "Python"}, got.Alternatives)
}

func TestPersistence_DeletePersists(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "decisions.json")

	s1, err := NewStore(fp)
	require.NoError(t, err)
	require.NoError(t, s1.Add(sampleDecision("d-1")))
	require.NoError(t, s1.Delete("d-1"))

	s2, err := NewStore(fp)
	require.NoError(t, err)
	assert.Empty(t, s2.List())
}

func TestNewStore_NonexistentFile(t *testing.T) {
	fp := filepath.Join(t.TempDir(), "subdir", "decisions.json")
	s, err := NewStore(fp)
	require.NoError(t, err)
	assert.Empty(t, s.List())
}

func TestNewStore_MemoryMode(t *testing.T) {
	s, err := NewStore("")
	require.NoError(t, err)
	require.NoError(t, s.Add(sampleDecision("d-1")))
	assert.NotNil(t, s.Get("d-1"))
}

// ---------------------------------------------------------------------------
// FormatSummary
// ---------------------------------------------------------------------------

func TestFormatSummary_Empty(t *testing.T) {
	s := newMemoryStore(t)
	md := s.FormatSummary(7*24*time.Hour, "")
	assert.Contains(t, md, "### Recent Decisions")
	assert.Contains(t, md, "No decisions recorded")
}

func TestFormatSummary_WithDecisions(t *testing.T) {
	s := newMemoryStore(t)
	d := sampleDecision("d-1")
	d.CreatedAt = time.Now().Add(-1 * time.Hour)
	require.NoError(t, s.Add(d))

	md := s.FormatSummary(24*time.Hour, "")
	assert.Contains(t, md, "### Recent Decisions")
	assert.Contains(t, md, "Use Go for backend")
	assert.Contains(t, md, "Go selected for ecosystem maturity")
	assert.Contains(t, md, "Tags: backend, language")
}

func TestFormatSummary_FiltersByParticipant(t *testing.T) {
	s := newMemoryStore(t)
	d1 := sampleDecision("d-1")
	d1.CreatedAt = time.Now()
	require.NoError(t, s.Add(d1))

	d2 := &Decision{ID: "d-2", Title: "Other", DecidedBy: "dave", CreatedAt: time.Now()}
	require.NoError(t, s.Add(d2))

	md := s.FormatSummary(24*time.Hour, "alice")
	assert.Contains(t, md, "Use Go for backend")
	assert.NotContains(t, md, "Other")
}

func TestFormatSummary_RespectsLookback(t *testing.T) {
	s := newMemoryStore(t)
	d := sampleDecision("d-old")
	d.CreatedAt = time.Now().Add(-30 * 24 * time.Hour)
	require.NoError(t, s.Add(d))

	md := s.FormatSummary(7*24*time.Hour, "")
	assert.Contains(t, md, "No decisions recorded")
}

// ---------------------------------------------------------------------------
// Edge cases
// ---------------------------------------------------------------------------

func TestTruncate(t *testing.T) {
	assert.Equal(t, "short", truncate("short", 20))
	assert.Equal(t, "abcdefghij...", truncate("abcdefghijklmnop", 13))
	assert.Equal(t, "", truncate("", 10))
	assert.Equal(t, "trimmed", truncate("  trimmed  ", 20))
}

func TestSearch_ReturnsCopies(t *testing.T) {
	s := newMemoryStore(t)
	require.NoError(t, s.Add(sampleDecision("d-1")))
	results := s.Search(SearchFilter{})
	results[0].Title = "mutated"
	assert.Equal(t, "Use Go for backend", s.Get("d-1").Title)
}

func TestFormatSummary_TruncatesLongOutcome(t *testing.T) {
	s := newMemoryStore(t)
	d := &Decision{
		ID:        "d-long",
		Title:     "Long decision",
		Outcome:   strings.Repeat("x", 200),
		CreatedAt: time.Now(),
	}
	require.NoError(t, s.Add(d))
	md := s.FormatSummary(24*time.Hour, "")
	// Outcome should be truncated to 120 chars + "..."
	assert.Contains(t, md, "...")
	assert.Less(t, len(md), 500)
}
