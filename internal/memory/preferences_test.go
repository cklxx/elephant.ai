package memory

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

func newTestPreferenceStore(t *testing.T) *FilePreferenceStore {
	t.Helper()
	return NewFilePreferenceStore(t.TempDir())
}

func TestSetPreferenceCreatesNew(t *testing.T) {
	store := newTestPreferenceStore(t)
	ctx := context.Background()

	pref, err := store.SetPreference(ctx, Preference{
		UserID:   "user-1",
		Category: PreferenceCategoryLanguage,
		Key:      "preferred_language",
		Value:    "en",
		Source:   "explicit",
	})
	if err != nil {
		t.Fatalf("SetPreference: %v", err)
	}
	if pref.ID == "" {
		t.Fatal("expected generated ID")
	}
	if pref.UserID != "user-1" {
		t.Fatalf("expected user_id user-1, got %s", pref.UserID)
	}
	if pref.Category != PreferenceCategoryLanguage {
		t.Fatalf("expected category language, got %s", pref.Category)
	}
	if pref.Key != "preferred_language" {
		t.Fatalf("expected key preferred_language, got %s", pref.Key)
	}
	if pref.Value != "en" {
		t.Fatalf("expected value en, got %s", pref.Value)
	}
	if pref.ObservationCount != 1 {
		t.Fatalf("expected observation count 1, got %d", pref.ObservationCount)
	}
	// New preference: confidence = 0.3 + 0.1*1 = 0.4
	expectedConf := 0.4
	if pref.Confidence != expectedConf {
		t.Fatalf("expected confidence %f, got %f", expectedConf, pref.Confidence)
	}
	if pref.Source != "explicit" {
		t.Fatalf("expected source explicit, got %s", pref.Source)
	}
	if pref.CreatedAt.IsZero() {
		t.Fatal("expected CreatedAt set")
	}
	if pref.LastObserved.IsZero() {
		t.Fatal("expected LastObserved set")
	}
}

func TestSetPreferenceUpsertExisting(t *testing.T) {
	store := newTestPreferenceStore(t)
	ctx := context.Background()

	// Create initial preference.
	original, err := store.SetPreference(ctx, Preference{
		UserID:   "user-1",
		Category: PreferenceCategoryFormat,
		Key:      "code_format",
		Value:    "tabs",
		Source:   "inferred",
	})
	if err != nil {
		t.Fatalf("SetPreference (create): %v", err)
	}
	originalLastObserved := original.LastObserved

	// Small sleep to ensure LastObserved changes.
	time.Sleep(10 * time.Millisecond)

	// Upsert same UserID+Category+Key.
	updated, err := store.SetPreference(ctx, Preference{
		UserID:   "user-1",
		Category: PreferenceCategoryFormat,
		Key:      "code_format",
		Value:    "spaces",
		Source:   "feedback",
	})
	if err != nil {
		t.Fatalf("SetPreference (upsert): %v", err)
	}

	if updated.ID != original.ID {
		t.Fatalf("expected same ID %s, got %s", original.ID, updated.ID)
	}
	if updated.ObservationCount != 2 {
		t.Fatalf("expected observation count 2, got %d", updated.ObservationCount)
	}
	// Confidence = min(1.0, 0.3 + 0.1*2) = 0.5
	expectedConf := 0.5
	if updated.Confidence != expectedConf {
		t.Fatalf("expected confidence %f, got %f", expectedConf, updated.Confidence)
	}
	if updated.Value != "spaces" {
		t.Fatalf("expected value spaces, got %s", updated.Value)
	}
	if updated.Source != "feedback" {
		t.Fatalf("expected source feedback, got %s", updated.Source)
	}
	if !updated.LastObserved.After(originalLastObserved) {
		t.Fatal("expected LastObserved to be updated")
	}
}

func TestSetPreferenceConfidenceIncreasesWithObservations(t *testing.T) {
	store := newTestPreferenceStore(t)
	ctx := context.Background()

	// Create and then upsert multiple times.
	for i := 0; i < 10; i++ {
		_, err := store.SetPreference(ctx, Preference{
			UserID:   "user-1",
			Category: PreferenceCategoryStyle,
			Key:      "verbosity",
			Value:    "concise",
			Source:   "inferred",
		})
		if err != nil {
			t.Fatalf("SetPreference iteration %d: %v", i, err)
		}
	}

	pref, err := store.GetPreference(ctx, "user-1", string(PreferenceCategoryStyle), "verbosity")
	if err != nil {
		t.Fatalf("GetPreference: %v", err)
	}

	if pref.ObservationCount != 10 {
		t.Fatalf("expected observation count 10, got %d", pref.ObservationCount)
	}
	// Confidence = min(1.0, 0.3 + 0.1*10) = min(1.0, 1.3) = 1.0
	if pref.Confidence != 1.0 {
		t.Fatalf("expected confidence 1.0 (capped), got %f", pref.Confidence)
	}
}

func TestGetPreference(t *testing.T) {
	store := newTestPreferenceStore(t)
	ctx := context.Background()

	_, err := store.SetPreference(ctx, Preference{
		UserID:   "user-1",
		Category: PreferenceCategoryTimezone,
		Key:      "tz",
		Value:    "Asia/Shanghai",
		Source:   "explicit",
	})
	if err != nil {
		t.Fatalf("SetPreference: %v", err)
	}

	got, err := store.GetPreference(ctx, "user-1", string(PreferenceCategoryTimezone), "tz")
	if err != nil {
		t.Fatalf("GetPreference: %v", err)
	}
	if got.Value != "Asia/Shanghai" {
		t.Fatalf("expected value Asia/Shanghai, got %s", got.Value)
	}
	if got.Category != PreferenceCategoryTimezone {
		t.Fatalf("expected category timezone, got %s", got.Category)
	}
}

func TestGetPreferenceNotFound(t *testing.T) {
	store := newTestPreferenceStore(t)
	ctx := context.Background()

	_, err := store.GetPreference(ctx, "user-1", "language", "nonexistent")
	if err == nil {
		t.Fatal("expected error for non-existent preference")
	}
}

func TestSearchPreferencesByCategory(t *testing.T) {
	store := newTestPreferenceStore(t)
	ctx := context.Background()

	if _, err := store.SetPreference(ctx, Preference{
		UserID:   "user-1",
		Category: PreferenceCategoryLanguage,
		Key:      "preferred_language",
		Value:    "en",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.SetPreference(ctx, Preference{
		UserID:   "user-1",
		Category: PreferenceCategoryLanguage,
		Key:      "secondary_language",
		Value:    "zh",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.SetPreference(ctx, Preference{
		UserID:   "user-1",
		Category: PreferenceCategoryFormat,
		Key:      "code_style",
		Value:    "go-standard",
	}); err != nil {
		t.Fatal(err)
	}

	results, err := store.SearchPreferences(ctx, PreferenceQuery{
		UserID:     "user-1",
		Categories: []PreferenceCategory{PreferenceCategoryLanguage},
	})
	if err != nil {
		t.Fatalf("SearchPreferences: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 language preferences, got %d", len(results))
	}
	for _, r := range results {
		if r.Category != PreferenceCategoryLanguage {
			t.Fatalf("expected category language, got %s", r.Category)
		}
	}
}

func TestSearchPreferencesByKeys(t *testing.T) {
	store := newTestPreferenceStore(t)
	ctx := context.Background()

	if _, err := store.SetPreference(ctx, Preference{
		UserID:   "user-1",
		Category: PreferenceCategoryLanguage,
		Key:      "preferred_language",
		Value:    "en",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.SetPreference(ctx, Preference{
		UserID:   "user-1",
		Category: PreferenceCategoryFormat,
		Key:      "code_style",
		Value:    "go-standard",
	}); err != nil {
		t.Fatal(err)
	}

	results, err := store.SearchPreferences(ctx, PreferenceQuery{
		UserID: "user-1",
		Keys:   []string{"code_style"},
	})
	if err != nil {
		t.Fatalf("SearchPreferences: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result for key code_style, got %d", len(results))
	}
	if results[0].Key != "code_style" {
		t.Fatalf("expected key code_style, got %s", results[0].Key)
	}
}

func TestSearchPreferencesByMinConfidence(t *testing.T) {
	store := newTestPreferenceStore(t)
	ctx := context.Background()

	// Create preference with 1 observation (confidence = 0.4).
	if _, err := store.SetPreference(ctx, Preference{
		UserID:   "user-1",
		Category: PreferenceCategoryLanguage,
		Key:      "low_conf",
		Value:    "en",
	}); err != nil {
		t.Fatal(err)
	}

	// Create preference with many observations (high confidence).
	for i := 0; i < 8; i++ {
		if _, err := store.SetPreference(ctx, Preference{
			UserID:   "user-1",
			Category: PreferenceCategoryFormat,
			Key:      "high_conf",
			Value:    "markdown",
		}); err != nil {
			t.Fatal(err)
		}
	}

	results, err := store.SearchPreferences(ctx, PreferenceQuery{
		UserID:        "user-1",
		MinConfidence: 0.8,
	})
	if err != nil {
		t.Fatalf("SearchPreferences: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result with min confidence 0.8, got %d", len(results))
	}
	if results[0].Key != "high_conf" {
		t.Fatalf("expected key high_conf, got %s", results[0].Key)
	}
}

func TestSearchPreferencesLimit(t *testing.T) {
	store := newTestPreferenceStore(t)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		if _, err := store.SetPreference(ctx, Preference{
			UserID:   "user-1",
			Category: PreferenceCategoryStyle,
			Key:      fmt.Sprintf("style_%d", i),
			Value:    fmt.Sprintf("value_%d", i),
		}); err != nil {
			t.Fatal(err)
		}
	}

	results, err := store.SearchPreferences(ctx, PreferenceQuery{
		UserID: "user-1",
		Limit:  3,
	})
	if err != nil {
		t.Fatalf("SearchPreferences: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 results (limit), got %d", len(results))
	}
}

func TestSearchPreferencesSortedByConfidence(t *testing.T) {
	store := newTestPreferenceStore(t)
	ctx := context.Background()

	// Create one preference with 1 observation.
	if _, err := store.SetPreference(ctx, Preference{
		UserID:   "user-1",
		Category: PreferenceCategoryLanguage,
		Key:      "low",
		Value:    "en",
	}); err != nil {
		t.Fatal(err)
	}

	// Create another with 5 observations (higher confidence).
	for i := 0; i < 5; i++ {
		if _, err := store.SetPreference(ctx, Preference{
			UserID:   "user-1",
			Category: PreferenceCategoryLanguage,
			Key:      "high",
			Value:    "zh",
		}); err != nil {
			t.Fatal(err)
		}
	}

	results, err := store.SearchPreferences(ctx, PreferenceQuery{
		UserID: "user-1",
	})
	if err != nil {
		t.Fatalf("SearchPreferences: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].Key != "high" {
		t.Fatalf("expected higher confidence first, got %s", results[0].Key)
	}
	if results[1].Key != "low" {
		t.Fatalf("expected lower confidence second, got %s", results[1].Key)
	}
}

func TestInferPreferencesReturnsHighConfidenceOnly(t *testing.T) {
	store := newTestPreferenceStore(t)
	ctx := context.Background()

	// Low confidence (1 observation, confidence = 0.4).
	if _, err := store.SetPreference(ctx, Preference{
		UserID:   "user-1",
		Category: PreferenceCategoryLanguage,
		Key:      "weak_pref",
		Value:    "en",
	}); err != nil {
		t.Fatal(err)
	}

	// Exactly at boundary (3 observations, confidence = 0.3 + 0.1*3 = 0.6 > 0.5).
	for i := 0; i < 3; i++ {
		if _, err := store.SetPreference(ctx, Preference{
			UserID:   "user-1",
			Category: PreferenceCategoryFormat,
			Key:      "medium_pref",
			Value:    "markdown",
		}); err != nil {
			t.Fatal(err)
		}
	}

	// High confidence (7 observations, confidence = 0.3 + 0.1*7 = 1.0).
	for i := 0; i < 7; i++ {
		if _, err := store.SetPreference(ctx, Preference{
			UserID:   "user-1",
			Category: PreferenceCategoryStyle,
			Key:      "strong_pref",
			Value:    "concise",
		}); err != nil {
			t.Fatal(err)
		}
	}

	results, err := store.InferPreferences(ctx, "user-1")
	if err != nil {
		t.Fatalf("InferPreferences: %v", err)
	}

	// weak_pref has confidence 0.4, should be excluded (not > 0.5).
	// medium_pref has confidence 0.6, should be included.
	// strong_pref has confidence 1.0, should be included.
	if len(results) != 2 {
		t.Fatalf("expected 2 inferred preferences, got %d", len(results))
	}

	// Should be sorted by confidence descending.
	if results[0].Key != "strong_pref" {
		t.Fatalf("expected strong_pref first, got %s", results[0].Key)
	}
	if results[1].Key != "medium_pref" {
		t.Fatalf("expected medium_pref second, got %s", results[1].Key)
	}
}

func TestInferPreferencesBoundary(t *testing.T) {
	store := newTestPreferenceStore(t)
	ctx := context.Background()

	// 2 observations: confidence = 0.3 + 0.1*2 = 0.5, NOT > 0.5 so excluded.
	for i := 0; i < 2; i++ {
		if _, err := store.SetPreference(ctx, Preference{
			UserID:   "user-1",
			Category: PreferenceCategoryLanguage,
			Key:      "exactly_half",
			Value:    "en",
		}); err != nil {
			t.Fatal(err)
		}
	}

	results, err := store.InferPreferences(ctx, "user-1")
	if err != nil {
		t.Fatalf("InferPreferences: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected 0 results for confidence exactly at 0.5, got %d", len(results))
	}
}

func TestDeletePreference(t *testing.T) {
	store := newTestPreferenceStore(t)
	ctx := context.Background()

	pref, err := store.SetPreference(ctx, Preference{
		UserID:   "user-1",
		Category: PreferenceCategoryTimezone,
		Key:      "tz",
		Value:    "UTC",
	})
	if err != nil {
		t.Fatal(err)
	}

	err = store.DeletePreference(ctx, pref.ID)
	if err != nil {
		t.Fatalf("DeletePreference: %v", err)
	}

	_, err = store.GetPreference(ctx, "user-1", string(PreferenceCategoryTimezone), "tz")
	if err == nil {
		t.Fatal("expected error after deletion")
	}
}

func TestDeletePreferenceNotFound(t *testing.T) {
	store := newTestPreferenceStore(t)
	ctx := context.Background()

	err := store.DeletePreference(ctx, "nonexistent-id")
	if err == nil {
		t.Fatal("expected error for non-existent preference")
	}
}

func TestMergePreferencesDisjoint(t *testing.T) {
	existing := []Preference{
		{Category: PreferenceCategoryLanguage, Key: "lang", Value: "en", Confidence: 0.8},
	}
	observed := []Preference{
		{Category: PreferenceCategoryFormat, Key: "fmt", Value: "markdown", Confidence: 0.6},
	}

	merged := MergePreferences(existing, observed)
	if len(merged) != 2 {
		t.Fatalf("expected 2 merged preferences, got %d", len(merged))
	}
	// Should be sorted by confidence descending.
	if merged[0].Confidence < merged[1].Confidence {
		t.Fatal("expected merged results sorted by confidence descending")
	}
}

func TestMergePreferencesOverlapping(t *testing.T) {
	existing := []Preference{
		{Category: PreferenceCategoryLanguage, Key: "lang", Value: "en", Confidence: 0.6},
		{Category: PreferenceCategoryFormat, Key: "fmt", Value: "plain", Confidence: 0.9},
	}
	observed := []Preference{
		{Category: PreferenceCategoryLanguage, Key: "lang", Value: "zh", Confidence: 0.8},
		{Category: PreferenceCategoryStyle, Key: "style", Value: "concise", Confidence: 0.5},
	}

	merged := MergePreferences(existing, observed)
	if len(merged) != 3 {
		t.Fatalf("expected 3 merged preferences, got %d", len(merged))
	}

	// Find the lang preference: observed has higher confidence so should win.
	var langPref *Preference
	for i, p := range merged {
		if p.Key == "lang" {
			langPref = &merged[i]
			break
		}
	}
	if langPref == nil {
		t.Fatal("expected lang preference in merged results")
	}
	if langPref.Value != "zh" {
		t.Fatalf("expected lang value zh (higher confidence), got %s", langPref.Value)
	}
	if langPref.Confidence != 0.8 {
		t.Fatalf("expected lang confidence 0.8, got %f", langPref.Confidence)
	}
}

func TestMergePreferencesExistingWinsWhenHigher(t *testing.T) {
	existing := []Preference{
		{Category: PreferenceCategoryLanguage, Key: "lang", Value: "en", Confidence: 0.9},
	}
	observed := []Preference{
		{Category: PreferenceCategoryLanguage, Key: "lang", Value: "zh", Confidence: 0.4},
	}

	merged := MergePreferences(existing, observed)
	if len(merged) != 1 {
		t.Fatalf("expected 1 merged preference, got %d", len(merged))
	}
	if merged[0].Value != "en" {
		t.Fatalf("expected existing value en (higher confidence), got %s", merged[0].Value)
	}
}

func TestMergePreferencesEmptyInputs(t *testing.T) {
	// Both empty.
	merged := MergePreferences(nil, nil)
	if len(merged) != 0 {
		t.Fatalf("expected 0 results for nil inputs, got %d", len(merged))
	}

	// One empty.
	existing := []Preference{
		{Category: PreferenceCategoryLanguage, Key: "lang", Value: "en", Confidence: 0.7},
	}
	merged = MergePreferences(existing, nil)
	if len(merged) != 1 {
		t.Fatalf("expected 1 result, got %d", len(merged))
	}

	merged = MergePreferences(nil, existing)
	if len(merged) != 1 {
		t.Fatalf("expected 1 result, got %d", len(merged))
	}
}

func TestPreferenceFilePersistence(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()

	// Write with one store instance.
	store1 := NewFilePreferenceStore(dir)
	created, err := store1.SetPreference(ctx, Preference{
		UserID:   "user-1",
		Category: PreferenceCategoryLanguage,
		Key:      "preferred_language",
		Value:    "en",
		Source:   "explicit",
	})
	if err != nil {
		t.Fatalf("SetPreference: %v", err)
	}

	// Read with a fresh store instance (cold cache).
	store2 := NewFilePreferenceStore(dir)
	got, err := store2.GetPreference(ctx, "user-1", string(PreferenceCategoryLanguage), "preferred_language")
	if err != nil {
		t.Fatalf("GetPreference from new store: %v", err)
	}
	if got.ID != created.ID {
		t.Fatalf("expected ID %s, got %s", created.ID, got.ID)
	}
	if got.Value != "en" {
		t.Fatalf("expected value en, got %s", got.Value)
	}
	if got.Source != "explicit" {
		t.Fatalf("expected source explicit, got %s", got.Source)
	}
}

func TestPreferenceFilePersistenceAfterUpsert(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()

	store1 := NewFilePreferenceStore(dir)

	// Create and upsert.
	for i := 0; i < 3; i++ {
		if _, err := store1.SetPreference(ctx, Preference{
			UserID:   "user-1",
			Category: PreferenceCategoryStyle,
			Key:      "theme",
			Value:    "dark",
		}); err != nil {
			t.Fatalf("SetPreference iteration %d: %v", i, err)
		}
	}

	// Verify from fresh store.
	store2 := NewFilePreferenceStore(dir)
	got, err := store2.GetPreference(ctx, "user-1", string(PreferenceCategoryStyle), "theme")
	if err != nil {
		t.Fatalf("GetPreference from new store: %v", err)
	}
	if got.ObservationCount != 3 {
		t.Fatalf("expected observation count 3, got %d", got.ObservationCount)
	}
	// Confidence = min(1.0, 0.3 + 0.1*3) = 0.6
	if got.Confidence != 0.6 {
		t.Fatalf("expected confidence 0.6, got %f", got.Confidence)
	}
}

func TestDeletePreferenceFromDisk(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()

	store1 := NewFilePreferenceStore(dir)
	pref, err := store1.SetPreference(ctx, Preference{
		UserID:   "user-1",
		Category: PreferenceCategoryTimezone,
		Key:      "tz",
		Value:    "UTC",
	})
	if err != nil {
		t.Fatal(err)
	}

	// Delete with a fresh store (cold cache).
	store2 := NewFilePreferenceStore(dir)
	err = store2.DeletePreference(ctx, pref.ID)
	if err != nil {
		t.Fatalf("DeletePreference from cold store: %v", err)
	}

	// Verify it is gone from a third fresh store.
	store3 := NewFilePreferenceStore(dir)
	_, err = store3.GetPreference(ctx, "user-1", string(PreferenceCategoryTimezone), "tz")
	if err == nil {
		t.Fatal("expected error after deletion from cold store")
	}
}

func TestSetPreferenceValidation(t *testing.T) {
	store := newTestPreferenceStore(t)
	ctx := context.Background()

	if _, err := store.SetPreference(ctx, Preference{
		Category: PreferenceCategoryLanguage,
		Key:      "lang",
	}); err == nil {
		t.Fatal("expected error for missing UserID")
	}

	if _, err := store.SetPreference(ctx, Preference{
		UserID: "user-1",
		Key:    "lang",
	}); err == nil {
		t.Fatal("expected error for missing Category")
	}

	if _, err := store.SetPreference(ctx, Preference{
		UserID:   "user-1",
		Category: PreferenceCategoryLanguage,
	}); err == nil {
		t.Fatal("expected error for missing Key")
	}
}

func TestGetPreferenceValidation(t *testing.T) {
	store := newTestPreferenceStore(t)
	ctx := context.Background()

	if _, err := store.GetPreference(ctx, "", "language", "key"); err == nil {
		t.Fatal("expected error for empty user_id")
	}
	if _, err := store.GetPreference(ctx, "user-1", "", "key"); err == nil {
		t.Fatal("expected error for empty category")
	}
	if _, err := store.GetPreference(ctx, "user-1", "language", ""); err == nil {
		t.Fatal("expected error for empty key")
	}
}

func TestSearchPreferencesValidation(t *testing.T) {
	store := newTestPreferenceStore(t)
	ctx := context.Background()

	_, err := store.SearchPreferences(ctx, PreferenceQuery{})
	if err == nil {
		t.Fatal("expected error for empty user_id in query")
	}
}

func TestInferPreferencesValidation(t *testing.T) {
	store := newTestPreferenceStore(t)
	ctx := context.Background()

	_, err := store.InferPreferences(ctx, "")
	if err == nil {
		t.Fatal("expected error for empty user_id")
	}
}

func TestDeletePreferenceValidation(t *testing.T) {
	store := newTestPreferenceStore(t)
	ctx := context.Background()

	err := store.DeletePreference(ctx, "")
	if err == nil {
		t.Fatal("expected error for empty id")
	}
}

func TestPreferenceConcurrentAccess(t *testing.T) {
	store := newTestPreferenceStore(t)
	ctx := context.Background()

	const goroutines = 10
	const opsPerGoroutine = 5

	var wg sync.WaitGroup
	errCh := make(chan error, goroutines*opsPerGoroutine)

	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(gID int) {
			defer wg.Done()
			for i := 0; i < opsPerGoroutine; i++ {
				_, err := store.SetPreference(ctx, Preference{
					UserID:   "user-1",
					Category: PreferenceCategoryStyle,
					Key:      fmt.Sprintf("style_%d_%d", gID, i),
					Value:    fmt.Sprintf("value_%d_%d", gID, i),
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
		t.Fatalf("concurrent SetPreference error: %v", err)
	}

	results, err := store.SearchPreferences(ctx, PreferenceQuery{
		UserID: "user-1",
	})
	if err != nil {
		t.Fatalf("SearchPreferences: %v", err)
	}
	expected := goroutines * opsPerGoroutine
	if len(results) != expected {
		t.Fatalf("expected %d preferences, got %d", expected, len(results))
	}
}

func TestPreferenceConcurrentUpsert(t *testing.T) {
	store := newTestPreferenceStore(t)
	ctx := context.Background()

	const goroutines = 10

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for g := 0; g < goroutines; g++ {
		go func() {
			defer wg.Done()
			_, _ = store.SetPreference(ctx, Preference{
				UserID:   "user-1",
				Category: PreferenceCategoryLanguage,
				Key:      "shared_key",
				Value:    "en",
			})
		}()
	}

	wg.Wait()

	pref, err := store.GetPreference(ctx, "user-1", string(PreferenceCategoryLanguage), "shared_key")
	if err != nil {
		t.Fatalf("GetPreference: %v", err)
	}
	// First goroutine creates (count=1), remaining 9 upsert (count increments by 1 each).
	if pref.ObservationCount != goroutines {
		t.Fatalf("expected observation count %d, got %d", goroutines, pref.ObservationCount)
	}
}

func TestEmptyStoreReturnsEmptyPreferences(t *testing.T) {
	store := newTestPreferenceStore(t)
	ctx := context.Background()

	results, err := store.SearchPreferences(ctx, PreferenceQuery{
		UserID: "user-1",
	})
	if err != nil {
		t.Fatalf("SearchPreferences on empty store: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected empty results on empty store, got %d", len(results))
	}

	inferred, err := store.InferPreferences(ctx, "user-1")
	if err != nil {
		t.Fatalf("InferPreferences on empty store: %v", err)
	}
	if len(inferred) != 0 {
		t.Fatalf("expected empty inferred results on empty store, got %d", len(inferred))
	}
}

func TestSearchPreferencesCombinedFilters(t *testing.T) {
	store := newTestPreferenceStore(t)
	ctx := context.Background()

	// Language preference with low confidence.
	if _, err := store.SetPreference(ctx, Preference{
		UserID:   "user-1",
		Category: PreferenceCategoryLanguage,
		Key:      "low_lang",
		Value:    "en",
	}); err != nil {
		t.Fatal(err)
	}

	// Language preference with high confidence.
	for i := 0; i < 6; i++ {
		if _, err := store.SetPreference(ctx, Preference{
			UserID:   "user-1",
			Category: PreferenceCategoryLanguage,
			Key:      "high_lang",
			Value:    "zh",
		}); err != nil {
			t.Fatal(err)
		}
	}

	// Format preference with high confidence.
	for i := 0; i < 6; i++ {
		if _, err := store.SetPreference(ctx, Preference{
			UserID:   "user-1",
			Category: PreferenceCategoryFormat,
			Key:      "high_fmt",
			Value:    "markdown",
		}); err != nil {
			t.Fatal(err)
		}
	}

	// Combined: language category + min confidence 0.8.
	results, err := store.SearchPreferences(ctx, PreferenceQuery{
		UserID:        "user-1",
		Categories:    []PreferenceCategory{PreferenceCategoryLanguage},
		MinConfidence: 0.8,
	})
	if err != nil {
		t.Fatalf("SearchPreferences: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result with combined filters, got %d", len(results))
	}
	if results[0].Key != "high_lang" {
		t.Fatalf("expected high_lang, got %s", results[0].Key)
	}
}

func TestComputeConfidence(t *testing.T) {
	tests := []struct {
		observations int
		expected     float64
	}{
		{1, 0.4},
		{2, 0.5},
		{3, 0.6},
		{5, 0.8},
		{7, 1.0},
		{10, 1.0}, // capped at 1.0
		{100, 1.0},
	}
	for _, tt := range tests {
		got := computeConfidence(tt.observations)
		if got != tt.expected {
			t.Errorf("computeConfidence(%d) = %f, want %f", tt.observations, got, tt.expected)
		}
	}
}

func TestPreferenceKeyCaseInsensitive(t *testing.T) {
	store := newTestPreferenceStore(t)
	ctx := context.Background()

	// Create with lowercase.
	if _, err := store.SetPreference(ctx, Preference{
		UserID:   "user-1",
		Category: PreferenceCategoryLanguage,
		Key:      "preferred_language",
		Value:    "en",
	}); err != nil {
		t.Fatal(err)
	}

	// Upsert with mixed case should match the existing one.
	updated, err := store.SetPreference(ctx, Preference{
		UserID:   "user-1",
		Category: PreferenceCategoryLanguage,
		Key:      "Preferred_Language",
		Value:    "zh",
	})
	if err != nil {
		t.Fatalf("SetPreference: %v", err)
	}
	if updated.ObservationCount != 2 {
		t.Fatalf("expected observation count 2 (case-insensitive match), got %d", updated.ObservationCount)
	}
}
