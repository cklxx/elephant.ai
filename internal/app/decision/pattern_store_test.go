package decision

import (
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestPatternStoreSaveAndGet(t *testing.T) {
	ps, err := NewPatternStore(filepath.Join(t.TempDir(), "patterns.json"))
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	now := time.Date(2026, 3, 18, 0, 0, 0, 0, time.UTC)
	p := &Pattern{
		ID: "p1", Category: "escalation", Condition: "high-risk",
		Action: "notify", Confidence: 0.95, CreatedAt: now, UpdatedAt: now,
	}

	tests := []struct {
		name  string
		op    func() error
		check func(t *testing.T)
	}{
		{
			name: "save and retrieve",
			op:   func() error { return ps.Save(p) },
			check: func(t *testing.T) {
				got := ps.Get("p1")
				if got == nil {
					t.Fatal("expected pattern, got nil")
				}
				if got.Category != "escalation" {
					t.Errorf("got category %q, want %q", got.Category, "escalation")
				}
				if got.Checksum == "" {
					t.Error("checksum should be computed on save")
				}
			},
		},
		{
			name: "get nonexistent returns nil",
			op:   func() error { return nil },
			check: func(t *testing.T) {
				if ps.Get("nonexistent") != nil {
					t.Error("expected nil for nonexistent pattern")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.op(); err != nil {
				t.Fatalf("op: %v", err)
			}
			tt.check(t)
		})
	}
}

func TestPatternStoreList(t *testing.T) {
	ps, err := NewPatternStore(filepath.Join(t.TempDir(), "patterns.json"))
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	for i, id := range []string{"p1", "p2", "p3"} {
		_ = ps.Save(&Pattern{
			ID: id, Category: "test", CreatedAt: time.Now().Add(time.Duration(i) * time.Hour),
		})
	}

	list := ps.List()
	if len(list) != 3 {
		t.Fatalf("got %d patterns, want 3", len(list))
	}
	// Newest first
	if list[0].ID != "p3" {
		t.Errorf("expected p3 first, got %s", list[0].ID)
	}
}

func TestPatternStoreFindMatching(t *testing.T) {
	ps, err := NewPatternStore("")
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	_ = ps.Save(&Pattern{ID: "p1", Category: "escalation", Condition: "high-risk deploy"})
	_ = ps.Save(&Pattern{ID: "p2", Category: "escalation", Condition: "low-risk change"})
	_ = ps.Save(&Pattern{ID: "p3", Category: "approval", Condition: "high-risk deploy"})

	matches := ps.FindMatching("escalation", "high-risk")
	if len(matches) != 1 {
		t.Errorf("got %d matches, want 1", len(matches))
	}
}

func TestPatternStoreDelete(t *testing.T) {
	ps, err := NewPatternStore("")
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	_ = ps.Save(&Pattern{ID: "p1", Category: "test"})
	if err := ps.Delete("p1"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if ps.Get("p1") != nil {
		t.Error("pattern should be deleted")
	}
	if err := ps.Delete("nonexistent"); err == nil {
		t.Error("expected error deleting nonexistent")
	}
}

func TestPatternStoreChecksumValidation(t *testing.T) {
	ps, err := NewPatternStore("")
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	_ = ps.Save(&Pattern{ID: "p1", Category: "test", Condition: "cond", Action: "act", Confidence: 0.95})

	// Corrupt the pattern by directly modifying the action without recomputing checksum
	ps.mu.Lock()
	ps.patterns["p1"].Action = "tampered"
	ps.mu.Unlock()

	corrupt := ps.ValidateIntegrity()
	if len(corrupt) != 1 || corrupt[0] != "p1" {
		t.Errorf("expected p1 as corrupt, got %v", corrupt)
	}
}

func TestPatternStoreIntegrityScan(t *testing.T) {
	ps, err := NewPatternStore("")
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	_ = ps.Save(&Pattern{ID: "p1", Category: "test", Condition: "cond", Action: "act", Confidence: 0.95})

	// Corrupt checksum
	ps.mu.Lock()
	ps.patterns["p1"].Action = "tampered"
	ps.mu.Unlock()

	if err := ps.RunIntegrityScan(); err != nil {
		t.Fatalf("scan: %v", err)
	}

	p := ps.Get("p1")
	if p.Confidence != 0 {
		t.Errorf("corrupt pattern confidence should be 0, got %f", p.Confidence)
	}
}

func TestPatternStorePersistence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "patterns.json")

	ps1, err := NewPatternStore(path)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	_ = ps1.Save(&Pattern{ID: "p1", Category: "test", Condition: "c", Action: "a"})

	// Reload from disk
	ps2, err := NewPatternStore(path)
	if err != nil {
		t.Fatalf("reload store: %v", err)
	}
	if ps2.Get("p1") == nil {
		t.Error("pattern should survive persistence reload")
	}
}

func TestPatternStoreConcurrentAccess(t *testing.T) {
	ps, err := NewPatternStore("")
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			id := "p" + time.Now().Format("150405.000000000")
			_ = ps.Save(&Pattern{ID: id, Category: "test"})
			_ = ps.List()
			_ = ps.FindMatching("test", "")
		}(i)
	}
	wg.Wait()
}
