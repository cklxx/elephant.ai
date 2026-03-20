package skill

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll %s: %v", dir, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile %s: %v", path, err)
	}
}

func TestDiscover_FindsMDFiles(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "deploy.md"), "Deploy instructions")
	writeFile(t, filepath.Join(dir, "rollback.md"), "Rollback instructions")
	writeFile(t, filepath.Join(dir, "notes.txt"), "Not a skill")

	skills, err := Discover(dir)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}

	if len(skills) != 2 {
		t.Fatalf("got %d skills, want 2", len(skills))
	}

	names := make(map[string]bool)
	for _, s := range skills {
		names[s.Name] = true
	}
	if !names["deploy"] || !names["rollback"] {
		t.Errorf("expected deploy and rollback, got %v", names)
	}
}

func TestDiscover_ParsesFrontmatter(t *testing.T) {
	dir := t.TempDir()
	content := `---
description: Deploys the app
author: alice
---
# Deploy

Run the deploy command.`

	writeFile(t, filepath.Join(dir, "deploy.md"), content)

	skills, err := Discover(dir)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(skills) != 1 {
		t.Fatalf("got %d skills, want 1", len(skills))
	}

	s := skills[0]
	if s.Description != "Deploys the app" {
		t.Errorf("Description: got %q, want %q", s.Description, "Deploys the app")
	}
	if s.Metadata["author"] != "alice" {
		t.Errorf("Metadata[author]: got %q, want %q", s.Metadata["author"], "alice")
	}
	if s.Body != "# Deploy\n\nRun the deploy command." {
		t.Errorf("Body: got %q", s.Body)
	}
}

func TestDiscover_DerivesNamesFromRelativePaths(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "ops", "deploy", "rollback.md"), "rollback skill")

	skills, err := Discover(dir)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(skills) != 1 {
		t.Fatalf("got %d skills, want 1", len(skills))
	}
	if skills[0].Name != "ops.deploy.rollback" {
		t.Errorf("Name: got %q, want %q", skills[0].Name, "ops.deploy.rollback")
	}
}

func TestDiscover_MissingDirectory(t *testing.T) {
	skills, err := Discover("/nonexistent/path/that/does/not/exist")
	if err != nil {
		t.Fatalf("Discover should not error for missing dir, got: %v", err)
	}
	if len(skills) != 0 {
		t.Errorf("got %d skills, want 0", len(skills))
	}
}

func TestDiscover_MultipleDirectories(t *testing.T) {
	dir1 := t.TempDir()
	dir2 := t.TempDir()
	writeFile(t, filepath.Join(dir1, "a.md"), "skill a")
	writeFile(t, filepath.Join(dir2, "b.md"), "skill b")

	skills, err := Discover(dir1, dir2)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(skills) != 2 {
		t.Fatalf("got %d skills, want 2", len(skills))
	}
}

func TestDiscover_SetsPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.md")
	writeFile(t, path, "test skill")

	skills, err := Discover(dir)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if skills[0].Path != path {
		t.Errorf("Path: got %q, want %q", skills[0].Path, path)
	}
}

func TestParseFrontmatter_NoFrontmatter(t *testing.T) {
	desc, meta, body := parseFrontmatter("# Just a heading\n\nSome body text.")

	if desc != "" {
		t.Errorf("description: got %q, want empty", desc)
	}
	if len(meta) != 0 {
		t.Errorf("metadata: got %v, want empty", meta)
	}
	if body != "# Just a heading\n\nSome body text." {
		t.Errorf("body: got %q", body)
	}
}

func TestParseFrontmatter_EmptyFrontmatter(t *testing.T) {
	// With no content between --- delimiters, parseFrontmatter finds
	// the closing "\n---" immediately and returns an empty frontmatter block.
	content := "---\n\n---\nBody here."

	desc, meta, body := parseFrontmatter(content)

	if desc != "" {
		t.Errorf("description: got %q, want empty", desc)
	}
	if len(meta) != 0 {
		t.Errorf("metadata: got %v, want empty", meta)
	}
	if body != "Body here." {
		t.Errorf("body: got %q, want %q", body, "Body here.")
	}
}

func TestParseFrontmatter_AdjacentDelimiters(t *testing.T) {
	// ---\n--- is valid empty frontmatter — same as TestParseFrontmatter_EmptyFrontmatter.
	content := "---\n---\nBody here."

	desc, meta, body := parseFrontmatter(content)

	if desc != "" {
		t.Errorf("description: got %q, want empty", desc)
	}
	if len(meta) != 0 {
		t.Errorf("metadata: got %v, want empty", meta)
	}
	if body != "Body here." {
		t.Errorf("body: got %q, want %q", body, "Body here.")
	}
}

func TestParseFrontmatter_UnclosedFrontmatter(t *testing.T) {
	content := `---
description: test
No closing delimiter`

	desc, _, body := parseFrontmatter(content)

	// With no closing ---, it should return the original content as body
	if desc != "" {
		t.Errorf("description: got %q, want empty", desc)
	}
	if body != content {
		t.Errorf("body: got %q, want original content", body)
	}
}

func TestParseFrontmatter_MultipleMetadataKeys(t *testing.T) {
	content := `---
description: My skill
version: 1.0
category: ops
---
The body.`

	desc, meta, body := parseFrontmatter(content)

	if desc != "My skill" {
		t.Errorf("description: got %q", desc)
	}
	if meta["version"] != "1.0" {
		t.Errorf("version: got %q", meta["version"])
	}
	if meta["category"] != "ops" {
		t.Errorf("category: got %q", meta["category"])
	}
	if body != "The body." {
		t.Errorf("body: got %q", body)
	}
}

func TestDeriveName(t *testing.T) {
	tests := []struct {
		base string
		path string
		want string
	}{
		{"skills", "skills/deploy.md", "deploy"},
		{"skills", "skills/ops/rollback.md", "ops.rollback"},
		{"skills", "skills/a/b/c.md", "a.b.c"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := deriveName(tt.base, tt.path)
			if got != tt.want {
				t.Errorf("deriveName(%q, %q): got %q, want %q", tt.base, tt.path, got, tt.want)
			}
		})
	}
}
