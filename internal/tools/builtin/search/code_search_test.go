package search

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetRepoHash_DistinguishesSameBasename(t *testing.T) {
	parentA := t.TempDir()
	parentB := t.TempDir()

	repoA := filepath.Join(parentA, "repo")
	repoB := filepath.Join(parentB, "repo")

	if err := os.MkdirAll(repoA, 0o755); err != nil {
		t.Fatalf("create repoA: %v", err)
	}
	if err := os.MkdirAll(repoB, 0o755); err != nil {
		t.Fatalf("create repoB: %v", err)
	}

	hashA := getRepoHash(repoA)
	hashB := getRepoHash(repoB)
	if hashA == "" || hashB == "" {
		t.Fatalf("expected non-empty hashes, got %q and %q", hashA, hashB)
	}
	if hashA == hashB {
		t.Fatalf("expected different hashes for different repos with same basename")
	}
}
