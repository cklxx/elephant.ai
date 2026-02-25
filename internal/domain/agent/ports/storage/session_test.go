package storage

import "testing"

func TestEnsureMetadataNilSession(t *testing.T) {
	if got := EnsureMetadata(nil); got != nil {
		t.Fatalf("expected nil metadata for nil session, got %v", got)
	}
}

func TestEnsureMetadataInitializesWhenMissing(t *testing.T) {
	session := &Session{}

	metadata := EnsureMetadata(session)
	if metadata == nil {
		t.Fatal("expected metadata to be initialized")
	}
	if session.Metadata == nil {
		t.Fatal("expected session metadata to be initialized")
	}

	metadata["k"] = "v"
	if session.Metadata["k"] != "v" {
		t.Fatalf("expected metadata mutation to persist on session, got %q", session.Metadata["k"])
	}
}

func TestEnsureMetadataPreservesExistingMap(t *testing.T) {
	session := &Session{Metadata: map[string]string{"title": "hello"}}

	metadata := EnsureMetadata(session)
	if metadata["title"] != "hello" {
		t.Fatalf("expected existing key to be preserved, got %q", metadata["title"])
	}

	metadata["title"] = "updated"
	if session.Metadata["title"] != "updated" {
		t.Fatalf("expected metadata updates to apply to session map, got %q", session.Metadata["title"])
	}
}

func TestCloneMetadataNilInput(t *testing.T) {
	if cloned := CloneMetadata(nil); cloned != nil {
		t.Fatalf("expected nil clone for nil input, got %v", cloned)
	}
}

func TestCloneMetadataEmptyInputReturnsNil(t *testing.T) {
	if cloned := CloneMetadata(map[string]string{}); cloned != nil {
		t.Fatalf("expected nil clone for empty input, got %v", cloned)
	}
}

func TestCloneMetadataDeepCopiesValues(t *testing.T) {
	original := map[string]string{"a": "1", "b": "2"}

	cloned := CloneMetadata(original)
	if len(cloned) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(cloned))
	}
	if cloned["a"] != "1" || cloned["b"] != "2" {
		t.Fatalf("expected cloned values copied, got %v", cloned)
	}

	original["a"] = "mutated"
	if cloned["a"] == "mutated" {
		t.Fatal("expected clone to be independent from original mutations")
	}

	cloned["b"] = "changed"
	if original["b"] == "changed" {
		t.Fatal("expected original to be independent from clone mutations")
	}
}
