package app

import "testing"

func TestAttachmentCacheReserveAndDeduplicate(t *testing.T) {
	cache := &attachmentCache{}

	first := cache.Reserve("report.txt")
	if first != "report.txt" {
		t.Fatalf("expected first reservation to keep name, got %s", first)
	}

	second := cache.Reserve("report.txt")
	if second == first {
		t.Fatalf("expected second reservation to gain suffix, got %s", second)
	}

	cache.Release(second)
	third := cache.Reserve("report.txt")
	if third == first {
		t.Fatalf("expected released slot to still add suffix to avoid collision, got %s", third)
	}

	cache.Remember("hash-one", first)
	if stored, ok := cache.HasDigest("hash-one"); !ok || stored != first {
		t.Fatalf("expected digest lookup to return %s, got %s (ok=%v)", first, stored, ok)
	}
}
