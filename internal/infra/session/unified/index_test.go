package unified

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func fixedNow() time.Time { return time.Date(2026, 3, 18, 12, 0, 0, 0, time.UTC) }

func newTestIndex(t *testing.T) *SurfaceIndex {
	t.Helper()
	path := filepath.Join(t.TempDir(), "index.json")
	idx, err := NewSurfaceIndex(path, fixedNow)
	if err != nil {
		t.Fatalf("NewSurfaceIndex: %v", err)
	}
	return idx
}

func TestSurfaceIndex(t *testing.T) {
	tests := []struct {
		name string
		run  func(t *testing.T)
	}{
		{"BindAndLookup", testBindAndLookup},
		{"LookupMiss", testLookupMiss},
		{"RemoveBySurface", testRemoveBySurface},
		{"RemoveBySession", testRemoveBySession},
		{"ListForSession", testListForSession},
		{"PersistAndReload", testPersistAndReload},
		{"ConcurrentBindLookup", testConcurrentBindLookup},
	}
	for _, tt := range tests {
		t.Run(tt.name, tt.run)
	}
}

func testBindAndLookup(t *testing.T) {
	idx := newTestIndex(t)
	if err := idx.Bind(SurfaceLark, "oc_123", "sess-1"); err != nil {
		t.Fatalf("Bind: %v", err)
	}

	sid, ok := idx.Lookup(SurfaceLark, "oc_123")
	if !ok || sid != "sess-1" {
		t.Fatalf("Lookup got (%q, %v), want (sess-1, true)", sid, ok)
	}
}

func testLookupMiss(t *testing.T) {
	idx := newTestIndex(t)
	_, ok := idx.Lookup(SurfaceCLI, "nonexistent")
	if ok {
		t.Fatal("expected lookup miss")
	}
}

func testRemoveBySurface(t *testing.T) {
	idx := newTestIndex(t)
	_ = idx.Bind(SurfaceLark, "oc_1", "sess-1")
	_ = idx.Bind(SurfaceCLI, "my-cli", "sess-2")

	if err := idx.Remove(SurfaceLark, "oc_1"); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	if _, ok := idx.Lookup(SurfaceLark, "oc_1"); ok {
		t.Fatal("binding should have been removed")
	}
	if _, ok := idx.Lookup(SurfaceCLI, "my-cli"); !ok {
		t.Fatal("unrelated binding should still exist")
	}
}

func testRemoveBySession(t *testing.T) {
	idx := newTestIndex(t)
	_ = idx.Bind(SurfaceLark, "oc_1", "sess-1")
	_ = idx.Bind(SurfaceCLI, "cli-1", "sess-1")
	_ = idx.Bind(SurfaceWeb, "web-1", "sess-2")

	if err := idx.RemoveBySession("sess-1"); err != nil {
		t.Fatalf("RemoveBySession: %v", err)
	}

	if _, ok := idx.Lookup(SurfaceLark, "oc_1"); ok {
		t.Fatal("lark binding for sess-1 should be gone")
	}
	if _, ok := idx.Lookup(SurfaceCLI, "cli-1"); ok {
		t.Fatal("cli binding for sess-1 should be gone")
	}
	if _, ok := idx.Lookup(SurfaceWeb, "web-1"); !ok {
		t.Fatal("web binding for sess-2 should remain")
	}
}

func testListForSession(t *testing.T) {
	idx := newTestIndex(t)
	_ = idx.Bind(SurfaceLark, "oc_1", "sess-1")
	_ = idx.Bind(SurfaceCLI, "cli-1", "sess-1")
	_ = idx.Bind(SurfaceWeb, "web-1", "sess-2")

	bindings := idx.ListForSession("sess-1")
	if len(bindings) != 2 {
		t.Fatalf("expected 2 bindings, got %d", len(bindings))
	}
}

func testPersistAndReload(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "index.json")

	idx, _ := NewSurfaceIndex(path, fixedNow)
	_ = idx.Bind(SurfaceLark, "oc_1", "sess-1")

	idx2, err := NewSurfaceIndex(path, fixedNow)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}

	sid, ok := idx2.Lookup(SurfaceLark, "oc_1")
	if !ok || sid != "sess-1" {
		t.Fatalf("after reload: got (%q, %v), want (sess-1, true)", sid, ok)
	}

	if !fileExists(path) {
		t.Fatal("index file should exist on disk")
	}
}

func testConcurrentBindLookup(t *testing.T) {
	idx := newTestIndex(t)

	var wg sync.WaitGroup
	for i := range 50 {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			sid := "sess-concurrent"
			surfaceID := "id-" + string(rune('A'+n%26))
			_ = idx.Bind(SurfaceLark, surfaceID, sid)
			idx.Lookup(SurfaceLark, surfaceID)
		}(i)
	}
	wg.Wait()

	snap := idx.snapshot()
	if len(snap) == 0 {
		t.Fatal("expected bindings after concurrent writes")
	}
}

func TestNewSurfaceIndex_MissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "subdir", "index.json")
	idx, err := NewSurfaceIndex(path, fixedNow)
	if err != nil {
		t.Fatalf("should handle missing file: %v", err)
	}
	if err := idx.Bind(SurfaceCLI, "test", "sess"); err != nil {
		t.Fatalf("Bind after missing file init: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("file should exist after Bind: %v", err)
	}
}
