package filestore

import (
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	jsonx "alex/internal/shared/json"
)

func newTestCollection(t *testing.T) *Collection[string, int] {
	t.Helper()
	fp := filepath.Join(t.TempDir(), "test.json")
	c := NewCollection[string, int](CollectionConfig{FilePath: fp, Name: "test"})
	if err := c.EnsureDir(); err != nil {
		t.Fatal(err)
	}
	return c
}

func TestCollection_PutGetDelete(t *testing.T) {
	c := newTestCollection(t)

	if err := c.Put("a", 1); err != nil {
		t.Fatal(err)
	}
	v, ok := c.Get("a")
	if !ok || v != 1 {
		t.Fatalf("Get(a) = %v, %v", v, ok)
	}

	if err := c.Delete("a"); err != nil {
		t.Fatal(err)
	}
	_, ok = c.Get("a")
	if ok {
		t.Fatal("expected a to be deleted")
	}
}

func TestCollection_Len(t *testing.T) {
	c := newTestCollection(t)
	_ = c.Put("a", 1)
	_ = c.Put("b", 2)
	if c.Len() != 2 {
		t.Fatalf("expected len=2, got %d", c.Len())
	}
}

func TestCollection_Snapshot(t *testing.T) {
	c := newTestCollection(t)
	_ = c.Put("x", 10)
	snap := c.Snapshot()
	if snap["x"] != 10 {
		t.Fatal("snapshot missing x")
	}
	// Mutating snapshot doesn't affect collection.
	snap["x"] = 99
	v, _ := c.Get("x")
	if v != 10 {
		t.Fatal("snapshot mutation leaked into collection")
	}
}

func TestCollection_Mutate(t *testing.T) {
	c := newTestCollection(t)
	_ = c.Put("a", 1)
	_ = c.Put("b", 2)

	err := c.Mutate(func(items map[string]int) error {
		delete(items, "a")
		items["c"] = 3
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := c.Get("a"); ok {
		t.Fatal("a should be deleted")
	}
	v, ok := c.Get("c")
	if !ok || v != 3 {
		t.Fatal("c should be 3")
	}
}

func TestCollection_MutateWithRollback(t *testing.T) {
	c := newTestCollection(t)
	_ = c.Put("a", 1)

	err := c.MutateWithRollback(func(items map[string]int) error {
		items["a"] = 999
		return fmt.Errorf("rollback")
	})
	if err == nil {
		t.Fatal("expected error")
	}
	v, _ := c.Get("a")
	if v != 999 {
		// Note: the map was directly mutated, but the collection restores via snapshot
	}
	// Actually verify rollback restored the map.
	v, _ = c.Get("a")
	if v != 1 {
		t.Fatalf("expected rollback to restore a=1, got %d", v)
	}
}

func TestCollection_PersistAndReload(t *testing.T) {
	fp := filepath.Join(t.TempDir(), "persist.json")

	c1 := NewCollection[string, int](CollectionConfig{FilePath: fp, Name: "c1"})
	_ = c1.EnsureDir()
	_ = c1.Put("x", 42)

	// Create a new collection pointing to the same file.
	c2 := NewCollection[string, int](CollectionConfig{FilePath: fp, Name: "c2"})
	if err := c2.Load(); err != nil {
		t.Fatal(err)
	}
	v, ok := c2.Get("x")
	if !ok || v != 42 {
		t.Fatalf("expected x=42 after reload, got %v, %v", v, ok)
	}
}

func TestCollection_CustomEnvelope(t *testing.T) {
	type doc struct {
		Items []string `json:"items"`
	}

	fp := filepath.Join(t.TempDir(), "envelope.json")
	c := NewCollection[int, string](CollectionConfig{FilePath: fp, Name: "env"})
	_ = c.EnsureDir()

	c.SetMarshalDoc(func(m map[int]string) ([]byte, error) {
		d := doc{}
		for _, v := range m {
			d.Items = append(d.Items, v)
		}
		return MarshalJSONIndent(d)
	})
	c.SetUnmarshalDoc(func(data []byte) (map[int]string, error) {
		var d doc
		if err := jsonx.Unmarshal(data, &d); err != nil {
			return nil, err
		}
		m := make(map[int]string, len(d.Items))
		for i, v := range d.Items {
			m[i] = v
		}
		return m, nil
	})

	_ = c.Put(0, "hello")
	_ = c.Put(1, "world")

	// Reload from disk.
	c2 := NewCollection[int, string](CollectionConfig{FilePath: fp, Name: "env2"})
	c2.SetUnmarshalDoc(c.unmarshalDoc)
	if err := c2.Load(); err != nil {
		t.Fatal(err)
	}
	if c2.Len() != 2 {
		t.Fatalf("expected 2 items after reload, got %d", c2.Len())
	}
}

func TestCollection_InMemoryMode(t *testing.T) {
	c := NewCollection[string, int](CollectionConfig{Name: "mem"})
	if err := c.Put("a", 1); err != nil {
		t.Fatal(err)
	}
	v, ok := c.Get("a")
	if !ok || v != 1 {
		t.Fatal("in-memory mode failed")
	}
}

func TestCollection_ConcurrentAccess(t *testing.T) {
	c := newTestCollection(t)
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			key := fmt.Sprintf("k%d", i)
			_ = c.Put(key, i)
			c.Get(key)
			_ = c.Delete(key)
		}(i)
	}
	wg.Wait()
}

func TestCollection_ReadLocked(t *testing.T) {
	c := newTestCollection(t)
	_ = c.Put("a", 10)
	_ = c.Put("b", 20)

	var sum int
	c.ReadLocked(func(items map[string]int) {
		for _, v := range items {
			sum += v
		}
	})
	if sum != 30 {
		t.Fatalf("expected sum=30, got %d", sum)
	}
}

func TestCollection_NowInjection(t *testing.T) {
	c := newTestCollection(t)
	fixed := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	c.Now = func() time.Time { return fixed }
	if !c.Now().Equal(fixed) {
		t.Fatal("Now injection failed")
	}
}
